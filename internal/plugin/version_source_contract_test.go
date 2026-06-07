package plugin_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

// sourcelessPlugins are version_capable plugins that intentionally ship no
// [version.source] because their upstream exposes no machine-readable "latest"
// cocoon can resolve to a reproducible version: aws-cli's download URL is an
// unversioned alias, android-sdk's build number is only on an HTML page,
// flutter keys releases by commit hash, and zig's only floating key is the
// rolling "master" dev build. `cocoon lock` does not error on "latest" for
// these — it skips them (records no lock entry) and `cocoon gen` installs the
// latest at build time, non-reproducibly (warned by UnlockedLatestPlugins).
// Their install scripts must resolve the latest from an empty $PIN. Pin an
// exact version to make them reproducible.
//
//nolint:gochecknoglobals // pin-down allowlist for the coverage contract.
var sourcelessPlugins = map[string]bool{
	"aws-cli":     true,
	"android-sdk": true,
	"flutter":     true,
	"zig":         true,
}

// TestCatalog_VersionSourceCoverage asserts that every version_capable catalog
// plugin either declares a valid [version.source] (so `cocoon lock` can
// resolve "latest" + checksums) or is an explicit sourceless plugin. This is
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
			// Strict-decode exactly as production plugin loading does
			// (config.StrictUnmarshal rejects unknown fields), so a misspelled
			// key in a catalog plugin.toml fails this contract instead of being
			// silently ignored here but rejected at runtime.
			var p plugin.Plugin
			require.NoError(t, config.StrictUnmarshal(path, data, &p))

			switch {
			case !p.Version.VersionCapable:
				require.Nil(t, p.Version.Source, "%s is not version_capable but declares [version.source]", id)
			case sourcelessPlugins[id]:
				require.Nil(t, p.Version.Source, "%s is sourceless; it must not declare [version.source]", id)
			default:
				require.NotNilf(t, p.Version.Source,
					"version_capable plugin %q must declare [version.source] (or be added to sourcelessPlugins)", id)
				// p.Validate covers the source schema (kinds, https URLs, arch map).
				require.NoErrorf(t, p.Validate(path), "%s [version.source] is invalid", id)
			}
		})
	}
}
