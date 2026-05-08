package verifyartifactscli

import (
	"sort"
	"testing"

	"github.com/sukekyo26/cocoon/internal/config"
)

func TestAsInt(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   any
		want int
	}{
		{"int", 7, 7},
		{"int64", int64(42), 42},
		{"float64_truncates", 3.9, 3},
		{"string_falls_through", "42", 0},
		{"nil_zero", nil, 0},
	}
	for _, c := range cases {
		if got := asInt(c.in); got != c.want {
			t.Errorf("asInt(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestAsFloat(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   any
		want float64
	}{
		{"float64", 1.5, 1.5},
		{"int", 7, 7.0},
		{"int64", int64(42), 42.0},
		{"string_falls_through", "1.5", 0},
		{"nil_zero", nil, 0},
	}
	for _, c := range cases {
		if got := asFloat(c.in); got != c.want {
			t.Errorf("asFloat(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestEnvAsList(t *testing.T) {
	t.Parallel()
	t.Run("list_form", func(t *testing.T) {
		t.Parallel()
		got := envAsList([]any{"FOO=1", "BAR=2"})
		sort.Strings(got)
		if got[0] != "BAR=2" || got[1] != "FOO=1" {
			t.Errorf("got %v", got)
		}
	})
	t.Run("map_form", func(t *testing.T) {
		t.Parallel()
		got := envAsList(map[string]any{"FOO": "1", "BAR": 2})
		sort.Strings(got)
		if got[0] != "BAR=2" || got[1] != "FOO=1" {
			t.Errorf("got %v", got)
		}
	})
	t.Run("unknown_returns_nil", func(t *testing.T) {
		t.Parallel()
		if got := envAsList("string"); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})
	t.Run("nil_returns_nil", func(t *testing.T) {
		t.Parallel()
		if got := envAsList(nil); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})
}

func TestComposeHasPort(t *testing.T) {
	t.Parallel()

	shortEntries := []any{"3000:3000", "127.0.0.1:5432:5432/tcp"}
	longEntries := []any{
		map[string]any{
			"target":    8080,
			"published": "8080",
			"protocol":  "tcp",
			"mode":      "host",
		},
	}

	cases := []struct {
		name    string
		entries []any
		want    config.ComposePort
		match   bool
	}{
		{
			name:    "short_match",
			entries: shortEntries,
			want:    config.ComposePort{Short: "3000:3000"},
			match:   true,
		},
		{
			name:    "short_no_match",
			entries: shortEntries,
			want:    config.ComposePort{Short: "9000:9000"},
			match:   false,
		},
		{
			name:    "short_skips_non_string_entries",
			entries: []any{42, "3000:3000"},
			want:    config.ComposePort{Short: "3000:3000"},
			match:   true,
		},
		{
			name:    "long_match_subset_keys",
			entries: longEntries,
			want: config.ComposePort{Long: map[string]any{
				"target":    8080,
				"published": "8080",
			}},
			match: true,
		},
		{
			name:    "long_skips_non_map_entries",
			entries: append([]any{"3000:3000"}, longEntries...),
			want: config.ComposePort{Long: map[string]any{
				"target": 8080,
			}},
			match: true,
		},
		{
			name:    "long_value_mismatch",
			entries: longEntries,
			want: config.ComposePort{Long: map[string]any{
				"target":   8080,
				"protocol": "udp",
			}},
			match: false,
		},
		{
			name:    "long_missing_key",
			entries: longEntries,
			want: config.ComposePort{Long: map[string]any{
				"host_ip": "127.0.0.1",
			}},
			match: false,
		},
		{
			name:    "long_no_entries",
			entries: nil,
			want:    config.ComposePort{Long: map[string]any{"target": 1}},
			match:   false,
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := composeHasPort(c.entries, c.want); got != c.match {
				t.Errorf("composeHasPort(%v, %+v) = %v, want %v", c.entries, c.want, got, c.match)
			}
		})
	}
}

func TestSameInts(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		a, b []int
		want bool
	}{
		{"both_empty", nil, nil, true},
		{"order_matters", []int{1, 2}, []int{2, 1}, false},
		{"length_diff", []int{1, 2}, []int{1}, false},
		{"identical", []int{3, 1, 2}, []int{3, 1, 2}, true},
	}
	for _, c := range cases {
		if got := sameInts(c.a, c.b); got != c.want {
			t.Errorf("%s: sameInts(%v, %v) = %v, want %v", c.name, c.a, c.b, got, c.want)
		}
	}
}
