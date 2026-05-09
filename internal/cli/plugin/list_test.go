package plugincli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	plugincli "github.com/sukekyo26/cocoon/internal/cli/plugin"
)

// withIsolatedHome points HOME at a fresh tempdir for the test's lifetime so
// ~/.cocoon/plugins is empty (the user-overlay layer starts vacant).
func withIsolatedHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

//nolint:paralleltest // t.Setenv on HOME forbids t.Parallel.
func TestPluginList_DefaultsToEmbeddedSources(t *testing.T) {
	withIsolatedHome(t)

	var stdout, stderr bytes.Buffer
	cmd := plugincli.NewCommand(&stdout, &stderr)
	cmd.SetArgs([]string{"list"})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list: err=%v stderr=%s", err, stderr.String())
	}

	out := stdout.String()
	for _, want := range []string{"ID", "SOURCE", "DEFAULT", "DESCRIPTION", "embedded"} {
		if !strings.Contains(out, want) {
			t.Errorf("list output missing %q\nstdout:\n%s", want, out)
		}
	}
	// At least one well-known embedded plugin should appear under "embedded".
	if !strings.Contains(out, "go") || !strings.Contains(out, "uv") {
		t.Errorf("expected go / uv plugins in list output, got:\n%s", out)
	}
}

//nolint:paralleltest // t.Setenv on HOME forbids t.Parallel.
func TestPluginList_UserOverlayShowsAsUserSource(t *testing.T) {
	home := withIsolatedHome(t)
	// Drop a user overlay for "uv" so it should now report source=user.
	dir := filepath.Join(home, ".cocoon", "plugins", "uv")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.toml"), []byte(`[metadata]
name = "uv (custom)"
description = "user overlay"
default = false

[install]
requires_root = false

[version]
version_capable = false
`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := plugincli.NewCommand(&stdout, &stderr)
	cmd.SetArgs([]string{"list", "--source", "user"})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list: err=%v stderr=%s", err, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "uv") {
		t.Errorf("uv missing from --source=user list\n%s", out)
	}
	if strings.Contains(out, "embedded") {
		t.Errorf("--source=user should hide embedded rows, got:\n%s", out)
	}
}
