package clihelpers

import "errors"

// ErrUsage signals a bad invocation (missing argument, unknown subcommand,
// mutually exclusive flags). The binary boundary maps it to exit code 2.
var ErrUsage = errors.New("usage error")

// ErrFailure signals a runtime failure of a cocoon subcommand (validation
// failure, write error, generation error). The binary boundary maps it to
// exit code 1.
var ErrFailure = errors.New("failure")

// ErrCanceled is returned when the user aborts an interactive prompt
// (Ctrl-C / Esc). The binary boundary maps it to exit code 130.
var ErrCanceled = errors.New("canceled")
