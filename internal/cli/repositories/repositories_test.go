package repositoriescli_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	repositoriescli "github.com/sukekyo26/cocoon/internal/cli/repositories"
)

//nolint:unparam // test helper returns both buffers; not all callers use both
func runCmd(args ...string) (stdout, stderr bytes.Buffer, err error) {
	cmd := repositoriescli.NewCommand(&stdout, &stderr)
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
	if !strings.Contains(stdout.String(), "wsd repositories") {
		t.Errorf("usage banner missing from stdout: %q", stdout.String())
	}
}

func TestRun_Help(t *testing.T) {
	t.Parallel()
	for _, flag := range []string{"help", "--help", "-h"} {
		stdout, _, err := runCmd(flag)
		if err != nil {
			t.Fatalf("flag=%q err = %v", flag, err)
		}
		if !strings.Contains(stdout.String(), "wsd repositories") {
			t.Errorf("flag=%q stdout missing banner: %q", flag, stdout.String())
		}
	}
}

func TestRun_UnknownSubcommand(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("bogus")
	if !errors.Is(err, repositoriescli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestClone_MissingArg(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("clone")
	if !errors.Is(err, repositoriescli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestClone_ExtraArgs(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("clone", "/a", "/b")
	if !errors.Is(err, repositoriescli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestStatus_MissingArg(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("status")
	if !errors.Is(err, repositoriescli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestStatus_ExtraArgs(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("status", "/a", "/b")
	if !errors.Is(err, repositoriescli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestStatus_NoWorkspaceTOML(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	_, _, err := runCmd("status", tmp)
	if errors.Is(err, repositoriescli.ErrUsage) {
		t.Fatalf("err = %v, want non-Usage failure", err)
	}
}

func TestClone_NoWorkspaceTOML(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	_, _, err := runCmd("clone", tmp)
	if errors.Is(err, repositoriescli.ErrUsage) {
		t.Fatalf("err = %v, want non-Usage failure", err)
	}
}

func TestRun_FlagErrorWrapsAsUsage(t *testing.T) {
	t.Parallel()
	// Unknown root flag triggers cobra flag error, which the SetFlagErrorFunc
	// wraps as ErrUsage via the usageErr helper.
	_, _, err := runCmd("--no-such-flag")
	if !errors.Is(err, repositoriescli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestStatus_FlagErrorWrapsAsUsage(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("status", "--bogus")
	if !errors.Is(err, repositoriescli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}
