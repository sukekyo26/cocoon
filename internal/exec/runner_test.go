package exec_test

import (
	"bytes"
	"context"
	"errors"
	osexec "os/exec"
	"strings"
	"testing"
	"time"

	"github.com/sukekyo26/cocoon/internal/exec"
)

func TestRunnerOutput(t *testing.T) {
	t.Parallel()
	r := exec.New()
	out, err := r.Output(context.Background(), "echo", "hello world")
	if err != nil {
		t.Fatalf("Output: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}
}

func TestRunnerRun(t *testing.T) {
	t.Parallel()
	r := exec.New()
	if err := r.Run(context.Background(), "true"); err != nil {
		t.Errorf("true: %v", err)
	}
}

func TestRunnerRunNonZeroExitWrapsExitError(t *testing.T) {
	t.Parallel()
	r := exec.New()
	err := r.Run(context.Background(), "false")
	if err == nil {
		t.Fatal("expected error from `false`")
	}
	var exitErr *osexec.ExitError
	if !errors.As(err, &exitErr) {
		t.Errorf("error is not *exec.ExitError; got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "false:") {
		t.Errorf("error message should be prefixed with command name: %v", err)
	}
}

func TestRunnerCombinedOutput(t *testing.T) {
	t.Parallel()
	r := exec.New()
	out, err := r.CombinedOutput(context.Background(), "sh", "-c", "echo out; echo err 1>&2")
	if err != nil {
		t.Fatalf("CombinedOutput: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "out") || !strings.Contains(s, "err") {
		t.Errorf("expected stdout+stderr merged; got %q", s)
	}
}

func TestRunnerRunWithIO(t *testing.T) {
	t.Parallel()
	r := exec.New()
	var stdout, stderr bytes.Buffer
	err := r.RunWithIO(context.Background(), exec.RunOptions{
		Name:   "sh",
		Args:   []string{"-c", "echo stdout; echo stderr 1>&2"},
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		t.Fatalf("RunWithIO: %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "stdout" {
		t.Errorf("stdout: got %q, want %q", got, "stdout")
	}
	if got := strings.TrimSpace(stderr.String()); got != "stderr" {
		t.Errorf("stderr: got %q, want %q", got, "stderr")
	}
}

func TestRunnerCancellation(t *testing.T) {
	t.Parallel()
	r := exec.New()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := r.Run(ctx, "sleep", "5")
	if err == nil {
		t.Fatal("expected ctx deadline error")
	}
}
