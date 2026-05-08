// Package cleancli implements the `wsd clean <subcommand>` router.
package cleancli

import "errors"

// ErrUsage signals a bad invocation; mapped to exit 2 at the binary boundary.
var ErrUsage = errors.New("usage error")

// ErrInsideContainer signals the command was invoked from inside a container.
var ErrInsideContainer = errors.New("clean cannot run from inside a container")
