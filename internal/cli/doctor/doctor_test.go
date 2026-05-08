package doctorcli_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	doctorcli "github.com/sukekyo26/cocoon/internal/cli/doctor"
)

//nolint:unparam // test helper returns both buffers; not all callers use both
func runCmd(args ...string) (stdout, stderr bytes.Buffer, err error) {
	cmd := doctorcli.NewCommand(&stdout, &stderr)
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
		if !strings.Contains(stdout.String(), "wsd doctor") {
			t.Errorf("flag=%q stdout missing banner: %q", flag, stdout.String())
		}
	}
}

func TestRun_MissingFlagValues(t *testing.T) {
	t.Parallel()
	cases := [][]string{
		{"--root"},
		{"--plugins-dir"},
		{"--root", "/x", "--plugins-dir"},
	}
	for _, args := range cases {
		_, _, err := runCmd(args...)
		if !errors.Is(err, doctorcli.ErrUsage) {
			t.Errorf("args=%v err = %v, want ErrUsage", args, err)
		}
	}
}

func TestRun_UnknownArg(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("--bogus")
	if !errors.Is(err, doctorcli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
	if !strings.Contains(err.Error(), "--bogus") {
		t.Errorf("err should mention --bogus: %v", err)
	}
}

func TestRun_FlagEqualsForm(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("--root=/nonexistent/path/for/doctor/test", "--plugins-dir=/nonexistent/plugins")
	if errors.Is(err, doctorcli.ErrUsage) {
		t.Fatalf("got ErrUsage, want nil or ErrFailure: %v", err)
	}
}
