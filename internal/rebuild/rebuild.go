// Package rebuild implements the host-side `rebuild-container.sh` flow that
// rebuilds the dev container image without cache and recreates the
// container via the devcontainer CLI.
package rebuild

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sukekyo26/cocoon/internal/devcontainer"
	"github.com/sukekyo26/cocoon/internal/envfile"
	"github.com/sukekyo26/cocoon/internal/exec"
	"github.com/sukekyo26/cocoon/internal/exec/dockerx"
)

// Sentinel errors propagated to the binary boundary for exit-code mapping.
var (
	ErrCanceled = errors.New("canceled")
	ErrConfig   = errors.New("rebuild: invalid options")
	ErrPrereq   = errors.New("prerequisite check failed")
	ErrFailure  = errors.New("rebuild failure")
)

// errPrereqResult is the inner error returned by execPrereq.Check when
// devcontainer.CheckPrerequisites reports a non-OK result. Run() wraps it
// with ErrPrereq so the prefix appears exactly once.
var errPrereqResult = errors.New("non-zero result")

// Translator is the subset of i18n.Catalog that rebuild needs.
type Translator interface {
	Msg(key string, args ...any) string
}

// ImageInspector abstracts `docker image inspect` so tests can inject fakes.
type ImageInspector interface {
	// Created returns the image's Created timestamp (RFC3339Nano-ish), or
	// ("", false) when the image does not exist.
	Created(ctx context.Context, image string) (string, bool)
}

// DevRunner abstracts the call into devcontainer up. The default
// implementation uses internal/devcontainer.Up.
type DevRunner interface {
	Up(args []string, stdout, stderr io.Writer) error
}

// PrereqChecker abstracts the docker / devcontainer CLI presence check so
// tests can bypass it without spawning subprocesses.
type PrereqChecker interface {
	Check(workspaceDir string, out io.Writer) error
}

// Options configures Run.
type Options struct {
	WorkspaceDir string
	Stdin        io.Reader
	Stdout       io.Writer
	Stderr       io.Writer
	Catalog      Translator
	Inspector    ImageInspector
	Runner       DevRunner
	Prereq       PrereqChecker
	// AssumeYes skips the confirmation prompt (currently used by tests).
	AssumeYes bool
}

// Run performs the rebuild flow: prereq check → image info → confirm →
// devcontainer up --build-no-cache --remove-existing-container → done.
func Run(ctx context.Context, opts Options) error {
	opts = defaults(opts)
	if opts.Catalog == nil {
		return fmt.Errorf("%w: Catalog is required", ErrConfig)
	}
	if opts.WorkspaceDir == "" {
		return fmt.Errorf("%w: WorkspaceDir is required", ErrConfig)
	}
	t := opts.Catalog
	out := opts.Stdout

	printHeader(out, t, opts.WorkspaceDir)

	if err := opts.Prereq.Check(opts.WorkspaceDir, out); err != nil {
		return fmt.Errorf("%w: %w", ErrPrereq, err)
	}

	imageName := imageName(opts.WorkspaceDir)
	printCurrentImageInfo(ctx, out, t, imageName, opts.Inspector)

	fmt.Fprintln(out)
	fmt.Fprintln(out, t.Msg("rebuild_notice"))
	fmt.Fprintln(out, t.Msg("rebuild_notice_1"))
	fmt.Fprintln(out, t.Msg("rebuild_notice_2"))
	fmt.Fprintln(out, t.Msg("rebuild_notice_3"))
	fmt.Fprintln(out)

	if !opts.AssumeYes {
		ok, err := envfile.ConfirmYN(opts.Stdin, out, t.Msg("rebuild_confirm"))
		if err != nil {
			return fmt.Errorf("read confirm: %w", err)
		}
		if !ok {
			fmt.Fprintln(out, t.Msg("rebuild_cancelled"))
			return ErrCanceled
		}
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, t.Msg("rebuild_starting"))
	fmt.Fprintln(out, t.Msg("rebuild_please_wait"))
	fmt.Fprintln(out)

	upArgs := []string{
		"up",
		"--workspace-folder", opts.WorkspaceDir,
		"--build-no-cache",
		"--remove-existing-container",
	}
	if err := opts.Runner.Up(upArgs, out, opts.Stderr); err != nil {
		return fmt.Errorf("%w: %w", ErrFailure, err)
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, t.Msg("rebuild_complete"))
	if created, ok := opts.Inspector.Created(ctx, imageName); ok {
		fmt.Fprintln(out, t.Msg("rebuild_new_image", formatTimestamp(created)))
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, t.Msg("rebuild_vscode_1"))
	fmt.Fprintln(out, t.Msg("rebuild_vscode_2"))
	fmt.Fprintln(out)
	return nil
}

