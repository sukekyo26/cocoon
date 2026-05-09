package plugincli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/plugin"
)

const pinLong = `cocoon plugin pin — print a [plugins.versions.<id>] block to paste into workspace.toml

Pinning lives in workspace.toml under [plugins.versions.<id>] (pin /
checksum_amd64 / checksum_arm64). cocoon refuses to mutate workspace.toml
itself because it would lose any inline comments the user wrote there;
this subcommand instead prints the snippet for you to paste under the
[plugins.versions] table.

Use the --amd64-checksum / --arm64-checksum flags when the upstream
release ships per-arch SHA256 sums you want install.sh to verify.`

func newPinCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		amd64Checksum string
		arm64Checksum string
	)
	cmd := &cobra.Command{
		Use:           "pin <id> <ref>",
		Short:         "Print a workspace.toml [plugins.versions.<id>] block",
		Long:          pinLong,
		Args:          cobra.ExactArgs(2),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, args []string) error {
			return runPin(stdout, stderr, args[0], args[1], amd64Checksum, arm64Checksum)
		},
	}
	cmd.Flags().StringVar(&amd64Checksum, "amd64-checksum", "", "sha256 of the amd64 artifact (optional)")
	cmd.Flags().StringVar(&arm64Checksum, "arm64-checksum", "", "sha256 of the arm64 artifact (optional)")
	return cmd
}

func runPin(stdout, _ io.Writer, id, ref, amd64sum, arm64sum string) error {
	if id == "" || ref == "" {
		return fmt.Errorf("%w: both <id> and <ref> are required", ErrUsage)
	}
	layered, err := resolveLayered()
	if err != nil {
		return err
	}
	if layered.Source(id) == "" {
		return fmt.Errorf("%w: plugin %q is not in any layer (cocoon plugin list)", ErrUsage, id)
	}

	var b strings.Builder
	fmt.Fprintln(&b, "# Append the following block to workspace.toml under [plugins.versions]:")
	fmt.Fprintln(&b)
	b.WriteString(plugin.FormatPinBlock(id, ref, amd64sum, arm64sum))
	fmt.Fprint(stdout, b.String())
	return nil
}
