package setupcli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/setup"
)

const setupLong = `wsd setup — bootstrap or regenerate a workspace from workspace.toml

Carries over the bash setup-docker.sh flags. ` + "`--lang`" + ` is consumed by the
shell wrapper which exports WORKSPACE_LANG; the Go side reads i18n.Detect().`

// NewCommand returns the cobra command for ` + "`wsd setup`" + `.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	//nolint:exhaustruct // remaining fields are optional and defaulted in setup.Run.
	opts := setup.Options{
		Stdout: stdout,
		Stderr: stderr,
	}
	cmd := &cobra.Command{
		Use:           "setup",
		Short:         "Bootstrap or regenerate a workspace from workspace.toml",
		Long:          setupLong,
		Args:          rejectPositionalArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			if opts.WorkspaceDir == "" {
				return fmt.Errorf("%w: --workspace-dir is required", ErrUsage)
			}
			opts.Catalog = i18n.New(i18n.Detect())
			//nolint:wrapcheck // sentinel errors propagated for exit-code mapping
			return setup.Run(opts)
		},
	}
	cmd.Flags().StringVar(&opts.WorkspaceDir, "workspace-dir", "", "workspace root (required)")
	cmd.Flags().StringVar(&opts.PluginsDir, "plugins-dir", "", "plugins directory")
	cmd.Flags().BoolVar(&opts.ForceInit, "init", false, "force interactive setup even when [container] already exists")
	cmd.Flags().BoolVarP(&opts.AutoYes, "yes", "y", false, "non-interactive defaults (service=dev, username=$USER)")
	cmd.Flags().BoolVar(&opts.RunDoctor, "doctor", false, "run prerequisite diagnostics and exit")
	cmd.Flags().BoolVar(&opts.RunDiff, "diff", false, "generate to a tmpdir and diff against the tree (exit 1 on diff)")
	cmd.Flags().BoolVar(&opts.NoClone, "no-clone", false, "skip the repository clone step")
	// --lang is honoured by the bash wrapper (exports WORKSPACE_LANG); the Go
	// side reads it via i18n.Detect(). Declare it so cobra accepts it.
	cmd.Flags().String("lang", "", "i18n catalog (en|ja); the wrapper exports WORKSPACE_LANG")
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return fmt.Errorf("%w: %w", ErrUsage, err)
	})
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	clihelpers.AttachHelpAlias(cmd)
	return cmd
}

// rejectPositionalArgs returns an ErrUsage-wrapped error for any unexpected
// positional argument. The legacy ` + "`<sub> help`" + ` synonym is matched as a
// subcommand by [clihelpers.AttachHelpAlias] before Args runs.
func rejectPositionalArgs(_ *cobra.Command, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("%w: unknown argument %q", ErrUsage, args[0])
	}
	return nil
}
