//nolint:testpackage // exercises unexported helper coercions.
package configcli

import (
	"reflect"
	"testing"
)

func TestAsMap(t *testing.T) {
	t.Parallel()
	if got := asMap(map[string]any{"k": 1}); !reflect.DeepEqual(got, map[string]any{"k": 1}) {
		t.Errorf("asMap on map = %v", got)
	}
	if got := asMap("not a map"); len(got) != 0 {
		t.Errorf("asMap fallback = %v, want empty", got)
	}
	if got := asMap(nil); len(got) != 0 {
		t.Errorf("asMap nil fallback = %v", got)
	}
}

func TestAsSliceAny(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   any
		want []any
	}{
		{"slice_any_passthrough", []any{1, "x"}, []any{1, "x"}},
		{"strings", []string{"a", "b"}, []any{"a", "b"}},
		{"int64", []int64{1, 2}, []any{int64(1), int64(2)}},
		{"bools", []bool{true, false}, []any{true, false}},
		{"unknown_returns_nil", "scalar", nil},
		{"nil_returns_nil", nil, nil},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := asSliceAny(c.in)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("asSliceAny(%v) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestAsString(t *testing.T) {
	t.Parallel()
	if got := asString("hello", "fb"); got != "hello" {
		t.Errorf("string passthrough: %q", got)
	}
	if got := asString(42, "fb"); got != "fb" {
		t.Errorf("non-string fallback: %q", got)
	}
	if got := asString(nil, "fb"); got != "fb" {
		t.Errorf("nil fallback: %q", got)
	}
}

func TestAsBool(t *testing.T) {
	t.Parallel()
	if !asBool(true, false) {
		t.Errorf("true passthrough")
	}
	if asBool("yes", false) {
		t.Errorf("string non-fallback")
	}
}

func TestAsInt(t *testing.T) {
	t.Parallel()
	if got := asInt(int64(7), 0); got != 7 {
		t.Errorf("int64 = %d", got)
	}
	if got := asInt(7, 0); got != 7 {
		t.Errorf("int = %d", got)
	}
	if got := asInt("7", 99); got != 99 {
		t.Errorf("string fallback = %d", got)
	}
}

func TestScalarString(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"string", "hello", "hello"},
		{"bool_true", true, "true"},
		{"bool_false", false, "false"},
		{"int64", int64(42), "42"},
		{"int", 7, "7"},
		{"float64", 1.5, "1.5"},
		{"nil_uses_fmt", nil, "<nil>"},
		{"long_form_short_collapse", map[string]any{
			"target":    int64(3000),
			"published": int64(3000),
		}, "3000:3000"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := scalarString(c.in); got != c.want {
				t.Errorf("scalarString(%v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestFormatPortMap(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   map[string]any
		want string
	}{
		{
			name: "short_form_with_host_ip_and_protocol",
			in: map[string]any{
				"target":    int64(5432),
				"published": int64(5432),
				"host_ip":   "127.0.0.1",
				"protocol":  "tcp",
			},
			want: "127.0.0.1:5432:5432/tcp",
		},
		{
			name: "short_form_basic",
			in: map[string]any{
				"target":    3000,
				"published": 3001,
			},
			want: "3001:3000",
		},
		{
			name: "key_value_fallback_when_missing_published",
			in: map[string]any{
				"target": int64(8080),
				"mode":   "host",
			},
			want: "target=8080 mode=host",
		},
		{
			name: "key_value_fallback_when_missing_target",
			in: map[string]any{
				"published": int64(80),
			},
			want: "published=80",
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := formatPortMap(c.in); got != c.want {
				t.Errorf("formatPortMap(%v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestPortInt(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   any
		want int
	}{
		{"int", 42, 42},
		{"int64", int64(7), 7},
		{"float64_integer_value", 3.0, 3},
		{"float64_non_integer_zero", 3.9, 0},
		{"string_zero", "8080", 0},
		{"nil_zero", nil, 0},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := portInt(c.in); got != c.want {
				t.Errorf("portInt(%v) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}
