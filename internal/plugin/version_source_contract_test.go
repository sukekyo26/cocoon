package plugin_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/require"

	"github.com/sukekyo26/cocoon/internal/plugin"
)

// exactOnlyPlugins are version_capable plugins that intentionally ship no
// [version.source] because their upstream exposes no machine-readable
// "latest": aws-cli's download URL has no version in it (an unversioned
// alias), and android-sdk's build number is only on an HTML page. Users must
// pin these to an exact version; `cocoon lock` rejects "latest" for them.
//
//nolint:gochecknoglobals // pin-down allowlist for the coverage contract.
var exactOnlyPlugins = map[string]bool{
	"aws-cli":     true,
	"android-sdk": true,
	// flutter's releases manifest keys the stable release by commit hash, not
	// a resolvable version field — see flutter/plugin.toml.
	"flutter": true,
}

// TestCatalog_VersionSourceCoverage asserts that every version_capable catalog
// plugin either declares a valid [version.source] (so `cocoon lock` can
// resolve "latest" + checksums) or is an explicit exact-only plugin. This is
// the lockstep guard that a new version_capable plugin cannot silently ship
// without a resolution path.
func TestCatalog_VersionSourceCoverage(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir("catalog")
	require.NoError(t, err)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join("catalog", id, "plugin.toml")
			data, rerr := os.ReadFile(path) //nolint:gosec // catalog file
			require.NoError(t, rerr)
			var p plugin.Plugin
			require.NoError(t, toml.Unmarshal(data, &p))

			switch {
			case !p.Version.VersionCapable:
				require.Nil(t, p.Version.Source, "%s is not version_capable but declares [version.source]", id)
			case exactOnlyPlugins[id]:
				require.Nil(t, p.Version.Source, "%s is exact-only; it must not declare [version.source]", id)
			default:
				require.NotNilf(t, p.Version.Source,
					"version_capable plugin %q must declare [version.source] (or be added to exactOnlyPlugins)", id)
				// p.Validate covers the source schema (kinds, https URLs, arch map).
				require.NoErrorf(t, p.Validate(path), "%s [version.source] is invalid", id)
			}
		})
	}
}
