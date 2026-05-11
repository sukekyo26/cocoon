package generate_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sukekyo26/cocoon/internal/generate"
)

// withSentinel swaps generate.ContainerSentinelPath for the duration of
// the test. The package var is mutated so callers cannot run in parallel.
func withSentinel(t *testing.T, path string) {
	t.Helper()
	prev := generate.ContainerSentinelPath
	generate.ContainerSentinelPath = path
	t.Cleanup(func() { generate.ContainerSentinelPath = prev })
}

func TestInContainer_TrueWhenSentinelExists(t *testing.T) { //nolint:paralleltest // ContainerSentinelPath は package 変数差し替え。
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "dockerenv")
	if err := os.WriteFile(sentinel, []byte{}, 0o600); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}
	withSentinel(t, sentinel)
	if !generate.InContainer() {
		t.Errorf("InContainer() = false, want true when %s exists", sentinel)
	}
}

func TestInContainer_FalseWhenSentinelMissing(t *testing.T) { //nolint:paralleltest // ContainerSentinelPath は package 変数差し替え。
	dir := t.TempDir()
	withSentinel(t, filepath.Join(dir, "absent"))
	if generate.InContainer() {
		t.Errorf("InContainer() = true, want false when sentinel is missing")
	}
}
