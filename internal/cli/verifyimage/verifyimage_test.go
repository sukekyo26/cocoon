package verifyimagecli_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	verifyimagecli "github.com/sukekyo26/cocoon/internal/cli/verifyimage"
	"github.com/sukekyo26/cocoon/internal/exec"
)

const minimalWorkspaceTOML = `
[container]
service_name = "dev"
username = "dev"
os = "ubuntu"
os_version = "24.04"

[plugins]
enable = ["go"]

[apt]
packages = ["jq", "; rm -rf /"]
`

func writeFixture(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "workspace.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func runCmd(r exec.Runner, args ...string) (stdout, stderr bytes.Buffer, err error) {
	cmd := verifyimagecli.NewCommandWithRunner(r, &stdout, &stderr)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return stdout, stderr, err
}

// TestVerifyimageQuotesAptPackageNames asserts that a malicious apt package
// name (containing shell metacharacters) is shell-quoted before it lands in
// the bash -c snippet so it cannot break out of the dpkg -s argument.
func TestVerifyimageQuotesAptPackageNames(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	r.Stub("docker", []string{
		"run", "--rm", "--entrypoint", "/bin/bash", "img:tag", "-c",
		"command -v go >/dev/null 2>&1",
	}, exec.Stub{Stdout: []byte("/usr/local/go/bin/go")})
	r.Stub("docker", []string{
		"run", "--rm", "--entrypoint", "/bin/bash", "img:tag", "-c",
		"dpkg -s jq >/dev/null 2>&1",
	}, exec.Stub{})
	r.Stub("docker", []string{
		"run", "--rm", "--entrypoint", "/bin/bash", "img:tag", "-c",
		`dpkg -s '; rm -rf /' >/dev/null 2>&1`,
	}, exec.Stub{})

	fixture := writeFixture(t, minimalWorkspaceTOML)
	if _, _, err := runCmd(r, "img:tag", fixture, "false"); err != nil &&
		!errors.Is(err, verifyimagecli.ErrFailure) {
		t.Fatalf("runCmd: unexpected error %v", err)
	}

	found := false
	for _, c := range r.Calls {
		if c.Name != "docker" || len(c.Args) < 7 {
			continue
		}
		script := c.Args[len(c.Args)-1]
		if strings.Contains(script, "rm -rf") {
			if !strings.Contains(script, `'; rm -rf /'`) {
				t.Errorf("malicious value not quoted; script = %q", script)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("did not see dpkg call for malicious package name")
	}
}

func TestVerifyimageRejectsBadArgs(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	_, _, err := runCmd(r, "only-one-arg")
	if !errors.Is(err, verifyimagecli.ErrUsage) {
		t.Errorf("expected ErrUsage, got %v", err)
	}
}

func TestVerifyimage_LoadFailure(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	_, _, err := runCmd(r, "img:tag", "/nonexistent.toml", "false")
	if !errors.Is(err, verifyimagecli.ErrFailure) {
		t.Errorf("expected ErrFailure, got %v", err)
	}
}

const verifyAllTOML = `
[container]
service_name = "dev"
username = "dev"
os = "ubuntu"
os_version = "24.04"

[plugins]
enable = ["go", "lazygit"]

[plugins.versions]
go = { pin = "1.22.0" }

[locale]
lang = "ja_JP.UTF-8"

[git]
user_name = "Alice"
user_email = "alice@example.com"

[dockerfile]
pre_user_setup = "RUN echo pre"
post_plugins = "RUN echo post"
`

func stubBash(r *exec.RecordingRunner, script string, stub exec.Stub) {
	r.Stub("docker", []string{
		"run", "--rm", "--entrypoint", "/bin/bash", "img:tag", "-c", script,
	}, stub)
}

func TestVerifyimage_VerifyPinSuccess(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	const img = "img:tag"
	stubBash(r, "command -v go >/dev/null 2>&1", exec.Stub{})
	stubBash(r, "command -v lazygit >/dev/null 2>&1", exec.Stub{})
	stubBash(r, "go version", exec.Stub{Stdout: []byte("go version go1.22.0 linux/amd64\n")})
	stubBash(r, "locale -a | tr -d '-' | tr '[:upper:]' '[:lower:]' | grep -qx 'ja_jp.utf8'",
		exec.Stub{})
	stubBash(r, `echo $LANG`, exec.Stub{Stdout: []byte("ja_JP.UTF-8\n")})
	stubBash(r, "git config --system user.name", exec.Stub{Stdout: []byte("Alice\n")})
	stubBash(r, "git config --system user.email", exec.Stub{Stdout: []byte("alice@example.com\n")})
	stubBash(r, "test -f /etc/marker-pre-user-setup", exec.Stub{})
	stubBash(r, "test -f /etc/marker-post-plugins", exec.Stub{})

	fixture := writeFixture(t, verifyAllTOML)
	stdout, stderr, err := runCmd(r, img, fixture, "true")
	if err != nil {
		t.Fatalf("err = %v, stdout=%s, stderr=%s", err, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "✓ All image-content assertions passed.") {
		t.Errorf("missing success line: %s", stdout.String())
	}
}

func TestVerifyimage_VerifyPinMismatch(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	const img = "img:tag"
	stubBash(r, "command -v go >/dev/null 2>&1", exec.Stub{})
	stubBash(r, "command -v lazygit >/dev/null 2>&1", exec.Stub{})
	stubBash(r, "go version", exec.Stub{Stdout: []byte("go version go1.21.5 linux/amd64\n")})
	stubBash(r, "locale -a | tr -d '-' | tr '[:upper:]' '[:lower:]' | grep -qx 'ja_jp.utf8'",
		exec.Stub{})
	stubBash(r, `echo $LANG`, exec.Stub{Stdout: []byte("ja_JP.UTF-8\n")})
	stubBash(r, "git config --system user.name", exec.Stub{Stdout: []byte("Alice\n")})
	stubBash(r, "git config --system user.email", exec.Stub{Stdout: []byte("alice@example.com\n")})
	stubBash(r, "test -f /etc/marker-pre-user-setup", exec.Stub{})
	stubBash(r, "test -f /etc/marker-post-plugins", exec.Stub{})

	fixture := writeFixture(t, verifyAllTOML)
	_, _, err := runCmd(r, img, fixture, "true")
	if !errors.Is(err, verifyimagecli.ErrFailure) {
		t.Fatalf("err = %v, want ErrFailure", err)
	}
}

func TestVerifyimage_GitIdentityMismatch(t *testing.T) {
	t.Parallel()
	const onlyGitTOML = `
[container]
service_name = "dev"
username = "dev"
os = "ubuntu"
os_version = "24.04"
[plugins]
enable = []
[git]
user_name = "Alice"
user_email = "alice@example.com"
`
	r := exec.NewRecordingRunner()
	const img = "img:tag"
	stubBash(r, "git config --system user.name", exec.Stub{Stdout: []byte("Bob\n")})
	stubBash(r, "git config --system user.email", exec.Stub{Stdout: []byte("bob@example.com\n")})

	fixture := writeFixture(t, onlyGitTOML)
	_, _, err := runCmd(r, img, fixture, "false")
	if !errors.Is(err, verifyimagecli.ErrFailure) {
		t.Fatalf("err = %v, want ErrFailure", err)
	}
}
