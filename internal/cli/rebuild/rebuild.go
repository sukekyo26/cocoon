// Package rebuildcli implements the `wsd rebuild` subcommand.
package rebuildcli

import "errors"

// ErrUsage signals a bad invocation; mapped to exit 2 at the binary boundary.
var ErrUsage = errors.New("usage error")

// ErrInsideContainer signals the command was invoked from inside a container.
var ErrInsideContainer = errors.New("rebuild cannot run from inside a container")
