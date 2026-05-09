package shellx_test

import (
	"testing"

	"github.com/sukekyo26/cocoon/internal/generate/shellx"
)

func TestShellQuote(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"", "''"},
		{"hello", "hello"},
		{"hello_world", "hello_world"},
		{"path/to/file-1.0", "path/to/file-1.0"},
		{"a@b.com", "a@b.com"},
		{"100%", "100%"},
		{"hello world", "'hello world'"},
		{"with 'quote'", `'with '"'"'quote'"'"''`},
		{"$var", "'$var'"},
		{"a&b", "'a&b'"},
		{"line1\nline2", "'line1\nline2'"},
		{"#comment", "'#comment'"},
	}
	for _, tc := range cases {
		got := shellx.ShellQuote(tc.in)
		if got != tc.want {
			t.Errorf("ShellQuote(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFishQuote(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"", "''"},
		{"hello", "'hello'"},
		{"hello world", "'hello world'"},
		{"$var", "'$var'"}, // $ does not expand inside single quotes in fish
		{"a&b", "'a&b'"},
		{"line1\nline2", "'line1\nline2'"},
		{"with 'quote'", `'with \'quote\''`},
		{`back\slash`, `'back\\slash'`},
		{`mixed '\and"`, `'mixed \'\\and"'`},
		{"unicode 日本語", "'unicode 日本語'"},
	}
	for _, tc := range cases {
		got := shellx.FishQuote(tc.in)
		if got != tc.want {
			t.Errorf("FishQuote(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
