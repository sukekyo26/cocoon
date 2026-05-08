package exec_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/sukekyo26/cocoon/internal/exec"
)

func TestRecordingRunnerCapturesCalls(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	if err := r.Run(context.Background(), "docker", "info"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := r.Output(context.Background(), "git", "rev-parse", "HEAD"); err != nil {
		t.Fatalf("Output: %v", err)
	}
	if len(r.Calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(r.Calls))
	}
	if r.Calls[0].Name != "docker" || r.Calls[0].Method != "Run" {
		t.Errorf("call[0] = %+v", r.Calls[0])
	}
	if r.Calls[1].Name != "git" || r.Calls[1].Method != "Output" {
		t.Errorf("call[1] = %+v", r.Calls[1])
	}
}

func TestRecordingRunnerStubOutput(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	r.Stub("docker", []string{"version", "--short"}, exec.Stub{Stdout: []byte("v25.0.3\n")})
	out, err := r.Output(context.Background(), "docker", "version", "--short")
	if err != nil {
		t.Fatalf("Output: %v", err)
	}
	if string(out) != "v25.0.3\n" {
		t.Errorf("got %q", out)
	}
}

var errStub = errors.New("stubbed failure")

func TestRecordingRunnerStubError(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	r.Stub("docker", []string{"info"}, exec.Stub{Err: errStub})
	if err := r.Run(context.Background(), "docker", "info"); !errors.Is(err, errStub) {
		t.Errorf("got %v, want errStub", err)
	}
}

func TestRecordingRunnerCombinedOutputDefault(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	r.Stub("sh", []string{"-c", "x"}, exec.Stub{Stdout: []byte("OUT"), Stderr: []byte("ERR")})
	out, err := r.CombinedOutput(context.Background(), "sh", "-c", "x")
	if err != nil {
		t.Fatalf("CombinedOutput: %v", err)
	}
	if string(out) != "OUTERR" {
		t.Errorf("got %q, want OUTERR", out)
	}
}

func TestRecordingRunnerRunWithIORoutesStreams(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	r.Stub("diff", []string{"a", "b"}, exec.Stub{Stdout: []byte("--- a\n+++ b\n")})
	var out bytes.Buffer
	err := r.RunWithIO(context.Background(), exec.RunOptions{
		Name: "diff", Args: []string{"a", "b"}, Stdout: &out,
	})
	if err != nil {
		t.Fatalf("RunWithIO: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("+++ b")) {
		t.Errorf("stdout not routed: %q", out.String())
	}
}
