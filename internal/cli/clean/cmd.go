package cleancli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/clean"
	"github.com/sukekyo26/cocoon/internal/hostguard"
	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/logx"
	"github.com/sukekyo26/cocoon/internal/tui"
)

const cleanLong = `wsd clean — host-side Docker cleanup

Subcommands:
  volumes   delete Docker volumes for this project (replaces clean-volumes.sh)
  docker    clean up Docker resources system-wide (replaces clean-docker.sh)`

// NewCommand returns the cobra subtree for ` + "`wsd clean`" + `.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "clean",
		Short:         "Host-side Docker cleanup (volumes, docker, ...)",
		Long:          cleanLong,
		Args:          rejectUnknownSubcommand,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help() //nolint:wrapcheck // help write error is descriptive
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return fmt.Errorf("%w: %w", ErrUsage, err)
	})
	cmd.AddCommand(newCleanDockerCmd(stdout, stderr))
	cmd.AddCommand(newCleanVolumesCmd(stdout, stderr))
	return cmd
}

// rejectUnknownSubcommand returns an ErrUsage-wrapped error when a stray
// positional appears under a parent that only carries subcommands.
func rejectUnknownSubcommand(_ *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	return fmt.Errorf("%w: unknown subcommand %q", ErrUsage, args[0])
}

func newCleanDockerCmd(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "docker",
		Short:         "Clean up Docker resources system-wide",
		Long:          "wsd clean docker — clean up Docker resources system-wide",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			cat := i18n.New(i18n.Detect())
			if hostguard.InsideContainer() {
				logx.New(stdout, stderr).Error(cat.Msg("docker_clean_inside_container"))
				return ErrInsideContainer
			}
			//nolint:exhaustruct // optional fields default inside DockerCleanRun.
			opts := clean.DockerOptions{
				Stdout:   stdout,
				Stderr:   stderr,
				Catalog:  cat,
				Selector: tui.HuhSelector{},
			}
			//nolint:wrapcheck // sentinel errors propagated for exit-code mapping
			return clean.DockerCleanRun(context.Background(), opts)
		},
	}
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return fmt.Errorf("%w: %w", ErrUsage, err)
	})
	return cmd
}

func newCleanVolumesCmd(stdout, stderr io.Writer) *cobra.Command {
	var workspaceDir string
	cmd := &cobra.Command{
		Use:           "volumes",
		Short:         "Delete Docker volumes for this project",
		Long:          "wsd clean volumes — delete Docker volumes for this project",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			if workspaceDir == "" {
				return fmt.Errorf("%w: --workspace-dir is required", ErrUsage)
			}
			cat := i18n.New(i18n.Detect())
			if hostguard.InsideContainer() {
				logx.New(stdout, stderr).Error(cat.Msg("clean_inside_container"))
				return ErrInsideContainer
			}
			//nolint:exhaustruct // optional fields default inside VolumesRun.
			opts := clean.VolumesOptions{
				WorkspaceDir: workspaceDir,
				Stdin:        os.Stdin,
				Stdout:       stdout,
				Stderr:       stderr,
				Catalog:      cat,
			}
			//nolint:wrapcheck // sentinel errors propagated for exit-code mapping
			return clean.VolumesRun(context.Background(), opts)
		},
	}
	cmd.Flags().StringVar(&workspaceDir, "workspace-dir", "", "workspace root (required)")
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return fmt.Errorf("%w: %w", ErrUsage, err)
	})
	return cmd
}
