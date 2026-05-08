package rebuild_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sukekyo26/cocoon/internal/rebuild"
)

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

type fakeInspector struct {
	created    string
	exists     bool
	afterRun   string
	afterExist bool
	calls      int
}

func (f *fakeInspector) Created(_ context.Context, _ string) (string, bool) {
	f.calls++
	if f.calls == 1 {
		return f.created, f.exists
	}
	return f.afterRun, f.afterExist
}

type fakeRunner struct {
	args []string
	err  error
}

func (f *fakeRunner) Up(args []string, _, _ io.Writer) error {
	f.args = append([]string(nil), args...)
	return f.err
}

type fakePrereq struct{ err error }

func (f fakePrereq) Check(_ string, _ io.Writer) error { return f.err }

func writeWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".devcontainer"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".devcontainer", "devcontainer.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("write devcontainer.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("CONTAINER_SERVICE_NAME=dev\n"), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}
	return dir
}

// Note: rebuild.Run still calls devcontainer.CheckPrerequisites which checks
// docker / devcontainer CLI presence; tests that run on CI without those
// dependencies will fail at the prereq stage. We only assert error mapping.

func TestRunMissingWorkspace(t *testing.T) {
	t.Parallel()
	out := &bytes.Buffer{}
	//nolint:exhaustruct // optional fields default inside Run.
	err := rebuild.Run(context.Background(), rebuild.Options{
		Stdout: out, Stderr: io.Discard, Catalog: stubCatalog{},
		Inspector: &fakeInspector{}, Runner: &fakeRunner{},
	})
	if !errors.Is(err, rebuild.ErrConfig) {
		t.Fatalf("expected ErrConfig, got %v", err)
	}
}

func TestRunMissingCatalog(t *testing.T) {
	t.Parallel()
	out := &bytes.Buffer{}
	//nolint:exhaustruct // optional fields default inside Run.
	err := rebuild.Run(context.Background(), rebuild.Options{
		WorkspaceDir: t.TempDir(),
		Stdout:       out, Stderr: io.Discard,
		Inspector: &fakeInspector{}, Runner: &fakeRunner{},
	})
	if !errors.Is(err, rebuild.ErrConfig) {
		t.Fatalf("expected ErrConfig, got %v", err)
	}
}

func TestRunCanceled(t *testing.T) {
	t.Parallel()
	dir := writeWorkspace(t)
	out := &bytes.Buffer{}
	runner := &fakeRunner{}
	//nolint:exhaustruct // optional fields default inside Run.
	opts := rebuild.Options{
		WorkspaceDir: dir,
		Stdin:        strings.NewReader("n\n"),
		Stdout:       out, Stderr: io.Discard,
		Catalog:   stubCatalog{},
		Inspector: &fakeInspector{exists: false},
		Runner:    runner,
		Prereq:    fakePrereq{},
	}
	err := rebuild.Run(context.Background(), opts)
	if !errors.Is(err, rebuild.ErrCanceled) {
		t.Fatalf("expected ErrCanceled, got %v", err)
	}
	if len(runner.args) != 0 {
		t.Errorf("runner should not have been called")
	}
	if !strings.Contains(out.String(), "rebuild_cancelled") {
		t.Errorf("missing cancelled msg: %s", out.String())
	}
}

func TestRunSuccess(t *testing.T) {
	t.Parallel()
	dir := writeWorkspace(t)
	out := &bytes.Buffer{}
	created := time.Now().Add(-48 * time.Hour).Format(time.RFC3339Nano)
	newCreated := time.Now().Format(time.RFC3339Nano)
	ins := &fakeInspector{created: created, exists: true, afterRun: newCreated, afterExist: true}
	runner := &fakeRunner{}
	//nolint:exhaustruct // optional fields default inside Run.
	opts := rebuild.Options{
		WorkspaceDir: dir,
		Stdout:       out, Stderr: io.Discard,
		Catalog:   stubCatalog{},
		Inspector: ins,
		Runner:    runner,
		Prereq:    fakePrereq{},
		AssumeYes: true,
	}
	if err := rebuild.Run(context.Background(), opts); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(runner.args) < 4 {
		t.Fatalf("runner args too short: %v", runner.args)
	}
	if runner.args[0] != "up" {
		t.Errorf("expected up subcommand, got %v", runner.args)
	}
	wantFlags := map[string]bool{"--build-no-cache": false, "--remove-existing-container": false}
	for _, a := range runner.args {
		if _, ok := wantFlags[a]; ok {
			wantFlags[a] = true
		}
	}
	for f, found := range wantFlags {
		if !found {
			t.Errorf("missing flag %q in %v", f, runner.args)
		}
	}
	if !strings.Contains(out.String(), "rebuild_complete") {
		t.Errorf("missing complete msg: %s", out.String())
	}
	if !strings.Contains(out.String(), "rebuild_new_image") {
		t.Errorf("missing new image msg: %s", out.String())
	}
}

