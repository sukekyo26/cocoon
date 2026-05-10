// Package installer_test holds runtime tests for the public install.sh
// script (`curl ... | sh` entry point at the repo root). Each test spins up
// an httptest.Server that mocks the relevant subset of the GitHub API and
// release-download endpoints, runs `sh install.sh` against the mock via
// COCOON_API_BASE / COCOON_RELEASE_BASE overrides, and asserts on the
// installer's exit code, stderr text, and the resulting on-disk file.
package installer_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

const (
	mockRepo = "sukekyo26/cocoon"
	mockTag  = "v0.2.0"
)

// repoRoot resolves the repository root from this test file's location.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

func installShPath(t *testing.T) string {
	t.Helper()
	p := filepath.Join(repoRoot(t), "install.sh")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("install.sh not found at %s: %v", p, err)
	}
	return p
}

// hostOSArch mirrors the OS/arch detection install.sh performs at runtime.
// Tests must use the actual runner's OS/arch in the mock asset filename
// since install.sh derives the asset from the live `uname` calls.
func hostOSArch(t *testing.T) (string, string) {
	t.Helper()
	switch runtime.GOOS {
	case "linux", "darwin":
	default:
		t.Skipf("install.sh only supports linux/darwin; runner is %s", runtime.GOOS)
	}
	var arch string
	switch runtime.GOARCH {
	case "amd64":
		arch = "amd64"
	case "arm64":
		arch = "arm64"
	default:
		t.Skipf("install.sh only supports amd64/arm64; runner is %s", runtime.GOARCH)
	}
	return runtime.GOOS, arch
}

func mockAssetName(t *testing.T) string {
	t.Helper()
	os, arch := hostOSArch(t)
	return fmt.Sprintf("cocoon-%s-%s", os, arch)
}

// writeMock copies body to a mock-server response. Write errors are
// intentionally dropped: clients are local httptest connections, and any
// I/O failure surfaces via the install.sh exit code which IS the assertion.
func writeMock(w http.ResponseWriter, body []byte) {
	_, _ = w.Write(body) //nolint:errcheck // mock server; assertions flow through install.sh exit code
}

// mockRelease describes the canned responses a single test wants from the
// fake GitHub server. Per-test customization is opt-in: zero values yield a
// happy-path server.
type mockRelease struct {
	tag     string // download tag (e.g. "v0.2.0"); defaults to mockTag.
	asset   string // asset filename; defaults to host's cocoon-<os>-<arch>.
	binary  []byte // bytes returned for the asset URL.
	apiBody string // JSON body for /repos/.../releases/latest; "" → default.
	apiCode int    // status code for the API endpoint; 0 → 200.
	sumsRaw string // raw SHA256SUMS body; "" → derived from binary+asset.
}

type mockServer struct {
	srv      *httptest.Server
	apiHits  *int32
	dlHits   *int32
	sumsHits *int32

	authMu      *sync.Mutex
	lastAPIAuth *string // captured Authorization header from the API endpoint
}

// LastAPIAuth returns the most recent Authorization header observed by the
// /releases/latest endpoint (empty if no header was sent or no request).
func (m *mockServer) LastAPIAuth() string {
	m.authMu.Lock()
	defer m.authMu.Unlock()
	return *m.lastAPIAuth
}

func newMockServer(t *testing.T, m mockRelease) *mockServer {
	t.Helper()
	if m.tag == "" {
		m.tag = mockTag
	}
	if m.asset == "" {
		m.asset = mockAssetName(t)
	}
	if m.binary == nil {
		m.binary = []byte("fake cocoon binary payload\n")
	}
	if m.apiBody == "" {
		m.apiBody = fmt.Sprintf(`{"tag_name":"%s","name":"Release %s"}`, m.tag, m.tag)
	}
	if m.apiCode == 0 {
		m.apiCode = http.StatusOK
	}
	if m.sumsRaw == "" {
		sum := sha256.Sum256(m.binary)
		m.sumsRaw = fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), m.asset)
	}

	var apiHits, dlHits, sumsHits int32
	var lastAPIAuth string
	var authMu sync.Mutex
	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/repos/%s/releases/latest", mockRepo),
		func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&apiHits, 1)
			authMu.Lock()
			lastAPIAuth = r.Header.Get("Authorization")
			authMu.Unlock()
			w.WriteHeader(m.apiCode)
			writeMock(w, []byte(m.apiBody))
		})
	mux.HandleFunc(fmt.Sprintf("/%s/releases/download/%s/%s", mockRepo, m.tag, m.asset),
		func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&dlHits, 1)
			writeMock(w, m.binary)
		})
	mux.HandleFunc(fmt.Sprintf("/%s/releases/download/%s/SHA256SUMS", mockRepo, m.tag),
		func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&sumsHits, 1)
			writeMock(w, []byte(m.sumsRaw))
		})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &mockServer{
		srv:         srv,
		apiHits:     &apiHits,
		dlHits:      &dlHits,
		sumsHits:    &sumsHits,
		authMu:      &authMu,
		lastAPIAuth: &lastAPIAuth,
	}
}

