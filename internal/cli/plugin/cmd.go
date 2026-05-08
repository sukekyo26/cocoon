package plugincli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/i18n"
)

const pluginLong = `wsd plugin — manage workspace-docker plugins

Subcommands:
  scaffold   create a new plugins/<id>/ directory from a template`

// NewCommand returns the cobra subtree for ` + "`wsd plugin`" + `.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "plugin",
		Short:         "Plugin authoring helpers (scaffold, ...)",
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
	cmd.AddCommand(newScaffoldCmd(stdout, stderr))
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
		pluginsDir: "plugins",
		template:   tmplGeneric,
	}
	cmd := &cobra.Command{
		Use:           "scaffold <id>",
		Short:         "Create a plugins/<id>/ directory from a template",
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
	cmd.Flags().StringVar(&opts.pluginsDir, "plugins-dir", "plugins", "output directory")
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
	cmd.Flags().BoolVar(&opts.force, "force", false, "overwrite plugins/<id>/ if it already exists")
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return fmt.Errorf("%w: %w", ErrUsage, err)
	})
	return cmd
}

const scaffoldLong = `wsd plugin scaffold — create a new plugins/<id>/ directory

The new directory contains a plugin.toml describing the plugin and an
install.sh skeleton matching the chosen template (curl-pipe / tarball /
generic).`

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
