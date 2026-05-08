// Package dockerx provides typed wrappers around the docker CLI on top of
// [exec.Runner]. Methods take typed arguments (image name, project name,
// compose file path) so call sites do not assemble shell strings; this
// removes the shell-injection risk and lets tests stub responses with a
// [exec.RecordingRunner].
package dockerx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/sukekyo26/cocoon/internal/exec"
)

// ErrUnknownPruneKind is returned by [Client.Prune] for unrecognised PruneKind values.
var ErrUnknownPruneKind = errors.New("dockerx: unknown prune kind")

// Client wraps docker CLI invocations behind a [exec.Runner].
type Client struct {
	Runner exec.Runner
	// DefaultTimeout is applied via context.WithTimeout when the caller's
	// ctx has no deadline. Zero disables the default.
	DefaultTimeout time.Duration
}

// New returns a Client that delegates to runner with a 30s default timeout.
func New(runner exec.Runner) *Client {
	return &Client{Runner: runner, DefaultTimeout: 30 * time.Second}
}

// withTimeout applies DefaultTimeout when ctx has no deadline.
func (c *Client) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.DefaultTimeout <= 0 {
		return ctx, func() {}
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, c.DefaultTimeout)
}

// PruneKind enumerates `docker <kind> prune` targets supported by [Client.Prune].
type PruneKind int

// Supported prune subcommands.
const (
	PruneContainers PruneKind = iota
	PruneBuilder
	PruneImages
	PruneNetworks
	PruneVolumes
)

func (k PruneKind) subcommand() (string, bool) {
	switch k {
	case PruneContainers:
		return "container", true
	case PruneBuilder:
		return "builder", true
	case PruneImages:
		return "image", true
	case PruneNetworks:
		return "network", true
	case PruneVolumes:
		return "volume", true
	default:
		return "", false
	}
}

// Info runs `docker info` and returns nil if the daemon responded.
func (c *Client) Info(ctx context.Context) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	if err := c.Runner.Run(ctx, "docker", "info"); err != nil {
		return fmt.Errorf("docker info: %w", err)
	}
	return nil
}

// InfoCombinedOutput is like [Client.Info] but returns merged stdout+stderr
// for diagnostics.
func (c *Client) InfoCombinedOutput(ctx context.Context) ([]byte, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	out, err := c.Runner.CombinedOutput(ctx, "docker", "info")
	if err != nil {
		return out, fmt.Errorf("docker info: %w", err)
	}
	return out, nil
}

// VolumeNames returns the names of all docker volumes on the host.
func (c *Client) VolumeNames(ctx context.Context) ([]string, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	out, err := c.Runner.Output(ctx, "docker", "volume", "ls", "--format", "{{.Name}}")
	if err != nil {
		return nil, fmt.Errorf("docker volume ls: %w", err)
	}
	return splitLines(out), nil
}

// VolumeRemove deletes a single docker volume by name.
func (c *Client) VolumeRemove(ctx context.Context, name string) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	if err := c.Runner.Run(ctx, "docker", "volume", "rm", name); err != nil {
		return fmt.Errorf("docker volume rm %s: %w", name, err)
	}
	return nil
}

// ContainersByProject returns container IDs labelled for the given compose project.
func (c *Client) ContainersByProject(ctx context.Context, project string) ([]string, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	out, err := c.Runner.Output(ctx, "docker", "ps", "-aq",
		"--filter", "label=com.docker.compose.project="+project)
	if err != nil {
		return nil, fmt.Errorf("docker ps: %w", err)
	}
	return splitLines(out), nil
}

// ContainerForceRemove force-removes the given container IDs.
func (c *Client) ContainerForceRemove(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	args := append([]string{"rm", "-f"}, ids...)
	if err := c.Runner.Run(ctx, "docker", args...); err != nil {
		return fmt.Errorf("docker rm -f: %w", err)
	}
	return nil
}

// SystemDF runs `docker system df` writing to w.
func (c *Client) SystemDF(ctx context.Context, w io.Writer) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	err := c.Runner.RunWithIO(ctx, exec.RunOptions{
		Name: "docker", Args: []string{"system", "df"},
		Stdin: nil, Stdout: w, Stderr: w, Env: nil, Dir: "",
	})
	if err != nil {
		return fmt.Errorf("docker system df: %w", err)
	}
	return nil
}

// Prune runs `docker <kind> prune -f` writing to w.
func (c *Client) Prune(ctx context.Context, kind PruneKind, w io.Writer) error {
	sub, ok := kind.subcommand()
	if !ok {
		return fmt.Errorf("%w: %d", ErrUnknownPruneKind, int(kind))
	}
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	err := c.Runner.RunWithIO(ctx, exec.RunOptions{
		Name: "docker", Args: []string{sub, "prune", "-f"},
		Stdin: nil, Stdout: w, Stderr: w, Env: nil, Dir: "",
	})
	if err != nil {
		return fmt.Errorf("docker %s prune: %w", sub, err)
	}
	return nil
}

// ImageInspectFormat returns `docker image inspect <image> --format <format>` stdout, trimmed.
func (c *Client) ImageInspectFormat(ctx context.Context, image, format string) (string, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	out, err := c.Runner.Output(ctx, "docker", "image", "inspect", image, "--format", format)
	if err != nil {
		return "", fmt.Errorf("docker image inspect %s: %w", image, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ComposeConfig runs `docker compose -f <file> config -q`.
func (c *Client) ComposeConfig(ctx context.Context, file string) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	if err := c.Runner.Run(ctx, "docker", "compose", "-f", file, "config", "-q"); err != nil {
		return fmt.Errorf("docker compose -f %s config: %w", file, err)
	}
	return nil
}

// ComposeVersion runs `docker compose version` returning nil if the plugin is installed.
func (c *Client) ComposeVersion(ctx context.Context) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	if err := c.Runner.Run(ctx, "docker", "compose", "version"); err != nil {
		return fmt.Errorf("docker compose version: %w", err)
	}
	return nil
}

// ComposeVersionShort returns the trimmed output of `docker compose version --short`.
func (c *Client) ComposeVersionShort(ctx context.Context) (string, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	out, err := c.Runner.Output(ctx, "docker", "compose", "version", "--short")
	if err != nil {
		return "", fmt.Errorf("docker compose version --short: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ComposePS runs `docker compose ps --format <format>` returning trimmed lines.
func (c *Client) ComposePS(ctx context.Context, format string) ([]string, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	out, err := c.Runner.Output(ctx, "docker", "compose", "ps", "--format", format)
	if err != nil {
		return nil, fmt.Errorf("docker compose ps: %w", err)
	}
	return splitLines(out), nil
}

// RunBash invokes `docker run --rm --entrypoint /bin/bash <image> -c <script>`.
//
// Caller MUST shell-quote any user-derived value embedded in script (use
// shellx.ShellQuote); this method does not parse the script.
func (c *Client) RunBash(ctx context.Context, image, script string) (string, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	out, err := c.Runner.Output(ctx, "docker", "run", "--rm",
		"--entrypoint", "/bin/bash", image, "-c", script)
	if err != nil {
		return strings.TrimSpace(string(out)), fmt.Errorf("docker run --rm %s: %w", image, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func splitLines(b []byte) []string {
	s := strings.TrimSpace(string(b))
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