// runInstallSh executes `sh install.sh` with the supplied env overrides
// merged on top of the current environment. exit==0 means success.
func runInstallSh(t *testing.T, env map[string]string) (stderr string, exit int) {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not on PATH")
	}
	cmd := exec.CommandContext(t.Context(), "sh", installShPath(t)) //nolint:gosec // installShPath is repo-internal, not user input
	cmd.Env = mergedEnv(env)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	if err == nil {
		return errb.String(), 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return errb.String(), exitErr.ExitCode()
	}
	t.Fatalf("run sh install.sh: %v", err)
	return "", -1
}

// mergedEnv returns a child-process env where keys in `overrides` deterministically
// win over any matching entry inherited from the parent. Just appending after
// os.Environ() leaves duplicates; libc getenv() typically returns the FIRST
// match, so the parent's value would silently shadow the test's override
// whenever the CI runner / dev shell happens to have COCOON_* already set.
func mergedEnv(overrides map[string]string) []string {
	merged := make(map[string]string, len(os.Environ())+len(overrides))
	for _, kv := range os.Environ() {
		if k, v, ok := strings.Cut(kv, "="); ok {
			merged[k] = v
		}
	}
	for k, v := range overrides {
		merged[k] = v
	}
	out := make([]string, 0, len(merged))
	for k, v := range merged {
		out = append(out, k+"="+v)
	}
	return out
}

// baseURL returns the underlying httptest.Server URL. httptest never
// appends a trailing slash, so install.sh can concatenate "/repos/..."
// directly. Trailing-slash and path-prefix variants are exercised by
// dedicated test cases below.
func (m *mockServer) baseURL() string { return m.srv.URL }

// ---------------------------------------------------------------------------
// Test cases (5-axis coverage per .claude/rules/testing.md)
// ---------------------------------------------------------------------------

func TestInstallSh_LatestHappyPath(t *testing.T) {
	t.Parallel()
	srv := newMockServer(t, mockRelease{})
	dst := t.TempDir()

	stderr, exit := runInstallSh(t, map[string]string{
		"COCOON_REPO":         mockRepo,
		"COCOON_API_BASE":     srv.baseURL(),
		"COCOON_RELEASE_BASE": srv.baseURL(),
		"COCOON_INSTALL_DIR":  dst,
	})
	if exit != 0 {
		t.Fatalf("exit=%d, stderr=%q", exit, stderr)
	}
	binPath := filepath.Join(dst, "cocoon")
	got, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatalf("read installed binary: %v", err)
	}
	if want := []byte("fake cocoon binary payload\n"); !bytes.Equal(got, want) {
		t.Errorf("binary content mismatch: got %q, want %q", got, want)
	}
	info, err := os.Stat(binPath)
	if err != nil {
		t.Fatalf("stat installed binary: %v", err)
	}
	if perm := info.Mode().Perm(); perm&0o111 == 0 {
		t.Errorf("binary not executable: mode=%v", perm)
	}
	apiHits := atomic.LoadInt32(srv.apiHits)
	dlHits := atomic.LoadInt32(srv.dlHits)
	sumsHits := atomic.LoadInt32(srv.sumsHits)
	if apiHits != 1 {
		t.Errorf("api endpoint hit count: got %d, want 1", apiHits)
	}
	if dlHits != 1 || sumsHits != 1 {
		t.Errorf("download/sums hits: dl=%d sums=%d (want 1/1)", dlHits, sumsHits)
	}
}

