package plugin_test

import (
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sukekyo26/cocoon/internal/plugin"
)

// arm64ExcludePath is the e2e data file shared with docker-roundtrip.sh.
// Relative to this package dir (the go test cwd) the repo root is two
// levels up.
const arm64ExcludePath = "../../e2e/arm64-exclude.txt"

// readArm64Exclude parses the shared exclude file the same way
// docker-roundtrip.sh does: one plugin id per line, skipping blanks and
// #-comments.
func readArm64Exclude(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(arm64ExcludePath)
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

// TestArm64ExcludeIDsExist guards that every id in e2e/arm64-exclude.txt
// is a real embedded catalog plugin, so a renamed/removed plugin cannot
// leave a stale exclude that silently drops a different plugin from the
// arm64-full e2e preset.
func TestArm64ExcludeIDsExist(t *testing.T) {
	t.Parallel()
	exclude := readArm64Exclude(t)
	require.NotEmpty(t, exclude, "arm64-exclude.txt parsed empty — path or format drift")
	ids := catalogIDs(t)
	require.NotEmpty(t, ids, "embedded catalog enumerated empty")
	for _, id := range exclude {
		require.Truef(t, ids[id], "arm64-exclude.txt id %q is not a catalog plugin", id)
	}
}

// TestArm64ExcludeNoDuplicates guards the shared data file against
// duplicate ids that would mask a typo.
func TestArm64ExcludeNoDuplicates(t *testing.T) {
	t.Parallel()
	seen := make(map[string]bool)
	for _, id := range readArm64Exclude(t) {
		require.Falsef(t, seen[id], "duplicate id %q in arm64-exclude.txt", id)
		seen[id] = true
	}
}
