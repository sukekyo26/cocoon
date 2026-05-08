// Package downcli implements the `cocoon down` lifecycle verb.
//
// `cocoon down` is a thin wrapper over `docker compose down` against the
// generated compose file. The full implementation lands in F3.
package downcli

import "errors"

// ErrUsage signals a bad invocation; mapped to exit 2 at the binary boundary.
var ErrUsage = errors.New("usage error")

// ErrFailure signals a runtime failure of `docker compose down`.
var ErrFailure = errors.New("down failed")
