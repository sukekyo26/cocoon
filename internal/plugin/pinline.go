package plugin

import (
	"fmt"
	"sort"
	"strings"
)

// FormatPinLine returns one inline-table assignment line for [plugins.versions]:
//
//	<id> = { pin = "<ref>", checksum_amd64 = "...", checksum_arm64 = "..." }
//
// Empty checksum fields are omitted. The output ends with exactly one newline.
// This is the per-id body emitted under the shared `[plugins.versions]`
// section header.
func FormatPinLine(id, ref, amd64Checksum, arm64Checksum string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s = { pin = %q", id, ref)
	if amd64Checksum != "" {
		fmt.Fprintf(&b, ", checksum_amd64 = %q", amd64Checksum)
	}
	if arm64Checksum != "" {
		fmt.Fprintf(&b, ", checksum_arm64 = %q", arm64Checksum)
	}
	b.WriteString(" }\n")
	return b.String()
}

// PinLine describes one entry to emit under [plugins.versions]. ID and Ref
// are required; checksum fields are optional.
type PinLine struct {
	ID            string
	Ref           string
	ChecksumAmd64 string
	ChecksumArm64 string
}

// FormatPinSection returns the full `[plugins.versions]` section with one
// inline-table line per pin, alphabetically sorted by id. Returns an empty
// string when pins is empty.
func FormatPinSection(pins []PinLine) string {
	if len(pins) == 0 {
		return ""
	}
	sorted := make([]PinLine, len(pins))
	copy(sorted, pins)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	var b strings.Builder
	b.WriteString("[plugins.versions]\n")
	for _, p := range sorted {
		b.WriteString(FormatPinLine(p.ID, p.Ref, p.ChecksumAmd64, p.ChecksumArm64))
	}
	return b.String()
}
