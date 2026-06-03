package plugin

import (
	"fmt"
	"sort"
	"strings"
)

// FormatPinLine returns one scalar-string assignment line for
// [plugins.versions]:
//
//	<id> = "<spec>"
//
// where spec is a version constraint ("=1.23.4" or "latest"). The output
// ends with exactly one newline. This is the per-id body emitted under the
// shared `[plugins.versions]` section header.
func FormatPinLine(id, spec string) string {
	return FormatPinLineWithExtras(id, spec, nil)
}

// FormatPinLineWithExtras renders the [plugins.versions] line for id. With
// no extras it is the scalar `<id> = "<spec>"`; with extras (keys a plugin
// declares via [install.extra_versions]) it is the inline-table form
// `<id> = { version = "<spec>", <key> = "<val>", … }`, where the constraint
// moves under the reserved "version" key. Extras are emitted in sorted key
// order so the output is stable across calls; a nil/empty extras map
// reproduces FormatPinLine's scalar output byte-for-byte.
//
// Keys are emitted unquoted as TOML bare keys, so any key that does not
// match rxExtraVersionKey (^[a-z][a-z0-9_]*$) is skipped — emitting it raw
// would produce invalid TOML. The caller upstream (validation on
// plugin.toml decode, plus rxExtraVersionKey enforcement in
// validateExtraVersions) is what actually rejects bad keys; this filter is
// a belt-and-suspenders guard for the mutator path where extras come from
// parsing an existing workspace.toml line and could legally carry TOML
// quoted-keys like "weird.key".
func FormatPinLineWithExtras(id, spec string, extras map[string]string) string {
	keys := make([]string, 0, len(extras))
	for k := range extras {
		if rxExtraVersionKey.MatchString(k) {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return fmt.Sprintf("%s = %q\n", id, spec)
	}
	sort.Strings(keys)
	var b strings.Builder
	fmt.Fprintf(&b, "%s = { version = %q", id, spec)
	for _, k := range keys {
		fmt.Fprintf(&b, ", %s = %q", k, extras[k])
	}
	b.WriteString(" }\n")
	return b.String()
}

// PinLine describes one entry to emit under [plugins.versions]. ID and Spec
// are required; Spec is the version constraint ("=1.23.4" or "latest").
type PinLine struct {
	ID   string
	Spec string
}

// FormatPinSection returns the full `[plugins.versions]` section with one
// scalar line per pin, alphabetically sorted by id. Returns an empty string
// when pins is empty.
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
		b.WriteString(FormatPinLine(p.ID, p.Spec))
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
