package release_test

import (
	"testing"

	"github.com/sukekyo26/cocoon/internal/release"
)

func TestCompare(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		a, b string
		want int
	}{
		{"equal_plain", "0.1.0", "0.1.0", 0},
		{"equal_v_prefix", "v0.1.0", "0.1.0", 0},
		{"patch_lt", "0.1.0", "0.1.1", -1},
		{"patch_gt", "0.1.2", "0.1.1", 1},
		{"minor_lt", "0.1.9", "0.2.0", -1},
		{"major_gt", "2.0.0", "1.99.99", 1},
		{"prerelease_lt_release", "1.0.0-rc1", "1.0.0", -1},
		{"release_gt_prerelease", "1.0.0", "1.0.0-rc1", 1},
		{"prerelease_alpha_lt_rc", "1.0.0-alpha", "1.0.0-rc1", -1},
		{"diff_length_zero_pad", "1.0", "1.0.0", 0},
		{"diff_length_lt", "1.0", "1.0.1", -1},
		{"malformed_treated_zero", "abc", "0.0.0", 0},
		{"whitespace_trim", "  v1.2.3  ", "1.2.3", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := release.Compare(tc.a, tc.b); got != tc.want {
				t.Errorf("Compare(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}
