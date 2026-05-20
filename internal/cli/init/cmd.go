package initcli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/aliasbundles"
	"github.com/sukekyo26/cocoon/internal/aptcategories"
	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/logx"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

const initLong = `cocoon init — generate workspace.toml in the current directory

Asks (when running interactively) for the container service name, the
inside-the-container username, the base image / version, the mount range,
whether to emit .devcontainer/devcontainer.json, and which categories
of common apt packages to install. service_name and username have no
default — you must type them — because cocoon refuses to bake either
the cwd basename or your host $USER into a file you may commit.

The interactive flow asks each question on its own screen. Empty
service-name / username are rejected on submission. shift+tab does
not navigate back across questions — re-run cocoon init to fix an
earlier answer. (Each prompt being its own form sidesteps a class of
viewport-sizing bugs in huh's multi-Group + OptionsFunc combination
that caused the cursor indicator to stay pinned while options
scrolled under it.)

Use --yes plus --service-name / --username (both required when --yes
is set) and any of --image / --image-version / --shell / --mount-root /
--dir / --devcontainer / --apt-categories / --plugins / --alias-bundles
to drive non-interactively from CI.`

// NewCommand returns the cobra command for `cocoon init`.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	var flags initFlags
	cmd := &cobra.Command{
		Use:           "init",
		Short:         "Create workspace.toml in the current directory",
		Long:          initLong,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInit(cmd, stdout, stderr, &flags)
		},
	}
	cmd.Flags().BoolVar(&flags.AutoYes, "yes", false, "skip optional prompts; --service-name and --username then required")
	cmd.Flags().StringVar(&flags.ServiceName, "service-name", "", "compose service name (required with --yes)")
	cmd.Flags().StringVar(&flags.Username, "username", "", "in-container user (required with --yes)")
	cmd.Flags().StringVar(&flags.Image, "image", "",
		fmt.Sprintf("base image: %s", strings.Join(config.SupportedImages, ", ")))
	cmd.Flags().StringVar(&flags.ImageVersion, "image-version", "",
		"base image tag — any well-formed Docker tag is accepted; --image must also be set")
	cmd.Flags().StringVar(&flags.Shell, "shell", "",
		fmt.Sprintf("container login shell: %s (default: bash)", strings.Join(config.SupportedShells, ", ")))
	cmd.Flags().StringVar(&flags.MountRoot, "mount-root", "", `mount range: "." (cwd, default) or ".." (parent)`)
	cmd.Flags().StringVar(&flags.Dir, "dir", "",
		`container workdir parent under /home/<user>/ `+
			`(default "workspace"; slashes allowed for nested paths, e.g. "work/myproject")`)
	cmd.Flags().BoolVar(&flags.Devcontainer, "devcontainer", false, "force-enable .devcontainer/devcontainer.json output")
	cmd.Flags().BoolVar(&flags.NoDevcontainer, "no-devcontainer", false, "skip .devcontainer/devcontainer.json output")
	cmd.Flags().BoolVar(&flags.Certificates, "certificates", false,
		"force-enable [certificates] auto-bake from ~/.cocoon/certs/")
	cmd.Flags().BoolVar(&flags.NoCertificates, "no-certificates", false,
		"skip the [certificates] section (default off)")
	cmd.Flags().StringVar(
		&flags.AptCategories,
		"apt-categories",
		"",
		"comma-separated apt category IDs (skips the multi-select prompt)",
	)
	cmd.Flags().StringVar(
		&flags.Plugins,
		"plugins",
		"",
		"comma-separated plugin IDs to enable (skips the plugin multi-select prompt)",
	)
	cmd.Flags().StringVar(
		&flags.PluginVersions,
		"plugin-versions",
		"",
		"comma-separated <id>=<ref> pins for version_capable plugins (each <id> must also appear in --plugins)",
	)
	cmd.Flags().StringVar(
		&flags.PluginMethods,
		"plugin-methods",
		"",
		"comma-separated <id>=<method> picks for plugins that declare multiple [install.methods] "+
			"(each <id> must also appear in --plugins; <method> must be a declared key)",
	)
	cmd.Flags().StringVar(
		&flags.AliasBundles,
		"alias-bundles",
		"",
		"comma-separated shell-alias bundle IDs (skips the bundles multi-select prompt; e.g. git,ls)",
	)
	cmd.Flags().StringVar(
		&flags.Ports,
		"ports",
		"",
		"comma-separated docker-compose short-form port mappings — "+
			"[HOST_IP:][HOST:]CONTAINER[/PROTOCOL]; numeric ranges (N-M) and tcp|udp are accepted "+
			"(e.g. 3000,3000-3005,8000:8000,127.0.0.1:5432:5432/tcp,6060:6060/udp); skips the ports prompt",
	)
	cmd.Flags().BoolVar(&flags.Force, "force", false, "overwrite an existing workspace.toml")
	return cmd
}

