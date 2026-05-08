// Package doctorcli implements the `wsd doctor` subcommand.
package doctorcli

import "errors"

// ErrUsage signals a bad invocation; mapped to exit 2 at the binary boundary.
var ErrUsage = errors.New("usage error")

// ErrFailure indicates at least one diagnostic failed; mapped to exit 1.
var ErrFailure = errors.New("doctor reported failures")