func printHeader(out io.Writer, t Translator, workspaceDir string) {
	fmt.Fprintln(out)
	fmt.Fprintln(out, "========================================")
	fmt.Fprintln(out, " "+t.Msg("rebuild_header"))
	fmt.Fprintln(out, "========================================")
	fmt.Fprintln(out)
	fmt.Fprintln(out, t.Msg("rebuild_workspace"), workspaceDir)
}

func printCurrentImageInfo(ctx context.Context, out io.Writer, t Translator, image string, ins ImageInspector) {
	created, ok := ins.Created(ctx, image)
	if !ok {
		fmt.Fprintln(out, t.Msg("rebuild_image_not_found", image))
		return
	}
	fmt.Fprintln(out, t.Msg("rebuild_current_image", image))
	formatted := formatTimestamp(created)
	days := daysAgo(created)
	fmt.Fprintln(out, t.Msg("rebuild_created", formatted, fmt.Sprint(days)))
}

func imageName(workspaceDir string) string {
	envPath := filepath.Join(workspaceDir, ".env")
	service := envfile.ReadOr(envPath, "CONTAINER_SERVICE_NAME", "dev")
	workspace := filepath.Base(workspaceDir)
	return workspace + "-" + service
}

func formatTimestamp(raw string) string {
	t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(raw))
	if err != nil {
		return raw
	}
	return t.Format("2006-01-02 15:04:05")
}

func daysAgo(raw string) int {
	t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	d := time.Since(t)
	if d < 0 {
		return 0
	}
	return int(d / (24 * time.Hour))
}

func defaults(opts Options) Options {
	if opts.Stdin == nil {
		opts.Stdin = os.Stdin
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	r := exec.New()
	if opts.Inspector == nil {
		opts.Inspector = execInspector{docker: dockerx.New(r)}
	}
	if opts.Runner == nil {
		opts.Runner = devcontainerRunner{runner: r}
	}
	if opts.Prereq == nil {
		opts.Prereq = execPrereq{runner: r}
	}
	return opts
}

type execPrereq struct{ runner exec.Runner }

// Check delegates to devcontainer.CheckPrerequisites and maps the result to
// an error so the seam matches the PrereqChecker interface. Returns a plain
// error (no sentinel wrapping) — Run() is the single place that wraps with
// ErrPrereq.
//
//nolint:contextcheck // CheckPrerequisites manages its own short-lived context.
func (e execPrereq) Check(workspaceDir string, out io.Writer) error {
	if res := devcontainer.CheckPrerequisites(e.runner, workspaceDir, out); res != devcontainer.PrereqOK {
		return fmt.Errorf("%w: %d", errPrereqResult, res)
	}
	return nil
}

type execInspector struct{ docker *dockerx.Client }

// Created shells out to `docker image inspect` to read the Created timestamp.
func (e execInspector) Created(ctx context.Context, image string) (string, bool) {
	s, err := e.docker.ImageInspectFormat(ctx, image, "{{.Created}}")
	if err != nil || s == "" {
		return "", false
	}
	return s, true
}

type devcontainerRunner struct{ runner exec.Runner }

// Up delegates to internal/devcontainer.Up.
func (d devcontainerRunner) Up(args []string, stdout, stderr io.Writer) error {
	if err := devcontainer.Up(d.runner, args, stdout, stderr); err != nil {
		return fmt.Errorf("devcontainer up: %w", err)
	}
	return nil
}
