package cleancli_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	cleancli "github.com/sukekyo26/cocoon/internal/cli/clean"
)

//nolint:unparam // test helper returns both buffers; not all callers use both
func runCmd(args ...string) (stdout, stderr bytes.Buffer, err error) {
	cmd := cleancli.NewCommand(&stdout, &stderr)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return stdout, stderr, err
}

func TestRun_NoArgs_PrintsUsage(t *testing.T) {
	t.Parallel()
	stdout, _, err := runCmd()
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(stdout.String(), "wsd clean") {
		t.Errorf("usage banner missing: %q", stdout.String())
	}
}

func TestRun_Help(t *testing.T) {
	t.Parallel()
	for _, flag := range []string{"help", "--help", "-h"} {
		stdout, _, err := runCmd(flag)
		if err != nil {
			t.Fatalf("flag=%q err = %v", flag, err)
		}
		if !strings.Contains(stdout.String(), "wsd clean") {
			t.Errorf("flag=%q stdout missing banner: %q", flag, stdout.String())
		}
	}
}

func TestRun_UnknownSubcommand(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("bogus")
	if !errors.Is(err, cleancli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestVolumes_MissingWorkspaceDir(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("volumes")
	if !errors.Is(err, cleancli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestVolumes_FlagValueMissing(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("volumes", "--workspace-dir")
	if !errors.Is(err, cleancli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestVolumes_UnknownArg(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("volumes", "--bogus")
	if !errors.Is(err, cleancli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestVolumes_Help(t *testing.T) {
	t.Parallel()
	stdout, _, err := runCmd("volumes", "--help")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(stdout.String(), "wsd clean volumes") {
		t.Errorf("missing volumes usage: %q", stdout.String())
	}
}

func TestDocker_Help(t *testing.T) {
	t.Parallel()
	stdout, _, err := runCmd("docker", "--help")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(stdout.String(), "wsd clean docker") {
		t.Errorf("missing docker usage: %q", stdout.String())
	}
}

func TestDocker_UnknownArg(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("docker", "--bogus")
	if !errors.Is(err, cleancli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}
