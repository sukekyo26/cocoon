package downcli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

const downLong = `cocoon down — stop the workspace container

Wraps ` + "`docker compose -f .devcontainer/docker-compose.yml down`" + ` against
the generated stack. With --volumes, also removes named volumes.

Status: stub. The full implementation is delivered in F3 of the v0.1.0 plan.`

// NewCommand returns the cobra command for `cocoon down`.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	_, _ = stdout, stderr
	cmd := &cobra.Command{
		Use:           "down",
		Short:         "Stop and remove the workspace container",
		Long:          downLong,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("%w: cocoon down is not yet implemented (planned for F3)", ErrFailure)
		},
	}
	cmd.Flags().Bool("volumes", false, "also remove named volumes declared in compose")
	cmd.Flags().Bool("rmi", false, "also remove the built workspace image")
	return cmd
}
