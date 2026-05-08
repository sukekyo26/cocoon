package certificatescli_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	certificatescli "github.com/sukekyo26/cocoon/internal/cli/certificates"
)

const validPEM = `-----BEGIN CERTIFICATE-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAtest
-----END CERTIFICATE-----
`

func makeProject(t *testing.T, withValid bool) string {
	t.Helper()
	root := t.TempDir()
	certsDir := filepath.Join(root, "certs")
	if err := os.MkdirAll(certsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if withValid {
		if err := os.WriteFile(filepath.Join(certsDir, "valid.crt"), []byte(validPEM), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

//nolint:unparam // test helper returns both buffers; not all callers use both
func runCmd(args ...string) (stdout, stderr bytes.Buffer, err error) {
	cmd := certificatescli.NewCommand(&stdout, &stderr)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return stdout, stderr, err
}

func TestRun_NoArgs_PrintsUsage(t *testing.T) {
	t.Parallel()
	stdout, _, err := runCmd()
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !strings.Contains(stdout.String(), "wsd certificates") {
		t.Errorf("usage banner missing from stdout: %q", stdout.String())
	}
}

func TestRun_Help_PrintsUsage(t *testing.T) {
	t.Parallel()
	for _, flag := range []string{"help", "--help", "-h"} {
		stdout, _, err := runCmd(flag)
		if err != nil {
			t.Fatalf("flag=%q err = %v", flag, err)
		}
		if !strings.Contains(stdout.String(), "wsd certificates") {
			t.Errorf("flag=%q stdout missing banner: %q", flag, stdout.String())
		}
	}
}

func TestRun_UnknownSubcommand(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("bogus")
	if !errors.Is(err, certificatescli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("expected mention of \"bogus\" in err, got %v", err)
	}
}

func TestList_PrintsValidCerts(t *testing.T) {
	t.Parallel()
	root := makeProject(t, true)
	stdout, _, err := runCmd("list", root)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "valid.crt" {
		t.Errorf("stdout = %q, want \"valid.crt\"", got)
	}
}

func TestList_NoCerts_EmptyOutput(t *testing.T) {
	t.Parallel()
	root := makeProject(t, false)
	stdout, _, err := runCmd("list", root)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
}

func TestList_MissingArg(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("list")
	if !errors.Is(err, certificatescli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestList_ExtraArgs(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("list", "/a", "/b")
	if !errors.Is(err, certificatescli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestCheck_Valid(t *testing.T) {
	t.Parallel()
	root := makeProject(t, true)
	_, _, err := runCmd("check", root)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestCheck_NoCerts_Failure(t *testing.T) {
	t.Parallel()
	root := makeProject(t, false)
	_, _, err := runCmd("check", root)
	if !errors.Is(err, certificatescli.ErrFailure) {
		t.Fatalf("err = %v, want ErrFailure", err)
	}
}

func TestCheck_MissingArg(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("check")
	if !errors.Is(err, certificatescli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestCheck_ExtraArgs(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("check", "/a", "/b")
	if !errors.Is(err, certificatescli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestRun_RootFlagErrorWrapsAsUsage(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("--no-such-flag")
	if !errors.Is(err, certificatescli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestList_FlagErrorWrapsAsUsage(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("list", "--bogus")
	if !errors.Is(err, certificatescli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}
