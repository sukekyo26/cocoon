// Package configcli implements the `wsd config` subcommand tree.
//
// Each public function corresponds to one Python Typer subcommand in
// src/wsd/cli/config.py and writes its output to the supplied stdout/stderr
// writers. Exit codes are surfaced via returned errors so callers can map
// them to os.Exit at the binary boundary.
package configcli

import (
	"errors"
	"fmt"
	"io"
)

// ErrUsage signals a usage error (missing argument, unknown subcommand) and
// maps to exit code 2 at the binary boundary.
var ErrUsage = errors.New("usage error")

// ErrFailure signals a runtime failure (validation failure, missing file)
// that should map to exit code 1.
var ErrFailure = errors.New("failure")

func requireArgs(args []string, want int, name string, stderr io.Writer) error {
	if len(args) < want {
		_, _ = fmt.Fprintf(stderr, "ERROR: %s requires %d argument(s), got %d\n", name, want, len(args))
		return ErrUsage
	}
	return nil
}

func decodeRaw(path string) (map[string]any, error) {
	return decodeRawFile(path)
}
