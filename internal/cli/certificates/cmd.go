package certificatescli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/certificates"
	"github.com/sukekyo26/cocoon/internal/logx"
)

const certificatesLong = `wsd certificates — PEM certificate helpers

Manages company / self-signed PEM certificates that the Dockerfile generator
copies into the image. The actual certificate files live under the project
root (next to workspace.toml).`

// NewCommand returns the cobra subtree for ` + "`wsd certificates`" + `.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "certificates",
		Short:         "PEM certificate helpers",
		Long:          certificatesLong,
		Args:          rejectUnknownSubcommand,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help() //nolint:wrapcheck // help write error is descriptive
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetFlagErrorFunc(usageErr)
	cmd.AddCommand(newListCmd(stdout, stderr))
	cmd.AddCommand(newCheckCmd())
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

func newListCmd(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "list <project-root>",
		Short:         "Print one valid certificate basename per line",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, args []string) error {
			log := logx.New(stdout, stderr)
			if len(args) != 1 {
				return fmt.Errorf("%w: list requires <project-root>", ErrUsage)
			}
			list, err := certificates.List(args[0])
			if err != nil {
				log.Errorf("ERROR: %v", err)
				return ErrFailure
			}
			for _, n := range list {
				log.Info(n)
			}
			return nil
		},
	}
	cmd.SetFlagErrorFunc(usageErr)
	return cmd
}

func newCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "check <project-root>",
		Short:         "Exit 0 if at least one valid cert exists, 1 otherwise",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("%w: check requires <project-root>", ErrUsage)
			}
			if !certificates.Has(args[0]) {
				return ErrFailure
			}
			return nil
		},
	}
	cmd.SetFlagErrorFunc(usageErr)
	return cmd
}

func usageErr(_ *cobra.Command, err error) error {
	return fmt.Errorf("%w: %w", ErrUsage, err)
}
