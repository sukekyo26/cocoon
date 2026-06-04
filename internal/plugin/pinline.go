package plugin

import (
	"fmt"
	"strings"
)

// FormatEnableEntry renders one [plugins].enable array element for id. The
// version is spelled bare (the enable-array grammar), so an "=<version>" spec
// becomes "<id>=<version>"; the floating "latest" (or an empty spec) becomes
// "<id>=latest". `cocoon plugin pin` always supplies a spec, so this never
// emits the bare "<id>" form — that is reserved for unpinned plugins written
// by `cocoon init`.
func FormatEnableEntry(id, spec string) string {
	if spec == "" || spec == "latest" {
		return id + "=latest"
	}
	return id + "=" + strings.TrimPrefix(spec, "=")
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
