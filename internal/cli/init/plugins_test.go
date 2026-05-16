//nolint:testpackage // exercises unexported plugin catalog helpers.
package initcli

import (
	"errors"
	"testing"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

// TestFilterPluginIDs pins the three shapes the picker relies on so the
// same id never surfaces in both the default list and the excluded list.
func TestFilterPluginIDs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		in        []string
		excludeID string
		want      []string
	}{
		{"no_exclude_returns_input", []string{"a", "b", "c"}, "", []string{"a", "b", "c"}},
		{"excludes_present_id", []string{"a", "rust", "c"}, "rust", []string{"a", "c"}},
		{"absent_id_is_noop", []string{"a", "b"}, "rust", []string{"a", "b"}},
		{"empty_input", []string{}, "rust", []string{}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := filterPluginIDs(tc.in, tc.excludeID)
			if len(got) != len(tc.want) {
				t.Fatalf("len=%d, want %d, got=%v", len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("at %d: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestValidatePluginConflicts(t *testing.T) {
	t.Parallel()
	// The embedded catalog currently ships no plugins with declared
	// conflicts (custom-ps1 was the only one and has been removed). Build
	// a synthetic pair in-memory so the validator's symmetric-detection
	// logic still has a regression guard.
	plugins := map[string]*plugin.Plugin{
		"alpha": {Metadata: plugin.Metadata{Name: "Alpha", Conflicts: []string{"beta"}}},
		"beta":  {Metadata: plugin.Metadata{Name: "Beta", Conflicts: []string{"alpha"}}},
		"gamma": {Metadata: plugin.Metadata{Name: "Gamma"}},
	}

	err := validatePluginConflicts(plugins, []string{"alpha", "beta"})
	if !errors.Is(err, clihelpers.ErrUsage) {
		t.Errorf("alpha+beta should be ErrUsage, got %v", err)
	}

	// Either alone is fine.
	if err := validatePluginConflicts(plugins, []string{"alpha"}); err != nil {
		t.Errorf("alpha alone should be ok, got %v", err)
	}
	if err := validatePluginConflicts(plugins, []string{"beta"}); err != nil {
		t.Errorf("beta alone should be ok, got %v", err)
	}

	// Empty list is trivially ok.
	if err := validatePluginConflicts(plugins, nil); err != nil {
		t.Errorf("nil enabled should be ok, got %v", err)
	}

	// Disjoint plugins do not conflict.
	if err := validatePluginConflicts(plugins, []string{"alpha", "gamma"}); err != nil {
		t.Errorf("disjoint plugins should be ok, got %v", err)
	}
}

func TestDefaultPluginIDs(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)

	got := defaultPluginIDs(plugins)
	// The current catalog ships no `default = true` plugin. If you add one,
	// update this assertion alongside the catalog change.
	if len(got) != 0 {
		t.Errorf("default plugin ids: got %v, want none", got)
	}
}