func runInit(cmd *cobra.Command, stdout, stderr io.Writer, flags *initFlags) error {
	if flags.Devcontainer && flags.NoDevcontainer {
		return fmt.Errorf("%w: --devcontainer and --no-devcontainer are mutually exclusive", clihelpers.ErrUsage)
	}
	if flags.Certificates && flags.NoCertificates {
		return fmt.Errorf("%w: --certificates and --no-certificates are mutually exclusive", clihelpers.ErrUsage)
	}
	cat := i18n.New(i18n.Detect())

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("%w: %w", clihelpers.ErrFailure, err)
	}
	target := filepath.Join(cwd, "workspace.toml")
	if _, statErr := os.Stat(target); statErr == nil && !flags.Force {
		return fmt.Errorf("%w: %s already exists; use --force to overwrite", clihelpers.ErrUsage, target)
	}

	plugins, err := loadEmbeddedPlugins()
	if err != nil {
		return fmt.Errorf("%w: %s", clihelpers.ErrFailure, cat.Msg("init_err_plugin_load_fmt", err))
	}

	ans, err := collectAnswers(flags, cat, plugins)
	if err != nil {
		return err
	}

	pkgs := aptcategories.ExpandAptCategories(ans.AptCategories)
	aliases := aliasbundles.ExpandAliasBundles(ans.AliasBundles)
	content := renderWorkspaceToml(containerSpec{
		ServiceName:    ans.ServiceName,
		Username:       ans.Username,
		Image:          ans.Image,
		ImageVersion:   ans.ImageVersion,
		Shell:          ans.Shell,
		Aliases:        aliases,
		MountRoot:      ans.MountRoot,
		Dir:            ans.Dir,
		Devcontainer:   ans.Devcontainer,
		Certificates:   ans.Certificates,
		Packages:       pkgs,
		Plugins:        ans.Plugins,
		PluginVersions: ans.PluginVersions,
		PluginMethods:  ans.PluginMethods,
		Ports:          ans.Ports,
	}, cat)
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil { //nolint:gosec // workspace.toml is user-readable.
		return fmt.Errorf("%w: write %s: %w", clihelpers.ErrFailure, target, err)
	}

	log := logx.New(stdout, stderr)
	log.Success(cat.Msg("init_wrote", target))
	printNextSteps(log, cat, ans.Devcontainer)
	_ = cmd // reserved for future ctx-aware flows
	return nil
}

func printNextSteps(log *logx.Logger, cat *i18n.Catalog, devcontainer bool) {
	log.Info("")
	log.Info(log.Bold(cat.Msg("init_next_header")))
	log.Info(cat.Msg("init_next_step_gen"))
	log.Info(cat.Msg("init_next_step_compose"))
	if devcontainer {
		log.Info(cat.Msg("init_next_step_vscode"))
	}
}

// collectAnswers runs the cross-check on both paths so the non-interactive
// route (`--plugins go --image golang`) fails fast instead of writing a
// workspace.toml that `cocoon gen` would later reject. The interactive
// picker already filters conflicts; the check is a no-op there.
func collectAnswers(flags *initFlags, cat *i18n.Catalog, plugins map[string]*plugin.Plugin) (initAnswers, error) {
	ans, err := applyFlags(flags, plugins)
	if err != nil {
		return ans, err
	}
	if flags.AutoYes {
		ans, err = applyDefaults(ans, plugins)
	} else {
		ans, err = promptForMissing(ans, cat, plugins)
	}
	if err != nil {
		return ans, err
	}
	if err := assertNoImagePluginConflict(ans); err != nil {
		return ans, err
	}
	return ans, nil
}
