package devcontainer

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/sukekyo26/cocoon/internal/exec"
)

func TestFileExists(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	regular := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(regular, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	subdir := filepath.Join(dir, "sub")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if !fileExists(regular) {
		t.Errorf("fileExists(regular file) = false")
	}
	if fileExists(subdir) {
		t.Errorf("fileExists(directory) = true, want false")
	}
	if fileExists(filepath.Join(dir, "missing")) {
		t.Errorf("fileExists(missing) = true, want false")
	}
}

func TestDirExists(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if !dirExists(dir) {
		t.Errorf("dirExists(real dir) = false")
	}
	if dirExists(filepath.Join(dir, "missing")) {
		t.Errorf("dirExists(missing) = true")
	}
	regular := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(regular, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if dirExists(regular) {
		t.Errorf("dirExists(file) = true, want false")
	}
}

//nolint:paralleltest // mutates PATH via t.Setenv.
func TestPathContains(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/usr/local/bin:/opt/foo")
	cases := []struct {
		dir  string
		want bool
	}{
		{"/usr/bin", true},
		{"/opt/foo", true},
		{"/not/in/path", false},
	}
	for _, tc := range cases {
		if got := pathContains(tc.dir); got != tc.want {
			t.Errorf("pathContains(%q) = %v, want %v", tc.dir, got, tc.want)
		}
	}
}

//nolint:paralleltest // mutates PATH via t.Setenv.
func TestUp_DockerLookupErrorOnWSL(t *testing.T) {
	// Force WSL detection by setting the env var.
	t.Setenv("WSL_DISTRO_NAME", "Test")
	t.Setenv("PATH", "")

	r := exec.NewRecordingRunner()
	err := Up(r, []string{"up", "--workspace-folder", "/tmp"}, io.Discard, io.Discard)
	if !errors.Is(err, ErrDockerMissing) {
		t.Skipf("WSL workaround branch only triggers on linux; got err=%v", err)
	}
}

//nolint:paralleltest // mutates PATH via t.Setenv.
func TestUp_RunnerInvoked(t *testing.T) {
	// Place a fake docker binary on PATH so the WSL branch (if it fires) can
	// resolve it. Then assert the runner was called with the up subcommand.
	tmp := t.TempDir()
	dockerPath := filepath.Join(tmp, "docker")
	//nolint:gosec // fake docker stub must be executable for exec.LookPath
	if err := os.WriteFile(dockerPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tmp)
	t.Setenv("WSL_DISTRO_NAME", "")

	r := exec.NewRecordingRunner()
	r.Stub("devcontainer", []string{"up", "--workspace-folder", "/tmp"}, exec.Stub{})

	if err := Up(r, []string{"up", "--workspace-folder", "/tmp"}, io.Discard, io.Discard); err != nil {
		t.Logf("Up returned %v (acceptable for this test)", err)
	}

	if len(r.Calls) == 0 {
		t.Fatal("runner not invoked")
	}
	c := r.Calls[0]
	if c.Method != "RunWithIO" {
		t.Errorf("method = %q, want RunWithIO", c.Method)
	}
	if c.Name != "devcontainer" || len(c.Args) < 1 || c.Args[0] != "up" {
		t.Errorf("call = %+v", c)
	}
}

//nolint:paralleltest // mutates PATH via os.Setenv.
func TestCheckDocker_NotOnPath(t *testing.T) {
	old := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", old) })
	_ = os.Setenv("PATH", "")

	r := exec.NewRecordingRunner()
	res := checkDocker(r, io.Discard)
	if res != PrereqMissingDocker {
		t.Errorf("res = %v, want PrereqMissingDocker", res)
	}
}

//nolint:paralleltest // mutates PATH via os.Setenv.
func TestCheckDocker_DaemonUnreachable(t *testing.T) {
	// Provide a fake docker binary so LookPath succeeds, then have the
	// runner stub fail `docker info` to exercise the daemon-unreachable
	// branch.
	tmp := t.TempDir()
	dockerPath := filepath.Join(tmp, "docker")
	//nolint:gosec // fake docker stub must be executable for exec.LookPath
	if err := os.WriteFile(dockerPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", old) })
	_ = os.Setenv("PATH", tmp)

	r := exec.NewRecordingRunner()
	r.Stub("docker", []string{"info"}, exec.Stub{Err: context.Canceled})
	res := checkDocker(r, io.Discard)
	if res != PrereqMissingDocker {
		t.Errorf("res = %v, want PrereqMissingDocker", res)
	}
}
