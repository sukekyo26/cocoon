// Package gitx provides a typed wrapper around `git clone` on top of
// [exec.Runner]. URLs / paths / branch names are passed as separate
// arguments so the shell never sees a composed string.
package gitx

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/sukekyo26/cocoon/internal/exec"
)

// CloneOptions configures a single [Client.Clone] invocation.
type CloneOptions struct {
	URL               string
	Target            string
	Branch            string // empty: server default
	Depth             int    // 0: full history
	RecurseSubmodules bool
}

// Client wraps git CLI invocations behind a [exec.Runner].
type Client struct {
	Runner exec.Runner
	// DefaultTimeout applies when the caller's ctx has no deadline.
	DefaultTimeout time.Duration
}

// New returns a Client that delegates to runner with a 5-minute default timeout.
func New(runner exec.Runner) *Client {
	return &Client{Runner: runner, DefaultTimeout: 5 * time.Minute}
}

func (c *Client) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.DefaultTimeout <= 0 {
		return ctx, func() {}
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, c.DefaultTimeout)
}

// Clone runs `git clone` with the given options. Returns CombinedOutput so
// the caller can surface git's verbose progress / error messages.
func (c *Client) Clone(ctx context.Context, opts CloneOptions) ([]byte, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	args := []string{"clone"}
	if opts.Branch != "" {
		args = append(args, "--branch", opts.Branch)
	}
	if opts.Depth > 0 {
		args = append(args, "--depth", strconv.Itoa(opts.Depth))
	}
	if opts.RecurseSubmodules {
		args = append(args, "--recurse-submodules")
	}
	args = append(args, opts.URL, opts.Target)
	out, err := c.Runner.CombinedOutput(ctx, "git", args...)
	if err != nil {
		return out, fmt.Errorf("git clone %s: %w", opts.URL, err)
	}
	return out, nil
}
