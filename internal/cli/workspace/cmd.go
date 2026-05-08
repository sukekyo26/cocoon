package workspacecli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/tui"
)

const workspaceLong = `wsd workspace — generate a .code-workspace file interactively

Reads workspaces/ for existing files, scans the parent of <scripts-dir> for
candidate folders, and writes a .code-workspace file selected via huh
prompts.`

// NewCommand returns the cobra command for ` + "`wsd workspace`" + `.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	return NewCommandWithIO(os.Stdin, stdout, stderr, tui.HuhSelector{})
}

// NewCommandWithIO is the testable entry point that lets callers inject a
// fake stdin and [tui.Selector].
func NewCommandWithIO(stdin io.Reader, stdout, stderr io.Writer, sel tui.Selector) *cobra.Command {
	var scriptsDir string
	cmd := &cobra.Command{
		Use:           "workspace",
		Short:         "Generate a .code-workspace file interactively",
		Long:          workspaceLong,
		Args:          rejectPositionalArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			if scriptsDir == "" {
				return fmt.Errorf("%w: --scripts-dir is required", ErrUsage)
			}
			return runWorkspace(scriptsDir, stdin, stderr, sel)
		},
	}
	cmd.Flags().StringVar(&scriptsDir, "scripts-dir", "", "workspace-docker repository root (required)")
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
