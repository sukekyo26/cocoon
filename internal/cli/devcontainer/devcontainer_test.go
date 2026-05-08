package devcontainercli_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	devcontainercli "github.com/sukekyo26/cocoon/internal/cli/devcontainer"
	"github.com/sukekyo26/cocoon/internal/exec"
)

//nolint:unparam // test helper returns both buffers; not all callers use both
func runCmd(args ...string) (stdout, stderr bytes.Buffer, err error) {
	cmd := devcontainercli.NewCommand(&stdout, &stderr)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return stdout, stderr, err
}

//nolint:unparam // test helper returns both buffers; not all callers use both
func runWithRunner(r exec.Runner, args ...string) (stdout, stderr bytes.Buffer, err error) {
	cmd := devcontainercli.NewCommandWithRunner(r, &stdout, &stderr)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return stdout, stderr, err
}

func TestRun_NoArgs(t *testing.T) {
	t.Parallel()
	_, stderr, err := runCmd()
	if !errors.Is(err, devcontainercli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
	if !strings.Contains(stderr.String(), "wsd devcontainer") {
		t.Errorf("usage banner missing in stderr: %q", stderr.String())
	}
}

func TestRun_Help(t *testing.T) {
	t.Parallel()
	for _, flag := range []string{"help", "--help", "-h"} {
		stdout, _, err := runCmd(flag)
		if err != nil {
			t.Fatalf("flag=%q err = %v", flag, err)
		}
		if !strings.Contains(stdout.String(), "wsd devcontainer") {
			t.Errorf("flag=%q stdout missing banner: %q", flag, stdout.String())
		}
	}
}

func TestRun_UnknownSubcommand(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("bogus")
	if !errors.Is(err, devcontainercli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestCheck_MissingArg(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("check")
	if !errors.Is(err, devcontainercli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestUpHelp(t *testing.T) {
	t.Parallel()
	for _, flag := range []string{"--help", "-h", "help"} {
		stdout, _, err := runCmd("up", flag)
		if err != nil {
			t.Fatalf("flag=%q err = %v", flag, err)
		}
		if !strings.Contains(stdout.String(), "wsd devcontainer up") {
			t.Errorf("flag=%q expected up help banner: %q", flag, stdout.String())
		}
	}
}

func TestCheck_ExtraArgs(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("check", "/a", "/b")
	if !errors.Is(err, devcontainercli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

//nolint:paralleltest // mutates PATH via t.Setenv.
func TestCheck_NoDockerOnPath(t *testing.T) {
	t.Setenv("PATH", "")
	tmp := t.TempDir()
	r := exec.NewRecordingRunner()
	_, _, err := runWithRunner(r, "check", tmp)
	if !errors.Is(err, devcontainercli.ErrMissingDocker) {
		t.Fatalf("err = %v, want ErrMissingDocker", err)
	}
}

func TestUp_MissingArg(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("up")
	if !errors.Is(err, devcontainercli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

//nolint:paralleltest // mutates PATH via t.Setenv to suppress IsWSL workaround.
func TestUp_InvokesRunner(t *testing.T) {
	tmp := t.TempDir()
	dockerBin := filepath.Join(tmp, "docker")
	//nolint:gosec // fake docker stub must be executable for exec.LookPath
	if err := os.WriteFile(dockerBin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tmp)

	wsDir := t.TempDir()
	r := exec.NewRecordingRunner()
	r.Stub("devcontainer", []string{"up", "--workspace-folder", wsDir}, exec.Stub{})

	if _, _, err := runWithRunner(r, "up", wsDir); err != nil {
		t.Logf("runWithRunner returned %v (acceptable for this test)", err)
	}

	if len(r.Calls) == 0 {
		t.Fatalf("expected runner to be invoked")
	}
	c := r.Calls[0]
	if c.Method != "RunWithIO" || c.Name != "devcontainer" {
		t.Errorf("call = %+v, want RunWithIO devcontainer", c)
	}
	if len(c.Args) < 3 || c.Args[0] != "up" || c.Args[1] != "--workspace-folder" || c.Args[2] != wsDir {
		t.Errorf("args = %v, want first three to be [up --workspace-folder %s]", c.Args, wsDir)
	}
}
