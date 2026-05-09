// Package initcli implements the `cocoon init` lifecycle verb.
//
// `cocoon init` interactively creates a fresh workspace.toml in the current
// directory. The user is asked for a mount range (cwd vs parent), whether to
// generate .devcontainer/devcontainer.json, and which categories of common
// apt packages to pre-populate. Each prompt has a non-interactive flag
// counterpart so the command works under --yes for CI usage.
//
// The full `[workspace]` schema (mount_root, devcontainer keys) is wired
// into config-loading in F3; for now this command produces the file but
// `cocoon up` still relies on the legacy [container] block to drive the
// generators.
package initcli

import "errors"

// ErrUsage signals a bad invocation; mapped to exit 2 at the binary boundary.
var ErrUsage = errors.New("usage error")

// ErrFailure signals a runtime failure of `cocoon init`.
var ErrFailure = errors.New("init failed")
