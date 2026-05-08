package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	cleancli "github.com/sukekyo26/cocoon/internal/cli/clean"
	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	configcli "github.com/sukekyo26/cocoon/internal/cli/config"
	downcli "github.com/sukekyo26/cocoon/internal/cli/down"
	execcli "github.com/sukekyo26/cocoon/internal/cli/exec"
	gencli "github.com/sukekyo26/cocoon/internal/cli/gen"
	generatecli "github.com/sukekyo26/cocoon/internal/cli/generate"
	initcli "github.com/sukekyo26/cocoon/internal/cli/init"
	logscli "github.com/sukekyo26/cocoon/internal/cli/logs"
	plugincli "github.com/sukekyo26/cocoon/internal/cli/plugin"
	rebuildcli "github.com/sukekyo26/cocoon/internal/cli/rebuild"
	selfupdatecli "github.com/sukekyo26/cocoon/internal/cli/selfupdate"
	setupcli "github.com/sukekyo26/cocoon/internal/cli/setup"
	upcli "github.com/sukekyo26/cocoon/internal/cli/up"
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
		// Lifecycle verbs (cocoon v0.1.0). Most are stubs in F2; F3 wires
		// them to the .devcontainer/-centred generators and docker compose
		// calls. `init` is fully implemented here.
		initcli.NewCommand(stdout, stderr),
		upcli.NewCommand(stdout, stderr),
		downcli.NewCommand(stdout, stderr),
		rebuildcli.NewCommand(stdout, stderr),
		logscli.NewCommand(stdout, stderr),
		execcli.NewCommand(stdout, stderr),
		gencli.NewCommand(stdout, stderr),
		selfupdatecli.NewCommand(stdout, stderr),
		// Noun groups
		configcli.NewCommand(stdout, stderr),
		plugincli.NewCommand(stdout, stderr),
		// Legacy commands kept until F3 retires them.
		setupcli.NewCommand(stdout, stderr),
		generatecli.NewCommand(stdout, stderr),
		cleancli.NewCommand(stdout, stderr),
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
