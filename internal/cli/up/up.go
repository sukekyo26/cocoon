// Package upcli implements the `cocoon up` lifecycle verb.
//
// `cocoon up` regenerates the workspace artifacts (Dockerfile, docker-compose
// .yml, devcontainer.json) under .devcontainer/ when their inputs changed,
// then runs `docker compose up -d --build` against the generated compose
// file. The full implementation lands in F3 once the .devcontainer/-centred
// layout is in place.
package upcli

import "errors"

// ErrUsage signals a bad invocation; mapped to exit 2 at the binary boundary.
var ErrUsage = errors.New("usage error")

// ErrFailure signals a runtime failure of the underlying compose stack.
var ErrFailure = errors.New("up failed")
