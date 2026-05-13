package fsx

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// tempFile is the *os.File subset AtomicWriteFile needs; an interface so
// fault-injection tests can swap createTempFn.
type tempFile interface {
	io.WriteCloser
	Sync() error
	Name() string
}

// Test seams for fault injection into Sync/Close paths that healthy local
// filesystems never trigger.
var (
	createTempFn = func(dir, pattern string) (tempFile, error) {
		return os.CreateTemp(dir, pattern)
	}
	chmodFn  = os.Chmod
	renameFn = os.Rename
)

// AtomicWriteFile writes via a same-directory temp file + os.Rename so
// readers never observe a partial write. The destination directory must
// already exist.
func AtomicWriteFile(path string, data []byte, perm os.FileMode) (retErr error) {
	dir := filepath.Dir(path)
	f, err := createTempFn(dir, ".cocoon-tmp-*")
	if err != nil {
		return fmt.Errorf("fsx: create temp: %w", err)
	}
	tmpName := f.Name()
	defer func() {
		if retErr != nil {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("fsx: write: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("fsx: sync: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("fsx: close: %w", err)
	}
	if err := chmodFn(tmpName, perm); err != nil {
		return fmt.Errorf("fsx: chmod: %w", err)
	}
	if err := renameFn(tmpName, path); err != nil {
		return fmt.Errorf("fsx: rename: %w", err)
	}
	return nil
}
