package exec

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

// Runner is the single seam every external-command invocation goes through.
type Runner interface {
	// Run executes the command and returns nil on success. stdout/stderr are
	// discarded.
	Run(ctx context.Context, name string, args ...string) error

	// Output executes the command and returns its stdout. The error wraps
	// the underlying *exec.ExitError so callers can use errors.As.
	Output(ctx context.Context, name string, args ...string) ([]byte, error)

	// CombinedOutput returns stdout+stderr merged.
	CombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error)

	// RunWithIO is the escape hatch for callers that need fine-grained
	// control over stdin/stdout/stderr/env/working-directory.
	RunWithIO(ctx context.Context, opts RunOptions) error
}

// RunOptions configures a single [Runner.RunWithIO] invocation.
type RunOptions struct {
	Name   string
	Args   []string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Env    []string
	Dir    string
}

// New returns a [Runner] that delegates to os/exec.
func New() Runner { return realRunner{} }

type realRunner struct{}

// Run implements [Runner].
func (realRunner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // caller is responsible for arg safety
	if err := cmd.Run(); err != nil {
		return wrapErr(name, err)
	}
	return nil
}

// Output implements [Runner].
func (realRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // caller is responsible for arg safety
	out, err := cmd.Output()
	if err != nil {
		return out, wrapErr(name, err)
	}
	return out, nil
}

// CombinedOutput implements [Runner].
func (realRunner) CombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // caller is responsible for arg safety
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, wrapErr(name, err)
	}
	return out, nil
}

// RunWithIO implements [Runner].
func (realRunner) RunWithIO(ctx context.Context, opts RunOptions) error {
	cmd := exec.CommandContext(ctx, opts.Name, opts.Args...) //nolint:gosec // caller is responsible for arg safety
	cmd.Stdin = opts.Stdin
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr
	if len(opts.Env) > 0 {
		cmd.Env = opts.Env
	}
	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}
	if err := cmd.Run(); err != nil {
		return wrapErr(opts.Name, err)
	}
	return nil
}

func wrapErr(name string, err error) error {
	return fmt.Errorf("%s: %w", name, err)
}
