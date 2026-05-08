//nolint:testpackage // white-box test for unexported wrappers and defaults().
package rebuild

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/devcontainer"
	"github.com/sukekyo26/cocoon/internal/exec"
	"github.com/sukekyo26/cocoon/internal/exec/dockerx"
)

func TestDefaults_FillsNilFields(t *testing.T) {
	t.Parallel()
	got := defaults(Options{}) //nolint:exhaustruct // exercising the nil-defaults path
	if got.Stdin == nil || got.Stdout == nil || got.Stderr == nil {
		t.Fatalf("std streams should default to os.{Stdin,Stdout,Stderr}: %+v", got)
	}
	if got.Inspector == nil {
		t.Errorf("Inspector should default to non-nil")
	}
	if got.Runner == nil {
		t.Errorf("Runner should default to non-nil")
	}
	if got.Prereq == nil {
		t.Errorf("Prereq should default to non-nil")
	}
}

func TestDefaults_PreservesProvidedFields(t *testing.T) {
	t.Parallel()
	stdin := strings.NewReader("y\n")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	ins := &fakeInspectorWB{}
	run := &fakeRunnerWB{}
	pre := fakePrereqWB{}
	in := Options{
		WorkspaceDir: "/tmp",
		Stdin:        stdin,
		Stdout:       stdout,
		Stderr:       stderr,
		Catalog:      nil,
		Inspector:    ins,
		Runner:       run,
		Prereq:       pre,
		AssumeYes:    true,
	}
	got := defaults(in)
	if got.Stdin != stdin || got.Stdout != stdout || got.Stderr != stderr {
		t.Errorf("std streams should not be replaced when set")
	}
	if got.Inspector != ins {
		t.Errorf("Inspector should not be replaced when set")
	}
	if got.Runner != run {
		t.Errorf("Runner should not be replaced when set")
	}
	if got.Prereq != pre {
		t.Errorf("Prereq should not be replaced when set")
	}
}

// fakeInspectorWB / fakeRunnerWB / fakePrereqWB are minimal package-internal
// fakes used by the white-box tests in this file. Tests in rebuild_test.go
// (package rebuild_test) define their own equivalents because they cannot
// reach into unexported types.
type fakeInspectorWB struct{}

func (fakeInspectorWB) Created(_ context.Context, _ string) (string, bool) { return "", false }

type fakeRunnerWB struct{}

func (*fakeRunnerWB) Up(_ []string, _, _ io.Writer) error { return nil }

type fakePrereqWB struct{}

func (fakePrereqWB) Check(_ string, _ io.Writer) error { return nil }

// TestExecPrereq_WrapsNonOK exercises execPrereq.Check, the production wrapper
// around devcontainer.CheckPrerequisites. We force LookPath("docker") to fail
// by clearing PATH, which makes CheckPrerequisites return the first non-OK
// enum (PrereqMissingDocker) without ever calling the runner. The test only
// asserts the wrap shape — it does not depend on which non-OK code triggers.
func TestExecPrereq_WrapsNonOK(t *testing.T) {
	t.Setenv("PATH", "")
	t.Setenv("WSL_DISTRO_NAME", "")
	out := &bytes.Buffer{}
	p := execPrereq{runner: exec.NewRecordingRunner()}
	err := p.Check(t.TempDir(), out)
	if err == nil {
		t.Fatalf("expected non-nil error when prereqs fail")
	}
	if !errors.Is(err, errPrereqResult) {
		t.Errorf("expected errors.Is(err, errPrereqResult); got %v", err)
	}
}

func TestExecInspector_Created(t *testing.T) {
	t.Parallel()
	ts := "2024-01-15T10:30:45.123Z"
	cases := []struct {
		name      string
		stub      exec.Stub
		wantValue string
		wantOK    bool
	}{
		{
			name:      "success",
			stub:      exec.Stub{Stdout: []byte(ts + "\n"), Stderr: nil, CombinedOutput: nil, Err: nil},
			wantValue: ts,
			wantOK:    true,
		},
		{
			name:      "error_returns_empty_false",
			stub:      exec.Stub{Stdout: nil, Stderr: nil, CombinedOutput: nil, Err: errors.New("inspect failed")},
			wantValue: "",
			wantOK:    false,
		},
		{
			name:      "empty_output_returns_empty_false",
			stub:      exec.Stub{Stdout: []byte("\n"), Stderr: nil, CombinedOutput: nil, Err: nil},
			wantValue: "",
			wantOK:    false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := exec.NewRecordingRunner()
			r.Stub("docker", []string{"image", "inspect", "img:tag", "--format", "{{.Created}}"}, tc.stub)
			ins := execInspector{docker: dockerx.New(r)}
			got, ok := ins.Created(context.Background(), "img:tag")
			if got != tc.wantValue || ok != tc.wantOK {
				t.Errorf("Created = (%q, %v), want (%q, %v)", got, ok, tc.wantValue, tc.wantOK)
			}
		})
	}
}

// alwaysErrRunner returns the same error from every method regardless of
// arguments. Used to bypass exec.RecordingRunner's exact-args stub matching
// when devcontainer.Up rewrites args (e.g. the WSL workaround appends
// --docker-path). The Runner contract only specifies that errors propagate,
// so this minimal fake is enough to exercise wrapper behaviour.
type alwaysErrRunner struct{ err error }

func (a alwaysErrRunner) Run(_ context.Context, _ string, _ ...string) error { return a.err }
func (a alwaysErrRunner) Output(_ context.Context, _ string, _ ...string) ([]byte, error) {
	return nil, a.err
}

func (a alwaysErrRunner) CombinedOutput(_ context.Context, _ string, _ ...string) ([]byte, error) {
	return nil, a.err
}
func (a alwaysErrRunner) RunWithIO(_ context.Context, _ exec.RunOptions) error { return a.err }

// TestDevcontainerRunner_UpWraps verifies the "devcontainer up: ..." prefix is
// applied to runner errors. On WSL hosts without /usr/bin/docker, devcontainer.Up
// short-circuits at exec.LookPath("docker") with ErrDockerMissing before the
// runner is called — handle both branches.
func TestDevcontainerRunner_UpWraps(t *testing.T) {
	t.Parallel()
	bootErr := errors.New("boot failed")
	d := devcontainerRunner{runner: alwaysErrRunner{err: bootErr}}
	err := d.Up([]string{"up", "--workspace-folder", "/ws"}, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if errors.Is(err, devcontainer.ErrDockerMissing) {
		// WSL without docker on PATH — Up short-circuited before our fake runner
		// was invoked. The wrap target (runner-error path) is unreachable here;
		// covered on non-WSL CI runners.
		return
	}
	if !strings.Contains(err.Error(), "devcontainer up:") {
		t.Errorf("expected wrap with 'devcontainer up:' prefix; got %v", err)
	}
	if !errors.Is(err, bootErr) {
		t.Errorf("expected errors.Is(err, bootErr); got %v", err)
	}
}

func TestDevcontainerRunner_UpSuccess(t *testing.T) {
	t.Parallel()
	d := devcontainerRunner{runner: alwaysErrRunner{err: nil}}
	err := d.Up([]string{"up", "--workspace-folder", "/ws"}, io.Discard, io.Discard)
	if err != nil {
		if errors.Is(err, devcontainer.ErrDockerMissing) {
			return // WSL without docker — see comment in TestDevcontainerRunner_UpWraps
		}
		t.Errorf("expected nil error on successful runner; got %v", err)
	}
}
