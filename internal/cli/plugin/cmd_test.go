package plugincli_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	plugincli "github.com/sukekyo26/cocoon/internal/cli/plugin"
)

// TestPluginAdd_NoLongerRegistered locks in the removal of the
// `cocoon plugin add` subcommand: the parent command must surface
// it as an unknown subcommand (ErrUsage) instead of silently
// accepting it. The name is deliberate — "no longer registered"
// signals "used to exist, has been removed", so a future reader
// does not assume `add` is supported behaviour.
//
//nolint:paralleltest // t.Setenv on HOME forbids t.Parallel.
func TestPluginAdd_NoLongerRegistered(t *testing.T) {
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
	if !errors.Is(err, clihelpers.ErrUsage) {
		t.Errorf("expected ErrUsage, got %v", err)
	}
	if !strings.Contains(err.Error(), "unknown subcommand") {
		t.Errorf("expected unknown-subcommand path, got %v", err)
	}
}

// TestPluginRemove_NoLongerRegistered locks in the removal of the
// `cocoon plugin remove` subcommand. Same shape as the add test
// above; the name mirrors it. The args intentionally omit the old
// `--scope` flag so cobra reaches the unknown-subcommand path
// (rejectUnknownSubcommand) instead of failing earlier on an
// unrecognised flag at the parent `plugin` level.
//
//nolint:paralleltest // t.Setenv on HOME forbids t.Parallel.
func TestPluginRemove_NoLongerRegistered(t *testing.T) {
	withIsolatedHome(t)
	var stdout, stderr bytes.Buffer
	cmd := plugincli.NewCommand(&stdout, &stderr)
	cmd.SetArgs([]string{"remove", "uv"})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("plugin remove should be unknown after removal; stdout=%s", stdout.String())
	}
	if !errors.Is(err, clihelpers.ErrUsage) {
		t.Errorf("expected ErrUsage, got %v", err)
	}
	if !strings.Contains(err.Error(), "unknown subcommand") {
		t.Errorf("expected unknown-subcommand path, got %v", err)
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
func TestPluginPin_PrintsConstraintLine(t *testing.T) {
	withIsolatedHome(t)
	var stdout, stderr bytes.Buffer
	cmd := plugincli.NewCommand(&stdout, &stderr)
	cmd.SetArgs([]string{"pin", "uv", "0.5.7"})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pin: err=%v stderr=%s", err, stderr.String())
	}
	out := stdout.String()
	for _, w := range []string{
		"[plugins].enable",
		`"uv=0.5.7"`,
	} {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in:\n%s", w, out)
		}
	}
}
