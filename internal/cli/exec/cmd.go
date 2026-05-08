package execcli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/cli/composex"
)

const execLong = `cocoon exec — run a command inside the workspace container

Wraps ` + "`docker compose -f .devcontainer/docker-compose.yml exec`" + `. With no
positional arguments after ` + "`--`" + `, drops you into ` + "`bash -l`" + ` inside the
default ` + "`dev`" + ` service.`

// NewCommand returns the cobra command for ` + "`cocoon exec`" + `.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	var service string
	cmd := &cobra.Command{
		Use:           "exec [-- command [args...]]",
		Short:         "Run a command inside the workspace container",
		Long:          execLong,
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			svc := service
			if svc == "" {
				svc = "dev"
			}
			args := []string{"exec", svc}
			if len(posArgs) == 0 {
				args = append(args, "bash", "-l")
			} else {
				args = append(args, posArgs...)
			}
			if err := composex.Run(cmd.Context(), os.Stdin, stdout, stderr, args...); err != nil {
				return fmt.Errorf("%w: %w", ErrFailure, err)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&service, "service", "s", "", "service to exec into (default: dev)")
	return cmd
}
