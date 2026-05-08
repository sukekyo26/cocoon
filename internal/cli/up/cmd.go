package upcli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

const upLong = `cocoon up — bring the workspace container up

Regenerates the Dockerfile, docker-compose.yml and (optionally)
devcontainer.json under .devcontainer/, then runs ` + "`docker compose up -d --build`" + `.

Status: stub. The full implementation is delivered in F3 of the v0.1.0 plan.`

// NewCommand returns the cobra command for `cocoon up`.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	_, _ = stdout, stderr
	cmd := &cobra.Command{
		Use:           "up",
		Short:         "Generate workspace artifacts and start the container",
		Long:          upLong,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("%w: cocoon up is not yet implemented (planned for F3)", ErrFailure)
		},
	}
	cmd.Flags().Bool("build", false, "force `docker compose --build` even when sources are unchanged")
	cmd.Flags().Bool("no-detach", false, "run compose in the foreground (drop -d)")
	cmd.Flags().StringP("service", "s", "", "operate on a single service from workspace.toml")
	return cmd
}
