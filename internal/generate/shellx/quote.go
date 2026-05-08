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
