// Package execcli implements the `cocoon exec` lifecycle verb.
//
// `cocoon exec` is a thin wrapper over `docker compose exec` against the
// generated stack. The full implementation lands in F3.
package execcli

import "errors"

// ErrUsage signals a bad invocation; mapped to exit 2 at the binary boundary.
var ErrUsage = errors.New("usage error")

// ErrFailure signals a runtime failure of `docker compose exec`.
var ErrFailure = errors.New("exec failed")
