package devcontainerjson

import "testing"

func TestNormalizeKey(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   any
		want any
	}{
		{"int_passthrough", 7, 7},
		{"int64_collapses", int64(7), 7},
		{"float64_integral_collapses", 7.0, 7},
		{"float64_fractional_passthrough", 1.5, 1.5},
		{"string_passthrough", "hello", "hello"},
		{"bool_passthrough", true, true},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeKey(c.in)
			if got != c.want {
				t.Errorf("normalizeKey(%v) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}
