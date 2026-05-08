// Package devcontainercli implements the `wsd devcontainer` subcommand tree.
package devcontainercli

import "errors"

// Sentinel errors mapped to specific exit codes at the binary boundary.
// The bash wrapper inspects the exit code to decide whether to offer the
// interactive devcontainer-CLI installer (exit code 3 = devcontainer CLI
// missing).
var (
	ErrUsage          = errors.New("usage error")
	ErrFailure        = errors.New("failure")
	ErrMissingDocker  = errors.New("docker missing")
	ErrMissingDcCLI   = errors.New("devcontainer CLI missing")
	ErrMissingDcJSON  = errors.New("devcontainer.json missing")
	ErrMissingEnvFile = errors.New(".env missing")
)
