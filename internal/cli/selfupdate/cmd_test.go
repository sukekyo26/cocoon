//nolint:testpackage // white-box tests for unexported downloadFile / readChecksum / sha256File / atomicReplace.
package selfupdatecli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/release"
	"github.com/sukekyo26/cocoon/internal/version"
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

// withSelfUpdateSeams snapshots the package-level seams (fetchLatest,
// executablePath, osExit) and restores them via t.Cleanup. Tests use this
// instead of touching the vars directly so a failing assertion can't leak
// fakes into sibling tests.
//
// All callers must be //nolint:paralleltest because the seams are shared
// package state.
func withSelfUpdateSeams(t *testing.T) {
	t.Helper()
	origFL, origEP, origExit := fetchLatest, executablePath, osExit
	t.Cleanup(func() {
		fetchLatest, executablePath, osExit = origFL, origEP, origExit
	})
}

// withVersion pins version.Version for the duration of the test so the
// "dev build" guard and the up-to-date / newer comparisons are exercised
// against a known string.
func withVersion(t *testing.T, v string) {
	t.Helper()
	orig := version.Version
	t.Cleanup(func() { version.Version = orig })
	version.Version = v
}

// TestRunSelfUpdate_DevBuildErrors covers the "no version baked in" guard:
// version.Version == "dev" must short-circuit to ErrFailure before any
// fetchLatest call is made.
//
//nolint:paralleltest // mutates the package-level version.Version
func TestRunSelfUpdate_DevBuildErrors(t *testing.T) {
	withSelfUpdateSeams(t)
	withVersion(t, "dev")
	fetchLatest = func(context.Context, ...release.Option) (*release.Release, error) {
		t.Error("fetchLatest must not run on a dev build")
		return nil, errors.New("should not be called")
	}

	var stderr bytes.Buffer
	err := runSelfUpdate(context.Background(), &bytes.Buffer{}, &stderr, false, false)
	if !errors.Is(err, clihelpers.ErrFailure) {
		t.Fatalf("err = %v, want errors.Is ErrFailure", err)
	}
	if !strings.Contains(stderr.String(), "dev build") {
		t.Errorf("stderr = %q, want substring %q", stderr.String(), "dev build")
	}
}

// TestRunSelfUpdate_FetchLatestFails pins that a transport-level fetch
// failure is wrapped as ErrFailure (the user-actionable class) without
// dropping the original cause from the chain.
//
//nolint:paralleltest // mutates the package-level fetchLatest seam
func TestRunSelfUpdate_FetchLatestFails(t *testing.T) {
	withSelfUpdateSeams(t)
	withVersion(t, "0.0.1")

	cause := errors.New("github api down")
	fetchLatest = func(context.Context, ...release.Option) (*release.Release, error) {
		return nil, cause
	}

	err := runSelfUpdate(context.Background(), &bytes.Buffer{}, &bytes.Buffer{}, false, false)
	if !errors.Is(err, clihelpers.ErrFailure) {
		t.Fatalf("err = %v, want errors.Is ErrFailure", err)
	}
	if !errors.Is(err, cause) {
		t.Errorf("err = %v lost original cause %v", err, cause)
	}
}

// TestRunSelfUpdate_AlreadyUpToDate covers the no-op path: when fetchLatest
// reports the same tag the binary already carries, runSelfUpdate exits
// with nil and prints "already up to date" without downloading anything.
//
//nolint:paralleltest // mutates the package-level fetchLatest seam
func TestRunSelfUpdate_AlreadyUpToDate(t *testing.T) {
	withSelfUpdateSeams(t)
	withVersion(t, "1.2.3")
	fetchLatest = func(context.Context, ...release.Option) (*release.Release, error) {
		return &release.Release{TagName: "v1.2.3"}, nil
	}

	var stdout bytes.Buffer
	err := runSelfUpdate(context.Background(), &stdout, &bytes.Buffer{}, false, false)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !strings.Contains(stdout.String(), "already up to date") {
		t.Errorf("stdout = %q, want substring %q", stdout.String(), "already up to date")
	}
}

