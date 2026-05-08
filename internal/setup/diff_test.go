//nolint:testpackage // exercises unexported runDiff / diffRelPaths.
package setup

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/logx"
)

type genFn func(wsPath, pluginsDir, outputDir string, stderr io.Writer) error

func (f genFn) GenerateAll(wsPath, pluginsDir, outputDir string, stderr io.Writer) error {
	return f(wsPath, pluginsDir, outputDir, stderr)
}

const minimalContainerTOML = `[container]
service_name = "diff-test"
username = "tester"
os = "ubuntu"
os_version = "24.04"

[plugins]
enable = []

[apt]
packages = []
`

func seedMinimalWorkspace(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "workspace.toml"), []byte(minimalContainerTOML), 0o600); err != nil {
		t.Fatalf("write workspace.toml: %v", err)
	}
}

func diffOpts(t *testing.T, work string) (Options, *bytes.Buffer) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	return Options{
		WorkspaceDir: work,
		Stdout:       &stdout,
		Stderr:       &stderr,
		Logger:       logx.New(&stdout, &stderr),
	}, &stdout
}

// genFile writes a single file under outputDir and returns a generator that
// produces it. Other diffRelPaths entries are deliberately absent so the
// remaining branches exercise the "missing both sides" path.
func genFile(rel, content string) genFn {
	return func(_, _, outputDir string, _ io.Writer) error {
		dst := filepath.Join(outputDir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dst, []byte(content), 0o600)
	}
}

func TestRunDiff_NewFile(t *testing.T) {
	t.Parallel()
	work := t.TempDir()
	seedMinimalWorkspace(t, work)

	opts, stdout := diffOpts(t, work)
	opts.Generator = genFile("Dockerfile", "FROM ubuntu:24.04\n")

	err := runDiff(opts, filepath.Join(work, "workspace.toml"), "")
	if !errors.Is(err, ErrDiffFound) {
		t.Fatalf("err = %v, want ErrDiffFound", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "### NEW FILE: Dockerfile") {
		t.Errorf("missing NEW FILE marker:\n%s", out)
	}
	if !strings.Contains(out, "FROM ubuntu:24.04") {
		t.Errorf("missing candidate body in stdout:\n%s", out)
	}
	if !strings.Contains(out, "Changes detected.") {
		t.Errorf("missing summary line:\n%s", out)
	}
}

func TestRunDiff_NoChanges(t *testing.T) {
	t.Parallel()
	work := t.TempDir()
	seedMinimalWorkspace(t, work)

	// Seed every diffRelPaths target with content that the generator will
	// reproduce byte-for-byte.
	files := map[string]string{
		"docker-compose.yml":               "services: {}\n",
		"Dockerfile":                       "FROM ubuntu:24.04\n",
		".devcontainer/devcontainer.json":  "{}\n",
		".devcontainer/docker-compose.yml": "services: {}\n",
		"config/.bashrc_custom.generated":  "# rc\n",
	}
	for rel, body := range files {
		dst := filepath.Join(work, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	opts, stdout := diffOpts(t, work)
	opts.Generator = genFn(func(_, _, outputDir string, _ io.Writer) error {
		for rel, body := range files {
			dst := filepath.Join(outputDir, rel)
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(dst, []byte(body), 0o600); err != nil {
				return err
			}
		}
		return nil
	})

	if err := runDiff(opts, filepath.Join(work, "workspace.toml"), ""); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "No changes.") {
		t.Errorf("missing No changes line:\n%s", out)
	}
	if strings.Contains(out, "### DIFF") || strings.Contains(out, "### NEW FILE") {
		t.Errorf("unexpected diff markers in clean run:\n%s", out)
	}
}

func TestRunDiff_ContentDiff(t *testing.T) {
	if _, err := exec.LookPath("diff"); err != nil {
		t.Skip("diff binary not available on host")
	}
	t.Parallel()
	work := t.TempDir()
	seedMinimalWorkspace(t, work)

	if err := os.WriteFile(filepath.Join(work, "Dockerfile"), []byte("FROM ubuntu:22.04\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	opts, stdout := diffOpts(t, work)
	opts.Generator = genFile("Dockerfile", "FROM ubuntu:24.04\n")

	err := runDiff(opts, filepath.Join(work, "workspace.toml"), "")
	if !errors.Is(err, ErrDiffFound) {
		t.Fatalf("err = %v, want ErrDiffFound", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "### DIFF: Dockerfile") {
		t.Errorf("missing DIFF marker:\n%s", out)
	}
	if !strings.Contains(out, "ubuntu:22.04") || !strings.Contains(out, "ubuntu:24.04") {
		t.Errorf("expected unified diff lines for both versions:\n%s", out)
	}
}

func TestRunDiff_GeneratorError(t *testing.T) {
	t.Parallel()
	work := t.TempDir()
	seedMinimalWorkspace(t, work)

	sentinel := errors.New("boom")
	opts, _ := diffOpts(t, work)
	opts.Generator = genFn(func(_, _, _ string, _ io.Writer) error { return sentinel })

	err := runDiff(opts, filepath.Join(work, "workspace.toml"), "")
	if err == nil || !strings.Contains(err.Error(), "generate") {
		t.Fatalf("err = %v, want wrapped generate error", err)
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error chain missing sentinel: %v", err)
	}
}

func TestDiffRelPaths_LoginShell(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		toml     string
		wantLast string
	}{
		{
			name:     "missing_workspace_falls_back_to_bash",
			toml:     "",
			wantLast: "config/.bashrc_custom.generated",
		},
		{
			name: "explicit_zsh",
			toml: `[container]
service_name = "x"
username = "x"
os = "ubuntu"
os_version = "24.04"
[container.shell]
default = "zsh"
[plugins]
enable = []
[apt]
packages = []
`,
			wantLast: "config/.zshrc_custom.generated",
		},
		{
			name: "explicit_fish",
			toml: `[container]
service_name = "x"
username = "x"
os = "ubuntu"
os_version = "24.04"
[container.shell]
default = "fish"
[plugins]
enable = []
[apt]
packages = []
`,
			wantLast: "config/config.fish_custom.generated",
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			work := t.TempDir()
			ws := filepath.Join(work, "workspace.toml")
			if c.toml != "" {
				if err := os.WriteFile(ws, []byte(c.toml), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got := diffRelPaths(ws)
			if len(got) == 0 {
				t.Fatal("empty diffRelPaths")
			}
			if got[len(got)-1] != c.wantLast {
				t.Errorf("last entry = %q, want %q (full = %v)", got[len(got)-1], c.wantLast, got)
			}
		})
	}
}
