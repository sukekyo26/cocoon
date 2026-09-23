package plugin_test

import (
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sukekyo26/cocoon/internal/plugin"
)

// The e2e data files shared with docker-roundtrip.sh / plugin-e2e.yml.
// Relative to this package dir (the go test cwd) the repo root is two
// levels up.
const (
	arm64ExcludePath     = "../../e2e/arm64-exclude.txt"
	pluginE2EExcludePath = "../../e2e/plugin-e2e-exclude.txt"
)

// readE2EIDList parses a shared id list the same way the shell consumers
// do: one plugin id per line, skipping blanks and #-comments. The TrimSpace
// mirrors the scripts' trimming reader — keep them in lockstep so a stray
// surrounding space can't pass this guard yet fail to match at runtime.
func readE2EIDList(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var ids []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ids = append(ids, line)
	}
	return ids
}

// catalogIDs enumerates the embedded catalog's top-level plugin ids.
func catalogIDs(t *testing.T) map[string]bool {
	t.Helper()
	catalogFS, err := plugin.CatalogFS()
	require.NoError(t, err)
	entries, err := fs.ReadDir(catalogFS, ".")
	require.NoError(t, err)
	ids := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			ids[e.Name()] = true
		}
	}
	return ids
}

// e2eExcludeLists are the shared exclude files: arm64-exclude.txt drops
// arm64-unsafe plugins from arm64 e2e (plugin-e2e.yml's arm64 matrix and
// docker-roundtrip.sh arm64-full); plugin-e2e-exclude.txt drops too-heavy
// plugins from plugin-e2e.yml's automatic runs.
//
//nolint:gochecknoglobals // table shared by the guards below.
var e2eExcludeLists = []string{arm64ExcludePath, pluginE2EExcludePath}

// TestE2EExcludeIDsExist guards that every id in each exclude file is a real
// embedded catalog plugin, so a renamed/removed plugin cannot leave a stale
// exclude that silently drops a different plugin from e2e.
func TestE2EExcludeIDsExist(t *testing.T) {
	t.Parallel()
	ids := catalogIDs(t)
	require.NotEmpty(t, ids, "embedded catalog enumerated empty")
	for _, path := range e2eExcludeLists {
		exclude := readE2EIDList(t, path)
		require.NotEmptyf(t, exclude, "%s parsed empty — path or format drift", path)
		for _, id := range exclude {
			require.Truef(t, ids[id], "%s id %q is not a catalog plugin", path, id)
		}
	}
}

// TestE2EExcludeNoDuplicates guards each exclude file against duplicate ids
// that would mask a typo.
func TestE2EExcludeNoDuplicates(t *testing.T) {
	t.Parallel()
	for _, path := range e2eExcludeLists {
		seen := make(map[string]bool)
		for _, id := range readE2EIDList(t, path) {
			require.Falsef(t, seen[id], "duplicate id %q in %s", id, path)
			seen[id] = true
		}
	}
}