// TestRunSelfUpdate_CheckOnlyExitsWith100 pins the contract that
// `--check-only` invokes osExit(ExitNewerAvailable) when a newer release
// is available, so CI scripts can branch on exit code 100 without
// parsing stdout.
//
//nolint:paralleltest // mutates the package-level osExit + fetchLatest seams
func TestRunSelfUpdate_CheckOnlyExitsWith100(t *testing.T) {
	withSelfUpdateSeams(t)
	withVersion(t, "1.0.0")
	fetchLatest = func(context.Context, ...release.Option) (*release.Release, error) {
		return &release.Release{TagName: "v2.0.0"}, nil
	}
	var exitCode int
	osExit = func(code int) { exitCode = code }

	err := runSelfUpdate(context.Background(), &bytes.Buffer{}, &bytes.Buffer{}, true, false)
	if err != nil {
		t.Fatalf("err = %v, want nil (osExit is fake, runSelfUpdate returns post-osExit)", err)
	}
	if exitCode != ExitNewerAvailable {
		t.Errorf("osExit code = %d, want %d", exitCode, ExitNewerAvailable)
	}
}

// TestRunSelfUpdate_AssetNotFound exercises the "release exists but has
// no matching asset for this OS/arch" branch.
//
//nolint:paralleltest // mutates the package-level fetchLatest seam
func TestRunSelfUpdate_AssetNotFound(t *testing.T) {
	withSelfUpdateSeams(t)
	withVersion(t, "1.0.0")
	fetchLatest = func(context.Context, ...release.Option) (*release.Release, error) {
		return &release.Release{TagName: "v2.0.0", Assets: nil}, nil
	}

	err := runSelfUpdate(context.Background(), &bytes.Buffer{}, &bytes.Buffer{}, false, false)
	if !errors.Is(err, clihelpers.ErrFailure) {
		t.Fatalf("err = %v, want errors.Is ErrFailure", err)
	}
	if !strings.Contains(err.Error(), "release asset") {
		t.Errorf("err = %v, want substring %q", err, "release asset")
	}
}

// TestRunSelfUpdate_SumsNotFound covers the symmetric "asset present but
// SHA256SUMS missing" branch — the second guard right after assetURL.
//
//nolint:paralleltest // mutates the package-level fetchLatest seam
func TestRunSelfUpdate_SumsNotFound(t *testing.T) {
	withSelfUpdateSeams(t)
	withVersion(t, "1.0.0")
	assetName := fmt.Sprintf("cocoon-%s-%s", runtime.GOOS, runtime.GOARCH)
	fetchLatest = func(context.Context, ...release.Option) (*release.Release, error) {
		return &release.Release{
			TagName: "v2.0.0",
			Assets:  []release.Asset{{Name: assetName, URL: "http://127.0.0.1:0/asset"}},
		}, nil
	}

	err := runSelfUpdate(context.Background(), &bytes.Buffer{}, &bytes.Buffer{}, false, false)
	if !errors.Is(err, clihelpers.ErrFailure) {
		t.Fatalf("err = %v, want errors.Is ErrFailure", err)
	}
	if !strings.Contains(err.Error(), "SHA256SUMS") {
		t.Errorf("err = %v, want substring %q", err, "SHA256SUMS")
	}
}

// TestRunSelfUpdate_HappyPath walks the full download → verify → replace
// pipeline. It stands up an httptest server that serves a fake binary
// and a matching SHA256SUMS, points the seams at it, and asserts that
// the executablePath target ends up with the new binary's bytes.
//
//nolint:paralleltest // mutates the package-level fetchLatest / executablePath seams
func TestRunSelfUpdate_HappyPath(t *testing.T) {
	withSelfUpdateSeams(t)
	withVersion(t, "1.0.0")

	const payload = "#!/bin/sh\necho new-cocoon\n"
	sum := sha256.Sum256([]byte(payload))
	hashHex := hex.EncodeToString(sum[:])
	assetName := fmt.Sprintf("cocoon-%s-%s", runtime.GOOS, runtime.GOARCH)
	sumsBody := hashHex + "  " + assetName + "\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/asset":
			_, _ = w.Write([]byte(payload)) //nolint:errcheck // test mock
		case "/sums":
			_, _ = w.Write([]byte(sumsBody)) //nolint:errcheck // test mock
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	fetchLatest = func(context.Context, ...release.Option) (*release.Release, error) {
		return &release.Release{
			TagName: "v2.0.0",
			Assets: []release.Asset{
				{Name: assetName, URL: srv.URL + "/asset"},
				{Name: "SHA256SUMS", URL: srv.URL + "/sums"},
			},
		}, nil
	}

	target := filepath.Join(t.TempDir(), "cocoon")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil { //nolint:gosec // test fixture
		t.Fatalf("seed target: %v", err)
	}
	executablePath = func() (string, error) { return target, nil }

	var stdout bytes.Buffer
	err := runSelfUpdate(context.Background(), &stdout, &bytes.Buffer{}, false, false)
	if err != nil {
		t.Fatalf("runSelfUpdate: %v", err)
	}
	got, err := os.ReadFile(target) //nolint:gosec // test asserts the file we just wrote
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(got) != payload {
		t.Errorf("target content = %q, want %q", got, payload)
	}
	if !strings.Contains(stdout.String(), "updated cocoon to 2.0.0") {
		t.Errorf("stdout = %q, want substring %q", stdout.String(), "updated cocoon to 2.0.0")
	}
}

