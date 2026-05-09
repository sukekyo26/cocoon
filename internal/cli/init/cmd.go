package initcli

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/setup"
)

const initLong = `cocoon init — generate workspace.toml in the current directory

Asks (when running interactively) for the container service name, the
inside-the-container username, the base OS / version, the mount range,
whether to emit .devcontainer/devcontainer.json, and which categories
of common apt packages to install. The answers are written into a fresh
workspace.toml at the project root.

Use --yes plus --service-name / --username / --os / --os-version /
--mount-root / --devcontainer / --apt-categories to drive non-
interactively from CI.`

var (
	rxServiceName = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	rxUsername    = regexp.MustCompile(`^[a-z_][a-z0-9_-]*$`)

	errBadServiceName = fmt.Errorf("must match %s", rxServiceName)
	errBadUsername    = fmt.Errorf("must match %s", rxUsername)
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
	cmd.Flags().BoolVar(&flags.AutoYes, "yes", false, "skip interactive prompts and use defaults")
	cmd.Flags().StringVar(&flags.ServiceName, "service-name", "", "compose service name (default: sanitized cwd basename)")
	cmd.Flags().StringVar(&flags.Username, "username", "", "in-container user (default: $USER, sanitized)")
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

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFailure, err)
	}
	target := filepath.Join(cwd, "workspace.toml")
	if _, statErr := os.Stat(target); statErr == nil && !flags.Force {
		return fmt.Errorf("%w: %s already exists; use --force to overwrite", ErrUsage, target)
	}

	serviceName, err := resolveServiceName(flags, cwd)
	if err != nil {
		return err
	}
	username, err := resolveUsername(flags)
	if err != nil {
		return err
	}
	osID, err := resolveOS(flags)
	if err != nil {
		return err
	}
	osVersion, err := resolveOSVersion(flags, osID)
	if err != nil {
		return err
	}
	mountRoot, err := resolveMountRoot(flags)
	if err != nil {
		return err
	}
	devcontainer, err := resolveDevcontainer(flags)
	if err != nil {
		return err
	}
	aptCatIDs, err := resolveAptCategories(flags)
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

	fmt.Fprintf(stdout, "wrote %s\n", target)
	printNextSteps(stdout, devcontainer)
	_ = cmd // reserved for future ctx-aware flows
	return nil
}

func printNextSteps(stdout io.Writer, devcontainer bool) {
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Next steps:")
	fmt.Fprintln(stdout, "  1. cocoon gen")
	fmt.Fprintln(stdout, "  2. docker compose -f .devcontainer/docker-compose.yml up -d")
	if devcontainer {
		fmt.Fprintln(stdout, `     (or open in VS Code → "Reopen in Container")`)
	}
}

// sanitizeIdent maps an arbitrary string to something that satisfies pat by
// lowercasing and replacing every disallowed rune with `-`. If the result
// would not start with a letter (or `_` for usernames), it is prefixed with
// `app-` / `user-` so the prompt has a viable default rather than empty.
func sanitizeIdent(s string, allowLeadingUnderscore bool) string {
	s = strings.ToLower(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= '0' && r <= '9',
			r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return ""
	}
	first := out[0]
	startOK := (first >= 'a' && first <= 'z') || (allowLeadingUnderscore && first == '_')
	if !startOK {
		prefix := "app-"
		if allowLeadingUnderscore {
			prefix = "user-"
		}
		out = prefix + out
	}
	return out
}

func defaultServiceName(cwd string) string {
	return sanitizeIdent(filepath.Base(cwd), false)
}

func defaultUsername() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		if name := sanitizeIdent(u.Username, true); name != "" {
			return name
		}
	}
	if v := os.Getenv("USER"); v != "" {
		if name := sanitizeIdent(v, true); name != "" {
			return name
		}
	}
	return "user"
}

