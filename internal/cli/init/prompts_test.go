//nolint:testpackage // exercises unexported prompt / form-builder helpers.
package initcli

import (
	"testing"

	"github.com/sukekyo26/cocoon/internal/i18n"
)

// TestPluginsMultiSelect_BuildsForEveryExcludeID is a smoke test: huh's
// option list isn't reachable through a stable API, so this only confirms
// construction does not panic. Exclusion behavior itself is covered by
// TestFilterPluginIDs.
func TestPluginsMultiSelect_BuildsForEveryExcludeID(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	cat := i18n.New(i18n.LangEN)

	for _, excludeID := range []string{"", "rust", "go", "node", "deno"} {
		excludeID := excludeID
		t.Run("exclude="+excludeID, func(t *testing.T) {
			t.Parallel()
			// Fresh slice per subtest: parallel subtests must not share the
			// pointer handed to huh's MultiSelect.Value.
			var target []string
			sel := pluginsMultiSelect(cat, plugins, excludeID, &target)
			if sel == nil {
				t.Fatal("pluginsMultiSelect returned nil")
			}
		})
	}
}