// TestRunSelfUpdate_ChecksumMismatch points fetchLatest at a server that
// serves a SHA256SUMS file whose digest disagrees with the asset bytes.
// runSelfUpdate must surface a "checksum mismatch" error and not touch
// the existing executable.
//
//nolint:paralleltest // mutates the package-level fetchLatest / executablePath seams
func TestRunSelfUpdate_ChecksumMismatch(t *testing.T) {
	withSelfUpdateSeams(t)
	withVersion(t, "1.0.0")

	const payload = "real-bytes"
	assetName := fmt.Sprintf("cocoon-%s-%s", runtime.GOOS, runtime.GOARCH)
	const wrongHash = "0000000000000000000000000000000000000000000000000000000000000000"
	sumsBody := wrongHash + "  " + assetName + "\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/asset":
			_, _ = w.Write([]byte(payload)) //nolint:errcheck // test mock
		case "/sums":
			_, _ = w.Write([]byte(sumsBody)) //nolint:errcheck // test mock
		}
	}))
	t.Cleanup(srv.Close)

	fetchLatest = func(context.Context, ...release.Option) (*release.Release, error) {
		return &release.Release{
			TagName: "v2.0.0",
			Assets: []release.Asset{
				{Name: assetName, URL: srv.URL + "/asset"},
				{Name: "SHA256SUMS", URL: srv.URL + "/sums"},
			},
		}, nil
	}
	target := filepath.Join(t.TempDir(), "cocoon")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil { //nolint:gosec // test fixture
		t.Fatalf("seed target: %v", err)
	}
	executablePath = func() (string, error) { return target, nil }

	err := runSelfUpdate(context.Background(), &bytes.Buffer{}, &bytes.Buffer{}, false, false)
	if !errors.Is(err, clihelpers.ErrFailure) {
		t.Fatalf("err = %v, want errors.Is ErrFailure", err)
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("err = %v, want substring %q", err, "checksum mismatch")
	}
	got, err := os.ReadFile(target) //nolint:gosec // assertion on a path we wrote above
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(got) != "old" {
		t.Errorf("target was overwritten despite checksum mismatch: %q", got)
	}
}

// TestNewCommand_FlagsWired pins the cobra wiring (flag registration,
// SilenceUsage / SilenceErrors / NoArgs) so a future refactor cannot
// silently drop the --check-only contract or start dumping usage on
// every error.
func TestNewCommand_FlagsWired(t *testing.T) {
	t.Parallel()

	cmd := NewCommand(&bytes.Buffer{}, &bytes.Buffer{})

	if cmd.Use != "self-update" {
		t.Errorf("Use = %q, want %q", cmd.Use, "self-update")
	}
	if !cmd.SilenceUsage {
		t.Error("SilenceUsage must be true; usage is dumped from main.go on ErrUsage")
	}
	if !cmd.SilenceErrors {
		t.Error("SilenceErrors must be true; main.go owns user-facing error output")
	}
	for _, name := range []string{"check-only", "force"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s missing", name)
		}
	}
}

// errSimulatedEXDEV stands in for the cross-filesystem rename failure
// (syscall.EXDEV) that atomicReplace's copy fallback exists to handle.
var errSimulatedEXDEV = errors.New("simulated cross-device rename")

