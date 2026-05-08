//nolint:testpackage // exercises unexported docker GID helpers.
package setup

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/logx"
)

func testLogger(_ *testing.T) *logx.Logger {
	return logx.New(io.Discard, io.Discard)
}

func TestDockerGroupGIDFromFile(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		body    string
		want    int
		wantErr bool
	}{
		{
			name: "single_docker_line",
			body: "root:x:0:\n" +
				"docker:x:998:user1,user2\n" +
				"users:x:100:\n",
			want: 998,
		},
		{
			name: "no_docker_group",
			body: "root:x:0:\n" +
				"users:x:100:\n",
			wantErr: true,
		},
		{
			name: "malformed_gid_skipped",
			body: "docker:x:notanumber:user1\n" +
				"docker:x:42:user2\n",
			want: 42,
		},
		{
			name:    "empty_file",
			body:    "",
			wantErr: true,
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "group")
			if err := os.WriteFile(path, []byte(c.body), 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := dockerGroupGIDFromFile(path)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got gid=%d", got)
				}
				if !errors.Is(err, ErrDockerGID) {
					t.Errorf("error chain missing ErrDockerGID: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("gid = %d, want %d", got, c.want)
			}
		})
	}
}

func TestDockerGroupGIDFromFile_ScanError(t *testing.T) {
	t.Parallel()
	// A single line larger than bufio.Scanner's default max token (64 KiB)
	// surfaces bufio.ErrTooLong via sc.Err(); the helper must not silently
	// fall through to the misleading "not found" message.
	path := filepath.Join(t.TempDir(), "group")
	huge := make([]byte, 128*1024)
	for i := range huge {
		huge[i] = 'a'
	}
	if err := os.WriteFile(path, huge, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := dockerGroupGIDFromFile(path)
	if err == nil {
		t.Fatal("expected scan error, got nil")
	}
	if !errors.Is(err, ErrDockerGID) {
		t.Errorf("error chain missing ErrDockerGID: %v", err)
	}
	if !strings.Contains(err.Error(), "scan") {
		t.Errorf("error should mention scan failure: %v", err)
	}
}

func TestDockerGroupGIDFromFile_MissingFile(t *testing.T) {
	t.Parallel()
	_, err := dockerGroupGIDFromFile(filepath.Join(t.TempDir(), "nope"))
	if err == nil {
		t.Fatal("expected error for missing path")
	}
	if !errors.Is(err, ErrDockerGID) {
		t.Errorf("error chain missing ErrDockerGID: %v", err)
	}
}

func TestSocketFileGID_MissingPath(t *testing.T) {
	t.Parallel()
	_, err := socketFileGID(filepath.Join(t.TempDir(), "no-such"))
	if err == nil {
		t.Fatal("expected error for missing path")
	}
	if !errors.Is(err, ErrDockerGID) {
		t.Errorf("error chain missing ErrDockerGID: %v", err)
	}
}

func TestDetectDockerGID_Smoke(t *testing.T) {
	t.Parallel()
	// Either the host has a docker socket (first branch) or /etc/group on
	// most Linux distros lists `docker` (fallback). On a CI runner without
	// either, the call returns ErrDockerGID, which we accept as the
	// well-formed failure path.
	gid, err := detectDockerGID()
	if err != nil {
		if !errors.Is(err, ErrDockerGID) {
			t.Errorf("error chain missing ErrDockerGID: %v", err)
		}
		return
	}
	if gid < 0 {
		t.Errorf("gid = %d, want >=0", gid)
	}
	// Detect is a thin wrapper; cover the method receiver too.
	if _, err := (defaultGIDDetector{}).Detect(); err != nil && !errors.Is(err, ErrDockerGID) {
		t.Errorf("Detect: %v", err)
	}
}

func TestCopyShellrcCustomIfMissing(t *testing.T) {
	t.Parallel()
	t.Run("dst_exists_no_op", func(t *testing.T) {
		t.Parallel()
		work := t.TempDir()
		cfg := filepath.Join(work, "config")
		if err := os.MkdirAll(cfg, 0o755); err != nil {
			t.Fatal(err)
		}
		dst := filepath.Join(cfg, "shell_custom")
		original := []byte("# already here\n")
		if err := os.WriteFile(dst, original, 0o600); err != nil {
			t.Fatal(err)
		}
		// Even with src present it must not overwrite.
		if err := os.WriteFile(filepath.Join(cfg, "shell_custom.example"), []byte("# new\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		err := copyShellrcCustomIfMissing(work, testLogger(t), noopCat{})
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		got, rerr := os.ReadFile(dst)
		if rerr != nil {
			t.Fatal(rerr)
		}
		if string(got) != string(original) {
			t.Errorf("dst was overwritten: got %q want %q", got, original)
		}
	})
	t.Run("src_missing_no_op", func(t *testing.T) {
		t.Parallel()
		work := t.TempDir()
		// Neither src nor dst exists; function returns nil silently.
		if err := copyShellrcCustomIfMissing(work, testLogger(t), noopCat{}); err != nil {
			t.Errorf("err = %v", err)
		}
		if _, err := os.Stat(filepath.Join(work, "config", "shell_custom")); !os.IsNotExist(err) {
			t.Errorf("dst should not exist, stat: %v", err)
		}
	})
	t.Run("seeds_from_example", func(t *testing.T) {
		t.Parallel()
		work := t.TempDir()
		cfg := filepath.Join(work, "config")
		if err := os.MkdirAll(cfg, 0o755); err != nil {
			t.Fatal(err)
		}
		example := []byte("# starter rc\n")
		if err := os.WriteFile(filepath.Join(cfg, "shell_custom.example"), example, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := copyShellrcCustomIfMissing(work, testLogger(t), noopCat{}); err != nil {
			t.Fatalf("err = %v", err)
		}
		got, err := os.ReadFile(filepath.Join(cfg, "shell_custom"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(example) {
			t.Errorf("dst content = %q, want %q", got, example)
		}
	})
}

type noopCat struct{}

func (noopCat) Msg(key string, _ ...any) string { return key }

func TestSocketFileGID_RegularFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	gid, err := socketFileGID(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gid < 0 {
		t.Errorf("gid = %d, want >=0", gid)
	}
}
