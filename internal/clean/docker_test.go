package clean_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/clean"
)

type fakePruner struct {
	dfErr    error
	pruneErr map[clean.PruneKind]error
	pruned   []clean.PruneKind
}

func (f *fakePruner) SystemDF(_ context.Context, w io.Writer) error {
	if f.dfErr != nil {
		return f.dfErr
	}
	if _, err := w.Write([]byte("DISK\n")); err != nil {
		return err
	}
	return nil
}

func (f *fakePruner) Prune(_ context.Context, k clean.PruneKind, w io.Writer) error {
	f.pruned = append(f.pruned, k)
	if err, ok := f.pruneErr[k]; ok && err != nil {
		return err
	}
	if _, err := w.Write([]byte("PRUNED\n")); err != nil {
		return err
	}
	return nil
}

type fakeSelector struct {
	picked []int
	err    error
}

func (f fakeSelector) SelectMulti(_ string, _ []string, _ []int) ([]int, error) {
	return f.picked, f.err
}

func newDockerOpts(picked []int, selErr error, p *fakePruner) (clean.DockerOptions, *bytes.Buffer) {
	stdout := &bytes.Buffer{}
	return clean.DockerOptions{
		Stdout:   stdout,
		Stderr:   &bytes.Buffer{},
		Catalog:  stubCatalog{},
		Pruner:   p,
		Selector: fakeSelector{picked: picked, err: selErr},
	}, stdout
}

func TestDockerCleanRunAllSucceed(t *testing.T) {
	t.Parallel()
	p := &fakePruner{pruneErr: map[clean.PruneKind]error{}}
	opts, out := newDockerOpts([]int{0, 2}, nil, p)
	if err := clean.DockerCleanRun(context.Background(), opts); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if got := len(p.pruned); got != 2 {
		t.Fatalf("expected 2 prune calls, got %d", got)
	}
	if !strings.Contains(out.String(), "docker_clean_all_done:2") {
		t.Errorf("missing all-done summary: %s", out.String())
	}
}

func TestDockerCleanRunPartial(t *testing.T) {
	t.Parallel()
	failErr := errors.New("boom")
	p := &fakePruner{pruneErr: map[clean.PruneKind]error{clean.PruneImages: failErr}}
	opts, out := newDockerOpts([]int{0, 2}, nil, p)
	err := clean.DockerCleanRun(context.Background(), opts)
	if !errors.Is(err, clean.ErrPartial) {
		t.Fatalf("expected ErrPartial, got %v", err)
	}
	if !strings.Contains(out.String(), "docker_clean_partial_done:1,1") {
		t.Errorf("missing partial summary: %s", out.String())
	}
}

func TestDockerCleanRunCancelled(t *testing.T) {
	t.Parallel()
	p := &fakePruner{pruneErr: map[clean.PruneKind]error{}}
	opts, _ := newDockerOpts(nil, errors.New("user aborted"), p)
	err := clean.DockerCleanRun(context.Background(), opts)
	if !errors.Is(err, clean.ErrCanceled) {
		t.Fatalf("expected ErrCanceled, got %v", err)
	}
	if len(p.pruned) != 0 {
		t.Errorf("no prune should have run")
	}
}

func TestDockerCleanRunEmptySelection(t *testing.T) {
	t.Parallel()
	p := &fakePruner{pruneErr: map[clean.PruneKind]error{}}
	opts, _ := newDockerOpts([]int{}, nil, p)
	if err := clean.DockerCleanRun(context.Background(), opts); err != nil {
		t.Fatalf("expected nil for empty selection, got %v", err)
	}
	if len(p.pruned) != 0 {
		t.Errorf("no prune should have run for empty selection")
	}
}
