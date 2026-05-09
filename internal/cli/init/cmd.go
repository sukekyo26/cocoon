package initcli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/setup"
)

const initLong = `cocoon init — generate workspace.toml in the current directory

Asks (when running interactively) for the container service name, the
inside-the-container username, the base OS / version, the mount range,
whether to emit .devcontainer/devcontainer.json, and which categories
of common apt packages to install. service_name and username have no
default — you must type them — because cocoon refuses to bake either
the cwd basename or your host $USER into a file you may commit.

Use --yes plus --service-name / --username (both required when --yes
is set) and any of --os / --os-version / --mount-root / --devcontainer
/ --apt-categories to drive non-interactively from CI.`

var (
	rxServiceName = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	rxUsername    = regexp.MustCompile(`^[a-z_][a-z0-9_-]*$`)
)

// NewCommand returns the cobra command for ` + "`cocoon init`" + `.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	flags := initFlags{
		AutoYes:        false,
		ServiceName:    "",
		Username:       "",
		OS:             "",
		OSVersion:      "",
		MountRoot:      "",
		Devcontainer:   false,
		NoDevcontainer: false,
		AptCategories:  "",
		Force:          false,
	}
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
	cmd.Flags().StringVar(&flags.OS, "os", "", fmt.Sprintf("base OS: %s", strings.Join(config.SupportedOSes, ", ")))
	cmd.Flags().StringVar(&flags.OSVersion, "os-version", "", "base OS version (must match --os)")
	cmd.Flags().StringVar(&flags.MountRoot, "mount-root", "", `mount range: "." (cwd, default) or ".." (parent)`)
	cmd.Flags().BoolVar(&flags.Devcontainer, "devcontainer", false, "force-enable .devcontainer/devcontainer.json output")
	cmd.Flags().BoolVar(&flags.NoDevcontainer, "no-devcontainer", false, "skip .devcontainer/devcontainer.json output")
	cmd.Flags().StringVar(
		&flags.AptCategories,
		"apt-categories",
		"",
		"comma-separated apt category IDs (skips the multi-select prompt)",
	)
	cmd.Flags().BoolVar(&flags.Force, "force", false, "overwrite an existing workspace.toml")
	return cmd
}

type initFlags struct {
	AutoYes        bool
	ServiceName    string
	Username       string
	OS             string
	OSVersion      string
	MountRoot      string
	Devcontainer   bool
	NoDevcontainer bool
	AptCategories  string
	Force          bool
}

func runInit(cmd *cobra.Command, stdout, _ io.Writer, flags *initFlags) error {
	if flags.Devcontainer && flags.NoDevcontainer {
		return fmt.Errorf("%w: --devcontainer and --no-devcontainer are mutually exclusive", ErrUsage)
	}
	cat := i18n.New(i18n.Detect())

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFailure, err)
	}
	target := filepath.Join(cwd, "workspace.toml")
	if _, statErr := os.Stat(target); statErr == nil && !flags.Force {
		return fmt.Errorf("%w: %s already exists; use --force to overwrite", ErrUsage, target)
	}

	serviceName, err := resolveServiceName(flags, cat)
	if err != nil {
		return err
	}
	username, err := resolveUsername(flags, cat)
	if err != nil {
		return err
	}
	osID, err := resolveOS(flags, cat)
	if err != nil {
		return err
	}
	osVersion, err := resolveOSVersion(flags, osID, cat)
	if err != nil {
		return err
	}
	mountRoot, err := resolveMountRoot(flags, cat)
	if err != nil {
		return err
	}
	devcontainer, err := resolveDevcontainer(flags, cat)
	if err != nil {
		return err
	}
	aptCatIDs, err := resolveAptCategories(flags, cat)
	if err != nil {
		return err
	}

	pkgs := setup.ExpandAptCategories(aptCatIDs)
	content := renderWorkspaceToml(containerSpec{
		ServiceName:  serviceName,
		Username:     username,
		OS:           osID,
		OSVersion:    osVersion,
		MountRoot:    mountRoot,
		Devcontainer: devcontainer,
		Packages:     pkgs,
	})
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil { //nolint:gosec // workspace.toml is user-readable.
		return fmt.Errorf("%w: write %s: %w", ErrFailure, target, err)
	}

	fmt.Fprintln(stdout, cat.Msg("init_wrote", target))
	printNextSteps(stdout, cat, devcontainer)
	_ = cmd // reserved for future ctx-aware flows
	return nil
}

