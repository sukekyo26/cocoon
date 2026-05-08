package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Discover returns the absolute path to the closest workspace.toml
// reachable from cwd, walking parent directories until a stop boundary.
//
// Search order at each directory:
//
//  1. <dir>/workspace.toml
//  2. <dir>/.cocoon/workspace.toml
//
// Walking stops at the first of:
//
//   - a directory containing a .git directory or file (worktrees);
//   - $HOME (so cocoon never wanders into shared parents above the user);
//   - the filesystem root.
//
// When nothing is found, Discover returns "" with a nil error so callers
// can decide how to respond (typically: ask the user to run `cocoon init`).
// A non-nil error is returned only for filesystem-level failures.
func Discover(cwd string) (string, error) {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return "", fmt.Errorf("resolve cwd: %w", err)
	}
	home, _ := os.UserHomeDir() //nolint:errcheck // empty home just disables the boundary; not fatal

	for dir := abs; ; {
		if path := candidateAt(dir, "workspace.toml"); path != "" {
			return path, nil
		}
		if path := candidateAt(dir, filepath.Join(".cocoon", "workspace.toml")); path != "" {
			return path, nil
		}
		if hasGitMarker(dir) {
			return "", nil
		}
		if home != "" && dir == home {
			return "", nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}

// candidateAt returns the absolute path joining dir and rel when the
// resulting entry exists and is a regular file, else "".
func candidateAt(dir, rel string) string {
	p := filepath.Join(dir, rel)
	info, err := os.Stat(p)
	if err != nil {
		return ""
	}
	if info.IsDir() {
		return ""
	}
	return p
}

// hasGitMarker reports whether dir contains a .git entry (directory or
// file — git worktrees use a regular file as the marker).
func hasGitMarker(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}
