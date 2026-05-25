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
	return FormatPinLineWithExtras(id, ref, amd64Checksum, arm64Checksum, nil)
}

// FormatPinLineWithExtras is FormatPinLine plus any caller-supplied extra
// keys (declared by the plugin via [install.extra_versions]). Extras are
// emitted in sorted key order so the output is stable across calls.
// Passing a nil/empty extras map reproduces FormatPinLine's output
// byte-for-byte.
//
// Keys are emitted unquoted as TOML bare keys, so any key that does not
// match rxExtraVersionKey (^[a-z][a-z0-9_]*$) is skipped — emitting it
// raw would produce invalid TOML. The caller upstream of this function
// (validation on plugin.toml decode, plus rxExtraVersionKey enforcement
// in validateExtraVersions) is what actually rejects bad keys; this
// filter is a belt-and-suspenders guard for the mutator path where
// extras come from parsing an existing workspace.toml line and could
// legally carry TOML quoted-keys like "weird.key".
func FormatPinLineWithExtras(id, ref, amd64Checksum, arm64Checksum string, extras map[string]string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s = { pin = %q", id, ref)
	if amd64Checksum != "" {
		fmt.Fprintf(&b, ", checksum_amd64 = %q", amd64Checksum)
	}
	if arm64Checksum != "" {
		fmt.Fprintf(&b, ", checksum_arm64 = %q", arm64Checksum)
	}
	if len(extras) > 0 {
		keys := make([]string, 0, len(extras))
		for k := range extras {
			if !rxExtraVersionKey.MatchString(k) {
				continue
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, ", %s = %q", k, extras[k])
		}
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

// FormatMethodLine returns one assignment line for [plugins.methods]:
//
//	<id> = "<method>"
//
// The output ends with exactly one newline. This is the per-id body emitted
// under the shared `[plugins.methods]` section header.
func FormatMethodLine(id, method string) string {
	return fmt.Sprintf("%s = %q\n", id, method)
}
