package plugincli_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	plugincli "github.com/sukekyo26/cocoon/internal/cli/plugin"
)

// runPinCmd is the pin-flavored peer of runCmd that scopes args to the pin
// subcommand. It avoids depending on PATH expansion or the full root CLI.
func runPinCmd(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := plugincli.NewCommand(&stdout, &stderr)
	cmd.SetArgs(append([]string{"pin"}, args...))
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

// seedWorkspace writes a minimal workspace.toml at <dir>/workspace.toml so
// config.Discover() finds it from cwd and pin --write has a target.
func seedWorkspace(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "workspace.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	return path
}

// Without --write, pin should print the snippet to stdout and leave the
// filesystem untouched. Locks the legacy behavior.
//
//nolint:paralleltest // t.Chdir mutates process cwd.
func TestPin_StdoutOnlyByDefault(t *testing.T) {
	withIsolatedHome(t)
	dir := t.TempDir()
	path := seedWorkspace(t, dir, "[plugins]\nenable = []\n")
	t.Chdir(dir)

	stdout, stderr, err := runPinCmd(t, "go", "1.23.4")
	if err != nil {
		t.Fatalf("pin: err=%v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, "[plugins.versions]") || !strings.Contains(stdout, `go = { pin = "1.23.4" }`) {
		t.Errorf("stdout missing inline pin line:\n%s", stdout)
	}
	body, rerr := os.ReadFile(path) //nolint:gosec // tmp under t.TempDir
	if rerr != nil {
		t.Fatalf("read: %v", rerr)
	}
	if string(body) != "[plugins]\nenable = []\n" {
		t.Errorf("workspace.toml was modified despite --write absent:\n%s", body)
	}
}

// --write happy path: workspace.toml gets a new [plugins.versions] section
// + inline-table line appended, stdout reports the path that was edited.
//
//nolint:paralleltest // t.Chdir mutates process cwd.
func TestPin_WriteAppendsInlineLineInPlace(t *testing.T) {
	withIsolatedHome(t)
	dir := t.TempDir()
	path := seedWorkspace(t, dir, "[plugins]\nenable = [\"go\"]\n")
	t.Chdir(dir)

	stdout, stderr, err := runPinCmd(t, "go", "1.23.4", "--write")
	if err != nil {
		t.Fatalf("pin --write: err=%v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, "Updated") || !strings.Contains(stdout, "[plugins.versions]") {
		t.Errorf("stdout missing Updated marker:\n%s", stdout)
	}
	got, rerr := os.ReadFile(path) //nolint:gosec // tmp under t.TempDir
	if rerr != nil {
		t.Fatalf("read: %v", rerr)
	}
	if !strings.Contains(string(got), "[plugins.versions]") || !strings.Contains(string(got), `go = { pin = "1.23.4" }`) {
		t.Errorf("workspace.toml missing new line:\n%s", got)
	}
}

// --write on a workspace.toml that already has [plugins.versions] with
// another id pinned must append the new line alongside (within the same
// section), not at EOF or under a new section.
//
//nolint:paralleltest // t.Chdir mutates process cwd.
func TestPin_WriteAppendsAlongsideExisting(t *testing.T) {
	withIsolatedHome(t)
	dir := t.TempDir()
	path := seedWorkspace(t, dir, `[plugins]
enable = ["go", "uv"]

[plugins.versions]
go = { pin = "1.23.4" }

[mounts]
host = "./src"
`)
	t.Chdir(dir)

	if _, stderr, err := runPinCmd(t, "uv", "0.5.7", "--write"); err != nil {
		t.Fatalf("pin --write: err=%v stderr=%s", err, stderr)
	}
	got, rerr := os.ReadFile(path) //nolint:gosec // tmp under t.TempDir
	if rerr != nil {
		t.Fatalf("read: %v", rerr)
	}
	body := string(got)
	// Section header must be unique, both ids must be present, [mounts] intact.
	if strings.Count(body, "[plugins.versions]") != 1 {
		t.Errorf("expected exactly one [plugins.versions] section header:\n%s", body)
	}
	goIdx := strings.Index(body, `go = { pin = "1.23.4" }`)
	uvIdx := strings.Index(body, `uv = { pin = "0.5.7" }`)
	mountsIdx := strings.Index(body, "[mounts]")
	if goIdx < 0 || uvIdx < 0 {
		t.Fatalf("expected both pins present:\n%s", body)
	}
	if goIdx >= uvIdx || uvIdx >= mountsIdx {
		t.Errorf("expected order: go < uv < mounts; got %d / %d / %d\n%s", goIdx, uvIdx, mountsIdx, body)
	}
	if !strings.Contains(body, "host = \"./src\"") {
		t.Errorf("[mounts] body lost in append:\n%s", body)
	}
}

// --write on a workspace.toml that already has a pin for the same id must
// replace the existing line rather than append a duplicate.
//
//nolint:paralleltest // t.Chdir mutates process cwd.
func TestPin_WriteReplacesExistingBlock(t *testing.T) {
	withIsolatedHome(t)
	dir := t.TempDir()
	path := seedWorkspace(t, dir, `[plugins]
enable = ["go"]

[plugins.versions]
go = { pin = "1.22.0" }
`)
	t.Chdir(dir)

	if _, stderr, err := runPinCmd(t, "go", "1.23.4", "--write"); err != nil {
		t.Fatalf("pin --write: err=%v stderr=%s", err, stderr)
	}
	got, rerr := os.ReadFile(path) //nolint:gosec // tmp under t.TempDir
	if rerr != nil {
		t.Fatalf("read: %v", rerr)
	}
	if strings.Contains(string(got), `pin = "1.22.0"`) {
		t.Errorf("old pin not replaced:\n%s", got)
	}
	if strings.Count(string(got), "go = {") != 1 {
		t.Errorf("expected exactly one `go = {` line:\n%s", got)
	}
}

// --write on a workspace.toml that still uses the legacy
// `[plugins.versions.<id>]` subsection format must refuse with ErrUsage and
// leave the file untouched. The error message must point at the migration.
//
//nolint:paralleltest // t.Chdir mutates process cwd.
func TestPin_WriteRefusesLegacySubsection(t *testing.T) {
	withIsolatedHome(t)
	dir := t.TempDir()
	original := `[plugins]
enable = ["go"]

[plugins.versions.go]
pin = "1.22.5"
`
	path := seedWorkspace(t, dir, original)
	t.Chdir(dir)

	_, stderr, err := runPinCmd(t, "go", "1.23.4", "--write")
	if !errors.Is(err, plugincli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
	if !strings.Contains(err.Error(), "subsection") {
		t.Errorf("err should mention subsection: %v\nstderr: %s", err, stderr)
	}
	got, rerr := os.ReadFile(path) //nolint:gosec // tmp under t.TempDir
	if rerr != nil {
		t.Fatalf("read: %v", rerr)
	}
	if string(got) != original {
		t.Errorf("workspace.toml modified despite refusal:\n--- got ---\n%s", got)
	}
}

// --write with no discoverable workspace.toml from cwd must surface ErrUsage
// with an actionable message rather than silently writing somewhere odd.
//
//nolint:paralleltest // t.Chdir mutates process cwd.
func TestPin_WriteRequiresWorkspace(t *testing.T) {
	withIsolatedHome(t)
	dir := t.TempDir()
	t.Chdir(dir)

	_, _, err := runPinCmd(t, "go", "1.23.4", "--write")
	if !errors.Is(err, plugincli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
	if !strings.Contains(err.Error(), "workspace.toml") {
		t.Errorf("err should mention workspace.toml: %v", err)
	}
}

// seedMethodPlugin drops a user-overlay plugin with two declared
// [install.methods] entries so pin --method has something to validate
// against. Install scripts are intentionally omitted — pin only reads
// plugin.toml; the loader's exclusivity check (install.sh forbidden
// when [install.methods] declared) is exercised by loader_test.go.
func seedMethodPlugin(t *testing.T, home string) {
	t.Helper()
	dir := filepath.Join(home, ".cocoon", "plugins", "multi-method")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := `[metadata]
name = "multi-method"
description = "fixture with two install methods"
url = "https://example.test"
default = false

[install]
requires_root = false
default_method = "official"

[install.methods.official]
description = "Install via official upstream installer"

[install.methods.binary]
description = "Direct binary download from releases"

[version]
version_capable = true
`
	if err := os.WriteFile(filepath.Join(dir, "plugin.toml"), []byte(body), 0o600); err != nil {
		t.Fatalf("write plugin.toml: %v", err)
	}
}

// --method on a plugin that declares methods, stdout (no --write): both
// the [plugins.methods] and [plugins.versions] snippets must appear, and
// the workspace.toml on disk stays untouched.
//
//nolint:paralleltest // t.Chdir + t.Setenv mutate process state.
func TestPin_MethodStdoutEmitsBothBlocks(t *testing.T) {
	home := withIsolatedHome(t)
	seedMethodPlugin(t, home)
	dir := t.TempDir()
	path := seedWorkspace(t, dir, "[plugins]\nenable = [\"multi-method\"]\n")
	t.Chdir(dir)

	stdout, stderr, err := runPinCmd(t, "multi-method", "1.2.3", "--method", "binary")
	if err != nil {
		t.Fatalf("pin: err=%v stderr=%s", err, stderr)
	}
	for _, want := range []string{
		"Under [plugins.methods]:",
		`multi-method = "binary"`,
		"Under [plugins.versions]:",
		`multi-method = { pin = "1.2.3" }`,
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
	body, rerr := os.ReadFile(path) //nolint:gosec // tmp under t.TempDir
	if rerr != nil {
		t.Fatalf("read: %v", rerr)
	}
	if string(body) != "[plugins]\nenable = [\"multi-method\"]\n" {
		t.Errorf("workspace.toml was modified despite --write absent:\n%s", body)
	}
}

// --method + --write: both [plugins.methods] and [plugins.versions]
// receive a new line in the same file. Order of insertion is methods
// after versions because UpsertMethodLine appends a new section at EOF
// when the section is absent — both grow as new sections in the seed
// workspace.
//
//nolint:paralleltest // t.Chdir + t.Setenv mutate process state.
func TestPin_MethodWriteUpdatesBothSections(t *testing.T) {
	home := withIsolatedHome(t)
	seedMethodPlugin(t, home)
	dir := t.TempDir()
	path := seedWorkspace(t, dir, "[plugins]\nenable = [\"multi-method\"]\n")
	t.Chdir(dir)

	stdout, stderr, err := runPinCmd(t, "multi-method", "1.2.3", "--method", "binary", "--write")
	if err != nil {
		t.Fatalf("pin --write --method: err=%v stderr=%s", err, stderr)
	}
	for _, want := range []string{
		"[plugins.versions] multi-method",
		`[plugins.methods] multi-method = "binary"`,
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
	got, rerr := os.ReadFile(path) //nolint:gosec // tmp under t.TempDir
	if rerr != nil {
		t.Fatalf("read: %v", rerr)
	}
	body := string(got)
	for _, want := range []string{
		"[plugins.versions]",
		`multi-method = { pin = "1.2.3" }`,
		"[plugins.methods]",
		`multi-method = "binary"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("workspace.toml missing %q:\n%s", want, body)
		}
	}
}

// --method with a method name the plugin does not declare: ErrUsage and
// the error must list the declared methods so the user can fix the typo
// without grepping plugin.toml.
//
//nolint:paralleltest // t.Chdir + t.Setenv mutate process state.
func TestPin_MethodUnknownNameFails(t *testing.T) {
	home := withIsolatedHome(t)
	seedMethodPlugin(t, home)
	dir := t.TempDir()
	seedWorkspace(t, dir, "[plugins]\nenable = [\"multi-method\"]\n")
	t.Chdir(dir)

	_, _, err := runPinCmd(t, "multi-method", "1.2.3", "--method", "nonexistent")
	if !errors.Is(err, plugincli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
	if !strings.Contains(err.Error(), "binary, official") {
		t.Errorf("err should list declared methods, got: %v", err)
	}
}

// --method on a user-overlay plugin whose plugin.toml declares no
// [install.methods] section at all: validateMethodForPin loads via
// loadPluginFromLayer (strict unmarshal only — the loader's catalog-wide
// `[install.methods]` enforcement is bypassed for user overlays at this
// call site), so an empty Methods map reaches the validator. The user
// must see an actionable ErrUsage that explains --method is meaningless
// here rather than the misleading "declared: " empty-list message the
// next branch would produce.
//
//nolint:paralleltest // t.Chdir + t.Setenv mutate process state.
func TestPin_MethodFailsWhenPluginDeclaresNoMethods(t *testing.T) {
	home := withIsolatedHome(t)
	pluginDir := filepath.Join(home, ".cocoon", "plugins", "no-methods")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := `[metadata]
name = "no-methods"
description = "fixture without install.methods"
url = "https://example.test"
default = false

[install]
requires_root = false

[version]
version_capable = false
`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.toml"), []byte(body), 0o600); err != nil {
		t.Fatalf("write plugin.toml: %v", err)
	}
	dir := t.TempDir()
	seedWorkspace(t, dir, "[plugins]\nenable = [\"no-methods\"]\n")
	t.Chdir(dir)

	_, _, err := runPinCmd(t, "no-methods", "1.2.3", "--method", "binary")
	if !errors.Is(err, plugincli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
	if !strings.Contains(err.Error(), "declares no [install.methods]") {
		t.Errorf("err should explain --method needs declared methods, got: %v", err)
	}
	if strings.Contains(err.Error(), "declared: )") {
		t.Errorf("err should not show empty declared list, got: %v", err)
	}
}

// --method with a name that's not in the plugin's declared methods on a
// catalog plugin with a single declared method (e.g. `go` declares only
// "archive"): the error lists the declared keys so typos are easy to
// fix. Catalog-wide enforcement of [install.methods] (loader rule) means
// the legacy "no methods declared at all" scenario is unreachable for
// catalog plugins — the loader rejects them first.
//
//nolint:paralleltest // t.Chdir + t.Setenv mutate process state.
func TestPin_MethodMismatchedNameListsDeclared(t *testing.T) {
	withIsolatedHome(t)
	dir := t.TempDir()
	seedWorkspace(t, dir, "[plugins]\nenable = [\"go\"]\n")
	t.Chdir(dir)

	// `go` is an embedded plugin whose only declared method is "archive".
	// Asking for --method binary should fail with a typo-friendly error
	// that names the declared keys.
	_, _, err := runPinCmd(t, "go", "1.23.4", "--method", "binary")
	if !errors.Is(err, plugincli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
	if !strings.Contains(err.Error(), `no method "binary"`) {
		t.Errorf("err should name the missing method, got: %v", err)
	}
	if !strings.Contains(err.Error(), "declared: archive") {
		t.Errorf("err should list declared methods, got: %v", err)
	}
}
