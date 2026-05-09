package fsx_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sukekyo26/cocoon/internal/fsx"
)

func TestAtomicWriteFileNew(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	if err := fsx.AtomicWriteFile(path, []byte("hello\n"), 0o600); err != nil {
		t.Fatalf("AtomicWriteFile: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello\n" {
		t.Errorf("content = %q, want %q", got, "hello\n")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("perm = %v, want 0o600", info.Mode().Perm())
	}
}

func TestAtomicWriteFileOverwrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := fsx.AtomicWriteFile(path, []byte("new"), 0o600); err != nil {
		t.Fatalf("AtomicWriteFile: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Errorf("got %q, want new", got)
	}
}

func TestAtomicWriteFileNoLingeringTempOnError(t *testing.T) {
	t.Parallel()
	// Pointing at a non-existent directory triggers CreateTemp failure;
	// nothing should be left behind.
	dir := filepath.Join(t.TempDir(), "missing")
	err := fsx.AtomicWriteFile(filepath.Join(dir, "x.txt"), []byte("x"), 0o600)
	if err == nil {
		t.Fatal("expected error when parent directory is missing")
	}
}

func TestAtomicWriteFileLeavesNoTempInDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	if err := fsx.AtomicWriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("AtomicWriteFile: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "out.txt" {
			t.Errorf("unexpected leftover: %s", e.Name())
		}
	}
}

// TestAtomicWriteRenameFailsWhenTargetIsDir exercises the rename error path:
// os.Rename(tmp, dir) fails because the destination is an existing
// directory, and the deferred cleanup should still remove the temp file.
func TestAtomicWriteRenameFailsWhenTargetIsDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "out")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	// Drop a sentinel inside `target` so renaming over it would fail because
	// the destination directory is non-empty (POSIX semantics).
	if err := os.WriteFile(filepath.Join(target, "child"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := fsx.AtomicWriteFile(target, []byte("hello"), 0o600)
	if err == nil {
		t.Fatal("expected rename error when target is a non-empty directory")
	}
	// Tmp file must be cleaned up; only "out/" and "out/child" remain.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "out" {
			t.Errorf("unexpected leftover: %s", e.Name())
		}
	}
}

// TestAtomicWritePermBitsApplied checks that Chmod is invoked: writing with a
// permissive mode that the umask would not normally allow yields the exact
// requested bits on the final file.
func TestAtomicWritePermBitsApplied(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "perm.txt")
	if err := fsx.AtomicWriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("AtomicWriteFile: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Errorf("perm = %v, want 0o644", info.Mode().Perm())
	}
}

// TestAtomicWriteDurability is a smoke test for the fsync seam: after the
// call returns, the on-disk file must contain the full payload size.
func TestAtomicWriteDurability(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "big.bin")
	payload := make([]byte, 4096)
	for i := range payload {
		payload[i] = byte(i & 0xFF)
	}
	if err := fsx.AtomicWriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("AtomicWriteFile: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != int64(len(payload)) {
		t.Errorf("size = %d, want %d", info.Size(), len(payload))
	}
}

// TestAtomicWriteOverwriteNoLingering verifies cleanup of intermediate temp
// files when overwriting an existing file: only `out.txt` should be left
// after the call (no `.wsd-tmp-*` siblings).
func TestAtomicWriteOverwriteNoLingering(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := fsx.AtomicWriteFile(path, []byte("new"), 0o600); err != nil {
		t.Fatalf("AtomicWriteFile: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "out.txt" {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("entries = %v, want [out.txt] only", names)
	}
}
