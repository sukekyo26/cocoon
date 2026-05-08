package dockersock_test

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/sukekyo26/cocoon/internal/dockersock"
)

func TestCandidatePathsAlwaysIncludesVarRun(t *testing.T) {
	t.Parallel()
	got := dockersock.CandidatePaths()
	if len(got) == 0 || got[0] != "/var/run/docker.sock" {
		t.Errorf("first candidate = %v, want /var/run/docker.sock", got)
	}
}

func TestCandidatePathsIncludesHomeDocker(t *testing.T) {
	t.Parallel()
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home dir")
	}
	want := filepath.Join(home, ".docker", "run", "docker.sock")
	for _, p := range dockersock.CandidatePaths() {
		if p == want {
			return
		}
	}
	t.Errorf("expected %s in candidates, got %v", want, dockersock.CandidatePaths())
}

// TestFirstSmoke exercises First() in whatever state the host happens to be
// in. Its only job is to keep the loop body reachable for coverage when more
// specific tests skip due to host state. Cannot run in parallel because the
// concurrent t.Setenv-using tests in this file race on the same env vars.
func TestFirstSmoke(t *testing.T) { //nolint:paralleltest // shares env with t.Setenv tests.
	_ = dockersock.First()
}

func TestFirstReturnsEmptyWhenNoSocket(t *testing.T) {
	// Override $HOME so the home-dir candidate cannot match a real socket.
	// t.Setenv is incompatible with t.Parallel.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	// /var/run/docker.sock may exist on the test host; this test is only
	// meaningful when it does not. Skip when it does.
	if info, err := os.Stat("/var/run/docker.sock"); err == nil && info.Mode()&os.ModeSocket != 0 {
		t.Skip("/var/run/docker.sock exists on host; cannot exercise no-socket path")
	}
	if got := dockersock.First(); got != "" {
		t.Errorf("First() = %q, want empty", got)
	}
}

func TestFirstReturnsSocketUnderHome(t *testing.T) {
	// t.Setenv is incompatible with t.Parallel.
	if _, err := os.Stat("/var/run/docker.sock"); err == nil {
		t.Skip("/var/run/docker.sock exists on host; would mask the home-dir candidate")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir()) // empty dir
	sockDir := filepath.Join(home, ".docker", "run")
	if err := os.MkdirAll(sockDir, 0o700); err != nil {
		t.Fatal(err)
	}
	sockPath := filepath.Join(sockDir, "docker.sock")
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "unix", sockPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	if got := dockersock.First(); got != sockPath {
		t.Errorf("First() = %q, want %q", got, sockPath)
	}
}

func TestFirstReturnsSocketUnderXDG(t *testing.T) {
	// t.Setenv is incompatible with t.Parallel.
	if _, err := os.Stat("/var/run/docker.sock"); err == nil {
		t.Skip("/var/run/docker.sock exists on host; would mask the XDG candidate")
	}
	xdg := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", xdg)
	t.Setenv("HOME", t.TempDir()) // empty home so it cannot match
	sockPath := filepath.Join(xdg, "docker.sock")
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "unix", sockPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	if got := dockersock.First(); got != sockPath {
		t.Errorf("First() = %q, want %q", got, sockPath)
	}
}

func TestFirstSkipsRegularFile(t *testing.T) {
	// t.Setenv is incompatible with t.Parallel.
	if _, err := os.Stat("/var/run/docker.sock"); err == nil {
		t.Skip("/var/run/docker.sock exists on host; cannot test rejection path")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	sockDir := filepath.Join(home, ".docker", "run")
	if err := os.MkdirAll(sockDir, 0o700); err != nil {
		t.Fatal(err)
	}
	// Place a regular file at the candidate path; isSocket must reject it
	// because the bitwise check on os.ModeSocket fails.
	if err := os.WriteFile(filepath.Join(sockDir, "docker.sock"), []byte("not a socket"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := dockersock.First(); got != "" {
		t.Errorf("First() = %q, want empty (regular file should not match)", got)
	}
}

func TestCandidatePathsFallsBackToRunUserUID(t *testing.T) {
	// t.Setenv is incompatible with t.Parallel.
	if os.Getuid() == 0 {
		t.Skip("UID 0 skips the /run/user/$UID fallback")
	}
	t.Setenv("XDG_RUNTIME_DIR", "")
	uidStr := strconv.Itoa(os.Getuid())
	want := filepath.Join("/run", "user", uidStr, "docker.sock")
	for _, p := range dockersock.CandidatePaths() {
		if p == want {
			return
		}
	}
	t.Errorf("expected %s in candidates, got %v", want, dockersock.CandidatePaths())
}
