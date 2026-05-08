package clean_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/sukekyo26/cocoon/internal/clean"
)

// fakeDocker is a thread-safe in-memory DockerClient used by tests.
type fakeDocker struct {
	mu              sync.Mutex
	infoErr         error
	volumes         []string
	removeFail      map[string]bool
	containers      map[string][]string // project -> container ids
	removedVolumes  []string
	removedContains []string
}

func (f *fakeDocker) Info(_ context.Context) error { return f.infoErr }

func (f *fakeDocker) VolumeNames(_ context.Context) ([]string, error) {
	out := make([]string, len(f.volumes))
	copy(out, f.volumes)
	return out, nil
}

func (f *fakeDocker) VolumeRemove(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.removeFail[name] {
		return fmt.Errorf("simulated failure removing %s", name)
	}
	f.removedVolumes = append(f.removedVolumes, name)
	return nil
}

func (f *fakeDocker) ContainersByProject(_ context.Context, project string) ([]string, error) {
	return f.containers[project], nil
}

func (f *fakeDocker) ContainerForceRemove(_ context.Context, ids []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removedContains = append(f.removedContains, ids...)
	return nil
}

type stubCatalog struct{}

func (stubCatalog) Msg(key string, args ...any) string {
	if len(args) == 0 {
		return key
	}
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = fmt.Sprint(a)
	}
	return key + ":" + strings.Join(parts, ",")
}

func writeEnvFile(t *testing.T, dir string) {
	t.Helper()
	content := "COMPOSE_PROJECT_NAME=myproj\nCONTAINER_SERVICE_NAME=dev\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
}

func newOpts(t *testing.T, dir string, fd *fakeDocker, stdin string) (clean.VolumesOptions, *bytes.Buffer) {
	t.Helper()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	return clean.VolumesOptions{
		WorkspaceDir: dir,
		Stdin:        strings.NewReader(stdin),
		Stdout:       stdout,
		Stderr:       stderr,
		Catalog:      stubCatalog{},
		Docker:       fd,
		AssumeYes:    false,
	}, stdout
}

func TestVolumesRunNoMatchingVolumes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeEnvFile(t, dir)
	fd := &fakeDocker{volumes: []string{"otherproj_dev_data"}}
	opts, stdout := newOpts(t, dir, fd, "")
	if err := clean.VolumesRun(context.Background(), opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "clean_no_volumes") {
		t.Fatalf("expected clean_no_volumes message, got: %s", stdout.String())
	}
}

func TestVolumesRunDeletesAndStopsContainers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeEnvFile(t, dir)
	fd := &fakeDocker{
		volumes:    []string{"myproj_dev_data", "myproj_dev_cache", "other_volume"},
		containers: map[string][]string{"myproj": {"c1", "c2"}},
	}
	opts, stdout := newOpts(t, dir, fd, "y\n")
	if err := clean.VolumesRun(context.Background(), opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fd.removedVolumes) != 2 {
		t.Fatalf("expected 2 volumes removed, got %v", fd.removedVolumes)
	}
	if len(fd.removedContains) != 2 {
		t.Fatalf("expected 2 containers removed, got %v", fd.removedContains)
	}
	if !strings.Contains(stdout.String(), "clean_all_deleted") {
		t.Fatalf("expected clean_all_deleted message, got: %s", stdout.String())
	}
}

func TestVolumesRunCancel(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeEnvFile(t, dir)
	fd := &fakeDocker{volumes: []string{"myproj_dev_data"}}
	opts, _ := newOpts(t, dir, fd, "n\n")
	err := clean.VolumesRun(context.Background(), opts)
	if !errors.Is(err, clean.ErrCanceled) {
		t.Fatalf("expected ErrCanceled, got %v", err)
	}
	if len(fd.removedVolumes) != 0 {
		t.Fatalf("expected no removals on cancel, got %v", fd.removedVolumes)
	}
}

func TestVolumesDefaults_FillsMissingFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeEnvFile(t, dir)
	// Provide just the bare minimum and a fake Docker; defaults should fill
	// the rest. AssumeYes avoids the prompt which uses Stdin.
	opts := clean.VolumesOptions{
		WorkspaceDir: dir,
		Catalog:      stubCatalog{},
		Docker:       &fakeDocker{volumes: nil},
		AssumeYes:    true,
	} //nolint:exhaustruct // intentionally minimal to exercise defaults.
	if err := clean.VolumesRun(context.Background(), opts); err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestVolumesRunPartial(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeEnvFile(t, dir)
	fd := &fakeDocker{
		volumes:    []string{"myproj_dev_a", "myproj_dev_b"},
		removeFail: map[string]bool{"myproj_dev_b": true},
	}
	opts, stdout := newOpts(t, dir, fd, "y\n")
	err := clean.VolumesRun(context.Background(), opts)
	if !errors.Is(err, clean.ErrPartial) {
		t.Fatalf("expected ErrPartial, got %v", err)
	}
	if !strings.Contains(stdout.String(), "clean_partial") {
		t.Fatalf("expected clean_partial message, got: %s", stdout.String())
	}
}
