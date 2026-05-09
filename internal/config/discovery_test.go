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

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