func TestRunPrereqFailure(t *testing.T) {
	t.Parallel()
	dir := writeWorkspace(t)
	out := &bytes.Buffer{}
	//nolint:exhaustruct // optional fields default inside Run.
	opts := rebuild.Options{
		WorkspaceDir: dir,
		Stdout:       out, Stderr: io.Discard,
		Catalog:   stubCatalog{},
		Inspector: &fakeInspector{},
		Runner:    &fakeRunner{},
		Prereq:    fakePrereq{err: errors.New("docker missing")},
	}
	err := rebuild.Run(context.Background(), opts)
	if !errors.Is(err, rebuild.ErrPrereq) {
		t.Fatalf("expected ErrPrereq, got %v", err)
	}
	if got := err.Error(); strings.Count(got, "prerequisite check failed") != 1 {
		t.Errorf("ErrPrereq message wrapped more than once: %q", got)
	}
}

func TestRunUpFailure(t *testing.T) {
	t.Parallel()
	dir := writeWorkspace(t)
	out := &bytes.Buffer{}
	runner := &fakeRunner{err: errors.New("up exploded")}
	//nolint:exhaustruct // optional fields default inside Run.
	opts := rebuild.Options{
		WorkspaceDir: dir,
		Stdout:       out, Stderr: io.Discard,
		Catalog:   stubCatalog{},
		Inspector: &fakeInspector{},
		Runner:    runner,
		Prereq:    fakePrereq{},
		AssumeYes: true,
	}
	err := rebuild.Run(context.Background(), opts)
	if !errors.Is(err, rebuild.ErrFailure) {
		t.Fatalf("expected ErrFailure, got %v", err)
	}
}

func TestRunSuccessNoNewImage(t *testing.T) {
	t.Parallel()
	dir := writeWorkspace(t)
	out := &bytes.Buffer{}
	// First call: image exists. Second call (after rebuild): image gone.
	ins := &fakeInspector{
		created: time.Now().Format(time.RFC3339Nano), exists: true,
		afterRun: "", afterExist: false,
	}
	runner := &fakeRunner{}
	//nolint:exhaustruct // optional fields default inside Run.
	opts := rebuild.Options{
		WorkspaceDir: dir,
		Stdout:       out, Stderr: io.Discard,
		Catalog:   stubCatalog{},
		Inspector: ins,
		Runner:    runner,
		Prereq:    fakePrereq{},
		AssumeYes: true,
	}
	if err := rebuild.Run(context.Background(), opts); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if strings.Contains(out.String(), "rebuild_new_image") {
		t.Errorf("rebuild_new_image should be skipped when post-rebuild inspect fails")
	}
}

func TestRunConfirmReadError(t *testing.T) {
	t.Parallel()
	dir := writeWorkspace(t)
	out := &bytes.Buffer{}
	runner := &fakeRunner{}
	//nolint:exhaustruct // optional fields default inside Run.
	opts := rebuild.Options{
		WorkspaceDir: dir,
		Stdin:        iotest{err: errors.New("read fail")},
		Stdout:       out, Stderr: io.Discard,
		Catalog:   stubCatalog{},
		Inspector: &fakeInspector{},
		Runner:    runner,
		Prereq:    fakePrereq{},
	}
	err := rebuild.Run(context.Background(), opts)
	if err == nil || !strings.Contains(err.Error(), "read confirm") {
		t.Errorf("expected read-confirm error, got %v", err)
	}
}

type iotest struct{ err error }

func (i iotest) Read(_ []byte) (int, error) { return 0, i.err }
