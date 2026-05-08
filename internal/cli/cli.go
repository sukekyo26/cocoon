// Package cli wires together the wsd command-line interface using
// spf13/cobra.
//
// Each subcommand lives in its own internal/cli/<sub> package and exposes a
// NewCommand(stdout, stderr) constructor. [newRootCommand] assembles those
// constructors into a cobra command tree; [App.Execute] is the binary
// boundary entry point.
package cli

import (
	"errors"
	"io"
)

// ErrUnknownCommand is kept for backwards compatibility with callers that
// matched on the legacy hand-written dispatcher's error sentinel. New code
// should prefer cobra's own error reporting (which contains "unknown command"
// in its message).
var ErrUnknownCommand = errors.New("unknown command")

// App is the root command-line application.
type App struct {
	stdout  io.Writer
	stderr  io.Writer
	version string
}

// New constructs an App with the given version string and output sinks.
func New(version string, stdout, stderr io.Writer) *App {
	return &App{
		stdout:  stdout,
		stderr:  stderr,
		version: version,
	}
}

// Execute dispatches a single invocation. It returns nil on success and an
// error suitable for printing to stderr on failure.
func (a *App) Execute(args []string) error {
	root := newRootCommand(a.version, a.stdout, a.stderr)
	root.SetArgs(args)
	//nolint:wrapcheck // sentinel errors from subcommands propagate verbatim for exit-code mapping.
	return root.Execute()
}
