package shellx_test

import (
	"strings"
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

func TestPosixExportValue(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, in, want string
	}{
		{"empty", "", `""`},
		{"safe word", "vim", "vim"},
		{"safe path", "path/to/file-1.0", "path/to/file-1.0"},
		{"safe email", "a@b.com", "a@b.com"},
		{"spaces", "less -R", `"less -R"`},
		// $ is preserved verbatim so the shell expands it at sourcing time.
		{"dollar var", "$HOME", `"$HOME"`},
		{"braced var", "${HOME}", `"${HOME}"`},
		{"dollar in path", "$HOME/.local", `"$HOME/.local"`},
		{"command sub", "$(whoami)", `"$(whoami)"`},
		// Backticks become \` so an accidental command substitution does not fire.
		{"backtick", "`whoami`", "\"\\`whoami\\`\""},
		{"double quote", `a"b`, `"a\"b"`},
		{"backslash", `a\b`, `"a\\b"`},
		{"single quote inside", "it's", `"it's"`},
		{"newline", "line1\nline2", "\"line1\nline2\""},
		{"unicode", "日本語", `"日本語"`},
		{"hash", "#comment", `"#comment"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := shellx.PosixExportValue(tc.in)
			if got != tc.want {
				t.Errorf("PosixExportValue(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestPosixExportValuePreservesDollarExpansion(t *testing.T) {
	t.Parallel()
	// Pins the doc claim "$ is left verbatim so the shell expands $VAR / ${VAR}
	// / $(cmd) when the rc file is sourced".
	cases := []string{"$HOME", "${HOME}", "$HOME/.local", "$(whoami)", "a${USER}b"}
	for _, in := range cases {
		got := shellx.PosixExportValue(in)
		if !strings.Contains(got, "$") {
			t.Errorf("PosixExportValue(%q) = %q, $ must be preserved verbatim", in, got)
		}
		if strings.Contains(got, `\$`) {
			t.Errorf("PosixExportValue(%q) = %q, $ must not be escaped", in, got)
		}
	}
}

func TestFishExportValue(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, in, want string
	}{
		{"empty", "", `""`},
		{"safe word", "vim", `"vim"`},
		{"spaces", "less -R", `"less -R"`},
		// fish double quotes expand $VAR; $ stays verbatim on emission.
		{"dollar var", "$HOME", `"$HOME"`},
		{"braced var", "${HOME}", `"${HOME}"`},
		{"dollar in path", "$HOME/.local", `"$HOME/.local"`},
		{"command sub", "$(whoami)", `"$(whoami)"`},
		// fish has no backtick command substitution, so backticks are literal —
		// but we still escape neither (no special meaning) and double quote stays
		// the escape target.
		{"backtick", "`whoami`", "\"`whoami`\""},
		{"double quote", `a"b`, `"a\"b"`},
		{"backslash", `a\b`, `"a\\b"`},
		{"single quote inside", "it's", `"it's"`},
		{"newline", "line1\nline2", "\"line1\nline2\""},
		{"unicode", "日本語", `"日本語"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := shellx.FishExportValue(tc.in)
			if got != tc.want {
				t.Errorf("FishExportValue(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFishExportValuePreservesDollarExpansion(t *testing.T) {
	t.Parallel()
	cases := []string{"$HOME", "${HOME}", "$HOME/.local", "$(date)"}
	for _, in := range cases {
		got := shellx.FishExportValue(in)
		if !strings.Contains(got, "$") {
			t.Errorf("FishExportValue(%q) = %q, $ must be preserved verbatim", in, got)
		}
		if strings.Contains(got, `\$`) {
			t.Errorf("FishExportValue(%q) = %q, $ must not be escaped", in, got)
		}
	}
}
