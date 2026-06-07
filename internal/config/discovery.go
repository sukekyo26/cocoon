package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Discover returns the absolute path to the closest config file reachable
// from cwd, walking parent directories until a stop boundary. cocoon.toml is
// preferred; workspace.toml is accepted as a fallback so existing projects
// keep working without a rename.
//
// Search order at each directory (closer directories still win over the
// preferred name in a parent — proximity is the primary key):
//
//  1. <dir>/cocoon.toml
//  2. <dir>/workspace.toml
//  3. <dir>/.cocoon/cocoon.toml
//  4. <dir>/.cocoon/workspace.toml
//
// Walking stops at the first of:
//
//   - a directory containing a .git directory or file (worktrees);
//   - $HOME (so cocoon never wanders into shared parents above the user);
//   - the filesystem root.
//
// When nothing is found, Discover returns "" with a nil error so callers
// can decide how to respond (typically: ask the user to run `cocoon init`).
// A non-nil error is returned only for filesystem-level failures — an
// os.Stat that fails for a reason other than the entry being absent (a
// permission or I/O error while probing a candidate path or a .git marker).
func Discover(cwd string) (string, error) {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return "", fmt.Errorf("resolve cwd: %w", err)
	}
	home, _ := os.UserHomeDir() //nolint:errcheck // empty home just disables the boundary; not fatal

	rels := []string{
		DefaultConfigFileName,
		LegacyConfigFileName,
		filepath.Join(".cocoon", DefaultConfigFileName),
		filepath.Join(".cocoon", LegacyConfigFileName),
	}
	for dir := abs; ; {
		for _, rel := range rels {
			path, err := candidateAt(dir, rel)
			if err != nil {
				return "", err
			}
			if path != "" {
				return path, nil
			}
		}
		marker, err := hasGitMarker(dir)
		if err != nil {
			return "", err
		}
		if marker {
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

// candidateAt returns the absolute path joining dir and rel when the entry
// exists and is a regular file, and "" when it is absent or a directory. A
// stat error other than fs.ErrNotExist (a permission or I/O failure) is
// returned so Discover surfaces it instead of silently skipping the directory.
func candidateAt(dir, rel string) (string, error) {
	p := filepath.Join(dir, rel)
	info, err := os.Stat(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("stat %s: %w", p, err)
	}
	if info.IsDir() {
		return "", nil
	}
	return p, nil
}

// hasGitMarker reports whether dir contains a .git entry (directory or file —
// git worktrees use a regular file as the marker). A stat error other than
// fs.ErrNotExist is returned rather than masked as "no marker".
func hasGitMarker(dir string) (bool, error) {
	p := filepath.Join(dir, ".git")
	if _, err := os.Stat(p); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("stat %s: %w", p, err)
	}
	return true, nil
}
