package selfupdatecli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

const selfUpdateLong = `cocoon self-update — replace the current binary with the latest release

Checks GitHub Releases for a newer cocoon binary, downloads the matching
asset under SHA256 verification, and atomically replaces this executable.

Status: stub. The full implementation lands in F5 of the v0.1.0 plan.`

// NewCommand returns the cobra command for ` + "`cocoon self-update`" + `.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	_, _ = stdout, stderr
	cmd := &cobra.Command{
		Use:           "self-update",
		Short:         "Replace this binary with the latest released version",
		Long:          selfUpdateLong,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("%w: cocoon self-update is not yet implemented (planned for F5)", ErrFailure)
		},
	}
	cmd.Flags().Bool("check-only", false, "exit 0 if up to date, exit 100 if a newer release exists; never download")
	cmd.Flags().Bool("force", false, "reinstall even when the local binary is already the latest version")
	return cmd
}
