package configcli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

const configLong = `wsd config — parse and validate workspace and plugin TOML

Inspects workspace.toml and the plugin tree and prints scalar / list / TOML
fragments for use by the bash entry point scripts.`

// NewCommand returns the cobra subtree for ` + "`wsd config`" + `.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "config",
		Short:         "Parse and validate workspace and plugin TOML",
		Long:          configLong,
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

	type sub struct {
		use     string
		short   string
		handler func([]string, io.Writer, io.Writer) error
	}
	subs := []sub{
		{"get <file> <field>", "Print scalar from workspace.toml", cmdGet},
		{"list <file> <field>", "Print array from workspace.toml, one per line", cmdList},
		{"volumes <file>", "Print [volumes] entries as name<TAB>path", cmdVolumes},
		{"plugin-get <dir-or-file> <field>", "Print scalar from plugin.toml", cmdPluginGet},
		{"plugin-list <dir-or-file> <field>", "Print array from plugin.toml", cmdPluginList},
		{"plugin-volumes <dir-or-file>", "Print plugin install.volumes as name<TAB>path", cmdPluginVolumes},
		{
			"plugins-table <dir>", "Print one plugin row per line: id<TAB>name<TAB>default<TAB>description",
			cmdPluginsTable,
		},
		{"validate-workspace <file> [plugins]", "Validate workspace.toml", cmdValidateWorkspace},
		{"validate-plugins <dir>", "Validate every plugin under <dir>", cmdValidatePlugins},
		{"has-section <file> <section>", "Print true/false", cmdHasSection},
		{"list-sidecars <file>", "Print one [services.<name>] key per line", cmdListSidecars},
		{"dump-devcontainer <file>", "Dump [devcontainer] as TOML", cmdDumpDevcontainer},
		{"dump-repositories <file>", "Dump [repositories] as TOML", cmdDumpRepositories},
		{"repositories <file>", "Emit [repositories].clone as JSON", cmdRepositories},
		{"format-repositories <file|->", "Format JSON entries as TOML", cmdFormatRepositories},
	}
	for _, s := range subs {
		cmd.AddCommand(newConfigSubcmd(s.use, s.short, s.handler, stdout, stderr))
	}
	return cmd
}

func newConfigSubcmd(
	use, short string,
	handler func([]string, io.Writer, io.Writer) error,
	stdout, stderr io.Writer,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:           use,
		Short:         short,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, args []string) error {
			return handler(args, stdout, stderr)
		},
	}
	cmd.SetFlagErrorFunc(usageErr)
	return cmd
}

func usageErr(_ *cobra.Command, err error) error {
	return fmt.Errorf("%w: %w", ErrUsage, err)
}

// rejectUnknownSubcommand returns an ErrUsage-wrapped error when a stray
// positional appears under a parent that only carries subcommands.
func rejectUnknownSubcommand(_ *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	return fmt.Errorf("%w: unknown subcommand %q", ErrUsage, args[0])
}
