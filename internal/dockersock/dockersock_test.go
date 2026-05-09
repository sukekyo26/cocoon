package dockersock_test

import (
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
