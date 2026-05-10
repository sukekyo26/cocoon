package plugin

import (
	"fmt"
	"strings"
)

// FormatPinBlock returns the canonical TOML form of a [plugins.versions.<id>]
// block: a header line, a `pin = <ref>` line, and optional checksum lines for
// each architecture. The output ends with exactly one newline so callers can
// concatenate multiple blocks with a blank line between them.
//
// Both `cocoon plugin pin` (which prints the block for the user to paste) and
// `cocoon init --plugin-versions` (which writes blocks straight into
// workspace.toml) use this helper so the two paths stay byte-identical.
func FormatPinBlock(id, ref, amd64Checksum, arm64Checksum string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[plugins.versions.%s]\n", id)
	fmt.Fprintf(&b, "pin = %q\n", ref)
	if amd64Checksum != "" {
		fmt.Fprintf(&b, "checksum_amd64 = %q\n", amd64Checksum)
	}
	if arm64Checksum != "" {
		fmt.Fprintf(&b, "checksum_arm64 = %q\n", arm64Checksum)
	}
	return b.String()
}