func TestInstallSh_PinnedVersion(t *testing.T) {
	t.Parallel()
	srv := newMockServer(t, mockRelease{tag: mockTag})
	dst := t.TempDir()

	stderr, exit := runInstallSh(t, map[string]string{
		"COCOON_REPO":         mockRepo,
		"COCOON_VERSION":      "0.2.0", // no leading v
		"COCOON_API_BASE":     srv.baseURL(),
		"COCOON_RELEASE_BASE": srv.baseURL(),
		"COCOON_INSTALL_DIR":  dst,
	})
	if exit != 0 {
		t.Fatalf("exit=%d, stderr=%q", exit, stderr)
	}
	if hits := atomic.LoadInt32(srv.apiHits); hits != 0 {
		t.Errorf("api endpoint should NOT be hit when COCOON_VERSION is pinned, got %d hits", hits)
	}
	if _, err := os.Stat(filepath.Join(dst, "cocoon")); err != nil {
		t.Fatalf("binary not installed: %v", err)
	}
}

func TestInstallSh_PinnedVersion_VPrefix(t *testing.T) {
	t.Parallel()
	srv := newMockServer(t, mockRelease{tag: mockTag})
	dst := t.TempDir()

	stderr, exit := runInstallSh(t, map[string]string{
		"COCOON_REPO":         mockRepo,
		"COCOON_VERSION":      "v0.2.0", // already has v prefix; must not double up
		"COCOON_API_BASE":     srv.baseURL(),
		"COCOON_RELEASE_BASE": srv.baseURL(),
		"COCOON_INSTALL_DIR":  dst,
	})
	if exit != 0 {
		t.Fatalf("exit=%d, stderr=%q", exit, stderr)
	}
	if hits := atomic.LoadInt32(srv.dlHits); hits != 1 {
		t.Errorf("expected download path /v0.2.0/ to be hit exactly once, got %d", hits)
	}
}

func TestInstallSh_DefaultInstallDir(t *testing.T) {
	// Cannot use t.Parallel here: t.Setenv("HOME", ...) forbids parallel.
	srv := newMockServer(t, mockRelease{})
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	stderr, exit := runInstallSh(t, map[string]string{
		"COCOON_REPO":         mockRepo,
		"COCOON_API_BASE":     srv.baseURL(),
		"COCOON_RELEASE_BASE": srv.baseURL(),
		// COCOON_INSTALL_DIR deliberately unset → defaults to $HOME/.local/bin
	})
	if exit != 0 {
		t.Fatalf("exit=%d, stderr=%q", exit, stderr)
	}
	wantPath := filepath.Join(tmpHome, ".local", "bin", "cocoon")
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("default install path %s missing: %v", wantPath, err)
	}
}

func TestInstallSh_APIError404(t *testing.T) {
	t.Parallel()
	srv := newMockServer(t, mockRelease{apiCode: http.StatusNotFound, apiBody: `{"message":"Not Found"}`})
	dst := t.TempDir()

	stderr, exit := runInstallSh(t, map[string]string{
		"COCOON_REPO":         mockRepo,
		"COCOON_API_BASE":     srv.baseURL(),
		"COCOON_RELEASE_BASE": srv.baseURL(),
		"COCOON_INSTALL_DIR":  dst,
	})
	if exit == 0 {
		t.Fatalf("expected non-zero exit, got 0; stderr=%q", stderr)
	}
	if !strings.Contains(stderr, "failed to fetch release metadata") {
		t.Errorf("stderr missing fail-fast message: %q", stderr)
	}
	expectedURL := fmt.Sprintf("%s/repos/%s/releases/latest", srv.baseURL(), mockRepo)
	if !strings.Contains(stderr, expectedURL) {
		t.Errorf("stderr missing failed URL %q: %q", expectedURL, stderr)
	}
	if _, err := os.Stat(filepath.Join(dst, "cocoon")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("cocoon should not have been installed on API failure: stat err=%v", err)
	}
}

