package plugincli

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/i18n"
)

func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	cat := i18n.New(i18n.Detect())
	cmd := &cobra.Command{
		Use:           "plugin",
		Short:         cat.Msg("cmd_plugin_short"),
		Long:          cat.Msg("cmd_plugin_long"),
		Args:          rejectUnknownSubcommand,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help() //nolint:wrapcheck // help write error is descriptive
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return clihelpers.UsageWrap(err, "")
	})
	cmd.AddCommand(
		newListCmd(stdout, stderr),
		newShowCmd(stdout, stderr),
		newPinCmd(stdout, stderr),
		newScaffoldCmd(stdout, stderr),
	)
	// Mirror `cocoon gen`'s legacy positional help alias so `cocoon plugin help`
	// keeps printing the parent usage instead of being intercepted by
	// rejectUnknownSubcommand. The recursive addLeafHelpAlias in root only
	// attaches the alias on leaf commands, so non-leaf parents like `plugin`
	// need to attach explicitly.
	clihelpers.AttachHelpAlias(cmd)
	return cmd
}

// rejectUnknownSubcommand returns a clihelpers.ErrUsage-wrapped error when a stray
// positional appears under a parent that only carries subcommands.
func rejectUnknownSubcommand(_ *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	return clihelpers.UsageErr("err_plugincmd_unknown_subcommand", args[0])
}

func newScaffoldCmd(stdout, stderr io.Writer) *cobra.Command {
	cat := i18n.New(i18n.Detect())
	//nolint:exhaustruct // setX flags populated post-parse from cmd.Flags().Changed
	opts := &scaffoldOpts{
		pluginsDir: "",
		template:   tmplInstaller,
	}
	cmd := &cobra.Command{
		Use:           "scaffold <id>",
		Short:         cat.Msg("cmd_plugin_scaffold_cmd_short"),
		Long:          cat.Msg("cmd_plugin_scaffold_cmd_long"),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return clihelpers.UsageErr("err_plugincmd_scaffold_too_many_args", len(args))
			}
			if len(args) == 1 {
				opts.id = args[0]
			}
			// Mirror the explicit-set tracking that the interactive form uses
			// to decide which fields to prompt for.
			opts.setName = cmd.Flags().Changed("name")
			opts.setDescription = cmd.Flags().Changed("description")
			opts.setURL = cmd.Flags().Changed("url")
			opts.setDefaultEnabled = cmd.Flags().Changed("default")
			opts.setRequiresRoot = cmd.Flags().Changed("requires-root")
			opts.setVersionCapable = cmd.Flags().Changed("version-capable")
			opts.setTemplate = cmd.Flags().Changed("template")
			opts.setWithInstallUser = cmd.Flags().Changed("with-install-user")
			return runScaffoldFlow(opts, stdout, stderr)
		},
	}
	cmd.Flags().StringVar(&opts.pluginsDir, "plugins-dir", "", cat.Msg("flag_plugin_scaffold_cmd_plugins_dir_usage"))
	cmd.Flags().StringVar(&opts.name, "name", "", cat.Msg("flag_plugin_scaffold_cmd_name_usage"))
	cmd.Flags().StringVar(&opts.description, "description", "", cat.Msg("flag_plugin_scaffold_cmd_description_usage"))
	cmd.Flags().StringVar(&opts.url, "url", "", cat.Msg("flag_plugin_scaffold_cmd_url_usage"))
	cmd.Flags().BoolVar(&opts.defaultEnabled, "default", false, cat.Msg("flag_plugin_scaffold_cmd_default_usage"))
	cmd.Flags().BoolVar(&opts.requiresRoot, "requires-root", false,
		cat.Msg("flag_plugin_scaffold_cmd_requires_root_usage"))
	cmd.Flags().BoolVar(&opts.versionCapable, "version-capable", false,
		cat.Msg("flag_plugin_scaffold_cmd_version_capable_usage"))
	cmd.Flags().Var(&templateFlag{kind: &opts.template}, "template", cat.Msg("flag_plugin_scaffold_cmd_template_usage"))
	cmd.Flags().BoolVar(&opts.withInstallUser, "with-install-user", false,
		cat.Msg("flag_plugin_scaffold_cmd_with_install_user_usage"))
	cmd.Flags().BoolVar(&opts.nonInteractive, "non-interactive", false,
		cat.Msg("flag_plugin_scaffold_cmd_non_interactive_usage"))
	cmd.Flags().BoolVar(&opts.force, "force", false, cat.Msg("flag_plugin_scaffold_cmd_force_usage"))
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return clihelpers.UsageWrap(err, "")
	})
	return cmd
}

func runScaffoldFlow(opts *scaffoldOpts, stdout, stderr io.Writer) error {
	cat := i18n.New(i18n.Detect())

	if err := validateID(opts.id); err != nil {
		return err
	}
	if !opts.nonInteractive {
		if err := promptMissing(opts, cat, huhPrompter{}); err != nil {
			return err
		}
	}
	if err := finalizeOpts(opts); err != nil {
		return err
	}
	return runScaffold(*opts, cat, stdout, stderr)
}

// templateFlag is a pflag.Value adapter for the template kind enum.
type templateFlag struct {
	kind *templateKind
}

// String renders the current template kind for `--help` output.
func (t *templateFlag) String() string {
	if t.kind == nil {
		return string(tmplInstaller)
	}
	return string(*t.kind)
}

// Set assigns the parsed string value back into the bound *templateKind.
func (t *templateFlag) Set(s string) error {
	*t.kind = templateKind(s)
	return nil
}

// Type satisfies pflag.Value; declaring "string" matches the underlying type.
func (*templateFlag) Type() string { return "string" }
