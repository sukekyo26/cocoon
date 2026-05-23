package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sukekyo26/cocoon/internal/config"
)

// TestImageOSFamilyLockstep enforces the invariant that SupportedImages and
// ImageOSFamily are bidirectionally in sync, and that every family value is
// a recognised distro family ("ubuntu" or "debian"). Without this lockstep,
// adding a new entry to SupportedImages without a matching ImageOSFamily row
// would make aptMirrorOriginHosts return nil for that image — fail-loud
// behaviour (no rewrite emitted) is still a regression. A typo'd family
// value would do the same. Pinning both directions here keeps the map and
// the slice from drifting silently, which is exactly the lockstep-desync
// hazard called out in .claude/rules/testing.md §5.
func TestImageOSFamilyLockstep(t *testing.T) {
	t.Parallel()

	validFamilies := map[string]bool{"ubuntu": true, "debian": true}

	for _, image := range config.SupportedImages {
		family, hit := config.ImageOSFamily[image]
		require.Truef(t, hit, "ImageOSFamily has no row for image %q (every SupportedImages entry must classify its distro family)", image)
		require.Truef(t, validFamilies[family], "ImageOSFamily[%q] = %q is not a recognised family (want %q or %q)", image, family, "ubuntu", "debian")
	}

	supported := make(map[string]bool, len(config.SupportedImages))
	for _, image := range config.SupportedImages {
		supported[image] = true
	}
	for image := range config.ImageOSFamily {
		require.Truef(t, supported[image], "ImageOSFamily has stray entry %q with no matching SupportedImages row", image)
	}
}
