package plugin_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sukekyo26/cocoon/internal/plugin"
)

func TestCheckConflicts(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		plugins map[string]*plugin.Plugin
		wantErr bool
		wantMsg string // substring of err.Error(), asserted when wantErr
	}{
		{
			name: "no_conflict",
			plugins: map[string]*plugin.Plugin{
				"a": {Metadata: plugin.Metadata{Name: "A"}},
				"b": {Metadata: plugin.Metadata{Name: "B"}},
			},
		},
		{
			name: "single_conflict",
			plugins: map[string]*plugin.Plugin{
				"a": {Metadata: plugin.Metadata{Name: "A", Conflicts: []string{"b"}}},
				"b": {Metadata: plugin.Metadata{Name: "B"}},
			},
			wantErr: true,
			wantMsg: "'A' (a) conflicts with 'B' (b)",
		},
		{
			name: "mutual_conflict_reports_sorted_first",
			plugins: map[string]*plugin.Plugin{
				"a": {Metadata: plugin.Metadata{Name: "A", Conflicts: []string{"b"}}},
				"b": {Metadata: plugin.Metadata{Name: "B", Conflicts: []string{"a"}}},
			},
			wantErr: true,
			wantMsg: "'A' (a) conflicts with 'B' (b)",
		},
		{
			name: "self_conflict_is_ignored",
			plugins: map[string]*plugin.Plugin{
				"a": {Metadata: plugin.Metadata{Name: "A", Conflicts: []string{"a"}}},
			},
		},
		{
			name: "self_conflict_plus_real_conflict",
			plugins: map[string]*plugin.Plugin{
				"a": {Metadata: plugin.Metadata{Name: "A", Conflicts: []string{"a", "b"}}},
				"b": {Metadata: plugin.Metadata{Name: "B"}},
			},
			wantErr: true,
			wantMsg: "'A' (a) conflicts with 'B' (b)",
		},
		{
			name: "conflict_partner_not_enabled",
			plugins: map[string]*plugin.Plugin{
				"a": {Metadata: plugin.Metadata{Name: "A", Conflicts: []string{"b"}}},
			},
		},
		{
			name: "empty_name_falls_back_to_id",
			plugins: map[string]*plugin.Plugin{
				"a": {Metadata: plugin.Metadata{Conflicts: []string{"b"}}},
				"b": {Metadata: plugin.Metadata{}},
			},
			wantErr: true,
			wantMsg: "'a' (a) conflicts with 'b' (b)",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := plugin.CheckConflicts(tc.plugins)
			if !tc.wantErr {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.ErrorIs(t, err, plugin.ErrConflict)
			require.Contains(t, err.Error(), tc.wantMsg)
		})
	}
}

// TestCheckConflicts_Deterministic pins the sorted-scan contract: with two
// independent conflicting pairs, Go's randomised map iteration would let
// either pair surface first. Scanning plugin ids in sorted order must
// report the conflict declared by the lexicographically-first plugin on
// every run — repeating the call exercises many iteration orders so a
// regression that drops the sort is caught.
func TestCheckConflicts_Deterministic(t *testing.T) {
	t.Parallel()

	plugins := map[string]*plugin.Plugin{
		"a": {Metadata: plugin.Metadata{Name: "A"}},
		"b": {Metadata: plugin.Metadata{Name: "B", Conflicts: []string{"a"}}},
		"c": {Metadata: plugin.Metadata{Name: "C"}},
		"d": {Metadata: plugin.Metadata{Name: "D", Conflicts: []string{"c"}}},
	}
	const want = "'B' (b) conflicts with 'A' (a)"

	const iterations = 64
	for range iterations {
		err := plugin.CheckConflicts(plugins)
		require.Error(t, err)
		require.ErrorIs(t, err, plugin.ErrConflict)
		require.Contains(t, err.Error(), want)
	}
}
