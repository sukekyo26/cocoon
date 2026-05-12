package plugincli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/logx"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

const pinLong = `cocoon plugin pin — emit a [plugins.versions.<id>] block

By default the block is printed to stdout for you to paste under the
[plugins.versions] table in workspace.toml. With --write the block is
inserted (or replaced) in place; comments and blank lines outside the
target block are preserved verbatim.

Use the --amd64-checksum / --arm64-checksum flags when the upstream
release ships per-arch SHA256 sums you want install.sh to verify.`

func newPinCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		amd64Checksum string
		arm64Checksum string
		write         bool
	)
	cmd := &cobra.Command{
		Use:           "pin <id> <ref>",
		Short:         "Emit a workspace.toml [plugins.versions.<id>] block (stdout, or in-place with --write)",
		Long:          pinLong,
		Args:          cobra.ExactArgs(2),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, args []string) error {
			return runPin(stdout, stderr, args[0], args[1], amd64Checksum, arm64Checksum, write)
		},
	}
	cmd.Flags().StringVar(&amd64Checksum, "amd64-checksum", "", "sha256 of the amd64 artifact (optional)")
	cmd.Flags().StringVar(&arm64Checksum, "arm64-checksum", "", "sha256 of the arm64 artifact (optional)")
	cmd.Flags().BoolVar(&write, "write", false,
		"insert (or replace) the block in workspace.toml (auto-discovered from cwd)")
	return cmd
}

func runPin(stdout, stderr io.Writer, id, ref, amd64sum, arm64sum string, write bool) error {
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

	if write {
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			return fmt.Errorf("%w: getwd: %w", ErrFailure, cwdErr)
		}
		wsPath, dErr := config.Discover(cwd)
		if dErr != nil {
			return fmt.Errorf("%w: discover workspace.toml: %w", ErrFailure, dErr)
		}
		if wsPath == "" {
			return fmt.Errorf(
				"%w: --write needs a discoverable workspace.toml (run inside a cocoon project)",
				ErrUsage)
		}
		if uErr := plugin.UpsertPinBlock(wsPath, id, ref, amd64sum, arm64sum); uErr != nil {
			if errors.Is(uErr, plugin.ErrPinBlockVersionsKeyAssign) {
				return fmt.Errorf("%w: %w (in %s)", ErrUsage, uErr, wsPath)
			}
			return fmt.Errorf("%w: %w", ErrFailure, uErr)
		}
		logx.New(stdout, stderr).Successf("Updated %s: [plugins.versions.%s]", wsPath, id)
		return nil
	}

	var b strings.Builder
	fmt.Fprintln(&b, "# Append the following block to workspace.toml under [plugins.versions]:")
	fmt.Fprintln(&b)
	b.WriteString(plugin.FormatPinBlock(id, ref, amd64sum, arm64sum))
	fmt.Fprint(stdout, b.String())
	return nil
}
