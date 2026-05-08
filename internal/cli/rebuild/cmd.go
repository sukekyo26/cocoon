package rebuildcli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/hostguard"
	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/logx"
	"github.com/sukekyo26/cocoon/internal/rebuild"
)

const rebuildLong = `wsd rebuild — rebuild dev container without cache

Runs ` + "`docker compose build --no-cache`" + ` for the dev service declared in the
generated docker-compose.yml. Refuses to run from inside a container.`

// NewCommand returns the cobra command for ` + "`wsd rebuild`" + `.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	var workspaceDir string
	cmd := &cobra.Command{
		Use:           "rebuild",
		Short:         "Rebuild the dev container without cache",
		Long:          rebuildLong,
		Args:          rejectPositionalArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			if workspaceDir == "" {
				return fmt.Errorf("%w: --workspace-dir is required", ErrUsage)
			}
			cat := i18n.New(i18n.Detect())
			if hostguard.InsideContainer() {
				logx.New(stdout, stderr).Error(cat.Msg("rebuild_inside_container"))
				return ErrInsideContainer
			}
			//nolint:exhaustruct // optional fields default inside rebuild.Run.
			opts := rebuild.Options{
				WorkspaceDir: workspaceDir,
				Stdin:        os.Stdin,
				Stdout:       stdout,
				Stderr:       stderr,
				Catalog:      cat,
			}
			//nolint:wrapcheck // sentinel errors propagated for exit-code mapping
			return rebuild.Run(context.Background(), opts)
		},
	}
	cmd.Flags().StringVar(&workspaceDir, "workspace-dir", "", "workspace root (required)")
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return fmt.Errorf("%w: %w", ErrUsage, err)
	})
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	clihelpers.AttachHelpAlias(cmd)
	return cmd
}

// rejectPositionalArgs returns an ErrUsage-wrapped error for any unexpected
// positional argument.
func rejectPositionalArgs(_ *cobra.Command, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("%w: unknown argument %q", ErrUsage, args[0])
	}
	return nil
}