// TestAtomicReplace_EXDEVFallback exercises the copy fallback: the
// renameFn seam fails the cross-directory fast path (as os.Rename would
// across filesystems — runSelfUpdate downloads into the system temp dir
// while the installed binary may live elsewhere), so atomicReplace must
// copy src onto dst and leave no temp file behind. The same-directory
// tmp -> dst rename inside the fallback is allowed through to os.Rename.
//
//nolint:paralleltest // mutates the package-level renameFn seam
func TestAtomicReplace_EXDEVFallback(t *testing.T) {
	orig := renameFn
	t.Cleanup(func() { renameFn = orig })
	renameFn = func(oldpath, newpath string) error {
		if filepath.Dir(oldpath) != filepath.Dir(newpath) {
			return errSimulatedEXDEV
		}
		return os.Rename(oldpath, newpath)
	}

	dstDir := t.TempDir()
	src := filepath.Join(t.TempDir(), "new-binary") // separate dir => fast path "fails"
	dst := filepath.Join(dstDir, "cocoon")
	content := []byte("#!/bin/sh\necho fallback\n")
	if err := os.WriteFile(src, content, 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	if err := os.WriteFile(dst, []byte("old"), 0o600); err != nil {
		t.Fatalf("write dst: %v", err)
	}

	if err := atomicReplace(src, dst); err != nil {
		t.Fatalf("atomicReplace via fallback: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("dst content = %q, want %q", got, content)
	}
	if _, err := os.Stat(dst + ".cocoon-update.tmp"); !os.IsNotExist(err) {
		t.Errorf("fallback left its temp file behind (err = %v)", err)
	}
}

// forceEXDEV makes renameFn always fail so atomicReplace takes the
// copy fallback. The cleanup restores the original seam.
//
//nolint:thelper // inline seam setup, not a generic helper
func forceEXDEV(t *testing.T) {
	orig := renameFn
	t.Cleanup(func() { renameFn = orig })
	renameFn = func(_, _ string) error { return errSimulatedEXDEV }
}

// TestAtomicReplace_OpenSrcError pins the fallback's "open src" branch:
// the fast-path rename is forced to fail, then the source does not exist.
//
//nolint:paralleltest // mutates the package-level renameFn seam
func TestAtomicReplace_OpenSrcError(t *testing.T) {
	forceEXDEV(t)
	dst := filepath.Join(t.TempDir(), "cocoon")
	err := atomicReplace(filepath.Join(t.TempDir(), "does-not-exist"), dst)
	if err == nil || !strings.Contains(err.Error(), "open src") {
		t.Fatalf("err = %v, want an \"open src\" error", err)
	}
}

// TestAtomicReplace_CreateTmpError pins the "create" branch: the dst's
// parent directory does not exist, so the sibling temp file cannot be
// created.
//
//nolint:paralleltest // mutates the package-level renameFn seam
func TestAtomicReplace_CreateTmpError(t *testing.T) {
	forceEXDEV(t)
	src := filepath.Join(t.TempDir(), "src")
	if err := os.WriteFile(src, []byte("x"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	dst := filepath.Join(t.TempDir(), "no-such-subdir", "cocoon")
	err := atomicReplace(src, dst)
	if err == nil || !strings.Contains(err.Error(), "create") {
		t.Fatalf("err = %v, want a \"create\" error", err)
	}
}

// TestAtomicReplace_CopyError pins the "copy" branch: a directory opens
// fine but cannot be read by io.Copy, and the partial temp file is
// cleaned up.
//
//nolint:paralleltest // mutates the package-level renameFn seam
func TestAtomicReplace_CopyError(t *testing.T) {
	forceEXDEV(t)
	srcDir := t.TempDir() // a directory passed as the source file
	dst := filepath.Join(t.TempDir(), "cocoon")
	err := atomicReplace(srcDir, dst)
	if err == nil || !strings.Contains(err.Error(), "copy") {
		t.Fatalf("err = %v, want a \"copy\" error", err)
	}
	if _, statErr := os.Stat(dst + ".cocoon-update.tmp"); !os.IsNotExist(statErr) {
		t.Errorf("temp file left behind after copy error (stat = %v)", statErr)
	}
}

// TestAtomicReplace_FinalRenameError pins the "rename ->" branch: the
// copy succeeds but the final tmp -> dst rename fails, and the temp file
// is cleaned up.
//
//nolint:paralleltest // mutates the package-level renameFn seam
func TestAtomicReplace_FinalRenameError(t *testing.T) {
	forceEXDEV(t)
	src := filepath.Join(t.TempDir(), "src")
	if err := os.WriteFile(src, []byte("payload"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	dst := filepath.Join(t.TempDir(), "cocoon")
	err := atomicReplace(src, dst)
	if err == nil || !strings.Contains(err.Error(), "rename") {
		t.Fatalf("err = %v, want a \"rename\" error", err)
	}
	if _, statErr := os.Stat(dst + ".cocoon-update.tmp"); !os.IsNotExist(statErr) {
		t.Errorf("temp file left behind after final-rename error (stat = %v)", statErr)
	}
}

// TestCheckInstallDirWritable_OK pins the happy path: when the parent
// dir is writable, the preflight returns nil and leaves no artefacts
// (the temp probe file must be removed even on success).
func TestCheckInstallDirWritable_OK(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	self := filepath.Join(dir, "cocoon")
	if err := checkInstallDirWritable(self); err != nil {
		t.Fatalf("checkInstallDirWritable: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".cocoon-update-preflight-") {
			t.Errorf("preflight left a probe file behind: %s", e.Name())
		}
	}
}

// TestCheckInstallDirWritable_ReadOnlyReturnsSentinel covers the
// permission-denied branch: when the parent dir refuses CreateTemp,
// the helper must return ErrInstallDirReadOnly (so callers can attach
// the actionable hint via errors.Is) and the message must name the
// offending directory.
func TestCheckInstallDirWritable_ReadOnlyReturnsSentinel(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root bypasses DAC; preflight can't be triggered without dropping privileges")
	}

	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod dir read-only: %v", err)
	}
	// Restore writable mode so t.TempDir() cleanup can rm -rf the tree.
	t.Cleanup(func() {
		if err := os.Chmod(dir, 0o755); err != nil {
			t.Logf("chmod cleanup failed (t.TempDir rm may struggle): %v", err)
		}
	})

	self := filepath.Join(dir, "cocoon")
	err := checkInstallDirWritable(self)
	if !errors.Is(err, ErrInstallDirReadOnly) {
		t.Fatalf("err = %v, want errors.Is ErrInstallDirReadOnly", err)
	}
	if !strings.Contains(err.Error(), dir) {
		t.Errorf("err = %v, want substring %q (offending dir)", err, dir)
	}
}

// TestRunSelfUpdate_CheckOnlySkipsPreflight pins the contract that
// `--check-only` is a read-only operation: the install-dir writability
// preflight must be skipped so users whose binary lives in a root-owned
// dir (e.g. /usr/local/bin) can still query the latest release without
// sudo. The test stages a read-only directory, points executablePath at
// it, and asserts that runSelfUpdate reaches fetchLatest and exits with
// the "already up to date" no-op rather than ErrInstallDirReadOnly.
//
//nolint:paralleltest // mutates the package-level fetchLatest / executablePath seams
func TestRunSelfUpdate_CheckOnlySkipsPreflight(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses DAC; preflight can't be triggered without dropping privileges")
	}
	withSelfUpdateSeams(t)
	withVersion(t, "1.0.0")

	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod dir read-only: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dir, 0o755); err != nil {
			t.Logf("chmod cleanup failed (t.TempDir rm may struggle): %v", err)
		}
	})
	target := filepath.Join(dir, "cocoon")
	executablePath = func() (string, error) {
		t.Error("executablePath must not be called when --check-only skips the preflight")
		return target, nil
	}
	fetchLatest = func(context.Context, ...release.Option) (*release.Release, error) {
		return &release.Release{TagName: "v1.0.0"}, nil
	}

	var stdout bytes.Buffer
	err := runSelfUpdate(context.Background(), &stdout, &bytes.Buffer{}, true, false)
	if err != nil {
		t.Fatalf("err = %v, want nil (--check-only on a read-only install dir must not fail)", err)
	}
	if !strings.Contains(stdout.String(), "already up to date") {
		t.Errorf("stdout = %q, want substring %q", stdout.String(), "already up to date")
	}
}