func printNextSteps(stdout io.Writer, cat *i18n.Catalog, devcontainer bool) {
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, cat.Msg("init_next_header"))
	fmt.Fprintln(stdout, cat.Msg("init_next_step_gen"))
	fmt.Fprintln(stdout, cat.Msg("init_next_step_compose"))
	if devcontainer {
		fmt.Fprintln(stdout, cat.Msg("init_next_step_vscode"))
	}
}

// promptedIdent factors out the resolveServiceName / resolveUsername bodies:
// validate the flag if set, fail on --yes if missing, otherwise prompt.
type promptedIdent struct {
	flagName, flagValue, msgKey string // msgKey: "service_name" or "username"
	pattern                     *regexp.Regexp
	autoYes                     bool
}

func resolveIdent(p promptedIdent, cat *i18n.Catalog) (string, error) {
	if p.flagValue != "" {
		if !p.pattern.MatchString(p.flagValue) {
			return "", fmt.Errorf("%w: --%s %q does not match %s", ErrUsage, p.flagName, p.flagValue, p.pattern)
		}
		return p.flagValue, nil
	}
	if p.autoYes {
		return "", fmt.Errorf("%w: --yes requires --%s", ErrUsage, p.flagName)
	}
	var v string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(cat.Msg("init_prompt_" + p.msgKey)).
				Description(cat.Msg("init_desc_" + p.msgKey)).
				Validate(func(s string) error {
					if !p.pattern.MatchString(s) {
						return fmt.Errorf("%s", cat.Msg("init_err_"+p.msgKey, p.pattern)) //nolint:err113 // user-facing
					}
					return nil
				}).
				Value(&v),
		),
	)
	if err := form.Run(); err != nil {
		return "", fmt.Errorf("%w: %s prompt: %w", ErrFailure, p.flagName, err)
	}
	return v, nil
}

func resolveServiceName(flags *initFlags, cat *i18n.Catalog) (string, error) {
	return resolveIdent(promptedIdent{
		flagName: "service-name", flagValue: flags.ServiceName, msgKey: "service_name",
		pattern: rxServiceName, autoYes: flags.AutoYes,
	}, cat)
}

func resolveUsername(flags *initFlags, cat *i18n.Catalog) (string, error) {
	return resolveIdent(promptedIdent{
		flagName: "username", flagValue: flags.Username, msgKey: "username",
		pattern: rxUsername, autoYes: flags.AutoYes,
	}, cat)
}

func resolveOS(flags *initFlags, cat *i18n.Catalog) (string, error) {
	if flags.OS != "" {
		if _, ok := config.SupportedOsVersions[flags.OS]; !ok {
			return "", fmt.Errorf("%w: --os %q not in %s", ErrUsage, flags.OS, strings.Join(config.SupportedOSes, ", "))
		}
		return flags.OS, nil
	}
	if flags.AutoYes {
		return "ubuntu", nil
	}
	v := "ubuntu"
	options := make([]huh.Option[string], len(config.SupportedOSes))
	for i, id := range config.SupportedOSes {
		options[i] = huh.NewOption(id, id)
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(cat.Msg("init_prompt_os")).
				Description(cat.Msg("init_desc_os")).
				Options(options...).
				Value(&v),
		),
	)
	if err := form.Run(); err != nil {
		return "", fmt.Errorf("%w: os prompt: %w", ErrFailure, err)
	}
	return v, nil
}

func resolveOSVersion(flags *initFlags, osID string, cat *i18n.Catalog) (string, error) {
	versions, ok := config.SupportedOsVersions[osID]
	if !ok || len(versions) == 0 {
		return "", fmt.Errorf("%w: no supported versions for os %q", ErrFailure, osID)
	}
	if flags.OSVersion != "" {
		for _, v := range versions {
			if v == flags.OSVersion {
				return flags.OSVersion, nil
			}
		}
		return "", fmt.Errorf("%w: --os-version %q not in %s for %s",
			ErrUsage, flags.OSVersion, strings.Join(versions, ", "), osID)
	}
	if flags.AutoYes {
		// Prefer the current LTS for ubuntu (24.04 over the newer 26.04
		// which may not yet be on dockerhub for all arches); fall back to
		// the first listed version for other distros.
		if osID == "ubuntu" {
			for _, v := range versions {
				if v == "24.04" {
					return v, nil
				}
			}
		}
		return versions[0], nil
	}
	v := versions[0]
	options := make([]huh.Option[string], len(versions))
	for i, ver := range versions {
		options[i] = huh.NewOption(ver, ver)
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(cat.Msg("init_prompt_os_version", osID)).
				Description(cat.Msg("init_desc_os_version", osID)).
				Options(options...).
				Value(&v),
		),
	)
	if err := form.Run(); err != nil {
		return "", fmt.Errorf("%w: os-version prompt: %w", ErrFailure, err)
	}
	return v, nil
}

