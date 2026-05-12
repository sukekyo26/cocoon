package fsx

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// tempFile is the minimal subset of *os.File used by AtomicWriteFile.
// Defined as an interface so fault-injection tests can swap createTempFn.
type tempFile interface {
	io.WriteCloser
	Sync() error
	Name() string
}

// Test seams. Defaulted to the stdlib equivalents; tests in package fsx
// override these to inject failures into otherwise-untriggerable paths
// (Sync/Close errors do not occur naturally on a healthy local fs).
var (
	createTempFn = func(dir, pattern string) (tempFile, error) {
		return os.CreateTemp(dir, pattern)
	}
	chmodFn  = os.Chmod
	renameFn = os.Rename
)

// AtomicWriteFile writes data to path via a same-directory temp file and
// os.Rename, so readers never observe a partially-written file. The temp
// file is removed if any step fails before the final rename.
//
// The destination directory must exist; AtomicWriteFile does not create it.
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
