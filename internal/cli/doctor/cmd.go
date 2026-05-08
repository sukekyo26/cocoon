package doctorcli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/doctor"
)

const doctorLong = `wsd doctor — environment diagnostics

Inspects the host (Docker, devcontainer CLI, available plugin scripts, etc.)
and reports any issue that would prevent ` + "`wsd setup`" + ` from succeeding.`

// NewCommand returns the cobra command for ` + "`wsd doctor`" + `.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	var opts doctor.Options
	cmd := &cobra.Command{
		Use:           "doctor",
		Short:         "Environment diagnostics",
		Long:          doctorLong,
		Args:          rejectPositionalArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			if !doctor.Run(opts, stdout) {
				return ErrFailure
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.Root, "root", "", "workspace-docker root (defaults to cwd)")
	cmd.Flags().StringVar(&opts.PluginsDir, "plugins-dir", "", "plugins directory (defaults to <root>/plugins)")
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
