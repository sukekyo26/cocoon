// Package shellx contains shell-related helpers shared by generators.
package shellx

import "strings"

// safeChars is the set of characters that ShellQuote leaves unquoted, matching
// Python's shlex.quote default _safechars (\w + "@%+=:,./-", where \w is
// [A-Za-z0-9_]).
const safeChars = "@%+=:,./-_"

// ShellQuote returns s in a form safe to embed in a POSIX shell command line.
// Empty strings become "”"; strings made up entirely of safe characters are
// returned unchanged; anything else is wrapped in single quotes with embedded
// single quotes escaped as '"'"'.
func ShellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if isSafe(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func isSafe(s string) bool {
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9':
		default:
			if !strings.ContainsRune(safeChars, r) {
				return false
			}
		}
	}
	return true
}

// FishQuote returns s in a form safe to embed in a fish command line.
//
// fish's single-quoted strings are simpler than POSIX: only ' and \ need
// escaping (with a leading backslash). $-expansion does not happen inside
// single quotes, so values containing $ are emitted verbatim.
func FishQuote(s string) string {
	if s == "" {
		return "''"
	}
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('\'')
	for _, r := range s {
		if r == '\'' || r == '\\' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('\'')
	return b.String()
}

// PosixExportValue quotes s for use as the right-hand side of `export K=V`
// in bash/zsh, deliberately preserving $-expansion so the shell expands
// $VAR / ${VAR} / $(cmd) when the rc file is sourced. Empty becomes ""; a
// string made up entirely of safe characters is returned unquoted; anything
// else is wrapped in double quotes with \, ", and ` escaped as \<c>. $ is
// left verbatim so callers can write `NPM_CONFIG_PREFIX = "$HOME/.local"`
// and have $HOME resolve at shell start-up. Command substitution is the
// caller's responsibility — they wrote it.
func PosixExportValue(s string) string {
	if s == "" {
		return `""`
	}
	if isSafe(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for _, r := range s {
		if r == '\\' || r == '"' || r == '`' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}

// FishExportValue quotes s for use as the value of `set -gx K V` in fish,
// deliberately preserving $-expansion. fish single quotes block expansion
// and fish double quotes expand $VAR, so the value is always wrapped in
// double quotes with \ and " escaped as \<c>. Empty becomes "".
func FishExportValue(s string) string {
	if s == "" {
		return `""`
	}
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for _, r := range s {
		if r == '\\' || r == '"' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}
