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

// ErrPinLineEmptyID is returned by UpsertPinAndMethod when called with an empty id.
var ErrPinLineEmptyID = errors.New("UpsertPinAndMethod: empty id")

// ErrPinLineEmptyRef is returned by UpsertPinAndMethod when called with an empty spec.
var ErrPinLineEmptyRef = errors.New("UpsertPinAndMethod: empty spec")

// sectionHeaderRE matches a TOML table header: `[name.space]` with optional
// surrounding whitespace and a trailing comment.
var sectionHeaderRE = regexp.MustCompile(`^\s*\[([A-Za-z0-9_.-]+)\]\s*(#.*)?$`)

// rxEnableLine matches the start of the `[plugins].enable` array assignment.
var rxEnableLine = regexp.MustCompile(`^\s*enable\s*=\s*\[`)

// rxQuotedElem matches a TOML basic-string array element so the enable array's
// entries can be extracted regardless of single- or multi-line layout.
var rxQuotedElem = regexp.MustCompile(`"([^"]*)"`)

// ErrLegacyPluginVersions is returned when the config file still carries a
// `[plugins.versions]` section. cocoon now pins versions inline in the enable
// array, so the mutator refuses rather than leave a stale section cocoon gen
// would reject.
var ErrLegacyPluginVersions = errors.New(
	"the config file has a [plugins.versions] section; cocoon now pins versions in the enable array " +
		`(enable = [ "go=1.23.4" ]) and puts extra knobs in [plugins.options]. ` +
		"Migrate it before invoking --write")

// UpsertPinAndMethod atomically upserts id's [plugins].enable entry
// (`<id>=<version>` or `<id>=latest`) and, when method != "", the
// [plugins.methods] `<id> = "<method>"` line. Both upserts share a single
// read-modify-write cycle so a transient I/O failure cannot leave
// the config file half-updated (writing the version and method in two separate
// passes would be non-transactional). Pass method = "" to upsert the version
// alone.
//
// spec is the normalised version constraint ("=<version>" or "latest").
// Returns ErrLegacyPluginVersions when the file carries the removed
// [plugins.versions] section; the file is not modified in that case. The
// enable array is re-emitted in cocoon's canonical multi-line style, so inline
// comments inside the array are not preserved.
func UpsertPinAndMethod(path, id, spec, method string) error {
	if id == "" {
		return ErrPinLineEmptyID
	}
	if spec == "" {
		return ErrPinLineEmptyRef
	}
	// method == "" is the "pin only" path; do not reject it.
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	body, err := os.ReadFile(path) //nolint:gosec // caller-provided workspace path
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	out, err := upsertEnableEntry(body, id, FormatEnableEntry(id, spec))
	if err != nil {
		return err
	}
	if method != "" {
		out, err = upsertMethodLineBytes(out, id, method)
		if err != nil {
			return err
		}
	}
	if err := fsx.AtomicWriteFile(path, out, info.Mode().Perm()); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// upsertEnableEntry inserts or replaces the enable-array element for id (the
// pre-formatted `<id>` / `<id>=<spec>` string) inside [plugins].enable.
func upsertEnableEntry(input []byte, id, entry string) ([]byte, error) {
	hadTrailingNewline := bytes.HasSuffix(input, []byte("\n"))
	lines := splitToLines(input)
	if hasLegacyVersionsSection(lines) {
		return nil, ErrLegacyPluginVersions
	}
	secStart, secEnd := findSection(lines, "plugins")
	if secStart < 0 {
		return renderLines(appendEnableSection(lines, entry), hadTrailingNewline)
	}
	arrStart, arrEnd, elems, ok := enableArrayIn(lines, secStart, secEnd)
	if !ok {
		// [plugins] exists without an enable array: insert one after the header.
		inserted := insertLinesAt(lines, secStart+1, formatEnableArray([]string{entry}))
		return renderLines(inserted, hadTrailingNewline)
	}
	elems = upsertElem(elems, id, entry)
	out := replaceLineRange(lines, arrStart, arrEnd, formatEnableArray(elems))
	return renderLines(out, hadTrailingNewline)
}

// enableArrayIn locates the `enable = [ … ]` array inside the [secStart,
// secEnd) [plugins] section. It returns the [arrStart, arrEnd] line span and
// the parsed string elements; ok is false when the section has no enable key.
func enableArrayIn(lines []string, secStart, secEnd int) (arrStart, arrEnd int, elems []string, ok bool) {
	for i := secStart + 1; i < secEnd; i++ {
		if !rxEnableLine.MatchString(lines[i]) {
			continue
		}
		arrStart = i
		arrEnd = i
		for arrEnd+1 < secEnd && !strings.Contains(lines[arrEnd], "]") {
			arrEnd++
		}
		span := strings.Join(lines[arrStart:arrEnd+1], "\n")
		for _, m := range rxQuotedElem.FindAllStringSubmatch(span, -1) {
			elems = append(elems, m[1])
		}
		return arrStart, arrEnd, elems, true
	}
	return 0, 0, nil, false
}

// upsertElem replaces the element whose id (text before the first "=") matches
// id, or appends entry when no element matches.
func upsertElem(elems []string, id, entry string) []string {
	for i, e := range elems {
		if enableEntryID(e) == id {
			elems[i] = entry
			return elems
		}
	}
	return append(elems, entry)
}

// enableEntryID returns the plugin id of one enable-array element (everything
// before the first "=").
func enableEntryID(e string) string {
	e = strings.TrimSpace(e)
	if i := strings.IndexByte(e, '='); i >= 0 {
		return strings.TrimSpace(e[:i])
	}
	return e
}

// formatEnableArray renders the canonical multi-line enable array; an empty
// element list collapses to `enable = []`.
func formatEnableArray(elems []string) []string {
	if len(elems) == 0 {
		return []string{"enable = []"}
	}
	out := make([]string, 0, len(elems)+2)
	out = append(out, "enable = [")
	for _, e := range elems {
		out = append(out, fmt.Sprintf("    %q,", e))
	}
	out = append(out, "]")
	return out
}

// appendEnableSection appends a fresh `[plugins]` section with a one-element
// enable array at end-of-file, guaranteeing at least one blank line of
// separation from the previous non-blank line.
func appendEnableSection(lines []string, entry string) []string {
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
		lines = append(lines, "")
	}
	lines = append(lines, "[plugins]")
	return append(lines, formatEnableArray([]string{entry})...)
}

// hasLegacyVersionsSection detects a `[plugins.versions]` header (or a
// `[plugins.versions.<id>]` subsection), the removed format cocoon emitted
// before folding versions into the enable array.
func hasLegacyVersionsSection(lines []string) bool {
	for _, ln := range lines {
		m := sectionHeaderRE.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		if m[1] == "plugins.versions" || strings.HasPrefix(m[1], "plugins.versions.") {
			return true
		}
	}
	return false
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
	return insertLinesAt(lines, insertAt, []string{newLine})
}

// insertLinesAt returns lines with ins spliced in at index at.
func insertLinesAt(lines []string, at int, ins []string) []string {
	out := make([]string, 0, len(lines)+len(ins))
	out = append(out, lines[:at]...)
	out = append(out, ins...)
	out = append(out, lines[at:]...)
	return out
}

// replaceLineRange returns lines with the inclusive [start, end] span replaced
// by repl.
func replaceLineRange(lines []string, start, end int, repl []string) []string {
	out := make([]string, 0, len(lines)-(end-start+1)+len(repl))
	out = append(out, lines[:start]...)
	out = append(out, repl...)
	out = append(out, lines[end+1:]...)
	return out
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
