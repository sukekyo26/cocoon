package logscli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

const logsLong = `cocoon logs — stream container logs

Wraps ` + "`docker compose logs`" + ` against the generated stack.

Status: stub. The full implementation is delivered in F3 of the v0.1.0 plan.`

// NewCommand returns the cobra command for `cocoon logs`.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	_, _ = stdout, stderr
	cmd := &cobra.Command{
		Use:           "logs [service]",
		Short:         "Tail logs from the workspace container",
		Long:          logsLong,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("%w: cocoon logs is not yet implemented (planned for F3)", ErrFailure)
		},
	}
	cmd.Flags().BoolP("follow", "f", false, "follow log output")
	cmd.Flags().Int("tail", -1, "number of recent lines to show before following")
	return cmd
}
