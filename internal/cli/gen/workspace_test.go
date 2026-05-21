package gencli_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	gencli "github.com/sukekyo26/cocoon/internal/cli/gen"
	"github.com/sukekyo26/cocoon/internal/generate/codeworkspace"
)

// codeWorkspaceTOML mirrors minimalWorkspaceTOML with a [code_workspace]
// section that exercises the all-features path: home expansion, explicit
// name override, settings, and extensions. The HOME-dependent rel paths
// are asserted in the test body, not in this fixture.
const codeWorkspaceTOML = minimalWorkspaceTOML + `
[code_workspace]
name = "ws-stack"
folders = [
    { path = "." },
    { path = "~/.claude" },
    { path = "~/.config/nvim", name = "Neovim" },
]

[code_workspace.settings]
"editor.tabSize" = 2

[code_workspace.extensions]
recommendations = ["golang.go"]
`

// TestGenWorkspace_DefaultRun verifies the happy path: `cocoon gen
// workspace` reads workspace.toml, writes <name>.code-workspace at the
// project root (not under .devcontainer/), and prints the next-step hint.
// The folders' relative paths are asserted via JSON parse so the test
// stays robust against whitespace tweaks in the generator.
func TestGenWorkspace_DefaultRun(t *testing.T) {
	// No t.Parallel(): t.Setenv blocks parallel execution.
	work := t.TempDir()
	home := filepath.Join(work, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", home)
	pinEnglish(t)
	t.Chdir(work)

	if err := os.WriteFile(filepath.Join(work, "workspace.toml"), []byte(codeWorkspaceTOML), 0o600); err != nil {
		t.Fatalf("write workspace.toml: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := gencli.NewCommand(&stdout, &stderr)
	cmd.SetArgs([]string{"workspace"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gen workspace: %v\nstderr:\n%s", err, stderr.String())
	}

	target := filepath.Join(work, "ws-stack.code-workspace")
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("expected %s to exist: %v", target, err)
	}
	if perm := info.Mode().Perm(); perm != 0o644 {
		t.Errorf(".code-workspace mode = %o, want 0644", perm)
	}

	raw, err := os.ReadFile(target) //nolint:gosec // test reads what we just wrote
	if err != nil {
		t.Fatalf("read .code-workspace: %v", err)
	}
	if !bytes.HasSuffix(raw, []byte{'\n'}) {
		t.Errorf("expected trailing newline; last byte = %x", raw[len(raw)-1])
	}

	var parsed struct {
		Folders []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"folders"`
		Settings   map[string]any `json:"settings"`
		Extensions struct {
			Recommendations []string `json:"recommendations"`
		} `json:"extensions"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, string(raw))
	}
	if len(parsed.Folders) != 3 {
		t.Fatalf("folders count = %d, want 3", len(parsed.Folders))
	}
	// Folder 0: project itself, name auto-derived from filepath.Base(projectDir).
	if parsed.Folders[0].Path != "." {
		t.Errorf("folders[0].path = %q, want \".\"", parsed.Folders[0].Path)
	}
	// Folder 1: ~/.claude. With ProjectDir == work and HomeDir == work/home,
	// the rel path is "home/.claude".
	if parsed.Folders[1].Path != "home/.claude" {
		t.Errorf("folders[1].path = %q, want \"home/.claude\"", parsed.Folders[1].Path)
	}
	// Folder 2: explicit name override.
	if parsed.Folders[2].Name != "Neovim" {
		t.Errorf("folders[2].name = %q, want \"Neovim\"", parsed.Folders[2].Name)
	}
	if parsed.Settings["editor.tabSize"] == nil {
		t.Errorf("settings.editor.tabSize missing")
	}
	if len(parsed.Extensions.Recommendations) != 1 || parsed.Extensions.Recommendations[0] != "golang.go" {
		t.Errorf("extensions.recommendations = %v, want [golang.go]", parsed.Extensions.Recommendations)
	}

	out := stdout.String()
	for _, want := range []string{
		"wrote ws-stack.code-workspace",
		"Next steps:",
		"code ws-stack.code-workspace",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q\n--- got ---\n%s", want, out)
		}
	}
}

// TestGenWorkspace_FolderFlagAppendsAndOverridesName covers the
// "--folder <path>=<name>" CLI surface: a flag-supplied folder lands after
// the workspace.toml entries and its name is taken from the "=NAME" half.
func TestGenWorkspace_FolderFlagAppendsAndOverridesName(t *testing.T) {
	work := t.TempDir()
	home := filepath.Join(work, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", home)
	pinEnglish(t)
	t.Chdir(work)

	if err := os.WriteFile(filepath.Join(work, "workspace.toml"), []byte(codeWorkspaceTOML), 0o600); err != nil {
		t.Fatalf("write workspace.toml: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := gencli.NewCommand(&stdout, &stderr)
	cmd.SetArgs([]string{"workspace", "--folder", "~/.config/zsh=Zsh", "--name", "from-flag"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gen workspace: %v\nstderr:\n%s", err, stderr.String())
	}

	target := filepath.Join(work, "from-flag.code-workspace")
	raw, err := os.ReadFile(target) //nolint:gosec
	if err != nil {
		t.Fatalf("expected %s: %v", target, err)
	}
	body := string(raw)
	if !strings.Contains(body, `"name": "Zsh"`) {
		t.Errorf("body missing flag-supplied name override; got:\n%s", body)
	}
	if !strings.Contains(body, `"path": "home/.config/zsh"`) {
		t.Errorf("body missing flag-supplied path (relativized); got:\n%s", body)
	}
	// Flag entry must follow the toml entries (last folder in the list).
	idxNeovim := strings.Index(body, `"name": "Neovim"`)
	idxZsh := strings.Index(body, `"name": "Zsh"`)
	if idxNeovim < 0 || idxZsh < 0 || idxZsh < idxNeovim {
		t.Errorf("--folder entry must appear after toml entries; got:\n%s", body)
	}
}

// TestGenWorkspace_NoFoldersReturnsUsage exercises the empty-folders
// failure path: workspace.toml with no [code_workspace] block at all and
// no --folder flags must surface ErrNoFolders as an ErrUsage-classified
// CLI error, not a generic ErrFailure.
func TestGenWorkspace_NoFoldersReturnsUsage(t *testing.T) {
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
	cmd.SetArgs([]string{"workspace"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error, got nil; stdout:\n%s", stdout.String())
	}
	if !errors.Is(err, clihelpers.ErrUsage) {
		t.Errorf("err = %v, want errors.Is clihelpers.ErrUsage", err)
	}
	// The CLI's gen_workspace_no_folders message references the user-fix.
	if !strings.Contains(err.Error(), "[code_workspace].folders") {
		t.Errorf("err message missing fix hint; got: %s", err.Error())
	}
}

// TestGenWorkspace_InvalidNameReturnsUsage verifies the --name validator
// catches path-separator characters before the output target is computed.
// A "/", "\", ":", or whitespace in --name would let a malicious or
// careless invocation write the .code-workspace outside the project root.
//
//nolint:paralleltest // t.Chdir + t.Setenv mutate process state.
func TestGenWorkspace_InvalidNameReturnsUsage(t *testing.T) {
	work := t.TempDir()
	home := filepath.Join(work, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", home)
	pinEnglish(t)
	t.Chdir(work)

	if err := os.WriteFile(filepath.Join(work, "workspace.toml"), []byte(codeWorkspaceTOML), 0o600); err != nil {
		t.Fatalf("write workspace.toml: %v", err)
	}

	cases := []string{"../escape", "with space", "name:colon", "."}
	for _, badName := range cases {
		t.Run(badName, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			cmd := gencli.NewCommand(&stdout, &stderr)
			cmd.SetArgs([]string{"workspace", "--name", badName})
			err := cmd.Execute()
			if err == nil {
				t.Fatalf("expected error for --name %q, got nil", badName)
			}
			if !errors.Is(err, clihelpers.ErrUsage) {
				t.Errorf("err = %v, want errors.Is clihelpers.ErrUsage", err)
			}
		})
	}
}

// TestGenWorkspace_HomeTraversalReachesUpward is the user-visible
// contract: a "~/.foo" entry must resolve to a relative path that
// traverses *upward* from the project root, not the literal "~" or an
// absolute "/home/...". The test sets HOME and project at parallel
// depths so the result is deterministic.
func TestGenWorkspace_HomeTraversalReachesUpward(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "ws", "myproject")
	home := filepath.Join(root, "home", "alice")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", home)
	pinEnglish(t)
	t.Chdir(project)

	const minimal = `[container]
service_name = "demo"
username = "alice"
image = "ubuntu"
image_version = "24.04"

[plugins]
enable = []

[code_workspace]
folders = [{ path = "~/.claude" }]
`
	if err := os.WriteFile(filepath.Join(project, "workspace.toml"), []byte(minimal), 0o600); err != nil {
		t.Fatalf("write workspace.toml: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := gencli.NewCommand(&stdout, &stderr)
	cmd.SetArgs([]string{"workspace"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gen workspace: %v\nstderr:\n%s", err, stderr.String())
	}

	target := filepath.Join(project, "myproject.code-workspace")
	raw, err := os.ReadFile(target) //nolint:gosec
	if err != nil {
		t.Fatalf("expected %s: %v", target, err)
	}
	// project = <root>/ws/myproject, ~/.claude = <root>/home/alice/.claude.
	// filepath.Rel = ../../home/alice/.claude.
	if !strings.Contains(string(raw), `"path": "../../home/alice/.claude"`) {
		t.Errorf("expected upward-traversing rel path; got:\n%s", string(raw))
	}
	// And the literal "~" must not survive into the output.
	if strings.Contains(string(raw), "~") {
		t.Errorf("output must not contain literal \"~\"; got:\n%s", string(raw))
	}
}

// TestGenWorkspace_NoCodeWorkspaceSectionWithFolderFlagSucceeds covers the
// "TOML has no [code_workspace] at all, but --folder is passed" path: the
// CLI must accept the flag-only input without complaint.
func TestGenWorkspace_NoCodeWorkspaceSectionWithFolderFlagSucceeds(t *testing.T) {
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
	cmd.SetArgs([]string{"workspace", "--folder", "."})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gen workspace: %v\nstderr:\n%s", err, stderr.String())
	}

	// Default name = filepath.Base(projectDir). projectDir is `work` (the
	// tempdir); its basename varies but the file must exist.
	entries, err := os.ReadDir(work)
	if err != nil {
		t.Fatalf("read project dir: %v", err)
	}
	found := false
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".code-workspace") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("no .code-workspace file in %s; entries: %v", work, entries)
	}
}

// TestGenWorkspaceErrorSentinels exposes the sentinels so other packages
// can match on classification. The test is a smoke check — if a future
// refactor drops the export, this file stops compiling.
func TestGenWorkspaceErrorSentinels(t *testing.T) {
	t.Parallel()
	_ = codeworkspace.ErrNoFolders
	_ = codeworkspace.ErrInvalidFolderPath
	_ = codeworkspace.ErrMissingHomeDir
}
