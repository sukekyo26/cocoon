// Package setupcli implements the `wsd setup` subcommand.
package setupcli

import "errors"

// ErrUsage signals a bad invocation; mapped to exit 2 at the binary boundary.
var ErrUsage = errors.New("usage error")
