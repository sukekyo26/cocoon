package plugincli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/i18n"
)

const pluginLong = `cocoon plugin — inspect and author cocoon plugins

Subcommands:
  list       list every plugin available in the layered view (project > user > embedded)
  show       print the resolved manifest for one plugin id
  pin        print a workspace.toml [plugins.versions.<id>] block
  scaffold   create a new <id>/ directory from a template

To use a plugin, add its id to [plugins].enable in workspace.toml — the
embedded catalog is picked up automatically. To customise an embedded
plugin, the supported workflow is "cocoon plugin scaffold <new-id>" and
adapting the logic. If you have a clone of the cocoon source repo (or an
unpacked source tarball), copying the embedded source from
internal/plugin/catalog/<id>/ into ~/.cocoon/plugins/<id>/ is a shortcut;
single-binary installs do not include the embedded source on disk.`

// NewCommand returns the cobra subtree for ` + "`cocoon plugin`" + `.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "plugin",
		Short:         "Inspect and author cocoon plugins (list / show / pin / scaffold)",
		Long:          pluginLong,
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
		return fmt.Errorf("%w: %w", ErrUsage, err)
	})
	cmd.AddCommand(
		newListCmd(stdout, stderr),
		newShowCmd(stdout, stderr),
		newPinCmd(stdout, stderr),
		newScaffoldCmd(stdout, stderr),
	)
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

func newScaffoldCmd(stdout, stderr io.Writer) *cobra.Command {
	//nolint:exhaustruct // setX flags populated post-parse from cmd.Flags().Changed
	opts := &scaffoldOpts{
		pluginsDir: "",
		template:   tmplGeneric,
	}
	cmd := &cobra.Command{
		Use:           "scaffold <id>",
		Short:         "Create a new <id>/ plugin directory (default <workspace>/.cocoon/plugins; --plugins-dir overrides)",
		Long:          scaffoldLong,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("%w: scaffold accepts at most one positional <id>, got %d", ErrUsage, len(args))
			}
			if len(args) == 1 {
				opts.id = args[0]
			}
			// Mirror the explicit-set tracking that the interactive form uses
			// to decide which fields to prompt for.
			opts.setName = cmd.Flags().Changed("name")
			opts.setDescription = cmd.Flags().Changed("description")
			opts.setDefaultEnabled = cmd.Flags().Changed("default")
			opts.setRequiresRoot = cmd.Flags().Changed("requires-root")
			opts.setVersionCapable = cmd.Flags().Changed("version-capable")
			opts.setTemplate = cmd.Flags().Changed("template")
			opts.setWithInstallUser = cmd.Flags().Changed("with-install-user")
			return runScaffoldFlow(opts, stdout, stderr)
		},
	}
	cmd.Flags().StringVar(&opts.pluginsDir, "plugins-dir", "",
		"output directory (default: <workspace>/.cocoon/plugins, auto-discovered from workspace.toml)")
	cmd.Flags().StringVar(&opts.name, "name", "", "display name (e.g. \"GitHub CLI\")")
	cmd.Flags().StringVar(&opts.description, "description", "",
		"short description; URL must be embedded as \"(...)\"")
	cmd.Flags().BoolVar(&opts.defaultEnabled, "default", false, "mark plugin enabled by default")
	cmd.Flags().BoolVar(&opts.requiresRoot, "requires-root", false, "install.sh runs as root")
	cmd.Flags().BoolVar(&opts.versionCapable, "version-capable", false, "generate $PIN / $CHECKSUM_* boilerplate")
	cmd.Flags().Var(&templateFlag{kind: &opts.template}, "template",
		"install.sh template: curl-pipe | tarball | generic")
	cmd.Flags().BoolVar(&opts.withInstallUser, "with-install-user", false, "also generate install_user.sh")
	cmd.Flags().BoolVar(&opts.nonInteractive, "non-interactive", false,
		"skip interactive prompts; require all fields above")
	cmd.Flags().BoolVar(&opts.force, "force", false, "overwrite <plugins-dir>/<id>/ if it already exists")
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return fmt.Errorf("%w: %w", ErrUsage, err)
	})
	return cmd
}

const scaffoldLong = `cocoon plugin scaffold — create a new <id>/ directory under the project plugins overlay

By default the new directory is created under <workspace>/.cocoon/plugins/<id>/,
auto-discovered from the nearest workspace.toml. Pass --plugins-dir <path> to
override (the path is taken as-is, joined with <id>).

The new directory contains a plugin.toml describing the plugin and an
install.sh skeleton matching the chosen template (curl-pipe / tarball /
generic). With --with-install-user a second install_user.sh hook is emitted.`

func runScaffoldFlow(opts *scaffoldOpts, stdout, stderr io.Writer) error {
	cat := i18n.New(i18n.Detect())

	if err := validateID(opts.id, cat, stderr); err != nil {
		return err
	}
	if !opts.nonInteractive {
		if err := promptMissing(opts, cat, huhPrompter{}); err != nil {
			return err
		}
	}
	if err := finalizeOpts(opts, cat, stderr); err != nil {
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
		return string(tmplGeneric)
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
