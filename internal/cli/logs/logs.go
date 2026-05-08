// Package logscli implements the `cocoon logs` lifecycle verb.
//
// `cocoon logs` is a thin wrapper over `docker compose logs` against the
// generated stack. The full implementation lands in F3.
package logscli

import "errors"

// ErrUsage signals a bad invocation; mapped to exit 2 at the binary boundary.
var ErrUsage = errors.New("usage error")

// ErrFailure signals a runtime failure of `docker compose logs`.
var ErrFailure = errors.New("logs failed")
