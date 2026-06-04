package plugin_test

import (
	"testing"

	"github.com/sukekyo26/cocoon/internal/plugin"
)

// TestFormatEnableEntry pins the enable-array element formatter by category:
// an exact "=<version>" spec drops the leading "=", "latest" (and the empty
// spec) collapse to "<id>=latest", and a "v"-prefixed version round-trips.
func TestFormatEnableEntry(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, id, spec, want string
	}{
		{"exact", "go", "=1.23.4", "go=1.23.4"},
		{"exact_v_prefix", "zig", "=v0.14.0", "zig=v0.14.0"},
		{"latest", "node", "latest", "node=latest"},
		{"empty_spec_is_latest", "uv", "", "uv=latest"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := plugin.FormatEnableEntry(tc.id, tc.spec); got != tc.want {
				t.Errorf("FormatEnableEntry(%q, %q) = %q, want %q", tc.id, tc.spec, got, tc.want)
			}
		})
	}
}
