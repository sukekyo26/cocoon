package repositoriescli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/exec"
	"github.com/sukekyo26/cocoon/internal/logx"
	"github.com/sukekyo26/cocoon/internal/repositories"
)

const repositoriesLong = `wsd repositories — companion repository helpers

Reads the [repositories] section in workspace.toml and either clones every
declared repo (idempotently) or prints a status table for each.`

// NewCommand returns the cobra subtree for ` + "`wsd repositories`" + `.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "repositories",
		Short:         "Companion repository helpers",
		Long:          repositoriesLong,
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
	cmd.AddCommand(newCloneCmd(stderr))
	cmd.AddCommand(newStatusCmd(stdout, stderr))
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

func newCloneCmd(stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "clone <script-dir>",
		Short:         "Clone all [repositories].clone entries (idempotent)",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, args []string) error {
			log := logx.New(io.Discard, stderr)
			if len(args) != 1 {
				return fmt.Errorf("%w: clone requires <script-dir>", ErrUsage)
			}
			logger := func(level, msg string) {
				log.Errorf("[%s] %s", level, msg)
			}
			if _, err := repositories.CloneAll(exec.New(), args[0], logger); err != nil {
				log.Errorf("ERROR: %v", err)
				return ErrFailure
			}
			return nil
		},
	}
	cmd.SetFlagErrorFunc(usageErr)
	return cmd
}

func newStatusCmd(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "status <script-dir>",
		Short:         "Print STATUS\\tPATH\\tURL per declared repo",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, args []string) error {
			log := logx.New(stdout, stderr)
			if len(args) != 1 {
				return fmt.Errorf("%w: status requires <script-dir>", ErrUsage)
			}
			reports, err := repositories.CheckStatus(args[0])
			if err != nil {
				log.Errorf("ERROR: %v", err)
				return ErrFailure
			}
			for _, r := range reports {
				log.Infof("%s\t%s\t%s", r.Status, r.Path, r.URL)
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
