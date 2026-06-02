package initcli

import (
	"errors"
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
	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/logx"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	cat := i18n.New(i18n.Detect())
	var flags initFlags
	cmd := &cobra.Command{
		Use:           "init",
		Short:         cat.Msg("cmd_init_short"),
		Long:          cat.Msg("cmd_init_long"),
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInit(cmd, stdout, stderr, &flags)
		},
	}
	cmd.Flags().BoolVar(&flags.AutoYes, "yes", false, cat.Msg("flag_init_yes_usage"))
	cmd.Flags().StringVar(&flags.ServiceName, "service-name", "", cat.Msg("flag_init_service_name_usage"))
	cmd.Flags().StringVar(&flags.Username, "username", "", cat.Msg("flag_init_username_usage"))
	cmd.Flags().StringVar(&flags.Image, "image", "",
		cat.Msg("flag_init_image_usage", strings.Join(config.SupportedImages, ", ")))
	cmd.Flags().StringVar(&flags.ImageVersion, "image-version", "", cat.Msg("flag_init_image_version_usage"))
	cmd.Flags().StringVar(&flags.Shell, "shell", "",
		cat.Msg("flag_init_shell_usage", strings.Join(config.SupportedShells, ", ")))
	cmd.Flags().StringVar(&flags.MountRoot, "mount-root", "", cat.Msg("flag_init_mount_root_usage"))
	cmd.Flags().StringVar(&flags.Dir, "dir", "", cat.Msg("flag_init_dir_usage"))
	cmd.Flags().BoolVar(&flags.Devcontainer, "devcontainer", false, cat.Msg("flag_init_devcontainer_usage"))
	cmd.Flags().BoolVar(&flags.NoDevcontainer, "no-devcontainer", false, cat.Msg("flag_init_no_devcontainer_usage"))
	cmd.Flags().BoolVar(&flags.Certificates, "certificates", false, cat.Msg("flag_init_certificates_usage"))
	cmd.Flags().BoolVar(&flags.NoCertificates, "no-certificates", false, cat.Msg("flag_init_no_certificates_usage"))
	cmd.Flags().StringVar(&flags.Sudo, "sudo", "",
		cat.Msg("flag_init_sudo_usage", strings.Join(sudoChoices, ", ")))
	cmd.Flags().BoolVar(&flags.ImagePathFix, "image-path-fix", false, cat.Msg("flag_init_image_path_fix_usage"))
	cmd.Flags().BoolVar(&flags.NoImagePathFix, "no-image-path-fix", false, cat.Msg("flag_init_no_image_path_fix_usage"))
	cmd.Flags().StringVar(&flags.AptCategories, "apt-categories", "", cat.Msg("flag_init_apt_categories_usage"))
	cmd.Flags().StringVar(&flags.Plugins, "plugins", "", cat.Msg("flag_init_plugins_usage"))
	cmd.Flags().StringVar(&flags.PluginVersions, "plugin-versions", "", cat.Msg("flag_init_plugin_versions_usage"))
	cmd.Flags().StringVar(&flags.PluginMethods, "plugin-methods", "", cat.Msg("flag_init_plugin_methods_usage"))
	cmd.Flags().StringVar(&flags.AliasBundles, "alias-bundles", "", cat.Msg("flag_init_alias_bundles_usage"))
	cmd.Flags().StringVar(&flags.Ports, "ports", "", cat.Msg("flag_init_ports_usage"))
	cmd.Flags().BoolVar(&flags.Force, "force", false, cat.Msg("flag_init_force_usage"))
	return cmd
}

func runInit(cmd *cobra.Command, stdout, stderr io.Writer, flags *initFlags) error {
	if flags.Devcontainer && flags.NoDevcontainer {
		return fmt.Errorf("%w: --devcontainer and --no-devcontainer are mutually exclusive", clihelpers.ErrUsage)
	}
	if flags.Certificates && flags.NoCertificates {
		return fmt.Errorf("%w: --certificates and --no-certificates are mutually exclusive", clihelpers.ErrUsage)
	}
	if flags.ImagePathFix && flags.NoImagePathFix {
		return fmt.Errorf("%w: --image-path-fix and --no-image-path-fix are mutually exclusive", clihelpers.ErrUsage)
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
		Sudo:           ans.Sudo,
		ImagePathFix:   ans.ImagePathFix,
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
	if err := seedSudoPasswordIfRequested(cwd, ans, log, cat); err != nil {
		return err
	}
	printNextSteps(log, cat, ans.Devcontainer)
	_ = cmd // reserved for future ctx-aware flows
	return nil
}

// seedSudoPasswordIfRequested seeds .env.local only when password sudo was
// chosen interactively (a password was collected). `--yes --sudo password`
// sets the mode but collects no password, so it leaves the file to the user.
func seedSudoPasswordIfRequested(cwd string, ans initAnswers, log *logx.Logger, cat *i18n.Catalog) error {
	if ans.Sudo != config.SudoModePassword || ans.SudoPassword == "" {
		return nil
	}
	return seedSudoPasswordEnvLocal(cwd, ans.SudoPassword, log, cat)
}

// seedSudoPasswordEnvLocal writes the interactively-entered sudo password into
// .devcontainer/.env.local (mode 0600) and the matching .gitignore so the
// secret is excluded from git from the moment it is created. .env.local is
// NEVER overwritten (it is the user's secret); .gitignore is deterministic and
// rewritten safely.
func seedSudoPasswordEnvLocal(cwd, password string, log *logx.Logger, cat *i18n.Catalog) error {
	devDir := filepath.Join(cwd, ".devcontainer")
	if err := os.MkdirAll(devDir, 0o755); err != nil {
		return fmt.Errorf("%w: %w", clihelpers.ErrFailure, err)
	}
	gitignore := filepath.Join(devDir, ".gitignore")
	//nolint:gosec // .gitignore is a public, committable file (0644 is intended).
	if err := os.WriteFile(gitignore, []byte(generate.SudoPasswordGitignore), 0o644); err != nil {
		return fmt.Errorf("%w: write %s: %w", clihelpers.ErrFailure, gitignore, err)
	}
	envLocal := filepath.Join(devDir, generate.SudoPasswordSecretFile)
	f, err := os.OpenFile(envLocal, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if errors.Is(err, os.ErrExist) {
		log.Warn(cat.Msg("init_sudo_env_local_exists", envLocal))
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: create %s: %w", clihelpers.ErrFailure, envLocal, err)
	}
	_, writeErr := f.WriteString(generate.SudoPasswordEnvKey + "=" + password + "\n")
	closeErr := f.Close()
	if writeErr != nil {
		return fmt.Errorf("%w: write %s: %w", clihelpers.ErrFailure, envLocal, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("%w: close %s: %w", clihelpers.ErrFailure, envLocal, closeErr)
	}
	log.Success(cat.Msg("init_sudo_env_local_wrote", envLocal))
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
