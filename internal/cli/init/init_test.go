//nolint:testpackage // exercises unexported helpers across the initcli package.
package initcli

import (
	"testing"

	"github.com/sukekyo26/cocoon/internal/plugin"
)

func loadPluginsForTest(t *testing.T) map[string]*plugin.Plugin {
	t.Helper()
	plugins, err := loadEmbeddedPlugins()
	if err != nil {
		t.Fatalf("loadEmbeddedPlugins: %v", err)
	}
	return plugins
}

// pinEnglish stabilizes assertions on hosts whose LANG starts with "ja".
func pinEnglish(t *testing.T) {
	t.Helper()
	for _, k := range []string{"WORKSPACE_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		t.Setenv(k, "")
	}
	t.Setenv("WORKSPACE_LANG", "en")
}
