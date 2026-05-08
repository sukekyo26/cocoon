package setupcli_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	setupcli "github.com/sukekyo26/cocoon/internal/cli/setup"
)

//nolint:unparam // test helper returns both buffers; not all callers use both
func runCmd(args ...string) (stdout, stderr bytes.Buffer, err error) {
	cmd := setupcli.NewCommand(&stdout, &stderr)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return stdout, stderr, err
}

func TestRun_Help(t *testing.T) {
	t.Parallel()
	for _, flag := range []string{"help", "--help", "-h"} {
		stdout, _, err := runCmd(flag)
		if err != nil {
			t.Fatalf("flag=%q err = %v", flag, err)
		}
		if !strings.Contains(stdout.String(), "wsd setup") {
			t.Errorf("flag=%q stdout missing banner: %q", flag, stdout.String())
		}
	}
}

func TestRun_MissingWorkspaceDir(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("--yes")
	if !errors.Is(err, setupcli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestRun_MissingFlagValues(t *testing.T) {
	t.Parallel()
	cases := [][]string{
		{"--workspace-dir"},
		{"--plugins-dir"},
		{"--lang"},
	}
	for _, args := range cases {
		_, _, err := runCmd(args...)
		if !errors.Is(err, setupcli.ErrUsage) {
			t.Errorf("args=%v err = %v, want ErrUsage", args, err)
		}
	}
}

func TestRun_UnknownFlag(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("--bogus")
	if !errors.Is(err, setupcli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestRun_RejectsStrayPositional(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("stray-positional")
	if !errors.Is(err, setupcli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestRun_AcceptsAllFlags(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	args := []string{
		"--workspace-dir", tmp,
		"--plugins-dir", tmp + "/plugins",
		"--init",
		"--yes",
		"--no-clone",
		"--lang", "ja",
	}
	_, _, err := runCmd(args...)
	if errors.Is(err, setupcli.ErrUsage) {
		t.Fatalf("got ErrUsage, want a downstream failure: %v", err)
	}
}

func TestRun_DoctorFlag(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	_, _, err := runCmd("--workspace-dir", tmp, "--doctor")
	if errors.Is(err, setupcli.ErrUsage) {
		t.Fatalf("got ErrUsage, want a downstream error: %v", err)
	}
}

func TestRun_ShortYesFlag(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	_, _, err := runCmd("--workspace-dir", tmp, "-y")
	if errors.Is(err, setupcli.ErrUsage) {
		t.Fatalf("got ErrUsage, want a downstream error: %v", err)
	}
}