func TestInstallSh_TagParseFailure(t *testing.T) {
	t.Parallel()
	srv := newMockServer(t, mockRelease{apiBody: `{"message":"no tag here"}`})
	dst := t.TempDir()

	stderr, exit := runInstallSh(t, map[string]string{
		"COCOON_REPO":         mockRepo,
		"COCOON_API_BASE":     srv.baseURL(),
		"COCOON_RELEASE_BASE": srv.baseURL(),
		"COCOON_INSTALL_DIR":  dst,
	})
	if exit == 0 {
		t.Fatalf("expected non-zero exit, got 0; stderr=%q", stderr)
	}
	if !strings.Contains(stderr, "could not parse tag_name") {
		t.Errorf("stderr missing tag-parse failure message: %q", stderr)
	}
}

func TestInstallSh_AssetMissingFromSums(t *testing.T) {
	t.Parallel()
	asset := mockAssetName(t)
	binary := []byte("fake cocoon binary payload\n")
	// SHA256SUMS lists a different asset only.
	sumsRaw := "0000000000000000000000000000000000000000000000000000000000000000  cocoon-some-other-target\n"
	srv := newMockServer(t, mockRelease{asset: asset, binary: binary, sumsRaw: sumsRaw})
	dst := t.TempDir()

	stderr, exit := runInstallSh(t, map[string]string{
		"COCOON_REPO":         mockRepo,
		"COCOON_API_BASE":     srv.baseURL(),
		"COCOON_RELEASE_BASE": srv.baseURL(),
		"COCOON_INSTALL_DIR":  dst,
	})
	if exit == 0 {
		t.Fatalf("expected non-zero exit, got 0; stderr=%q", stderr)
	}
	if !strings.Contains(stderr, "not listed in SHA256SUMS") {
		t.Errorf("stderr missing missing-asset message: %q", stderr)
	}
}

func TestInstallSh_APITokenForwarded(t *testing.T) {
	t.Parallel()
	srv := newMockServer(t, mockRelease{})
	dst := t.TempDir()
	const token = "ghs_dummy_test_token" //nolint:gosec // mock token for test assertion only

	stderr, exit := runInstallSh(t, map[string]string{
		"COCOON_REPO":         mockRepo,
		"COCOON_API_BASE":     srv.baseURL(),
		"COCOON_RELEASE_BASE": srv.baseURL(),
		"COCOON_API_TOKEN":    token,
		"COCOON_INSTALL_DIR":  dst,
	})
	if exit != 0 {
		t.Fatalf("exit=%d, stderr=%q", exit, stderr)
	}
	if got := srv.LastAPIAuth(); got != "Bearer "+token {
		t.Errorf("Authorization header forwarded = %q, want %q", got, "Bearer "+token)
	}
}

// TestInstallSh_NoTokenNoAuthHeader pins the contract that the
// non-COCOON_API_TOKEN code path must not attach an Authorization header
// (avoids accidentally leaking shell environment auth to api.github.com).
func TestInstallSh_NoTokenNoAuthHeader(t *testing.T) {
	t.Parallel()
	srv := newMockServer(t, mockRelease{})
	dst := t.TempDir()

	stderr, exit := runInstallSh(t, map[string]string{
		"COCOON_REPO":         mockRepo,
		"COCOON_API_BASE":     srv.baseURL(),
		"COCOON_RELEASE_BASE": srv.baseURL(),
		"COCOON_INSTALL_DIR":  dst,
		// COCOON_API_TOKEN deliberately unset
	})
	if exit != 0 {
		t.Fatalf("exit=%d, stderr=%q", exit, stderr)
	}
	if got := srv.LastAPIAuth(); got != "" {
		t.Errorf("expected no Authorization header without token, got %q", got)
	}
}

// TestInstallSh_TrailingSlashOnBases pins the contract that a trailing `/`
// on COCOON_API_BASE / COCOON_RELEASE_BASE does NOT produce `//<rest>` URLs.
// install.sh must strip a single trailing slash so end users on GHES /
// mirrors can configure either form interchangeably.
func TestInstallSh_TrailingSlashOnBases(t *testing.T) {
	t.Parallel()
	srv := newMockServer(t, mockRelease{})
	dst := t.TempDir()

	stderr, exit := runInstallSh(t, map[string]string{
		"COCOON_REPO":         mockRepo,
		"COCOON_API_BASE":     srv.baseURL() + "/",
		"COCOON_RELEASE_BASE": srv.baseURL() + "/",
		"COCOON_INSTALL_DIR":  dst,
	})
	if exit != 0 {
		t.Fatalf("exit=%d, stderr=%q", exit, stderr)
	}
	if _, err := os.Stat(filepath.Join(dst, "cocoon")); err != nil {
		t.Fatalf("binary not installed despite trailing-slash bases: %v", err)
	}
	apiHits := atomic.LoadInt32(srv.apiHits)
	dlHits := atomic.LoadInt32(srv.dlHits)
	if apiHits != 1 || dlHits != 1 {
		t.Errorf("expected exactly 1 hit on each endpoint (no double-slash drift); got api=%d dl=%d", apiHits, dlHits)
	}
}

