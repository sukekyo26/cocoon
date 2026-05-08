// Package composex centralizes the `docker compose -f
// .devcontainer/docker-compose.yml ...` invocations the cocoon lifecycle
// verbs share. Keeping the path and the wiring in one place means adding
// a new verb is a one-liner and any future change to compose-file
// resolution lands in a single spot.
package composex

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// ComposePath is the canonical location of the generated compose file
// relative to the project root. Lifecycle verbs assume the working
// directory is the project root.
const ComposePath = ".devcontainer/docker-compose.yml"

// ErrComposeNotFound is returned when ComposePath does not exist on
// disk. Lifecycle verbs surface this as "run `cocoon gen` first".
var ErrComposeNotFound = errors.New("compose file not found")

// Run shells out to `docker compose -f ComposePath <args...>` with the
// supplied stdio. Returns ErrComposeNotFound (wrapped) when the
// generated compose file is missing, so callers can present a tidy
// "did you forget cocoon gen?" hint instead of the raw exec error.
//
// The supplied context.Context is used to interrupt the underlying
// docker compose call; pass context.Background() when the verb has no
// cancellation source of its own.
func Run(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, args ...string) error {
	if _, err := os.Stat(ComposePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s (run `cocoon gen` first)", ErrComposeNotFound, ComposePath)
		}
		return fmt.Errorf("stat %s: %w", ComposePath, err)
	}
	full := append([]string{"compose", "-f", ComposePath}, args...)
	// docker is the trusted CLI; args are validated by the verb-specific cmd.go.
	c := exec.CommandContext(ctx, "docker", full...) //nolint:gosec

	c.Stdin = stdin
	c.Stdout = stdout
	c.Stderr = stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("docker compose: %w", err)
	}
	return nil
}
