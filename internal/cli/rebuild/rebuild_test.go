package rebuildcli_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	rebuildcli "github.com/sukekyo26/cocoon/internal/cli/rebuild"
)

//nolint:unparam // test helper returns both buffers; not all callers use both
func runCmd(args ...string) (stdout, stderr bytes.Buffer, err error) {
	cmd := rebuildcli.NewCommand(&stdout, &stderr)
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
		if !strings.Contains(stdout.String(), "wsd rebuild") {
			t.Errorf("flag=%q stdout missing banner: %q", flag, stdout.String())
		}
	}
}

func TestRun_MissingWorkspaceDir(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd()
	if !errors.Is(err, rebuildcli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestRun_FlagValueMissing(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("--workspace-dir")
	if !errors.Is(err, rebuildcli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestRun_UnknownArg(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("--bogus")
	if !errors.Is(err, rebuildcli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestRun_RejectsPositional(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("--workspace-dir", "/tmp/x", "stray-positional")
	if !errors.Is(err, rebuildcli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestRun_HelpAlias(t *testing.T) {
	t.Parallel()
	stdout, _, err := runCmd("help")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(stdout.String(), "wsd rebuild") {
		t.Errorf("expected help banner via alias: %q", stdout.String())
	}
}