// TestInstallSh_GHESPathPrefix pins that COCOON_API_BASE accepts a path
// prefix (e.g. GitHub Enterprise Server's `https://ghe.example/api/v3`)
// without losing the prefix when constructing release URLs. The mock server
// here serves the API under /api/v3 to mirror GHES; the release endpoints
// stay at the root because GHES serves release downloads from the host root,
// not the API prefix.
func TestInstallSh_GHESPathPrefix(t *testing.T) {
	t.Parallel()
	binary := []byte("ghes binary payload\n")
	asset := mockAssetName(t)
	sum := sha256.Sum256(binary)
	sumsBody := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), asset)

	mux := http.NewServeMux()
	var apiHits int32
	mux.HandleFunc(fmt.Sprintf("/api/v3/repos/%s/releases/latest", mockRepo),
		func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&apiHits, 1)
			writeMock(w, []byte(fmt.Sprintf(`{"tag_name":"%s"}`, mockTag)))
		})
	mux.HandleFunc(fmt.Sprintf("/%s/releases/download/%s/%s", mockRepo, mockTag, asset),
		func(w http.ResponseWriter, _ *http.Request) { writeMock(w, binary) })
	mux.HandleFunc(fmt.Sprintf("/%s/releases/download/%s/SHA256SUMS", mockRepo, mockTag),
		func(w http.ResponseWriter, _ *http.Request) { writeMock(w, []byte(sumsBody)) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	dst := t.TempDir()
	stderr, exit := runInstallSh(t, map[string]string{
		"COCOON_REPO":         mockRepo,
		"COCOON_API_BASE":     srv.URL + "/api/v3",
		"COCOON_RELEASE_BASE": srv.URL,
		"COCOON_INSTALL_DIR":  dst,
	})
	if exit != 0 {
		t.Fatalf("exit=%d, stderr=%q", exit, stderr)
	}
	if hits := atomic.LoadInt32(&apiHits); hits != 1 {
		t.Errorf("/api/v3 prefix not honoured: API endpoint hit %d times", hits)
	}
	got, err := os.ReadFile(filepath.Join(dst, "cocoon"))
	if err != nil {
		t.Fatalf("read installed binary: %v", err)
	}
	if !bytes.Equal(got, binary) {
		t.Errorf("binary content mismatch under GHES prefix: got %q", got)
	}
}

func TestInstallSh_ChecksumMismatch(t *testing.T) {
	t.Parallel()
	asset := mockAssetName(t)
	binary := []byte("real bytes\n")
	// Compute SHA256SUMS for *different* bytes so the actual hash won't match.
	bad := sha256.Sum256([]byte("not the real bytes\n"))
	sumsRaw := fmt.Sprintf("%s  %s\n", hex.EncodeToString(bad[:]), asset)

	srv := newMockServer(t, mockRelease{asset: asset, binary: binary, sumsRaw: sumsRaw})
	dst := t.TempDir()

	stderr, exit := runInstallSh(t, map[string]string{
		"COCOON_REPO":         mockRepo,
		"COCOON_API_BASE":     srv.baseURL(),
		"COCOON_RELEASE_BASE": srv.baseURL(),
		"COCOON_INSTALL_DIR":  dst,
	})
	if exit == 0 {
		t.Fatalf("expected non-zero exit, got 0; stderr=%q", stderr)
	}
	if !strings.Contains(stderr, "checksum mismatch") {
		t.Errorf("stderr missing checksum-mismatch message: %q", stderr)
	}
	if _, err := os.Stat(filepath.Join(dst, "cocoon")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("cocoon should not have been installed on checksum failure: stat err=%v", err)
	}
}