func resolveServiceName(flags *initFlags, cwd string) (string, error) {
	if flags.ServiceName != "" {
		if !rxServiceName.MatchString(flags.ServiceName) {
			return "", fmt.Errorf("%w: --service-name %q does not match %s", ErrUsage, flags.ServiceName, rxServiceName)
		}
		return flags.ServiceName, nil
	}
	def := defaultServiceName(cwd)
	if flags.AutoYes {
		if def == "" || !rxServiceName.MatchString(def) {
			return "", fmt.Errorf("%w: cannot derive service_name from cwd %q; pass --service-name", ErrUsage, cwd)
		}
		return def, nil
	}
	v := def
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Service name").
				Description("Compose service name; lowercase letters, digits, _ and - only.").
				Validate(func(s string) error {
					if !rxServiceName.MatchString(s) {
						return errBadServiceName
					}
					return nil
				}).
				Value(&v),
		),
	)
	if err := form.Run(); err != nil {
		return "", fmt.Errorf("%w: service-name prompt: %w", ErrFailure, err)
	}
	return v, nil
}

func resolveUsername(flags *initFlags) (string, error) {
	if flags.Username != "" {
		if !rxUsername.MatchString(flags.Username) {
			return "", fmt.Errorf("%w: --username %q does not match %s", ErrUsage, flags.Username, rxUsername)
		}
		return flags.Username, nil
	}
	def := defaultUsername()
	if flags.AutoYes {
		if !rxUsername.MatchString(def) {
			return "", fmt.Errorf("%w: cannot derive username from $USER; pass --username", ErrUsage)
		}
		return def, nil
	}
	v := def
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Username").
				Description("In-container user; lowercase letters, digits, _ and - only.").
				Validate(func(s string) error {
					if !rxUsername.MatchString(s) {
						return errBadUsername
					}
					return nil
				}).
				Value(&v),
		),
	)
	if err := form.Run(); err != nil {
		return "", fmt.Errorf("%w: username prompt: %w", ErrFailure, err)
	}
	return v, nil
}

func resolveOS(flags *initFlags) (string, error) {
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
				Title("Base OS").
				Description("Linux distribution that backs the container image (FROM <os>:<os_version>).").
				Options(options...).
				Value(&v),
		),
	)
	if err := form.Run(); err != nil {
		return "", fmt.Errorf("%w: os prompt: %w", ErrFailure, err)
	}
	return v, nil
}

func resolveOSVersion(flags *initFlags, osID string) (string, error) {
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
		// Default: the first LTS-ish entry. For ubuntu we skip the leading
		// 26.04 (newest, may not yet be on dockerhub for all arches) and
		// pick 24.04; for other distros take the first listed version.
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
				Title(fmt.Sprintf("%s version", osID)).
				Description(fmt.Sprintf("Pulled as FROM %s:<version> in the generated Dockerfile.", osID)).
				Options(options...).
				Value(&v),
		),
	)
	if err := form.Run(); err != nil {
		return "", fmt.Errorf("%w: os-version prompt: %w", ErrFailure, err)
	}
	return v, nil
}

func resolveMountRoot(flags *initFlags) (string, error) {
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
				Title("Mount range").
				Description("How much of your filesystem should be visible inside the container?").
				Options(
					huh.NewOption("Just this project (.)", "."),
					huh.NewOption("Parent directory — sibling repos visible (..)", ".."),
				).
				Value(&v),
		),
	)
	if err := form.Run(); err != nil {
		return "", fmt.Errorf("%w: mount-root prompt: %w", ErrFailure, err)
	}
	return v, nil
}

func resolveDevcontainer(flags *initFlags) (bool, error) {
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
				Title("Generate .devcontainer/devcontainer.json for VS Code Dev Containers?").
				Description("Says yes if you ever open this repo in VS Code Dev Containers; harmless otherwise.").
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

func resolveAptCategories(flags *initFlags) ([]string, error) {
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
				Title("Select common apt packages to install").
				Description("Pre-checked categories are installed by default; uncheck what you do not need.").
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
