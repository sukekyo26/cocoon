// Package initcli implements `cocoon init`, which creates a fresh
// workspace.toml in the current directory.
package initcli

import "errors"

// ErrUsage signals a bad invocation; mapped to exit 2 at the binary boundary.
var ErrUsage = errors.New("usage error")

// ErrFailure signals a runtime failure of `cocoon init`.
var ErrFailure = errors.New("init failed")
