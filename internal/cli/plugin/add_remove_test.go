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

//nolint:paralleltest // t.Setenv on HOME forbids t.Parallel.
func TestPluginAdd_CopiesEmbeddedToUserOverlay(t *testing.T) {
	home := withIsolatedHome(t)

	var stdout, stderr bytes.Buffer
	cmd := plugincli.NewCommand(&stdout, &stderr)
	cmd.SetArgs([]string{"add", "uv"})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("add: err=%v stderr=%s", err, stderr.String())
	}

	dst := filepath.Join(home, ".cocoon", "plugins", "uv")
	if _, err := os.Stat(filepath.Join(dst, "plugin.toml")); err != nil {
		t.Fatalf("expected plugin.toml at %s: %v", dst, err)
	}
	if _, err := os.Stat(filepath.Join(dst, "install.sh")); err != nil {
		t.Fatalf("expected install.sh at %s: %v", dst, err)
	}
	info, err := os.Stat(filepath.Join(dst, "install.sh"))
	if err != nil {
		t.Fatalf("stat install.sh: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Errorf("install.sh perm: got %o, want 0755", got)
	}
}

//nolint:paralleltest // t.Setenv on HOME forbids t.Parallel.
func TestPluginAdd_RefusesExistingWithoutForce(t *testing.T) {
	home := withIsolatedHome(t)
	dst := filepath.Join(home, ".cocoon", "plugins", "uv")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := plugincli.NewCommand(&stdout, &stderr)
	cmd.SetArgs([]string{"add", "uv"})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("add expected to fail when overlay exists; stdout=%s", stdout.String())
	}
	if !errors.Is(err, plugincli.ErrUsage) {
		t.Errorf("expected ErrUsage, got %v", err)
	}
}

//nolint:paralleltest // t.Setenv on HOME forbids t.Parallel.
func TestPluginAdd_ForceOverwritesExisting(t *testing.T) {
	home := withIsolatedHome(t)
	dst := filepath.Join(home, ".cocoon", "plugins", "uv")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dst, "stale.txt"), []byte("old"), 0o600); err != nil {
		t.Fatalf("seed stale: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := plugincli.NewCommand(&stdout, &stderr)
	cmd.SetArgs([]string{"add", "uv", "--force"})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("add --force: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "stale.txt")); err == nil {
		t.Errorf("stale.txt should have been swept by Materialize on --force")
	}
}

//nolint:paralleltest // t.Setenv on HOME forbids t.Parallel.
func TestPluginAdd_RejectsUnknownID(t *testing.T) {
	withIsolatedHome(t)
	var stdout, stderr bytes.Buffer
	cmd := plugincli.NewCommand(&stdout, &stderr)
	cmd.SetArgs([]string{"add", "no-such-plugin"})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("add expected to fail for unknown id; stdout=%s", stdout.String())
	}
	if !errors.Is(err, plugincli.ErrUsage) {
		t.Errorf("expected ErrUsage, got %v", err)
	}
}

//nolint:paralleltest // t.Setenv on HOME forbids t.Parallel.
func TestPluginRemove_RejectsUnknownSubcommand(t *testing.T) {
	withIsolatedHome(t)
	var stdout, stderr bytes.Buffer
	cmd := plugincli.NewCommand(&stdout, &stderr)
	cmd.SetArgs([]string{"remove", "uv", "--scope", "user"})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("plugin remove should be unknown after removal; stdout=%s", stdout.String())
	}
	if !errors.Is(err, plugincli.ErrUsage) {
		t.Errorf("expected ErrUsage, got %v", err)
	}
}

//nolint:paralleltest // t.Setenv on HOME forbids t.Parallel.
func TestPluginShow_PrintsResolvedManifest(t *testing.T) {
	withIsolatedHome(t)
	var stdout, stderr bytes.Buffer
	cmd := plugincli.NewCommand(&stdout, &stderr)
	cmd.SetArgs([]string{"show", "uv"})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("show: err=%v stderr=%s", err, stderr.String())
	}
	out := stdout.String()
	for _, w := range []string{"id:           uv", "source:       embedded"} {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in:\n%s", w, out)
		}
	}
}

//nolint:paralleltest // t.Setenv on HOME forbids t.Parallel.
func TestPluginPin_PrintsTomlBlock(t *testing.T) {
	withIsolatedHome(t)
	var stdout, stderr bytes.Buffer
	cmd := plugincli.NewCommand(&stdout, &stderr)
	cmd.SetArgs([]string{"pin", "uv", "0.5.7", "--amd64-checksum", "abc123"})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pin: err=%v stderr=%s", err, stderr.String())
	}
	out := stdout.String()
	for _, w := range []string{
		"[plugins.versions.uv]",
		`pin = "0.5.7"`,
		`checksum_amd64 = "abc123"`,
	} {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in:\n%s", w, out)
		}
	}
}
