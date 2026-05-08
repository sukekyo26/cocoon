// Package certificatescli implements the `wsd certificates` subcommand tree.
package certificatescli

import "errors"

// ErrUsage signals a bad invocation.
var ErrUsage = errors.New("usage error")

// ErrFailure signals a runtime error or absent valid certificates (for `check`).
var ErrFailure = errors.New("failure")
