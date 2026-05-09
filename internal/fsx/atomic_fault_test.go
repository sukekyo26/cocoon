//nolint:testpackage // white-box tests swap unexported seams (createTempFn, chmodFn).
package fsx

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeFile is a programmable tempFile for fault injection.
type fakeFile struct {
	name     string
	writeErr error
	syncErr  error
	closeErr error
	closed   bool
	written  []byte
}

func (f *fakeFile) Write(p []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	f.written = append(f.written, p...)
	return len(p), nil
}

func (f *fakeFile) Sync() error  { return f.syncErr }
func (f *fakeFile) Close() error { f.closed = true; return f.closeErr }
func (f *fakeFile) Name() string { return f.name }

// useFakeCreateTemp swaps createTempFn to return the supplied fake; restored
// at test cleanup.
func useFakeCreateTemp(t *testing.T, fake *fakeFile) {
	t.Helper()
	prev := createTempFn
	createTempFn = func(_, _ string) (tempFile, error) {
		return fake, nil
	}
	t.Cleanup(func() { createTempFn = prev })
}

// useChmod swaps chmodFn; restored at test cleanup.
func useChmod(t *testing.T, fn func(string, os.FileMode) error) {
	t.Helper()
	prev := chmodFn
	chmodFn = fn
	t.Cleanup(func() { chmodFn = prev })
}

var errInjected = errors.New("injected")

//nolint:paralleltest // mutates package-level seams (createTempFn)
func TestAtomicWriteFileWriteError(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeFile{name: filepath.Join(dir, ".wsd-tmp-fake"), writeErr: errInjected}
	useFakeCreateTemp(t, fake)

	err := AtomicWriteFile(filepath.Join(dir, "out"), []byte("payload"), 0o644)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errInjected) {
		t.Errorf("err = %v, want errors.Is errInjected", err)
	}
	if !strings.HasPrefix(err.Error(), "fsx: write:") {
		t.Errorf("err prefix = %q, want fsx: write:", err.Error())
	}
	if !fake.closed {
		t.Error("fake file should be closed after write failure")
	}
}

//nolint:paralleltest // mutates package-level seams (createTempFn)
func TestAtomicWriteFileSyncError(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeFile{name: filepath.Join(dir, ".wsd-tmp-fake"), syncErr: errInjected}
	useFakeCreateTemp(t, fake)

	err := AtomicWriteFile(filepath.Join(dir, "out"), []byte("payload"), 0o644)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errInjected) {
		t.Errorf("err = %v, want errors.Is errInjected", err)
	}
	if !strings.HasPrefix(err.Error(), "fsx: sync:") {
		t.Errorf("err prefix = %q, want fsx: sync:", err.Error())
	}
	if !fake.closed {
		t.Error("fake file should be closed after sync failure")
	}
}

//nolint:paralleltest // mutates package-level seams (createTempFn)
func TestAtomicWriteFileCloseError(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeFile{name: filepath.Join(dir, ".wsd-tmp-fake"), closeErr: errInjected}
	useFakeCreateTemp(t, fake)

	err := AtomicWriteFile(filepath.Join(dir, "out"), []byte("payload"), 0o644)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errInjected) {
		t.Errorf("err = %v, want errors.Is errInjected", err)
	}
	if !strings.HasPrefix(err.Error(), "fsx: close:") {
		t.Errorf("err prefix = %q, want fsx: close:", err.Error())
	}
	if string(fake.written) != "payload" {
		t.Errorf("written = %q, want payload (Write should run before Close)", fake.written)
	}
}

//nolint:paralleltest // mutates package-level seams (chmodFn)
func TestAtomicWriteFileChmodError(t *testing.T) {
	dir := t.TempDir()
	useChmod(t, func(string, os.FileMode) error { return errInjected })

	target := filepath.Join(dir, "out")
	err := AtomicWriteFile(target, []byte("payload"), 0o644)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errInjected) {
		t.Errorf("err = %v, want errors.Is errInjected", err)
	}
	if !strings.HasPrefix(err.Error(), "fsx: chmod:") {
		t.Errorf("err prefix = %q, want fsx: chmod:", err.Error())
	}

	if _, statErr := os.Stat(target); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("target file should not exist after chmod failure, stat err = %v", statErr)
	}
	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		t.Fatalf("read dir: %v", readErr)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".wsd-tmp-") {
			t.Errorf("temp file leaked after chmod failure: %s", e.Name())
		}
	}
}
