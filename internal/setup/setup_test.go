//nolint:testpackage // white-box tests for unexported helpers in internal/setup.
package setup

import (
	"errors"
	"reflect"
	"testing"
)

type mockTranslator struct{}

func (mockTranslator) Msg(key string, args ...any) string { return key }

func TestParsePorts(t *testing.T) {
	t.Parallel()
	tr := mockTranslator{}
	cases := []struct {
		name    string
		input   string
		want    []any
		wantErr bool
	}{
		{"empty", "", nil, false},
		{"single", "8080", []any{"8080:8080"}, false},
		{"multi", "80, 443, 8080", []any{"80:80", "443:443", "8080:8080"}, false},
		{"trailing-comma", "80,,", []any{"80:80"}, false},
		{"zero-invalid", "0", nil, true},
		{"too-large", "65536", nil, true},
		{"non-numeric", "abc", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parsePorts(tc.input, tr)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestValidateServiceName(t *testing.T) {
	t.Parallel()
	tr := mockTranslator{}
	good := []string{"dev", "web", "api-1", "service_2", "abc123"}
	bad := []string{"", "1abc", "Dev", "_svc", "-svc", "svc!"}
	for _, s := range good {
		if err := validateServiceName(s, tr); err != nil {
			t.Errorf("expected %q to be valid, got %v", s, err)
		}
	}
	for _, s := range bad {
		if err := validateServiceName(s, tr); err == nil {
			t.Errorf("expected %q to be invalid", s)
		}
	}
}

func TestValidateUsername(t *testing.T) {
	t.Parallel()
	tr := mockTranslator{}
	good := []string{"alice", "_bob", "user-1", "user_2"}
	bad := []string{"", "1user", "Alice", "-user", "user!"}
	for _, s := range good {
		if err := validateUsername(s, tr); err != nil {
			t.Errorf("expected %q to be valid, got %v", s, err)
		}
	}
	for _, s := range bad {
		if err := validateUsername(s, tr); err == nil {
			t.Errorf("expected %q to be invalid", s)
		}
	}
}

func TestRunRequiresOptions(t *testing.T) {
	t.Parallel()
	if err := Run(Options{}); !errors.Is(err, ErrConfig) {
		t.Fatalf("expected ErrConfig, got %v", err)
	}
}

func TestFormatPortValue(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"string_quoted", "3000:3000", `"3000:3000"`},
		{"int", 3000, "3000"},
		{"int64", int64(8080), "8080"},
		{"fallback_uses_fmt", true, "true"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := formatPortValue(c.in); got != c.want {
				t.Errorf("formatPortValue(%v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestFormatPortEntry(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"string", "3000:3000", `"3000:3000"`},
		{"legacy_int_auto_migrates", 3000, `"3000:3000"`},
		{"legacy_int64_auto_migrates", int64(8080), `"8080:8080"`},
		{
			name: "long_form_emits_compose_keys_in_order",
			in: map[string]any{
				"target":    3000,
				"published": "3000",
				"protocol":  "tcp",
			},
			want: `{ target = 3000, published = "3000", protocol = "tcp" }`,
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := formatPortEntry(c.in); got != c.want {
				t.Errorf("formatPortEntry(%v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestFormatPortsSlice(t *testing.T) {
	t.Parallel()
	if got := formatPortsSlice(nil); got != "[]" {
		t.Errorf("empty: got %q", got)
	}
	got := formatPortsSlice([]any{"3000:3000", 8080})
	want := `["3000:3000", "8080:8080"]`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
