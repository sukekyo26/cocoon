package gencli_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gencli "github.com/sukekyo26/cocoon/internal/cli/gen"
)

// pinEnglish forces the i18n catalog to English for assertion stability.
// The cocoon binary picks Japanese when LANG / LC_* / WORKSPACE_LANG starts
// with "ja"; CI machines and Japanese developer hosts both fit that profile.
func pinEnglish(t *testing.T) {
	t.Helper()
	for _, k := range []string{"WORKSPACE_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		t.Setenv(k, "")
	}
	t.Setenv("WORKSPACE_LANG", "en")
}

const minimalWorkspaceTOML = `[workspace]
mount_root = "."
devcontainer = true

[container]
service_name = "demo"
username = "alice"
os = "ubuntu"
os_version = "24.04"

[plugins]
enable = []

[apt]
packages = []
`

// TestGen_DefaultRun runs `cocoon gen` with no flags inside a tempdir
// containing workspace.toml. It asserts that the three .devcontainer/
// artifacts land on disk and that the next-step message points the user
// at docker compose.
func TestGen_DefaultRun(t *testing.T) {
	// t.Parallel() is intentionally omitted because t.Setenv below is
	// incompatible with parallel execution.
	work := t.TempDir()
	home := filepath.Join(work, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", home)
	pinEnglish(t)
	t.Chdir(work)

	if err := os.WriteFile(filepath.Join(work, "workspace.toml"), []byte(minimalWorkspaceTOML), 0o600); err != nil {
		t.Fatalf("write workspace.toml: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := gencli.NewCommand(&stdout, &stderr)
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gen: %v\nstderr:\n%s", err, stderr.String())
	}

	for _, rel := range []string{
		".devcontainer/Dockerfile",
		".devcontainer/docker-compose.yml",
		".devcontainer/devcontainer.json",
	} {
		if _, statErr := os.Stat(filepath.Join(work, rel)); statErr != nil {
			t.Errorf("expected %s to exist: %v", rel, statErr)
		}
	}

	out := stdout.String()
	for _, want := range []string{
		"wrote .devcontainer/Dockerfile",
		"wrote .devcontainer/docker-compose.yml",
		"wrote .devcontainer/devcontainer.json",
		"To start the container:",
		"docker compose -f .devcontainer/docker-compose.yml up -d",
		`Reopen in Container`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q\n--- got ---\n%s", want, out)
		}
	}
}

// TestGen_NoDevcontainerSuppressesVSCodeHint covers the
// `[workspace] devcontainer = false` branch: devcontainer.json is not
// written and the VS Code line is suppressed, but the docker compose
// hint still appears.
func TestGen_NoDevcontainerSuppressesVSCodeHint(t *testing.T) {
	// t.Parallel() is intentionally omitted because t.Setenv below is
	// incompatible with parallel execution.
	work := t.TempDir()
	home := filepath.Join(work, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", home)
	pinEnglish(t)
	t.Chdir(work)

	body := strings.Replace(minimalWorkspaceTOML, "devcontainer = true", "devcontainer = false", 1)
	if err := os.WriteFile(filepath.Join(work, "workspace.toml"), []byte(body), 0o600); err != nil {
		t.Fatalf("write workspace.toml: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := gencli.NewCommand(&stdout, &stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gen: %v\nstderr:\n%s", err, stderr.String())
	}

	if _, statErr := os.Stat(filepath.Join(work, ".devcontainer/devcontainer.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("devcontainer.json should not exist when devcontainer=false: %v", statErr)
	}

	out := stdout.String()
	if !strings.Contains(out, "docker compose -f .devcontainer/docker-compose.yml up -d") {
		t.Errorf("docker compose hint missing in stdout:\n%s", out)
	}
	if strings.Contains(out, "Reopen in Container") {
		t.Errorf("VS Code line should be suppressed when devcontainer=false:\n%s", out)
	}
}

// TestGen_ExplicitWorkspaceAndOutput checks that --workspace and
// --output redirect both inputs and outputs without relying on cwd
// discovery.
func TestGen_ExplicitWorkspaceAndOutput(t *testing.T) {
	// t.Parallel() is intentionally omitted because t.Setenv below is
	// incompatible with parallel execution.
	work := t.TempDir()
	home := filepath.Join(work, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", home)
	pinEnglish(t)

	wsPath := filepath.Join(work, "custom-workspace.toml")
	if err := os.WriteFile(wsPath, []byte(minimalWorkspaceTOML), 0o600); err != nil {
		t.Fatalf("write workspace.toml: %v", err)
	}
	outDir := filepath.Join(work, "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir out: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := gencli.NewCommand(&stdout, &stderr)
	cmd.SetArgs([]string{"--workspace", wsPath, "--output", outDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gen: %v\nstderr:\n%s", err, stderr.String())
	}

	for _, rel := range []string{"Dockerfile", "docker-compose.yml", "devcontainer.json"} {
		path := filepath.Join(outDir, ".devcontainer", rel)
		if _, statErr := os.Stat(path); statErr != nil {
			t.Errorf("expected %s to exist: %v", path, statErr)
		}
	}
}

// TestGen_MissingWorkspaceReturnsUsage covers the discovery-miss path
// when workspace.toml is absent and no --workspace flag is given.
func TestGen_MissingWorkspaceReturnsUsage(t *testing.T) {
	// t.Parallel() is intentionally omitted because t.Setenv below is
	// incompatible with parallel execution.
	work := t.TempDir()
	home := filepath.Join(work, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", home)
	pinEnglish(t)
	t.Chdir(work)

	var stdout, stderr bytes.Buffer
	cmd := gencli.NewCommand(&stdout, &stderr)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected ErrUsage, got nil")
	}
	if !errors.Is(err, gencli.ErrUsage) {
		t.Errorf("expected ErrUsage, got %v", err)
	}
}
