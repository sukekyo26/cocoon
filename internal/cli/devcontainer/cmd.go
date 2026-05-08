package devcontainercli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/devcontainer"
	"github.com/sukekyo26/cocoon/internal/exec"
	"github.com/sukekyo26/cocoon/internal/logx"
)

const devcontainerLong = `wsd devcontainer — devcontainer CLI prerequisites and runner

Subcommands:
  check  verify Docker, devcontainer CLI, devcontainer.json, and .env
  up     run "devcontainer up --workspace-folder <dir>"`

// NewCommand returns the cobra subtree for ` + "`wsd devcontainer`" + `.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	return NewCommandWithRunner(exec.New(), stdout, stderr)
}

// NewCommandWithRunner is the testable entry point that lets callers inject
// an [exec.RecordingRunner] in place of the real OS runner.
func NewCommandWithRunner(runner exec.Runner, stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "devcontainer",
		Short:         "devcontainer CLI prerequisites and runner",
		Long:          devcontainerLong,
		Args:          rejectUnknownSubcommand,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Legacy behaviour: `wsd devcontainer` with no subcommand prints
			// usage to stderr and exits 2 (ErrUsage). Redirect cobra's help
			// writer to stderr for this error path; `--help` still routes to
			// stdout because it bypasses RunE.
			cmd.SetOut(stderr)
			if err := cmd.Help(); err != nil {
				return err //nolint:wrapcheck // help write error is descriptive
			}
			return ErrUsage
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return fmt.Errorf("%w: %w", ErrUsage, err)
	})
	cmd.AddCommand(newCheckCmd(runner, stdout, stderr))
	cmd.AddCommand(newUpCmd(runner, stdout, stderr))
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

func newCheckCmd(runner exec.Runner, stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "check <workspace-dir>",
		Short:         "Verify Docker, devcontainer CLI, devcontainer.json, and .env",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				logx.New(stdout, stderr).Error("ERROR: check requires <workspace-dir>")
				return ErrUsage
			}
			res := devcontainer.CheckPrerequisites(runner, args[0], stdout)
			switch res {
			case devcontainer.PrereqOK:
				return nil
			case devcontainer.PrereqMissingDocker:
				return ErrMissingDocker
			case devcontainer.PrereqMissingDevcontainerCLI:
				return ErrMissingDcCLI
			case devcontainer.PrereqMissingDevcontainerJSON:
				return ErrMissingDcJSON
			case devcontainer.PrereqMissingEnv:
				return ErrMissingEnvFile
			default:
				return ErrFailure
			}
		},
	}
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return fmt.Errorf("%w: %w", ErrUsage, err)
	})
	return cmd
}

// newUpCmd uses DisableFlagParsing because extra arguments must be forwarded
// verbatim to ` + "`devcontainer up`" + ` after the workspace dir.
func newUpCmd(runner exec.Runner, stdout, stderr io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:                "up <workspace-dir> [extra-args...]",
		Short:              "Run \"devcontainer up --workspace-folder <dir>\"",
		DisableFlagParsing: true,
		SilenceUsage:       true,
		SilenceErrors:      true,
		RunE: func(_ *cobra.Command, args []string) error {
			log := logx.New(stdout, stderr)
			// Strip a leading "--" that cobra inserts when the user typed it
			// to terminate parent flags.
			if len(args) > 0 && args[0] == "--" {
				args = args[1:]
			}
			// Treat help-style args specially so DisableFlagParsing doesn't
			// silently pass them through to `devcontainer up`.
			for _, a := range args {
				if a == "--help" || a == "-h" || a == "help" {
					return printUpHelp(stdout)
				}
			}
			if len(args) < 1 {
				log.Error("ERROR: up requires <workspace-dir> [extra-args...]")
				return ErrUsage
			}
			upArgs := append([]string{"up", "--workspace-folder", args[0]}, args[1:]...)
			if err := devcontainer.Up(runner, upArgs, stdout, stderr); err != nil {
				log.Errorf("ERROR: %v", err)
				return ErrFailure
			}
			return nil
		},
	}
}

func printUpHelp(w io.Writer) error {
	const usage = `wsd devcontainer up — run "devcontainer up --workspace-folder <dir>"

Usage:
  wsd devcontainer up <workspace-dir> [extra-args...]
`
	if _, err := io.WriteString(w, usage); err != nil {
		return fmt.Errorf("write usage: %w", err)
	}
	return nil
}