// TestRunSelfUpdate_ReadOnlyInstallDirShortCircuits is the fail-fast
// contract: when the install dir is read-only, runSelfUpdate must
// surface ErrInstallDirReadOnly + the remediation hint *before* any
// network I/O. The fetchLatest seam is wired to fail the test if
// invoked — that guarantees no 12MB download happens after a guaranteed
// permission failure.
//
//nolint:paralleltest // mutates the package-level fetchLatest / executablePath seams
func TestRunSelfUpdate_ReadOnlyInstallDirShortCircuits(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses DAC; preflight can't be triggered without dropping privileges")
	}
	withSelfUpdateSeams(t)
	withVersion(t, "1.0.0")

	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod dir read-only: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dir, 0o755); err != nil {
			t.Logf("chmod cleanup failed (t.TempDir rm may struggle): %v", err)
		}
	})
	target := filepath.Join(dir, "cocoon")
	executablePath = func() (string, error) { return target, nil }
	fetchLatest = func(context.Context, ...release.Option) (*release.Release, error) {
		t.Error("fetchLatest must not run when the install dir preflight fails")
		return nil, errors.New("fetchLatest should not be reached")
	}

	err := runSelfUpdate(context.Background(), &bytes.Buffer{}, &bytes.Buffer{}, false, false)
	if !errors.Is(err, clihelpers.ErrFailure) {
		t.Fatalf("err = %v, want errors.Is ErrFailure", err)
	}
	if !errors.Is(err, ErrInstallDirReadOnly) {
		t.Fatalf("err = %v, want errors.Is ErrInstallDirReadOnly", err)
	}
	for _, want := range []string{"sudo", target} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %v, want substring %q", err, want)
		}
	}
}
