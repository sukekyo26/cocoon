package initcli

import (
	"errors"
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

The interactive flow asks each question on its own screen. Empty
service-name / username are rejected on submission. shift+tab does
not navigate back across questions — re-run cocoon init to fix an
earlier answer. (Each prompt being its own form sidesteps a class of
viewport-sizing bugs in huh's multi-Group + OptionsFunc combination
that caused the cursor indicator to stay pinned while options
scrolled under it.)

Use --yes plus --service-name / --username (both required when --yes
is set) and any of --os / --os-version / --mount-root / --devcontainer
/ --apt-categories to drive non-interactively from CI.`

var (
	rxServiceName = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	rxUsername    = regexp.MustCompile(`^[a-z_][a-z0-9_-]*$`)
)

// NewCommand returns the cobra command for ` + "`cocoon init`" + `.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	flags := zeroFlags()
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

func zeroFlags() initFlags {
	return initFlags{
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
}

// initAnswers is the resolved value set written into workspace.toml.
// The *Set companions distinguish "not yet provided" from a zero value
// the user actively chose (e.g. devcontainer = false). Without them
// flag-set vs prompt-pending would be ambiguous and the prompt builder
// would skip groups whose value happens to look empty.
type initAnswers struct {
	ServiceName     string
	Username        string
	OS              string
	OSSet           bool
	OSVersion       string
	OSVersionSet    bool
	MountRoot       string
	MountRootSet    bool
	Devcontainer    bool
	DevcontainerSet bool
	AptCategories   []string
	AptSet          bool
}

func zeroAnswers() initAnswers {
	return initAnswers{
		ServiceName:     "",
		Username:        "",
		OS:              "",
		OSSet:           false,
		OSVersion:       "",
		OSVersionSet:    false,
		MountRoot:       "",
		MountRootSet:    false,
		Devcontainer:    false,
		DevcontainerSet: false,
		AptCategories:   nil,
		AptSet:          false,
	}
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

	ans, err := collectAnswers(flags, cat)
	if err != nil {
		return err
	}

	pkgs := setup.ExpandAptCategories(ans.AptCategories)
	content := renderWorkspaceToml(containerSpec{
		ServiceName:  ans.ServiceName,
		Username:     ans.Username,
		OS:           ans.OS,
		OSVersion:    ans.OSVersion,
		MountRoot:    ans.MountRoot,
		Devcontainer: ans.Devcontainer,
		Packages:     pkgs,
	})
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil { //nolint:gosec // workspace.toml is user-readable.
		return fmt.Errorf("%w: write %s: %w", ErrFailure, target, err)
	}

	fmt.Fprintln(stdout, cat.Msg("init_wrote", target))
	printNextSteps(stdout, cat, ans.Devcontainer)
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

// collectAnswers folds CLI flags into initAnswers, then either applies
// safe defaults (when --yes is set) or runs an interactive form for
// whatever is still missing.
func collectAnswers(flags *initFlags, cat *i18n.Catalog) (initAnswers, error) {
	ans, err := applyFlags(flags)
	if err != nil {
		return ans, err
	}
	if flags.AutoYes {
		return applyDefaults(ans)
	}
	return promptForMissing(ans, cat)
}

// applyFlags copies validated CLI-flag values into initAnswers and marks
// the corresponding *Set flags. Empty flags leave the field zero so the
// prompt or default layer knows to fill it in.
//
//nolint:gocognit,gocyclo // sequence of independent flag checks; splitting hides intent.
func applyFlags(flags *initFlags) (initAnswers, error) {
	ans := zeroAnswers()
	if flags.ServiceName != "" {
		if !rxServiceName.MatchString(flags.ServiceName) {
			return ans, fmt.Errorf("%w: --service-name %q does not match %s",
				ErrUsage, flags.ServiceName, rxServiceName)
		}
		ans.ServiceName = flags.ServiceName
	}
	if flags.Username != "" {
		if !rxUsername.MatchString(flags.Username) {
			return ans, fmt.Errorf("%w: --username %q does not match %s",
				ErrUsage, flags.Username, rxUsername)
		}
		ans.Username = flags.Username
	}
	if flags.OS != "" {
		if _, ok := config.SupportedOsVersions[flags.OS]; !ok {
			return ans, fmt.Errorf("%w: --os %q not in %s",
				ErrUsage, flags.OS, strings.Join(config.SupportedOSes, ", "))
		}
		ans.OS, ans.OSSet = flags.OS, true
	}
	if flags.OSVersion != "" {
		if !versionMatchesOS(flags.OS, flags.OSVersion) {
			osID := flags.OS
			if osID == "" {
				osID = "(unset; pass --os too)"
			}
			supported := strings.Join(config.SupportedOsVersions[flags.OS], ", ")
			if supported == "" {
				supported = "(none — set --os first)"
			}
			return ans, fmt.Errorf("%w: --os-version %q not in %s for %s",
				ErrUsage, flags.OSVersion, supported, osID)
		}
		ans.OSVersion, ans.OSVersionSet = flags.OSVersion, true
	}
	if flags.MountRoot != "" {
		if flags.MountRoot != "." && flags.MountRoot != ".." {
			return ans, fmt.Errorf(`%w: --mount-root must be "." or ".."`, ErrUsage)
		}
		ans.MountRoot, ans.MountRootSet = flags.MountRoot, true
	}
	switch {
	case flags.Devcontainer:
		ans.Devcontainer, ans.DevcontainerSet = true, true
	case flags.NoDevcontainer:
		ans.Devcontainer, ans.DevcontainerSet = false, true
	}
	if flags.AptCategories != "" {
		ids, err := parseAptCategories(flags.AptCategories)
		if err != nil {
			return ans, err
		}
		ans.AptCategories, ans.AptSet = ids, true
	}
	return ans, nil
}

// applyDefaults fills the still-empty answer fields with sensible
// defaults so --yes can proceed without prompts. service_name and
// username are required and never defaulted; missing them returns
// ErrUsage so CI scripts know to pass the flags.
func applyDefaults(ans initAnswers) (initAnswers, error) {
	if ans.ServiceName == "" {
		return ans, fmt.Errorf("%w: --yes requires --service-name", ErrUsage)
	}
	if ans.Username == "" {
		return ans, fmt.Errorf("%w: --yes requires --username", ErrUsage)
	}
	if !ans.OSSet {
		ans.OS, ans.OSSet = "ubuntu", true
	}
	if !ans.OSVersionSet {
		ans.OSVersion, ans.OSVersionSet = defaultOSVersion(ans.OS), true
	}
	if !ans.MountRootSet {
		ans.MountRoot, ans.MountRootSet = ".", true
	}
	if !ans.DevcontainerSet {
		ans.Devcontainer, ans.DevcontainerSet = true, true
	}
	if !ans.AptSet {
		ans.AptCategories, ans.AptSet = setup.DefaultAptCategoryIDs(), true
	}
	return ans, nil
}

// promptForMissing runs the interactive flow as a sequence of
// independent single-field huh.Forms — one prompt per screen. Each
// form is built and Run()d in order; the value picked by an earlier
// form feeds into how the next form's options are constructed (e.g.
// the OS-version form computes its option list from the OS the user
// just picked).
//
// We deliberately avoid the multi-Group / OptionsFunc combination.
// huh's WindowSize-driven viewport sizing was racing with
// OptionsFunc / TitleFunc async evaluation and the resulting viewport
// was sometimes smaller than the option list — symptom: cursor
// "stays put" while the option list scrolls under it. Independent
// forms with statically-built Options() side-step the race entirely.
//
// Tradeoff: shift+tab cannot navigate back across forms. Re-running
// `cocoon init` is the way to fix an earlier answer.
//
//nolint:gocognit,gocyclo // sequence of independent prompt steps; splitting hides intent.
func promptForMissing(ans initAnswers, cat *i18n.Catalog) (initAnswers, error) {
	if ans.ServiceName == "" {
		if err := runStrictIdentForm(cat, "init_prompt_service_name",
			"init_desc_service_name", "init_err_service_name_fmt",
			rxServiceName, &ans.ServiceName); err != nil {
			return ans, err
		}
	}
	if ans.Username == "" {
		if err := runStrictIdentForm(cat, "init_prompt_username",
			"init_desc_username", "init_err_username_fmt",
			rxUsername, &ans.Username); err != nil {
			return ans, err
		}
	}
	if !ans.OSSet {
		if ans.OS == "" {
			ans.OS = "ubuntu"
		}
		if err := runSingleFieldForm(osSelect(cat, &ans.OS)); err != nil {
			return ans, err
		}
		ans.OSSet = true
	}
	if !ans.OSVersionSet {
		// Build the version Select's options from the now-known OS so
		// no OptionsFunc binding / async evaluation is needed.
		versions := config.SupportedOsVersions[ans.OS]
		if ans.OSVersion == "" {
			ans.OSVersion = defaultOSVersion(ans.OS)
		}
		if err := runSingleFieldForm(osVersionSelect(cat, versions, &ans.OSVersion)); err != nil {
			return ans, err
		}
		ans.OSVersionSet = true
	}
	if !ans.MountRootSet {
		ans.MountRoot = "."
		if err := runSingleFieldForm(mountRootSelect(cat, &ans.MountRoot)); err != nil {
			return ans, err
		}
		ans.MountRootSet = true
	}
	if !ans.DevcontainerSet {
		ans.Devcontainer = true
		if err := runSingleFieldForm(devcontainerConfirm(cat, &ans.Devcontainer)); err != nil {
			return ans, err
		}
		ans.DevcontainerSet = true
	}
	if !ans.AptSet {
		ans.AptCategories = setup.DefaultAptCategoryIDs()
		if err := runSingleFieldForm(aptMultiSelect(cat, &ans.AptCategories)); err != nil {
			return ans, err
		}
		ans.AptSet = true
	}
	if !versionMatchesOS(ans.OS, ans.OSVersion) {
		ans.OSVersion = defaultOSVersion(ans.OS)
	}
	return ans, nil
}

// runSingleFieldForm wraps a single huh.Field into a one-Group, one-
// Field Form and runs it. Centralises the ErrUserAborted / generic-
// failure error wrapping so the call sites stay readable.
func runSingleFieldForm(field huh.Field) error {
	if err := huh.NewForm(huh.NewGroup(field)).Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return fmt.Errorf("%w: aborted", ErrUsage)
		}
		return fmt.Errorf("%w: prompt: %w", ErrFailure, err)
	}
	return nil
}

// runStrictIdentForm prompts for one required text identifier with a
// strict-on-empty validator. Single-Group / single-Field form means
// there is no previous group to navigate back to, so the validator
// cannot trap the user inside a blurred-with-error field.
func runStrictIdentForm(cat *i18n.Catalog, titleKey, descKey, charsKey string,
	pattern *regexp.Regexp, target *string,
) error {
	return runSingleFieldForm(identInput(cat, titleKey, descKey, charsKey, pattern, target))
}

// versionMatchesOS reports whether the given version is in the
// supported set for the given OS. Used by applyFlags to validate the
// --os-version + --os pair, and by promptForMissing as a final
// sanity check against stale combinations.
func versionMatchesOS(osID, version string) bool {
	for _, v := range config.SupportedOsVersions[osID] {
		if v == version {
			return true
		}
	}
	return false
}

func identInput(cat *i18n.Catalog, titleKey, descKey, charsKey string,
	pattern *regexp.Regexp, target *string,
) *huh.Input {
	return huh.NewInput().
		Title(cat.Msg(titleKey)).
		Description(cat.Msg(descKey)).
		Validate(makeStrictValidator(pattern, charsKey, cat)).
		Value(target)
}

// All Select / MultiSelect helpers below intentionally omit Height().
// huh's default behaviour with no explicit Height() is:
//
//   - Static Options(): viewport.Height equals the rendered options
//     line count, so every option is always visible and the cursor
//     moves between them without scrolling.
//   - OptionsFunc(): height defaults to 10, viewport ends up around
//     10 minus title+description height — comfortably bigger than
//     our largest version list (3 entries).
//
// We previously set Height(len(options) + 2) to "be safe", which
// actively backfired: huh subtracts the rendered title+description
// height from that, so a description long enough to wrap to two
// lines shrunk the viewport below the options count. The user then
// saw the `>` cursor stay fixed while options scrolled under it.

func osSelect(cat *i18n.Catalog, target *string) *huh.Select[string] {
	options := make([]huh.Option[string], len(config.SupportedOSes))
	for i, id := range config.SupportedOSes {
		options[i] = huh.NewOption(id, id)
	}
	return huh.NewSelect[string]().
		Title(cat.Msg("init_prompt_os")).
		Description(cat.Msg("init_desc_os")).
		Options(options...).
		Value(target)
}

func osVersionSelect(cat *i18n.Catalog, versions []string, target *string) *huh.Select[string] {
	// Static Title / Description / Options. The OS is already known by
	// the time this form is built (the OS picker ran in a prior form),
	// so we have the version list up front and never need OptionsFunc.
	// Avoiding OptionsFunc avoids huh's WindowSize-vs-async-eval race
	// that was shrinking the viewport below the option count and
	// turning Down into "scroll the option list" rather than "advance
	// the cursor".
	options := make([]huh.Option[string], len(versions))
	for i, v := range versions {
		options[i] = huh.NewOption(v, v)
	}
	return huh.NewSelect[string]().
		Title(cat.Msg("init_prompt_os_version_static")).
		Description(cat.Msg("init_desc_os_version_static")).
		Options(options...).
		Value(target)
}

func mountRootSelect(cat *i18n.Catalog, target *string) *huh.Select[string] {
	return huh.NewSelect[string]().
		Title(cat.Msg("init_prompt_mount_root")).
		Description(cat.Msg("init_desc_mount_root")).
		Options(
			huh.NewOption(cat.Msg("init_option_mount_cwd"), "."),
			huh.NewOption(cat.Msg("init_option_mount_parent"), ".."),
		).
		Value(target)
}

func devcontainerConfirm(cat *i18n.Catalog, target *bool) *huh.Confirm {
	return huh.NewConfirm().
		Title(cat.Msg("init_prompt_devcontainer")).
		Description(cat.Msg("init_desc_devcontainer")).
		Affirmative(cat.Msg("init_confirm_yes")).
		Negative(cat.Msg("init_confirm_no")).
		Value(target)
}

func aptMultiSelect(cat *i18n.Catalog, target *[]string) *huh.MultiSelect[string] {
	options := make([]huh.Option[string], len(setup.AptCategories))
	for i, c := range setup.AptCategories {
		options[i] = huh.NewOption(fmt.Sprintf("%s (%s)", c.Label, c.Description), c.ID)
	}
	return huh.NewMultiSelect[string]().
		Title(cat.Msg("init_prompt_apt")).
		Description(cat.Msg("init_desc_apt")).
		Options(options...).
		Value(target)
}

// makeStrictValidator rejects empty input and bad characters. Used by
// the standalone Inputs in promptForMissing. Showing the regex itself
// to end users is useless; this surfaces a human sentence instead.
//
// Strict-on-empty is safe here only because each strict Input runs in
// its own standalone huh.Form — there is no previous group to
// navigate back to, so the validator cannot trap the user inside a
// blurred-with-error field.
func makeStrictValidator(pattern *regexp.Regexp, charsKey string, cat *i18n.Catalog) func(string) error {
	return func(s string) error {
		if s == "" {
			return errors.New(cat.Msg("init_err_required")) //nolint:err113 // user-facing prompt
		}
		if !pattern.MatchString(s) {
			return errors.New(cat.Msg(charsKey)) //nolint:err113 // user-facing prompt
		}
		return nil
	}
}

// parseAptCategories splits a comma-separated category id list and
// returns the parsed ids, rejecting any unknown entry. Used by the
// --apt-categories flag path.
func parseAptCategories(raw string) ([]string, error) {
	var ids []string
	for _, part := range strings.Split(raw, ",") {
		id := strings.TrimSpace(part)
		if id == "" {
			continue
		}
		if setup.AptCategoryByID(id) == nil {
			return nil, fmt.Errorf("%w: unknown apt category %q (run `cocoon init --help` for the list)",
				ErrUsage, id)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// defaultOSVersion picks the LTS-flavoured default for ubuntu (24.04
// over the newer 26.04 which is not yet on dockerhub for every arch);
// other distros fall back to the first listed version.
func defaultOSVersion(osID string) string {
	versions := config.SupportedOsVersions[osID]
	if len(versions) == 0 {
		return ""
	}
	if osID == "ubuntu" {
		for _, v := range versions {
			if v == "24.04" {
				return v
			}
		}
	}
	return versions[0]
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
