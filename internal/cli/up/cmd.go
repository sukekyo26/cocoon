package upcli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/cli/composex"
)

const upLong = `cocoon up — bring the workspace container up

Runs ` + "`docker compose -f .devcontainer/docker-compose.yml up -d --build`" + ` against
the generated stack. Run ` + "`cocoon gen`" + ` first when ` + "`.devcontainer/`" + `
artifacts do not yet exist.`

// NewCommand returns the cobra command for ` + "`cocoon up`" + `.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	var (
		noDetach bool
		service  string
	)
	cmd := &cobra.Command{
		Use:           "up",
		Short:         "Generate workspace artifacts and start the container",
		Long:          upLong,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			args := []string{"up", "--build"}
			if !noDetach {
				args = append(args, "-d")
			}
			if service != "" {
				args = append(args, service)
			}
			if err := composex.Run(cmd.Context(), os.Stdin, stdout, stderr, args...); err != nil {
				return fmt.Errorf("%w: %w", ErrFailure, err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&noDetach, "no-detach", false, "run compose in the foreground (drop -d)")
	cmd.Flags().StringVarP(&service, "service", "s", "", "operate on a single service from workspace.toml")
	return cmd
}
