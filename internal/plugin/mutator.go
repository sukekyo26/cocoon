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

// ErrPinLineEmptyID is returned by UpsertPinLine when called with an empty id.
var ErrPinLineEmptyID = errors.New("UpsertPinLine: empty id")

// ErrPinLineEmptyRef is returned by UpsertPinLine when called with an empty ref.
var ErrPinLineEmptyRef = errors.New("UpsertPinLine: empty ref")

// ErrMethodLineEmptyID is returned by UpsertMethodLine when called with an empty id.
var ErrMethodLineEmptyID = errors.New("UpsertMethodLine: empty id")

// ErrMethodLineEmptyMethod is returned by UpsertMethodLine when called with an empty method.
var ErrMethodLineEmptyMethod = errors.New("UpsertMethodLine: empty method")

// sectionHeaderRE matches a TOML table header: `[name.space]` with optional
// surrounding whitespace and a trailing comment.
var sectionHeaderRE = regexp.MustCompile(`^\s*\[([A-Za-z0-9_.-]+)\]\s*(#.*)?$`)

// ErrLegacyPinSubsection is returned when workspace.toml carries a legacy
// `[plugins.versions.<id>]` subsection block. cocoon emits inline tables under
// a single `[plugins.versions]` section; mixing the two forms would produce
// duplicate-key TOML, so the mutator refuses until the user converts.
var ErrLegacyPinSubsection = errors.New(
	"workspace.toml has legacy `[plugins.versions.<id>]` subsection block(s); " +
		"cocoon now emits inline tables under a single `[plugins.versions]` section. " +
		"Convert each block to `<id> = { pin = \"...\" }` under `[plugins.versions]` " +
		"before invoking --write")

// UpsertPinLine atomically inserts or replaces an inline-table assignment
// `<id> = { pin = "<ref>", ... }` under the [plugins.versions] section of
// workspace.toml at path. Comments and blank lines outside the modified line
// are preserved verbatim.
//
//   - existing inline line for <id>: replaced in place.
//   - [plugins.versions] section present, <id> new: line appended at the
//     section's last non-blank position so it sits adjacent to existing pins.
//   - section absent: a fresh `[plugins.versions]\n<id> = { ... }\n` section
//     is appended at EOF, separated from the previous non-blank line by at
//     least one blank line (existing trailing blanks are preserved verbatim,
//     so the separation may be more than one).
func UpsertPinLine(path, id, ref, amd64Sum, arm64Sum string) error {
	if id == "" {
		return ErrPinLineEmptyID
	}
	if ref == "" {
		return ErrPinLineEmptyRef
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

func upsertPinLineBytes(input []byte, id, ref, amd64Sum, arm64Sum string) ([]byte, error) {
	hadTrailingNewline := bytes.HasSuffix(input, []byte("\n"))
	lines := splitToLines(input)
	if hasLegacySubsection(lines) {
		return nil, ErrLegacyPinSubsection
	}
	newLine := strings.TrimSuffix(FormatPinLine(id, ref, amd64Sum, arm64Sum), "\n")
	updated := upsertIDLineInSection(lines, "plugins.versions", id, newLine)
	return renderLines(updated, hadTrailingNewline)
}

// UpsertMethodLine atomically inserts or replaces an assignment
// `<id> = "<method>"` under the [plugins.methods] section of workspace.toml
// at path. Comments and blank lines outside the modified line are preserved
// verbatim, mirroring UpsertPinLine.
//
//   - existing line for <id>: replaced in place.
//   - [plugins.methods] section present, <id> new: line appended at the
//     section's last non-blank position so it sits adjacent to existing
//     picks.
//   - section absent: a fresh `[plugins.methods]\n<id> = "<method>"\n`
//     section is appended at EOF, separated from the previous non-blank
//     line by at least one blank line (existing trailing blanks are
//     preserved verbatim).
func UpsertMethodLine(path, id, method string) error {
	if id == "" {
		return ErrMethodLineEmptyID
	}
	if method == "" {
		return ErrMethodLineEmptyMethod
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	body, err := os.ReadFile(path) //nolint:gosec // caller-provided workspace path
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	out, err := upsertMethodLineBytes(body, id, method)
	if err != nil {
		return err
	}
	if err := fsx.AtomicWriteFile(path, out, info.Mode().Perm()); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func upsertMethodLineBytes(input []byte, id, method string) ([]byte, error) {
	hadTrailingNewline := bytes.HasSuffix(input, []byte("\n"))
	lines := splitToLines(input)
	newLine := strings.TrimSuffix(FormatMethodLine(id, method), "\n")
	updated := upsertIDLineInSection(lines, "plugins.methods", id, newLine)
	return renderLines(updated, hadTrailingNewline)
}

// splitToLines splits input on newlines and treats a single empty trailing
// element as "no content" so callers can append a fresh section without
// inheriting a phantom blank first line.
func splitToLines(input []byte) []string {
	raw := strings.TrimSuffix(string(input), "\n")
	lines := strings.Split(raw, "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

// upsertIDLineInSection inserts or replaces `<id> = ...` inside the named
// TOML section. The returned slice may share storage with lines.
func upsertIDLineInSection(lines []string, section, id, newLine string) []string {
	sectionStart, sectionEnd := findSection(lines, section)
	if sectionStart < 0 {
		return appendNewSection(lines, section, newLine)
	}
	idAssignRE := regexp.MustCompile(`^\s*` + regexp.QuoteMeta(id) + `\s*=`)
	for i := sectionStart + 1; i < sectionEnd; i++ {
		if idAssignRE.MatchString(lines[i]) {
			lines[i] = newLine
			return lines
		}
	}
	// Section exists, <id> is new — append at the last non-blank position
	// within the section so the new line sits adjacent to existing entries
	// instead of orphaned after the section's trailing blank lines.
	insertAt := sectionEnd
	for insertAt > sectionStart+1 && strings.TrimSpace(lines[insertAt-1]) == "" {
		insertAt--
	}
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:insertAt]...)
	out = append(out, newLine)
	out = append(out, lines[insertAt:]...)
	return out
}

// hasLegacySubsection detects `[plugins.versions.<id>]` blocks, the format
// cocoon emitted before switching to inline tables.
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

// findSection returns the [start, end) line indices of the named TOML
// section. start is the header line; end is the next section header or
// len(lines) when the section runs to EOF. Returns (-1, len(lines)) when
// no matching header is present.
func findSection(lines []string, name string) (start, end int) {
	start = -1
	end = len(lines)
	for i, ln := range lines {
		m := sectionHeaderRE.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		if start < 0 {
			if m[1] == name {
				start = i
			}
			continue
		}
		end = i
		return start, end
	}
	return start, end
}

// appendNewSection appends a fresh `[<name>]` section (header + one body
// line) at end-of-file. Guarantees at least one blank line of separation
// from the previous non-blank line: when the file already ends in one or
// more blank lines those are preserved verbatim (so the resulting
// separation may be >1), and when it ends with content a single blank
// line is inserted.
func appendNewSection(lines []string, name, bodyLine string) []string {
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
		lines = append(lines, "")
	}
	lines = append(lines, "["+name+"]")
	lines = append(lines, bodyLine)
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
