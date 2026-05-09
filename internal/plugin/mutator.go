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

// UpsertPinBlock loads the workspace.toml at path, inserts (or replaces) a
// `[plugins.versions.<id>]` block formatted by FormatPinBlock, and writes the
// file back atomically. Comments and blank lines outside the target block are
// preserved verbatim.
//
// Behavior:
//   - If `[plugins.versions.<id>]` already exists, its body (header through
//     the line before the next section header or EOF) is replaced in place.
//   - Otherwise the new block is appended just after the last existing
//     `[plugins.versions.*]` block, separated by one blank line.
//   - If no `[plugins.versions.*]` block exists, the new block is appended at
//     end-of-file, separated by one blank line.
func UpsertPinBlock(path, id, ref, amd64Sum, arm64Sum string) error {
	if id == "" {
		return ErrPinBlockEmptyID
	}
	if ref == "" {
		return ErrPinBlockEmptyRef
	}
	body, err := os.ReadFile(path) //nolint:gosec // caller-provided workspace path
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	out, err := upsertPinBlockBytes(body, id, ref, amd64Sum, arm64Sum)
	if err != nil {
		return err
	}
	if err := fsx.AtomicWriteFile(path, out, 0o644); err != nil {
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
// and the next section.
func replaceTargetBlock(lines []string, target *pinSpan, repl []string) []string {
	end := target.end
	trailingBlanks := 0
	for end > target.start && strings.TrimSpace(lines[end-1]) == "" {
		end--
		trailingBlanks++
	}
	out := make([]string, 0, len(lines)-(end-target.start)+len(repl)+trailingBlanks)
	out = append(out, lines[:target.start]...)
	out = append(out, repl...)
	for i := 0; i < trailingBlanks; i++ {
		out = append(out, "")
	}
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

// appendAtEOF returns lines with repl appended at end-of-file, separated by a
// blank line if the file was non-empty. Trailing blank lines on lines are
// dropped first so blanks do not stack.
func appendAtEOF(lines, repl []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > 0 {
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

// renderLines joins lines back into a byte slice, restoring the trailing
// newline convention of the source.
func renderLines(lines []string, hadTrailingNewline bool) ([]byte, error) {
	var out bytes.Buffer
	w := bufio.NewWriter(&out)
	for i, ln := range lines {
		if _, err := w.WriteString(ln); err != nil {
			return nil, fmt.Errorf("write line: %w", err)
		}
		if i < len(lines)-1 || hadTrailingNewline || len(lines) > 0 {
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
