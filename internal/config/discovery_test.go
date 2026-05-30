package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sukekyo26/cocoon/internal/config"
)

func TestDiscover_FindsRootWorkspaceToml(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "workspace.toml"), "")

	got, err := config.Discover(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != filepath.Join(dir, "workspace.toml") {
		t.Fatalf("got %q, want %q", got, filepath.Join(dir, "workspace.toml"))
	}
}

func TestDiscover_FallsBackToDotCocoon(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".cocoon"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mustWrite(t, filepath.Join(dir, ".cocoon", "workspace.toml"), "")

	got, err := config.Discover(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(dir, ".cocoon", "workspace.toml")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDiscover_PrefersRootOverDotCocoon(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "workspace.toml"), "root")
	if err := os.MkdirAll(filepath.Join(dir, ".cocoon"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mustWrite(t, filepath.Join(dir, ".cocoon", "workspace.toml"), "fallback")

	got, err := config.Discover(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != filepath.Join(dir, "workspace.toml") {
		t.Fatalf("got %q, want root workspace.toml", got)
	}
}

func TestDiscover_AscendsToParent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "workspace.toml"), "")
	child := filepath.Join(root, "subdir", "deep")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	got, err := config.Discover(child)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != filepath.Join(root, "workspace.toml") {
		t.Fatalf("got %q, want %q", got, filepath.Join(root, "workspace.toml"))
	}
}

func TestDiscover_StopsAtGitMarker(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "workspace.toml"), "outside")

	innerRepo := filepath.Join(root, "repo")
	if err := os.MkdirAll(filepath.Join(innerRepo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	leaf := filepath.Join(innerRepo, "src")
	if err := os.MkdirAll(leaf, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	got, err := config.Discover(leaf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("expected '' (stopped at .git boundary), got %q", got)
	}
}

func TestDiscover_NoWorkspaceFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	got, err := config.Discover(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

// TestDiscover_PropagatesStatError pins that a stat failure other than
// "not found" (here, a permission-denied directory) surfaces as an error
// instead of being silently treated as "no workspace here".
func TestDiscover_PropagatesStatError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses directory permission bits")
	}
	t.Parallel()
	locked := filepath.Join(t.TempDir(), "locked")
	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Drop all permissions so os.Stat on an entry inside locked fails with a
	// permission error (not fs.ErrNotExist), exercising the propagation path.
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) }) //nolint:errcheck // best-effort cleanup

	if _, err := config.Discover(locked); err == nil {
		t.Fatal("expected a non-nil error for a permission-denied stat, got nil")
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
