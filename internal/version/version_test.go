package version_test

import (
	"testing"

	"github.com/sukekyo26/cocoon/internal/version"
)

// TestGet_ReturnsVersionVar pins the contract that Get() returns the
// package-level Version variable so the ldflags-baked release string
// reaches the CLI verbatim. t.Parallel is intentionally not called: the
// test mutates the package variable.
//
//nolint:paralleltest // mutates the package-level version.Version
func TestGet_ReturnsVersionVar(t *testing.T) {
	orig := version.Version
	t.Cleanup(func() { version.Version = orig })

	version.Version = "9.9.9-test"
	if got := version.Get(); got != "9.9.9-test" {
		t.Errorf("Get() = %q, want %q", got, "9.9.9-test")
	}
}
