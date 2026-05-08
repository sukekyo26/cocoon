package downcli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/cli/composex"
)

const downLong = `cocoon down — stop the workspace container

Wraps ` + "`docker compose -f .devcontainer/docker-compose.yml down`" + ` against
the generated stack. With --volumes, also removes named volumes; with
--rmi, also removes the built workspace image.`

// NewCommand returns the cobra command for ` + "`cocoon down`" + `.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	var (
		volumes bool
		rmi     bool
	)
	cmd := &cobra.Command{
		Use:           "down",
		Short:         "Stop and remove the workspace container",
		Long:          downLong,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			args := []string{"down"}
			if volumes {
				args = append(args, "--volumes")
			}
			if rmi {
				args = append(args, "--rmi", "all")
			}
			if err := composex.Run(cmd.Context(), os.Stdin, stdout, stderr, args...); err != nil {
				return fmt.Errorf("%w: %w", ErrFailure, err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&volumes, "volumes", false, "also remove named volumes declared in compose")
	cmd.Flags().BoolVar(&rmi, "rmi", false, "also remove the built workspace image")
	return cmd
}