func resolveMountRoot(flags *initFlags, cat *i18n.Catalog) (string, error) {
	if flags.MountRoot != "" {
		if flags.MountRoot != "." && flags.MountRoot != ".." {
			return "", fmt.Errorf(`%w: --mount-root must be "." or ".."`, ErrUsage)
		}
		return flags.MountRoot, nil
	}
	if flags.AutoYes {
		return ".", nil
	}
	v := "."
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(cat.Msg("init_prompt_mount_root")).
				Description(cat.Msg("init_desc_mount_root")).
				Options(
					huh.NewOption(cat.Msg("init_option_mount_cwd"), "."),
					huh.NewOption(cat.Msg("init_option_mount_parent"), ".."),
				).
				Value(&v),
		),
	)
	if err := form.Run(); err != nil {
		return "", fmt.Errorf("%w: mount-root prompt: %w", ErrFailure, err)
	}
	return v, nil
}

func resolveDevcontainer(flags *initFlags, cat *i18n.Catalog) (bool, error) {
	if flags.Devcontainer {
		return true, nil
	}
	if flags.NoDevcontainer {
		return false, nil
	}
	if flags.AutoYes {
		return true, nil
	}
	v := true
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(cat.Msg("init_prompt_devcontainer")).
				Description(cat.Msg("init_desc_devcontainer")).
				Affirmative("Yes").
				Negative("No").
				Value(&v),
		),
	)
	if err := form.Run(); err != nil {
		return false, fmt.Errorf("%w: devcontainer prompt: %w", ErrFailure, err)
	}
	return v, nil
}

func resolveAptCategories(flags *initFlags, cat *i18n.Catalog) ([]string, error) {
	if flags.AptCategories != "" {
		var ids []string
		for _, raw := range strings.Split(flags.AptCategories, ",") {
			id := strings.TrimSpace(raw)
			if id == "" {
				continue
			}
			if setup.AptCategoryByID(id) == nil {
				return nil, fmt.Errorf("%w: unknown apt category %q (run `cocoon init --help` for the list)", ErrUsage, id)
			}
			ids = append(ids, id)
		}
		return ids, nil
	}
	if flags.AutoYes {
		return setup.DefaultAptCategoryIDs(), nil
	}
	selected := setup.DefaultAptCategoryIDs()
	options := make([]huh.Option[string], len(setup.AptCategories))
	for i, c := range setup.AptCategories {
		options[i] = huh.NewOption(fmt.Sprintf("%s (%s)", c.Label, c.Description), c.ID)
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title(cat.Msg("init_prompt_apt")).
				Description(cat.Msg("init_desc_apt")).
				Options(options...).
				Value(&selected),
		),
	)
	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("%w: apt-categories prompt: %w", ErrFailure, err)
	}
	return selected, nil
}

type containerSpec struct {
	ServiceName  string
	Username     string
	OS           string
	OSVersion    string
	MountRoot    string
	Devcontainer bool
	Packages     []string
}

func renderWorkspaceToml(s containerSpec) string {
	var sb strings.Builder
	sb.WriteString("# workspace.toml — cocoon configuration (generated by `cocoon init`)\n")
	sb.WriteString("# Edit freely; re-run `cocoon gen` to regenerate .devcontainer/.\n\n")

	sb.WriteString("[workspace]\n")
	fmt.Fprintf(&sb, "mount_root = %q\n", s.MountRoot)
	fmt.Fprintf(&sb, "devcontainer = %t\n\n", s.Devcontainer)

	sb.WriteString("[container]\n")
	fmt.Fprintf(&sb, "service_name = %q\n", s.ServiceName)
	fmt.Fprintf(&sb, "username = %q\n", s.Username)
	fmt.Fprintf(&sb, "os = %q\n", s.OS)
	fmt.Fprintf(&sb, "os_version = %q\n\n", s.OSVersion)

	sb.WriteString("[plugins]\n")
	sb.WriteString("enable = []\n\n")

	sb.WriteString("[apt]\n")
	if len(s.Packages) == 0 {
		sb.WriteString("packages = []\n")
		return sb.String()
	}
	sb.WriteString("packages = [\n")
	for _, pkg := range s.Packages {
		fmt.Fprintf(&sb, "  %q,\n", pkg)
	}
	sb.WriteString("]\n")
	return sb.String()
}
