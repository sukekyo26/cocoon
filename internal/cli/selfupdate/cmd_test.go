//nolint:testpackage // white-box tests for unexported downloadFile / readChecksum / sha256File / atomicReplace.
package selfupdatecli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/sukekyo26/cocoon/internal/release"
)

// TestDownloadFile covers the three states of the binary/SHA256SUMS
// fetch: a 2xx body landing on disk verbatim, a non-2xx status mapped to
// release.ErrHTTPStatus, and a transport failure surfaced as an error.
func TestDownloadFile(t *testing.T) {
	t.Parallel()

	t.Run("happy path writes body", func(t *testing.T) {
		t.Parallel()
		const payload = "cocoon-binary-bytes\x00\x01"
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(payload)) //nolint:errcheck // test mock server response
		}))
		defer srv.Close()

		dst := filepath.Join(t.TempDir(), "asset")
		if err := downloadFile(context.Background(), srv.URL, dst); err != nil {
			t.Fatalf("downloadFile: %v", err)
		}
		got, err := os.ReadFile(dst)
		if err != nil {
			t.Fatalf("read dst: %v", err)
		}
		if string(got) != payload {
			t.Errorf("dst content = %q, want %q", got, payload)
		}
	})

	t.Run("non-2xx maps to ErrHTTPStatus", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		err := downloadFile(context.Background(), srv.URL, filepath.Join(t.TempDir(), "asset"))
		if !errors.Is(err, release.ErrHTTPStatus) {
			t.Fatalf("err = %v, want errors.Is release.ErrHTTPStatus", err)
		}
	})

	t.Run("transport failure surfaces an error", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		url := srv.URL
		srv.Close() // close so the connection is refused

		if err := downloadFile(context.Background(), url, filepath.Join(t.TempDir(), "asset")); err == nil {
			t.Fatal("expected an error from a closed server, got nil")
		}
	})
}

// TestReadChecksum exercises the SHA256SUMS line parser: bare and
// `*`-prefixed asset names, malformed lines that must be skipped, a
// missing asset mapped to errAssetMissing, and an unreadable file.
func TestReadChecksum(t *testing.T) {
	t.Parallel()

	const (
		asset = "cocoon-linux-amd64"
		hash  = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	)
	cases := []struct {
		name    string
		content string
		want    string
		wantErr error
	}{
		{
			name:    "bare name",
			content: hash + "  " + asset + "\n",
			want:    hash,
		},
		{
			name:    "binary-mode star prefix",
			content: hash + " *" + asset + "\n",
			want:    hash,
		},
		{
			name:    "malformed lines skipped before the match",
			content: "garbage\n\nsome three field line\n" + hash + "  " + asset + "\n",
			want:    hash,
		},
		{
			name:    "asset absent",
			content: hash + "  other-asset\n",
			wantErr: errAssetMissing,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sums := filepath.Join(t.TempDir(), "SHA256SUMS")
			if err := os.WriteFile(sums, []byte(tc.content), 0o600); err != nil {
				t.Fatalf("write SHA256SUMS: %v", err)
			}
			got, err := readChecksum(sums, asset)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want errors.Is %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("readChecksum: %v", err)
			}
			if got != tc.want {
				t.Errorf("checksum = %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("unreadable file", func(t *testing.T) {
		t.Parallel()
		_, err := readChecksum(filepath.Join(t.TempDir(), "does-not-exist"), asset)
		if err == nil {
			t.Fatal("expected an error for a missing SHA256SUMS file, got nil")
		}
		if errors.Is(err, errAssetMissing) {
			t.Errorf("a read failure must not collapse into errAssetMissing: %v", err)
		}
	})
}

// TestSha256File pins that sha256File hashes file content and reports a
// read failure rather than a zero hash.
func TestSha256File(t *testing.T) {
	t.Parallel()

	t.Run("known content", func(t *testing.T) {
		t.Parallel()
		content := []byte("the quick brown fox\n")
		sum := sha256.Sum256(content)
		want := hex.EncodeToString(sum[:])

		path := filepath.Join(t.TempDir(), "asset")
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatalf("write asset: %v", err)
		}
		got, err := sha256File(path)
		if err != nil {
			t.Fatalf("sha256File: %v", err)
		}
		if got != want {
			t.Errorf("sha256File = %q, want %q", got, want)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		t.Parallel()
		if _, err := sha256File(filepath.Join(t.TempDir(), "nope")); err == nil {
			t.Fatal("expected an error for a missing file, got nil")
		}
	})
}

// TestAtomicReplace covers the same-filesystem rename path: dst ends up
// with src's bytes and mode, and src no longer exists.
func TestAtomicReplace(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "new-binary")
	dst := filepath.Join(dir, "cocoon")
	content := []byte("#!/bin/sh\necho new\n")
	if err := os.WriteFile(src, content, 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	if err := os.WriteFile(dst, []byte("old"), 0o600); err != nil {
		t.Fatalf("write dst: %v", err)
	}

	if err := atomicReplace(src, dst); err != nil {
		t.Fatalf("atomicReplace: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("dst content = %q, want %q", got, content)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("stat dst: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("dst mode = %o, want 600 (atomicReplace must carry src's mode)", perm)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("src still exists after rename (err = %v)", err)
	}
}
