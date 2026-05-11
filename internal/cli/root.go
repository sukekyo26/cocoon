package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	gencli "github.com/sukekyo26/cocoon/internal/cli/gen"
	initcli "github.com/sukekyo26/cocoon/internal/cli/init"
	plugincli "github.com/sukekyo26/cocoon/internal/cli/plugin"
	selfupdatecli "github.com/sukekyo26/cocoon/internal/cli/selfupdate"
	"github.com/sukekyo26/cocoon/internal/logx"
	"github.com/sukekyo26/cocoon/internal/updatecheck"
	"github.com/sukekyo26/cocoon/internal/version"
)

const rootLong = `cocoon — project-aware container workspace generator

Run cocoon from any project directory to read its workspace.toml and
materialize a Dev Container or plain docker-compose stack tailored to
that repository.`

const rootHelpTemplate = `{{.Long}}

Usage:
  {{.UseLine}}{{if .HasAvailableSubCommands}}

Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}

Run 'cocoon <command> --help' for command-specific usage.
`

// newRootCommand constructs the cobra root command tree. It is stateless
// across calls — each invocation builds a fresh tree wired to the supplied
// writers, so concurrent uses (tests in parallel) are safe.
func newRootCommand(version string, stdout, stderr io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:           "cocoon",
		Short:         "Project-aware container workspace generator",
		Long:          rootLong,
		Version:       version,
		Args:          cobra.ArbitraryArgs, // RunE handles unknown args explicitly.
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			maybeNotifyUpdate(cmd, stderr)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help() //nolint:wrapcheck // top-level help write error is descriptive
			}
			return fmt.Errorf("%w: %q (try `cocoon help`)", ErrUnknownCommand, args[0])
		},
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetVersionTemplate("{{.Version}}\n")
	root.SetHelpTemplate(rootHelpTemplate)
	root.AddCommand(
		// Generator commands. `init` writes a fresh workspace.toml; `gen`
		// reads it and emits .devcontainer/{Dockerfile, docker-compose.yml,
		// devcontainer.json}. Container start-up is left to docker compose
		// or VS Code's Reopen in Container — cocoon does not wrap them.
		initcli.NewCommand(stdout, stderr),
		gencli.NewCommand(stdout, stderr),
		selfupdatecli.NewCommand(stdout, stderr),
		// Noun groups
		plugincli.NewCommand(stdout, stderr),
		newVersionSubcommand(version, stdout),
	)
	addLeafHelpAlias(root)
	return root
}

// addLeafHelpAlias walks the tree decorating every leaf command with a
// hidden ` + "`help`" + ` subcommand (via [clihelpers.AttachHelpAlias]). Subcommand
// constructors typically call AttachHelpAlias themselves, but this catch-all
// pass guarantees coverage even if a future addition forgets.
func addLeafHelpAlias(c *cobra.Command) {
	for _, child := range c.Commands() {
		if child.Name() == "help" || child.Hidden {
			continue
		}
		if child.HasSubCommands() {
			addLeafHelpAlias(child)
			continue
		}
		clihelpers.AttachHelpAlias(child)
	}
}

// maybeNotifyUpdate runs the once-per-day update check unless an opt-out
// applies. Failures (network down, malformed cache, missing $HOME) are
// silent so the notifier never interferes with the user's invocation.
func maybeNotifyUpdate(cmd *cobra.Command, stderr io.Writer) {
	if shouldSkipUpdateCheck(cmd, stderr) {
		return
	}
	notice := updatecheck.Check(cmd.Context(), version.Get(), updatecheck.Options{
		Now:        nil,
		CacheDir:   "",
		HTTPClient: nil,
	})
	if notice == nil {
		return
	}
	logx.New(io.Discard, stderr).Notice(notice.Format())
}

func shouldSkipUpdateCheck(cmd *cobra.Command, stderr io.Writer) bool {
	if os.Getenv("COCOON_NO_UPDATE_CHECK") == "1" {
		return true
	}
	switch cmd.Name() {
	case "version", "self-update", "help":
		return true
	}
	// stderr must be an *os.File pointing at a terminal; otherwise the
	// caller is piping output to a file/pipe/CI log and a notice would be
	// noise. Buffers and io.Discard are not *os.File so they always skip.
	f, ok := stderr.(*os.File)
	if !ok {
		return true
	}
	return !isatty.IsTerminal(f.Fd())
}

// newVersionSubcommand mirrors the bare positional `cocoon version`
// invocation. Cobra's built-in `--version` / `-v` covers the flag forms via
// SetVersionTemplate.
func newVersionSubcommand(version string, stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:           "version",
		Short:         "Print the cocoon binary version",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			_, err := io.WriteString(stdout, version+"\n")
			if err != nil {
				return err //nolint:wrapcheck // top-level write error is already descriptive
			}
			return nil
		},
	}
}
