package plugin

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/sukekyo26/cocoon/internal/fsx"
)

// ErrPinBlockEmptyID is returned by UpsertPinBlock when called with an empty id.
var ErrPinBlockEmptyID = errors.New("UpsertPinBlock: empty id")

// ErrPinBlockEmptyRef is returned by UpsertPinBlock when called with an empty ref.
var ErrPinBlockEmptyRef = errors.New("UpsertPinBlock: empty ref")

// sectionHeaderRE matches a TOML table header line: optional whitespace, then
// `[name.space]`, then optional whitespace and inline comment. Inline-table
// values like `key = { x = 1 }` are not table headers; values containing `[`
// are also not matched because the regex anchors to start-of-line and requires
// the whole line (modulo a trailing comment) to be the header.
var sectionHeaderRE = regexp.MustCompile(`^\s*\[([A-Za-z0-9_.-]+)\]\s*(#.*)?$`)

// versionsKeyAssignRE matches any `<id> = ...` assignment line under
// [plugins.versions] regardless of the right-hand side (`go = "1.23.4"`,
// `go = { pin = "1.23.4" }`, `go = [..]`, etc.). All such forms collide with
// an appended `[plugins.versions.<id>]` block at parse time (TOML rejects
// "value + table with the same key"), so the mutator refuses uniformly
// rather than handling only the inline-table form.
var versionsKeyAssignRE = regexp.MustCompile(`^\s*([A-Za-z0-9_-]+)\s*=`)

// ErrPinBlockVersionsKeyAssign is returned when workspace.toml has any
// per-id key assignment under [plugins.versions] (e.g. `go = "1.23.4"` or
// `go = { pin = "..." }`) which the line-based mutator cannot safely edit
// without producing duplicate-key TOML.
var ErrPinBlockVersionsKeyAssign = errors.New(
	"workspace.toml has key assignments directly under [plugins.versions] " +
		"(e.g. `<id> = \"...\"` or `<id> = { ... }`); " +
		"convert each to a [plugins.versions.<id>] block before using --write")

