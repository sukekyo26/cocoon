// Package lockcli implements `cocoon lock`: it resolves every enabled
// version_capable plugin's constraint to a concrete version (and per-arch
// SHA256 checksums) over the network and writes cocoon.lock. `cocoon gen`
// then consumes the lock offline for reproducible builds.
package lockcli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	generatecli "github.com/sukekyo26/cocoon/internal/cli/generate"
	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/lockfile"
	"github.com/sukekyo26/cocoon/internal/logx"
	"github.com/sukekyo26/cocoon/internal/plugin"
	"github.com/sukekyo26/cocoon/internal/plugin/resolve"
	"github.com/sukekyo26/cocoon/internal/warn"
)

// defaultFetcher is the network Fetcher `cocoon lock` resolves through. Tests
// in this package swap it for an offline fake.
//
//nolint:gochecknoglobals // test seam; a nil Client uses http.DefaultClient.
var defaultFetcher resolve.Fetcher = resolve.HTTPFetcher{Client: nil}

// NewCommand returns the `cocoon lock` cobra command.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	cat := i18n.New(i18n.Detect())
	var (
		workspaceFlag string
		check         bool
		upgrade       bool
	)
	cmd := &cobra.Command{
		Use:           "lock",
		Short:         cat.Msg("cmd_lock_short"),
		Long:          cat.Msg("cmd_lock_long"),
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(c *cobra.Command, _ []string) error {
			return runLock(c.Context(), stdout, stderr, lockOptions{
				workspaceFlag: workspaceFlag, check: check, upgrade: upgrade,
			})
		},
	}
	cmd.Flags().StringVar(&workspaceFlag, "workspace", "", cat.Msg("flag_lock_workspace_usage"))
	cmd.Flags().BoolVar(&check, "check", false, cat.Msg("flag_lock_check_usage"))
	cmd.Flags().BoolVar(&upgrade, "upgrade", false, cat.Msg("flag_lock_upgrade_usage"))
	return cmd
}

type lockOptions struct {
	workspaceFlag string
	check         bool
	upgrade       bool
}

func runLock(ctx context.Context, stdout, stderr io.Writer, opts lockOptions) error {
	wsPath, err := resolveWorkspace(opts.workspaceFlag)
	if err != nil {
		return err
	}
	wctx, err := loadContext(wsPath)
	if err != nil {
		return err
	}
	requested := requestedSpecs(wctx)
	lockPath := lockfile.PathFor(wsPath, wctx.WS)
	log := logx.New(stdout, stderr)
	cat := i18n.New(i18n.Detect())
	clihelpers.DrainWarnings(log, cat, wctx.Warnings)
	existing, err := loadExistingLock(lockPath, opts.check, log, cat)
	if err != nil {
		return err
	}
	if opts.check {
		return checkLock(log, cat, lockPath, existing, requested)
	}
	lock, err := buildLock(ctx, wctx, requested, existing, opts.upgrade, log, cat)
	if err != nil {
		return err
	}
	if saveErr := lockfile.Save(lockPath, lock); saveErr != nil {
		return fmt.Errorf("%w: %w", clihelpers.ErrFailure, saveErr)
	}
	log.Success(cat.Msg("lock_wrote", lockPath, len(lock.Plugins)))
	return nil
}

// loadContext builds the layered plugin FS and loads workspace + plugins,
// reusing the generation pipeline's loader so plugin conflict checks and
// overlay resolution match `cocoon gen` exactly.
func loadContext(wsPath string) (*generate.WorkspaceContext, error) {
	embedded, err := plugin.CatalogFS()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", clihelpers.ErrFailure, err)
	}
	userDir, err := userPluginsDir()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", clihelpers.ErrFailure, err)
	}
	projectDir := filepath.Join(filepath.Dir(wsPath), ".cocoon", "plugins")
	layered := plugin.NewLayeredFS(embedded, userDir, projectDir)
	//nolint:wrapcheck // LoadContext already wraps failures in clihelpers.ErrFailure.
	return generatecli.LoadContext(wsPath, layered, "", warn.New())
}

// loadExistingLock returns the current lock for reuse, with "no lock yet" as a
// nil, non-error state. A malformed existing lock is fatal for --check (the lock
// is broken and CI must catch it) but recoverable for a write: `cocoon lock`
// owns and fully rewrites the file, so it warns and regenerates from scratch
// (nil) rather than refusing to run over a corrupt file.
func loadExistingLock(path string, check bool, log *logx.Logger, cat *i18n.Catalog) (*lockfile.Lock, error) {
	l, err := lockfile.Load(path)
	switch {
	case err == nil:
		return l, nil
	case lockfile.IsNotExist(err):
		return nil, nil //nolint:nilnil // "no lock yet" is a valid, non-error state.
	case check:
		return nil, fmt.Errorf("%w: %s is malformed (%w); run `cocoon lock` to regenerate it",
			clihelpers.ErrUsage, path, err)
	default:
		log.Warn(cat.Msg("lock_ignoring_malformed", path, err))
		return nil, nil //nolint:nilnil // malformed-and-regenerating is a valid write-path state.
	}
}

func resolveWorkspace(flag string) (string, error) {
	if flag != "" {
		abs, err := filepath.Abs(flag)
		if err != nil {
			return "", fmt.Errorf("%w: resolve --workspace: %w", clihelpers.ErrUsage, err)
		}
		if _, statErr := os.Stat(abs); statErr != nil {
			return "", fmt.Errorf("%w: --workspace %s: %w", clihelpers.ErrUsage, abs, statErr)
		}
		return abs, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("%w: %w", clihelpers.ErrFailure, err)
	}
	found, err := config.Discover(cwd)
	if err != nil {
		return "", fmt.Errorf("%w: discover workspace.toml: %w", clihelpers.ErrFailure, err)
	}
	if found == "" {
		return "", fmt.Errorf(
			"%w: workspace.toml not found in %s or any parent (try `cocoon init`)",
			clihelpers.ErrUsage, cwd)
	}
	return found, nil
}

// userPluginsDir is the LayeredFS user-layer root (~/.cocoon/plugins).
func userPluginsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".cocoon", "plugins"), nil
}
