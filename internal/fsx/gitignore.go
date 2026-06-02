package fsx

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// EnsureGitignoreEntry makes sure pattern is ignored by the .gitignore at path,
// preserving any content already there. It is a no-op when pattern is already
// present as its own (trimmed) line, so it is idempotent. A missing file is
// created (0644) with comment + pattern; an existing file gets comment +
// pattern appended — existing rules are never dropped and the file's existing
// permission bits are preserved (so a user-managed .gitignore is not clobbered,
// mirroring plugin.UpsertPinAndMethod). comment is a single line without a
// trailing newline (pass "" to skip it). Returns true when the file was created
// or modified. The parent directory must already exist (AtomicWriteFile does
// not create it).
func EnsureGitignoreEntry(path, pattern, comment string) (bool, error) {
	info, err := os.Stat(path)
	switch {
	case err == nil:
		existing, readErr := os.ReadFile(path) //nolint:gosec // G304: cocoon's own .devcontainer/.gitignore, not user input.
		if readErr != nil {
			return false, fmt.Errorf("fsx: read %s: %w", path, readErr)
		}
		for _, line := range strings.Split(string(existing), "\n") {
			if strings.TrimSpace(line) == pattern {
				return false, nil // already ignored — preserve the file as-is
			}
		}
		var b strings.Builder
		b.Write(existing)
		if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
			b.WriteByte('\n')
		}
		writeGitignoreEntry(&b, pattern, comment)
		return true, writeErr(path, b.String(), info.Mode().Perm())
	case errors.Is(err, os.ErrNotExist):
		var b strings.Builder
		writeGitignoreEntry(&b, pattern, comment)
		return true, writeErr(path, b.String(), 0o644)
	default:
		return false, fmt.Errorf("fsx: stat %s: %w", path, err)
	}
}

func writeGitignoreEntry(b *strings.Builder, pattern, comment string) {
	if comment != "" {
		b.WriteString(comment)
		b.WriteByte('\n')
	}
	b.WriteString(pattern)
	b.WriteByte('\n')
}

func writeErr(path, body string, perm os.FileMode) error {
	if err := AtomicWriteFile(path, []byte(body), perm); err != nil {
		return fmt.Errorf("fsx: write %s: %w", path, err)
	}
	return nil
}
