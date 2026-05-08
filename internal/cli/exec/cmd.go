package execcli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

const execLong = `cocoon exec — run a command inside the workspace container

Wraps ` + "`docker compose exec`" + ` against the generated stack. With no
positional arguments the user's login shell is opened.

Status: stub. The full implementation is delivered in F3 of the v0.1.0 plan.`

// NewCommand returns the cobra command for `cocoon exec`.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	_, _ = stdout, stderr
	cmd := &cobra.Command{
		Use:           "exec [-- command [args...]]",
		Short:         "Run a command inside the workspace container",
		Long:          execLong,
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("%w: cocoon exec is not yet implemented (planned for F3)", ErrFailure)
		},
	}
	cmd.Flags().StringP("service", "s", "", "service to exec into (default: dev)")
	return cmd
}
