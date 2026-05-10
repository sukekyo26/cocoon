package plugincli_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	plugincli "github.com/sukekyo26/cocoon/internal/cli/plugin"
)

//nolint:paralleltest // t.Setenv on HOME forbids t.Parallel.
func TestPluginAdd_RejectsUnknownSubcommand(t *testing.T) {
	withIsolatedHome(t)
	var stdout, stderr bytes.Buffer
	cmd := plugincli.NewCommand(&stdout, &stderr)
	cmd.SetArgs([]string{"add", "uv"})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("plugin add should be unknown after removal; stdout=%s", stdout.String())
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
