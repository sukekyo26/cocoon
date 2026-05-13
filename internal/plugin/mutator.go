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

// sectionHeaderRE matches a TOML table header: `[name.space]` with optional
// surrounding whitespace and a trailing comment.
var sectionHeaderRE = regexp.MustCompile(`^\s*\[([A-Za-z0-9_.-]+)\]\s*(#.*)?$`)

// ErrPinBlockSubsection is returned when workspace.toml carries a legacy
// `[plugins.versions.<id>]` subsection block. cocoon emits inline tables under
// a single `[plugins.versions]` section; mixing the two forms would produce
// duplicate-key TOML, so the mutator refuses until the user converts.
var ErrPinBlockSubsection = errors.New(
	"workspace.toml has legacy `[plugins.versions.<id>]` subsection block(s); " +
		"cocoon now emits inline tables under a single `[plugins.versions]` section. " +
		"Convert each block to `<id> = { pin = \"...\" }` under `[plugins.versions]` " +
		"before invoking --write")

// UpsertPinBlock atomically inserts or replaces an inline-table assignment
// `<id> = { pin = "<ref>", ... }` under the [plugins.versions] section of
// workspace.toml at path. Comments and blank lines outside the modified line
// are preserved verbatim.
//
//   - existing inline line for <id>: replaced in place.
//   - [plugins.versions] section present, <id> new: line appended at the
//     section's last non-blank position so it sits adjacent to existing pins.
//   - section absent: a fresh `[plugins.versions]\n<id> = { ... }\n` block is
//     appended at EOF, separated from the previous non-blank line by one
//     blank.
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
	out, err := upsertPinLineBytes(body, id, ref, amd64Sum, arm64Sum)
	if err != nil {
		return err
	}
	if err := fsx.AtomicWriteFile(path, out, info.Mode().Perm()); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// upsertPinLineBytes is the pure transformation core, exposed for unit tests
// without requiring filesystem fixtures.
func upsertPinLineBytes(input []byte, id, ref, amd64Sum, arm64Sum string) ([]byte, error) {
	hadTrailingNewline := bytes.HasSuffix(input, []byte("\n"))
	raw := strings.TrimSuffix(string(input), "\n")
	lines := strings.Split(raw, "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}

	if hasLegacySubsection(lines) {
		return nil, ErrPinBlockSubsection
	}

	newLine := strings.TrimSuffix(FormatPinLine(id, ref, amd64Sum, arm64Sum), "\n")

	sectionStart, sectionEnd := findVersionsSection(lines)
	if sectionStart < 0 {
		lines = appendNewVersionsSection(lines, newLine)
		return renderLines(lines, hadTrailingNewline)
	}

	idAssignRE := regexp.MustCompile(`^\s*` + regexp.QuoteMeta(id) + `\s*=`)
	for i := sectionStart + 1; i < sectionEnd; i++ {
		if idAssignRE.MatchString(lines[i]) {
			lines[i] = newLine
			return renderLines(lines, hadTrailingNewline)
		}
	}

	// Section exists, <id> is new — append at the last non-blank position
	// within the section so the new line sits adjacent to existing pins
	// instead of orphaned after the section's trailing blank lines.
	insertAt := sectionEnd
	for insertAt > sectionStart+1 && strings.TrimSpace(lines[insertAt-1]) == "" {
		insertAt--
	}
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:insertAt]...)
	out = append(out, newLine)
	out = append(out, lines[insertAt:]...)
	return renderLines(out, hadTrailingNewline)
}

// hasLegacySubsection reports whether lines contain any `[plugins.versions.<id>]`
// subsection header — the format cocoon used before inline-table emission.
func hasLegacySubsection(lines []string) bool {
	const versionsPrefix = "plugins.versions."
	for _, ln := range lines {
		m := sectionHeaderRE.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		if strings.HasPrefix(m[1], versionsPrefix) {
			return true
		}
	}
	return false
}

// findVersionsSection returns the [start, end) line indices of the
// `[plugins.versions]` section. start is the header line; end is the next
// section header or len(lines) when the section runs to EOF. Returns
// (-1, len(lines)) when no `[plugins.versions]` header is present.
func findVersionsSection(lines []string) (start, end int) {
	const versionsSection = "plugins.versions"
	start = -1
	end = len(lines)
	for i, ln := range lines {
		m := sectionHeaderRE.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		if start < 0 {
			if m[1] == versionsSection {
				start = i
			}
			continue
		}
		end = i
		return start, end
	}
	return start, end
}

// appendNewVersionsSection appends a fresh `[plugins.versions]` section
// (header + one inline-table line) at end-of-file. Guarantees at least
// one blank line of separation from the previous non-blank line: when
// the file already ends in one or more blank lines those are preserved
// verbatim (so the resulting separation may be >1), and when it ends
// with content a single blank line is inserted.
func appendNewVersionsSection(lines []string, inlineLine string) []string {
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
		lines = append(lines, "")
	}
	lines = append(lines, "[plugins.versions]")
	lines = append(lines, inlineLine)
	return lines
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