// UpsertPinBlock atomically inserts or replaces a `[plugins.versions.<id>]`
// block in the workspace.toml at path. Comments and blank lines outside
// the target block are preserved verbatim.
//
//   - existing block: body replaced in place; surrounding blanks intact.
//   - other versions blocks present: new block appended after the last
//     `[plugins.versions.*]`, with a fresh blank separator before it.
//   - no versions blocks: new block appended at EOF, single blank
//     separator only if the file did not already end with one.
func UpsertPinBlock(path, id, ref, amd64Sum, arm64Sum string) error {
	if id == "" {
		return ErrPinBlockEmptyID
	}
	if ref == "" {
		return ErrPinBlockEmptyRef
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	body, err := os.ReadFile(path) //nolint:gosec // caller-provided workspace path
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	out, err := upsertPinBlockBytes(body, id, ref, amd64Sum, arm64Sum)
	if err != nil {
		return err
	}
	if err := fsx.AtomicWriteFile(path, out, info.Mode().Perm()); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// pinSpan tracks the [start, end) line indices (end exclusive) of a section
// inside the line slice produced by upsertPinBlockBytes.
type pinSpan struct{ start, end int }

// findPinSpans returns the span of the existing [plugins.versions.<id>] block
// (or nil) and the last [plugins.versions.*] block (or nil) by walking the
// line slice once and tracking the active section header.
func findPinSpans(lines []string, target, prefix string) (targetSpan, lastVersionsSpan *pinSpan) {
	curSection := ""
	curSpanStart := -1
	closeSpan := func(endIdx int) {
		if curSpanStart < 0 {
			return
		}
		s := pinSpan{start: curSpanStart, end: endIdx}
		switch {
		case curSection == target:
			t := s
			targetSpan = &t
		case strings.HasPrefix(curSection, prefix):
			t := s
			lastVersionsSpan = &t
		}
		curSpanStart = -1
		curSection = ""
	}
	for i, ln := range lines {
		m := sectionHeaderRE.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		closeSpan(i)
		curSection = m[1]
		if curSection == target || strings.HasPrefix(curSection, prefix) {
			curSpanStart = i
		}
	}
	closeSpan(len(lines))
	return targetSpan, lastVersionsSpan
}

// replaceTargetBlock returns lines with the target span swapped for repl,
// preserving any trailing blank lines that previously sat between the block
// and the next section. The original blank-line slice is reused (not
// regenerated as ""), so whitespace-only blank lines like "  " or "\t"
// round-trip verbatim.
func replaceTargetBlock(lines []string, target *pinSpan, repl []string) []string {
	end := target.end
	for end > target.start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	trailingBlanks := lines[end:target.end]
	out := make([]string, 0, len(lines)-(end-target.start)+len(repl)+len(trailingBlanks))
	out = append(out, lines[:target.start]...)
	out = append(out, repl...)
	out = append(out, trailingBlanks...)
	out = append(out, lines[target.end:]...)
	return out
}

// appendAfterVersions returns lines with repl inserted just after the last
// [plugins.versions.*] block, separated by one blank line.
func appendAfterVersions(lines []string, last *pinSpan, repl []string) []string {
	insertAt := last.end
	for insertAt > 0 && strings.TrimSpace(lines[insertAt-1]) == "" {
		insertAt--
	}
	out := make([]string, 0, len(lines)+len(repl)+1)
	out = append(out, lines[:insertAt]...)
	out = append(out, "")
	out = append(out, repl...)
	out = append(out, lines[insertAt:]...)
	return out
}

// appendAtEOF returns lines with repl appended at end-of-file. Existing
// trailing blank lines are preserved verbatim — they belong to the source
// and the docstring on UpsertPinBlock promises blank lines outside the
// target block stay untouched. A single blank-line separator is inserted
// only when the source file did not already end with one (so the new
// block does not collide with the last non-blank line).
func appendAtEOF(lines, repl []string) []string {
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
		lines = append(lines, "")
	}
	return append(lines, repl...)
}

// upsertPinBlockBytes is the pure transformation core, exposed for unit tests
// without requiring filesystem fixtures.
func upsertPinBlockBytes(input []byte, id, ref, amd64Sum, arm64Sum string) ([]byte, error) {
	const versionsPrefix = "plugins.versions."
	target := versionsPrefix + id
	block := FormatPinBlock(id, ref, amd64Sum, arm64Sum)
	repl := strings.Split(strings.TrimSuffix(block, "\n"), "\n")

	hadTrailingNewline := bytes.HasSuffix(input, []byte("\n"))
	raw := strings.TrimSuffix(string(input), "\n")
	lines := strings.Split(raw, "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}

	if hasKeyAssignUnderVersions(lines) {
		return nil, ErrPinBlockVersionsKeyAssign
	}
	targetSpan, lastVersionsSpan := findPinSpans(lines, target, versionsPrefix)
	switch {
	case targetSpan != nil:
		lines = replaceTargetBlock(lines, targetSpan, repl)
	case lastVersionsSpan != nil:
		lines = appendAfterVersions(lines, lastVersionsSpan, repl)
	default:
		lines = appendAtEOF(lines, repl)
	}
	return renderLines(lines, hadTrailingNewline)
}

// hasKeyAssignUnderVersions reports whether lines contain an active
// [plugins.versions] section with a per-id key assignment in any form
// (`go = "1.23.4"`, `go = { pin = "1.22.5" }`, `go = [..]`, etc.). Any such
// assignment collides with a later [plugins.versions.<id>] block, so the
// caller refuses with ErrPinBlockVersionsKeyAssign.
func hasKeyAssignUnderVersions(lines []string) bool {
	const versionsSection = "plugins.versions"
	inVersions := false
	for _, ln := range lines {
		if m := sectionHeaderRE.FindStringSubmatch(ln); m != nil {
			inVersions = m[1] == versionsSection
			continue
		}
		if !inVersions {
			continue
		}
		if versionsKeyAssignRE.MatchString(ln) {
			return true
		}
	}
	return false
}

// renderLines joins lines back into a byte slice, restoring the trailing
// newline convention of the source: every line gets a separator newline, and
// the final newline is appended only when the source file had one.
func renderLines(lines []string, hadTrailingNewline bool) ([]byte, error) {
	var out bytes.Buffer
	w := bufio.NewWriter(&out)
	for i, ln := range lines {
		if _, err := w.WriteString(ln); err != nil {
			return nil, fmt.Errorf("write line: %w", err)
		}
		isLast := i == len(lines)-1
		if !isLast || hadTrailingNewline {
			if _, err := w.WriteString("\n"); err != nil {
				return nil, fmt.Errorf("write newline: %w", err)
			}
		}
	}
	if err := w.Flush(); err != nil {
		return nil, fmt.Errorf("flush: %w", err)
	}
	return out.Bytes(), nil
}
