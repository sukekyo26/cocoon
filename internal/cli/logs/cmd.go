package logscli

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/cli/composex"
)

const logsLong = `cocoon logs — stream container logs

Wraps ` + "`docker compose -f .devcontainer/docker-compose.yml logs`" + `. Pass a
service name to scope output, -f to follow, --tail N to limit history.`

// NewCommand returns the cobra command for ` + "`cocoon logs`" + `.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	var (
		follow bool
		tail   int
	)
	cmd := &cobra.Command{
		Use:           "logs [service]",
		Short:         "Tail logs from the workspace container",
		Long:          logsLong,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			args := []string{"logs"}
			if follow {
				args = append(args, "-f")
			}
			if tail >= 0 {
				args = append(args, "--tail", strconv.Itoa(tail))
			}
			args = append(args, posArgs...)
			if err := composex.Run(cmd.Context(), os.Stdin, stdout, stderr, args...); err != nil {
				return fmt.Errorf("%w: %w", ErrFailure, err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow log output")
	cmd.Flags().IntVar(&tail, "tail", -1, "number of recent lines to show before following (default: all)")
	return cmd
}
