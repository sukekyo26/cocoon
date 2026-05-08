// Package repositoriescli implements the `wsd repositories` subcommand tree.
package repositoriescli

import "errors"

// ErrUsage signals a bad invocation.
var ErrUsage = errors.New("usage error")

// ErrFailure signals a runtime error (I/O, TOML parse).
var ErrFailure = errors.New("failure")
