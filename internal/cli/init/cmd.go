package initcli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
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
--devcontainer / --apt-categories / --plugins / --alias-bundles to drive
non-interactively from CI.`

var (
	rxServiceName = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	rxUsername    = regexp.MustCompile(`^[a-z_][a-z0-9_-]*$`)
	// rxImageVersionInput must stay in lockstep with config.rxImageVersion
	// so prompts reject the same set `cocoon gen` does (Docker tag spec:
	// alnum / underscore can lead, period / hyphen cannot).
	rxImageVersionInput = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]*$`)
)

// versionStringValidator enforces the TOML-safe charset via rxImageVersionInput.
// Upstream existence is intentionally NOT verified. latestSentinel, when
// non-empty, is accepted unconditionally (the plugin picker uses "LATEST" for
// "leave unpinned"). Empty input is always rejected so a stray Enter does not
// silently encode as a sentinel.
func versionStringValidator(cat *i18n.Catalog, formatErrKey, latestSentinel string) func(string) error {
	return func(s string) error {
		s = strings.TrimSpace(s)
		if latestSentinel != "" && s == latestSentinel {
			return nil
		}
		if s == "" {
			return errors.New(cat.Msg("init_err_required")) //nolint:err113 // user-facing prompt
		}
		if !rxImageVersionInput.MatchString(s) {
			return errors.New(cat.Msg(formatErrKey)) //nolint:err113 // user-facing prompt
		}
		return nil
	}
}

// NewCommand returns the cobra command for `cocoon init`.
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
	cmd.Flags().StringVar(&flags.Image, "image", "",
		fmt.Sprintf("base image: %s", strings.Join(config.SupportedImages, ", ")))
	cmd.Flags().StringVar(&flags.ImageVersion, "image-version", "",
		"base image tag — any well-formed Docker tag is accepted; --image must also be set")
	cmd.Flags().StringVar(&flags.Shell, "shell", "",
		fmt.Sprintf("container login shell: %s (default: bash)", strings.Join(config.SupportedShells, ", ")))
	cmd.Flags().StringVar(&flags.MountRoot, "mount-root", "", `mount range: "." (cwd, default) or ".." (parent)`)
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

type initFlags struct {
	AutoYes        bool
	ServiceName    string
	Username       string
	Image          string
	ImageVersion   string
	Shell          string
	MountRoot      string
	Devcontainer   bool
	NoDevcontainer bool
	Certificates   bool
	NoCertificates bool
	AptCategories  string
	Plugins        string
	PluginVersions string
	PluginMethods  string
	AliasBundles   string
	Ports          string
	Force          bool
}

func zeroFlags() initFlags {
	return initFlags{
		AutoYes:        false,
		ServiceName:    "",
		Username:       "",
		Image:          "",
		ImageVersion:   "",
		Shell:          "",
		MountRoot:      "",
		Devcontainer:   false,
		NoDevcontainer: false,
		Certificates:   false,
		NoCertificates: false,
		AptCategories:  "",
		Plugins:        "",
		PluginVersions: "",
		PluginMethods:  "",
		AliasBundles:   "",
		Ports:          "",
		Force:          false,
	}
}

// initAnswers is what gets written into workspace.toml. The *Set companions
// distinguish "not yet provided" from a zero value the user actively chose
// (e.g. devcontainer = false), so the prompt builder doesn't skip groups
// whose value happens to look empty.
type initAnswers struct {
	ServiceName       string
	Username          string
	Image             string
	ImageSet          bool
	ImageVersion      string
	ImageVersionSet   bool
	Shell             string
	ShellSet          bool
	MountRoot         string
	MountRootSet      bool
	Devcontainer      bool
	DevcontainerSet   bool
	Certificates      bool
	CertificatesSet   bool
	AptCategories     []string
	AptSet            bool
	Plugins           []string
	PluginsSet        bool
	PluginVersions    map[string]string
	PluginVersionsSet bool
	PluginMethods     map[string]string
	PluginMethodsSet  bool
	AliasBundles      []string
	AliasBundlesSet   bool
	Ports             []string
	PortsSet          bool
}

func zeroAnswers() initAnswers {
	return initAnswers{
		ServiceName:       "",
		Username:          "",
		Image:             "",
		ImageSet:          false,
		ImageVersion:      "",
		ImageVersionSet:   false,
		Shell:             "",
		ShellSet:          false,
		MountRoot:         "",
		MountRootSet:      false,
		Devcontainer:      false,
		DevcontainerSet:   false,
		Certificates:      false,
		CertificatesSet:   false,
		AptCategories:     nil,
		AptSet:            false,
		Plugins:           nil,
		PluginsSet:        false,
		PluginVersions:    nil,
		PluginVersionsSet: false,
		PluginMethods:     nil,
		PluginMethodsSet:  false,
		AliasBundles:      nil,
		AliasBundlesSet:   false,
		Ports:             nil,
		PortsSet:          false,
	}
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

// assertNoImagePluginConflict names the matching --plugins / --image rewrite
// in the error so the fix is one edit.
func assertNoImagePluginConflict(ans initAnswers) error {
	conflict, hit := config.ImageProvidesPlugin[ans.Image]
	if !hit {
		return nil
	}
	if !slices.Contains(ans.Plugins, conflict) {
		return nil
	}
	return fmt.Errorf(
		"%w: image=%q already provides %s; drop %q from --plugins, "+
			"or pick --image=ubuntu/debian to pin a custom %s via the plugin",
		clihelpers.ErrUsage, ans.Image, conflict, conflict, conflict,
	)
}

// applyFlags marks *Set on every populated flag. Empty flags leave the field
// zero so the prompt or default layer fills it in.
//
//nolint:gocognit,gocyclo,funlen // sequence of independent flag checks; splitting hides intent.
func applyFlags(flags *initFlags, plugins map[string]*plugin.Plugin) (initAnswers, error) {
	ans := zeroAnswers()
	if flags.ServiceName != "" {
		if !rxServiceName.MatchString(flags.ServiceName) {
			return ans, fmt.Errorf("%w: --service-name %q does not match %s",
				clihelpers.ErrUsage, flags.ServiceName, rxServiceName)
		}
		ans.ServiceName = flags.ServiceName
	}
	if flags.Username != "" {
		if !rxUsername.MatchString(flags.Username) {
			return ans, fmt.Errorf("%w: --username %q does not match %s",
				clihelpers.ErrUsage, flags.Username, rxUsername)
		}
		ans.Username = flags.Username
	}
	if flags.Image != "" {
		if _, ok := config.SupportedImageVersions[flags.Image]; !ok {
			return ans, fmt.Errorf("%w: --image %q not in %s",
				clihelpers.ErrUsage, flags.Image, strings.Join(config.SupportedImages, ", "))
		}
		ans.Image, ans.ImageSet = flags.Image, true
	}
	if flags.ImageVersion != "" {
		if flags.Image == "" {
			return ans, fmt.Errorf(
				"%w: --image-version %q requires --image (so the registry path is known)",
				clihelpers.ErrUsage, flags.ImageVersion)
		}
		if !rxImageVersionInput.MatchString(flags.ImageVersion) {
			return ans, fmt.Errorf(
				"%w: --image-version %q must match %s",
				clihelpers.ErrUsage, flags.ImageVersion, rxImageVersionInput.String())
		}
		ans.ImageVersion, ans.ImageVersionSet = flags.ImageVersion, true
	}
	if flags.Shell != "" {
		if !slices.Contains(config.SupportedShells, flags.Shell) {
			return ans, fmt.Errorf("%w: --shell %q not in %s",
				clihelpers.ErrUsage, flags.Shell, strings.Join(config.SupportedShells, ", "))
		}
		ans.Shell, ans.ShellSet = flags.Shell, true
	}
	if flags.MountRoot != "" {
		if flags.MountRoot != "." && flags.MountRoot != ".." {
			return ans, fmt.Errorf(`%w: --mount-root must be "." or ".."`, clihelpers.ErrUsage)
		}
		ans.MountRoot, ans.MountRootSet = flags.MountRoot, true
	}
	switch {
	case flags.Devcontainer:
		ans.Devcontainer, ans.DevcontainerSet = true, true
	case flags.NoDevcontainer:
		ans.Devcontainer, ans.DevcontainerSet = false, true
	}
	switch {
	case flags.Certificates:
		ans.Certificates, ans.CertificatesSet = true, true
	case flags.NoCertificates:
		ans.Certificates, ans.CertificatesSet = false, true
	}
	if flags.AptCategories != "" {
		ids, err := parseAptCategories(flags.AptCategories)
		if err != nil {
			return ans, err
		}
		ans.AptCategories, ans.AptSet = ids, true
	}
	if flags.Plugins != "" {
		ids, err := parsePlugins(flags.Plugins, plugins)
		if err != nil {
			return ans, err
		}
		if conflictErr := validatePluginConflicts(plugins, ids); conflictErr != nil {
			return ans, conflictErr
		}
		ans.Plugins, ans.PluginsSet = ids, true
	}
	if flags.PluginVersions != "" {
		pins, err := parsePluginVersions(flags.PluginVersions, plugins, ans.Plugins)
		if err != nil {
			return ans, err
		}
		ans.PluginVersions, ans.PluginVersionsSet = pins, true
	}
	if flags.PluginMethods != "" {
		picks, err := parsePluginMethods(flags.PluginMethods, plugins, ans.Plugins)
		if err != nil {
			return ans, err
		}
		ans.PluginMethods, ans.PluginMethodsSet = picks, true
	}
	if flags.AliasBundles != "" {
		ids, err := parseAliasBundles(flags.AliasBundles)
		if err != nil {
			return ans, err
		}
		ans.AliasBundles, ans.AliasBundlesSet = ids, true
	}
	if flags.Ports != "" {
		ports, err := parsePorts(flags.Ports)
		if err != nil {
			return ans, err
		}
		ans.Ports, ans.PortsSet = ports, true
	}
	return ans, nil
}

// applyDefaults fills the still-empty answer fields with sensible
// defaults so --yes can proceed without prompts. service_name and
// username are required and never defaulted; missing them returns
// clihelpers.ErrUsage so CI scripts know to pass the flags.
func applyDefaults(ans initAnswers, plugins map[string]*plugin.Plugin) (initAnswers, error) {
	if ans.ServiceName == "" {
		return ans, fmt.Errorf("%w: --yes requires --service-name", clihelpers.ErrUsage)
	}
	if ans.Username == "" {
		return ans, fmt.Errorf("%w: --yes requires --username", clihelpers.ErrUsage)
	}
	if !ans.ImageSet {
		ans.Image, ans.ImageSet = "ubuntu", true
	}
	if !ans.ImageVersionSet {
		ans.ImageVersion, ans.ImageVersionSet = defaultImageVersion(ans.Image), true
	}
	if !ans.ShellSet {
		ans.Shell, ans.ShellSet = "bash", true
	}
	if !ans.MountRootSet {
		ans.MountRoot, ans.MountRootSet = ".", true
	}
	if !ans.DevcontainerSet {
		ans.Devcontainer, ans.DevcontainerSet = true, true
	}
	if !ans.CertificatesSet {
		ans.Certificates, ans.CertificatesSet = false, true
	}
	if !ans.AptSet {
		ans.AptCategories, ans.AptSet = aptcategories.DefaultAptCategoryIDs(), true
	}
	if !ans.PluginsSet {
		ans.Plugins, ans.PluginsSet = defaultPluginIDs(plugins), true
	}
	if !ans.AliasBundlesSet {
		ans.AliasBundles, ans.AliasBundlesSet = aliasbundles.DefaultAliasBundleIDs(), true
	}
	if !ans.PortsSet {
		ans.Ports, ans.PortsSet = nil, true
	}
	if !ans.PluginMethodsSet {
		ans.PluginMethods, ans.PluginMethodsSet = nil, true
	}
	return ans, nil
}

// promptForMissing uses single-field huh.Forms (one prompt per screen).
// Multi-Group + OptionsFunc had a viewport race vs. async TitleFunc that
// left the cursor stuck under scrolling options; independent forms side-step
// it. Tradeoff: no shift+tab back-nav; re-run `cocoon init` to fix.
//
//nolint:funlen,gocognit,gocyclo // sequence of independent prompt steps; splitting hides intent.
func promptForMissing(ans initAnswers, cat *i18n.Catalog, plugins map[string]*plugin.Plugin) (initAnswers, error) {
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
	if !ans.ImageSet {
		if ans.Image == "" {
			ans.Image = "ubuntu"
		}
		if err := runSingleFieldForm(imageSelect(cat, &ans.Image)); err != nil {
			return ans, err
		}
		ans.ImageSet = true
	}
	if !ans.ImageVersionSet {
		versions := config.SupportedImageVersions[ans.Image]
		if ans.ImageVersion == "" {
			ans.ImageVersion = defaultImageVersion(ans.Image)
		}
		field := newSelectOrInputField("image_version", &ans.ImageVersion, versions,
			cat.Msg("init_option_other_manual_input")).
			Title(cat.Msg("init_prompt_image_version_static")).
			Description(cat.Msg("init_desc_image_version_static")).
			Validate(versionStringValidator(cat, "init_err_image_version_fmt", ""))
		if err := runSingleFieldForm(field); err != nil {
			return ans, err
		}
		ans.ImageVersionSet = true
	}
	if !ans.ShellSet {
		ans.Shell = "bash"
		if err := runSingleFieldForm(shellSelect(cat, &ans.Shell)); err != nil {
			return ans, err
		}
		ans.ShellSet = true
	}
	if !ans.AliasBundlesSet {
		ans.AliasBundles = aliasbundles.DefaultAliasBundleIDs()
		if err := runSingleFieldForm(aliasBundlesMultiSelect(cat, &ans.AliasBundles)); err != nil {
			return ans, err
		}
		ans.AliasBundlesSet = true
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
	if !ans.CertificatesSet {
		ans.Certificates = false
		if err := runSingleFieldForm(certificatesConfirm(cat, &ans.Certificates)); err != nil {
			return ans, err
		}
		ans.CertificatesSet = true
	}
	if !ans.PortsSet {
		ports, err := promptForPorts(cat)
		if err != nil {
			return ans, err
		}
		ans.Ports, ans.PortsSet = ports, true
	}
	if !ans.AptSet {
		ans.AptCategories = aptcategories.DefaultAptCategoryIDs()
		if err := runSingleFieldForm(aptMultiSelect(cat, &ans.AptCategories)); err != nil {
			return ans, err
		}
		ans.AptSet = true
	}
	if !ans.PluginsSet {
		// Hide plugins whose toolchain duplicates the chosen base image so the
		// user cannot accidentally pick a combination validateImagePluginConflict
		// would later reject.
		excludeID := config.ImageProvidesPlugin[ans.Image]
		ans.Plugins = filterPluginIDs(defaultPluginIDs(plugins), excludeID)
		if err := promptPluginsWithRetry(cat, plugins, excludeID, &ans.Plugins); err != nil {
			return ans, err
		}
		ans.PluginsSet = true
	}
	// Method prompts run BEFORE version prompts because picking a method may
	// change the upstream URL shown beside the version picker (e.g. official
	// installer page vs. GitHub Releases). Allocate the map up front for the
	// same reason promptPluginVersionsForCapable does: an empty map is the
	// "use default_method everywhere" signal, indistinguishable from nil for
	// downstream consumers.
	if ans.PluginMethods == nil {
		ans.PluginMethods = make(map[string]string)
	}
	if err := promptPluginMethodsForMulti(cat, plugins, ans.Plugins, ans.PluginMethods); err != nil {
		return ans, err
	}
	ans.PluginMethodsSet = true
	// Allocate the map even if the prompt produces no picks; an empty map
	// is equivalent to nil in writePluginVersions (both hit the len==0
	// fallback that emits the commented-out template).
	if ans.PluginVersions == nil {
		ans.PluginVersions = make(map[string]string)
	}
	if err := promptPluginVersionsForCapable(cat, plugins, ans.Plugins, ans.PluginVersions); err != nil {
		return ans, err
	}
	ans.PluginVersionsSet = true
	return ans, nil
}

// promptPluginsWithRetry re-runs the multi-select on conflict (up to 2 more
// times) so the user can reconcile without restarting the whole flow. Three
// failures in a row return clihelpers.ErrUsage so scripted invocations cannot loop
// forever. excludeID hides one plugin id from the picker (empty = none).
func promptPluginsWithRetry(cat *i18n.Catalog, plugins map[string]*plugin.Plugin,
	excludeID string, target *[]string,
) error {
	const maxAttempts = 3
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := runSingleFieldForm(pluginsMultiSelect(cat, plugins, excludeID, target)); err != nil {
			return err
		}
		err := validatePluginConflicts(plugins, *target)
		if err == nil {
			return nil
		}
		// huh redraws the prompt body and drops prior status lines, so
		// echo the conflict to stderr or the user has no idea why the
		// form is reappearing.
		fmt.Fprintln(os.Stderr, err)
	}
	return fmt.Errorf("%w: plugin conflict not resolved after %d attempts", clihelpers.ErrUsage, maxAttempts)
}

// filterPluginIDs returns ids with excludeID removed. Empty excludeID
// returns ids verbatim; the caller does not need to special-case the
// "no exclusion" path.
func filterPluginIDs(ids []string, excludeID string) []string {
	if excludeID == "" {
		return ids
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == excludeID {
			continue
		}
		out = append(out, id)
	}
	return out
}

// runSingleFieldForm wraps a single huh.Field into a one-Group, one-
// Field Form and runs it. Centralises the ErrUserAborted / generic-
// failure error wrapping so the call sites stay readable.
//
// Every prompt is its own form (see initLong), so Shift+Tab has no
// previous field to land on. The form keymap below blanks the Prev
// binding's help text in every field profile so huh's help bar stops
// advertising a "shift+tab back" affordance that does nothing.
func runSingleFieldForm(field huh.Field) error {
	if err := huh.NewForm(huh.NewGroup(field)).WithKeyMap(keyMapWithoutPrevHelp()).Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return fmt.Errorf("%w: aborted", clihelpers.ErrUsage)
		}
		return fmt.Errorf("%w: prompt: %w", clihelpers.ErrFailure, err)
	}
	return nil
}

// keyMapWithoutPrevHelp strips the Shift+Tab "back" help text from every
// field profile. The binding still routes through Update (no-op) so the
// keystroke is silently ignored rather than erroring.
func keyMapWithoutPrevHelp() *huh.KeyMap {
	km := huh.NewDefaultKeyMap()
	km.Confirm.Prev.SetHelp("", "")
	km.FilePicker.Prev.SetHelp("", "")
	km.Input.Prev.SetHelp("", "")
	km.MultiSelect.Prev.SetHelp("", "")
	km.Note.Prev.SetHelp("", "")
	km.Select.Prev.SetHelp("", "")
	km.Text.Prev.SetHelp("", "")
	return km
}

// runStrictIdentForm uses a strict-on-empty validator. Safe only because
// the single-Group / single-Field form has no previous group to navigate
// back to, so the validator cannot trap the user in a blurred-with-error
// field.
func runStrictIdentForm(cat *i18n.Catalog, titleKey, descKey, charsKey string,
	pattern *regexp.Regexp, target *string,
) error {
	return runSingleFieldForm(identInput(cat, titleKey, descKey, charsKey, pattern, target))
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

// Select/MultiSelect helpers omit Height() intentionally: explicit Height(len+2)
// breaks when title+description wraps to two lines (cursor stuck under a
// scrolling viewport); huh's default already covers our options count.

func imageSelect(cat *i18n.Catalog, target *string) *huh.Select[string] {
	options := make([]huh.Option[string], len(config.SupportedImages))
	for i, id := range config.SupportedImages {
		options[i] = huh.NewOption(id, id)
	}
	return huh.NewSelect[string]().
		Title(cat.Msg("init_prompt_image")).
		Description(cat.Msg("init_desc_image")).
		Options(options...).
		Value(target)
}

func shellSelect(cat *i18n.Catalog, target *string) *huh.Select[string] {
	options := make([]huh.Option[string], len(config.SupportedShells))
	for i, id := range config.SupportedShells {
		options[i] = huh.NewOption(id, id)
	}
	return huh.NewSelect[string]().
		Title(cat.Msg("init_prompt_shell")).
		Description(cat.Msg("init_desc_shell")).
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

func certificatesConfirm(cat *i18n.Catalog, target *bool) *huh.Confirm {
	return huh.NewConfirm().
		Title(cat.Msg("init_prompt_certificates")).
		Description(cat.Msg("init_desc_certificates")).
		Affirmative(cat.Msg("init_confirm_yes")).
		Negative(cat.Msg("init_confirm_no")).
		Value(target)
}

func aptMultiSelect(cat *i18n.Catalog, target *[]string) *huh.MultiSelect[string] {
	options := make([]huh.Option[string], len(aptcategories.AptCategories))
	for i, c := range aptcategories.AptCategories {
		options[i] = huh.NewOption(fmt.Sprintf("%s (%s)", c.Label, c.Description), c.ID)
	}
	return huh.NewMultiSelect[string]().
		Title(cat.Msg("init_prompt_apt")).
		Description(cat.Msg("init_desc_apt")).
		Options(options...).
		Value(target)
}

func aliasBundlesMultiSelect(cat *i18n.Catalog, target *[]string) *huh.MultiSelect[string] {
	options := make([]huh.Option[string], len(aliasbundles.AliasBundles))
	for i, b := range aliasbundles.AliasBundles {
		options[i] = huh.NewOption(fmt.Sprintf("%s (%s)", b.Label, b.Description), b.ID)
	}
	return huh.NewMultiSelect[string]().
		Title(cat.Msg("init_prompt_alias_bundles")).
		Description(cat.Msg("init_desc_alias_bundles")).
		Options(options...).
		Value(target)
}

// promptForPorts returns (nil, nil) on blank input — the renderer then
// emits the commented-out [ports] template.
func promptForPorts(cat *i18n.Catalog) ([]string, error) {
	var raw string
	if err := runSingleFieldForm(portsInput(cat, &raw)); err != nil {
		return nil, err
	}
	// A failure here means validator and parser disagree — fail loud
	// rather than silently dropping it.
	return parsePorts(raw)
}

// portsInput accepts blank input (renderer emits the commented template).
// Non-empty input is validated by portsInputValidator so init never
// accepts a string `cocoon gen` would later reject.
func portsInput(cat *i18n.Catalog, target *string) *huh.Input {
	return huh.NewInput().
		Title(cat.Msg("init_prompt_ports")).
		Description(cat.Msg("init_desc_ports")).
		Validate(portsInputValidator(cat)).
		Value(target)
}

// portsInputValidator localizes the rejection message via the catalog.
// Separate from parsePorts so the `--ports` flag path keeps its English
// usage error consistent with the other init flag validators.
func portsInputValidator(cat *i18n.Catalog) func(string) error {
	return func(s string) error {
		for _, part := range strings.Split(s, ",") {
			p := strings.TrimSpace(part)
			if p == "" {
				continue
			}
			if err := config.ValidateShortForm(p); err != nil {
				return errors.New(cat.Msg("init_err_port_invalid_fmt", p)) //nolint:err113 // user-facing prompt
			}
		}
		return nil
	}
}

// pluginsMultiSelect sorts options by id so the order is stable across runs
// (LoadDir returns a map). excludeID hides one plugin id (empty = none) and
// is used to keep validateImagePluginConflict-tripping picks out of the
// picker entirely.
func pluginsMultiSelect(cat *i18n.Catalog, plugins map[string]*plugin.Plugin,
	excludeID string, target *[]string,
) *huh.MultiSelect[string] {
	ids := slices.Sorted(maps.Keys(plugins))
	options := make([]huh.Option[string], 0, len(ids))
	for _, id := range ids {
		if id == excludeID {
			continue
		}
		options = append(options, huh.NewOption(formatPluginLabel(id, plugins[id]), id))
	}
	return huh.NewMultiSelect[string]().
		Title(cat.Msg("init_prompt_plugins")).
		Description(cat.Msg("init_desc_plugins")).
		Options(options...).
		Value(target)
}

// formatPluginLabel shows conflicts up front so the user does not have to
// dig into plugin.toml.
func formatPluginLabel(id string, p *plugin.Plugin) string {
	name := p.Metadata.Name
	if name == "" {
		name = id
	}
	hint := p.Metadata.Description
	if len(p.Metadata.Conflicts) > 0 {
		// Conflicts is the actionable signal; drop description if both
		// are set so the warning is what the user sees.
		hint = "conflicts: " + strings.Join(p.Metadata.Conflicts, ", ")
	}
	if hint == "" {
		return name
	}
	return fmt.Sprintf("%s (%s)", name, hint)
}

// makeStrictValidator surfaces a localized sentence instead of the raw
// regex. Strict-on-empty is safe only because each Input is a standalone
// huh.Form with no previous group, so the validator cannot trap the user
// in a blurred-with-error field.
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

// parseAptCategories rejects unknown ids. Used by --apt-categories.
func parseAptCategories(raw string) ([]string, error) {
	var ids []string
	for _, part := range strings.Split(raw, ",") {
		id := strings.TrimSpace(part)
		if id == "" {
			continue
		}
		if aptcategories.AptCategoryByID(id) == nil {
			return nil, fmt.Errorf("%w: unknown apt category %q (run `cocoon init --help` for the list)",
				clihelpers.ErrUsage, id)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// parsePorts validates entries through config.ValidateShortForm so init
// accepts the same set `cocoon gen` does. (nil, nil) for blank input;
// the renderer treats nil as "user opted out" and emits the commented
// [ports] template.
func parsePorts(raw string) ([]string, error) {
	var ports []string
	for _, part := range strings.Split(raw, ",") {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		if err := config.ValidateShortForm(p); err != nil {
			return nil, fmt.Errorf("%w: --ports %w", clihelpers.ErrUsage, err)
		}
		ports = append(ports, p)
	}
	return ports, nil
}

// parseAliasBundles rejects unknown ids. Used by --alias-bundles.
func parseAliasBundles(raw string) ([]string, error) {
	var ids []string
	for _, part := range strings.Split(raw, ",") {
		id := strings.TrimSpace(part)
		if id == "" {
			continue
		}
		if aliasbundles.AliasBundleByID(id) == nil {
			return nil, fmt.Errorf("%w: unknown alias bundle %q", clihelpers.ErrUsage, id)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// parsePlugins leaves the conflict check to validatePluginConflicts so the
// same logic covers both flag and prompt paths.
func parsePlugins(raw string, plugins map[string]*plugin.Plugin) ([]string, error) {
	var ids []string
	for _, part := range strings.Split(raw, ",") {
		id := strings.TrimSpace(part)
		if id == "" {
			continue
		}
		if _, ok := plugins[id]; !ok {
			return nil, fmt.Errorf("%w: unknown plugin %q (run `cocoon plugin list` for the catalog)",
				clihelpers.ErrUsage, id)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// parsePluginVersions parses `--plugin-versions=<id>=<ref>,…` into a map.
// Each id must be in plugins, in enabled (silent no-op otherwise), and
// version_capable. Empty input returns a non-nil empty map (nilnil lint).
// Duplicate ids are rejected so a typo can't silently pick the last value.
func parsePluginVersions(raw string, plugins map[string]*plugin.Plugin, enabled []string) (map[string]string, error) {
	enabledSet := make(map[string]struct{}, len(enabled))
	for _, id := range enabled {
		enabledSet[id] = struct{}{}
	}
	out := map[string]string{}
	for _, part := range strings.Split(raw, ",") {
		token := strings.TrimSpace(part)
		if token == "" {
			continue
		}
		// Reject typos like "go==1.23" instead of silently using "=1.23"
		// as the pin ref. Real pin refs never contain '='.
		if strings.Count(token, "=") != 1 {
			return nil, fmt.Errorf(
				"%w: --plugin-versions token %q must be <id>=<ref>", clihelpers.ErrUsage, token)
		}
		eq := strings.IndexByte(token, '=')
		id := strings.TrimSpace(token[:eq])
		ref := strings.TrimSpace(token[eq+1:])
		if id == "" || ref == "" {
			return nil, fmt.Errorf(
				"%w: --plugin-versions token %q must be <id>=<ref>", clihelpers.ErrUsage, token)
		}
		p, ok := plugins[id]
		if !ok {
			return nil, fmt.Errorf(
				"%w: --plugin-versions: unknown plugin %q (run `cocoon plugin list`)",
				clihelpers.ErrUsage, id)
		}
		if !p.Version.VersionCapable {
			return nil, fmt.Errorf(
				"%w: --plugin-versions: plugin %q is not version_capable",
				clihelpers.ErrUsage, id)
		}
		if _, on := enabledSet[id]; !on {
			return nil, fmt.Errorf(
				"%w: --plugin-versions: plugin %q must also appear in --plugins",
				clihelpers.ErrUsage, id)
		}
		if _, dup := out[id]; dup {
			return nil, fmt.Errorf(
				"%w: --plugin-versions: duplicate id %q", clihelpers.ErrUsage, id)
		}
		out[id] = ref
	}
	// Returning an empty (non-nil) map for an all-whitespace input is fine —
	// the writer falls back to the commented template when len == 0, and
	// keeping a non-nil sentinel keeps `nilnil` happy without forcing a
	// custom error for "I parsed your input but it was empty."
	return out, nil
}

// parsePluginMethods parses `--plugin-methods=<id>=<method>,…` into a map.
// Each id must be a known plugin, listed in enabled (--plugins), and declare
// [install.methods] in plugin.toml. The method must be one of the declared
// keys. Empty input returns a non-nil empty map (nilnil lint); duplicate ids
// are rejected so a typo cannot silently pick the last value.
func parsePluginMethods(raw string, plugins map[string]*plugin.Plugin, enabled []string) (map[string]string, error) {
	enabledSet := make(map[string]struct{}, len(enabled))
	for _, id := range enabled {
		enabledSet[id] = struct{}{}
	}
	out := map[string]string{}
	for _, part := range strings.Split(raw, ",") {
		token := strings.TrimSpace(part)
		if token == "" {
			continue
		}
		if strings.Count(token, "=") != 1 {
			return nil, fmt.Errorf(
				"%w: --plugin-methods token %q must be <id>=<method>", clihelpers.ErrUsage, token)
		}
		eq := strings.IndexByte(token, '=')
		id := strings.TrimSpace(token[:eq])
		method := strings.TrimSpace(token[eq+1:])
		if id == "" || method == "" {
			return nil, fmt.Errorf(
				"%w: --plugin-methods token %q must be <id>=<method>", clihelpers.ErrUsage, token)
		}
		p, ok := plugins[id]
		if !ok {
			return nil, fmt.Errorf(
				"%w: --plugin-methods: unknown plugin %q (run `cocoon plugin list`)",
				clihelpers.ErrUsage, id)
		}
		if _, on := enabledSet[id]; !on {
			return nil, fmt.Errorf(
				"%w: --plugin-methods: plugin %q must also appear in --plugins",
				clihelpers.ErrUsage, id)
		}
		if len(p.Install.Methods) == 0 {
			return nil, fmt.Errorf(
				"%w: --plugin-methods: plugin %q has no [install.methods] — drop it from --plugin-methods",
				clihelpers.ErrUsage, id)
		}
		if _, declared := p.Install.Methods[method]; !declared {
			return nil, fmt.Errorf(
				"%w: --plugin-methods: plugin %q has no method %q in [install.methods]",
				clihelpers.ErrUsage, id, method)
		}
		if _, dup := out[id]; dup {
			return nil, fmt.Errorf(
				"%w: --plugin-methods: duplicate id %q", clihelpers.ErrUsage, id)
		}
		out[id] = method
	}
	return out, nil
}

// validatePluginConflicts reports the first incompatible pair in the
// enabled list. Conflicts are declared on plugin.toml's metadata.conflicts
// field; the relation is required to be symmetric so checking one
// direction is enough.
func validatePluginConflicts(plugins map[string]*plugin.Plugin, enabled []string) error {
	enabledSet := make(map[string]struct{}, len(enabled))
	for _, id := range enabled {
		enabledSet[id] = struct{}{}
	}
	// Iterate enabled in a deterministic order so the first-failure
	// message is stable when more than one conflict exists.
	sorted := make([]string, len(enabled))
	copy(sorted, enabled)
	sort.Strings(sorted)
	for _, id := range sorted {
		p, ok := plugins[id]
		if !ok {
			continue
		}
		for _, other := range p.Metadata.Conflicts {
			if _, hit := enabledSet[other]; hit {
				return fmt.Errorf("%w: %s conflicts with %s — pick one",
					clihelpers.ErrUsage, id, other)
			}
		}
	}
	return nil
}

// defaultPluginIDs returns the ids of plugins whose plugin.toml metadata
// has `default = true`. Order is sorted by id for determinism.
func defaultPluginIDs(plugins map[string]*plugin.Plugin) []string {
	var ids []string
	for id, p := range plugins {
		if p.Metadata.Default {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

// loadEmbeddedPlugins reads only the embedded catalog (no project /
// user overlays); init bootstraps a fresh project where overlays are
// not meaningful and could not be tampered with by stray files.
func loadEmbeddedPlugins() (map[string]*plugin.Plugin, error) {
	fsys, err := plugin.CatalogFS()
	if err != nil {
		return nil, fmt.Errorf("plugin catalog: %w", err)
	}
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read catalog dir: %w", err)
	}
	out := make(map[string]*plugin.Plugin, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		body, readErr := fs.ReadFile(fsys, id+"/plugin.toml")
		if readErr != nil {
			return nil, fmt.Errorf("read %s/plugin.toml: %w", id, readErr)
		}
		var p plugin.Plugin
		if uerr := config.StrictUnmarshal(id+"/plugin.toml", body, &p); uerr != nil {
			return nil, fmt.Errorf("parse %s/plugin.toml: %w", id, uerr)
		}
		out[id] = &p
	}
	return out, nil
}

// defaultImageVersion returns SupportedImageVersions[image][0], which is
// ordered newest-first.
func defaultImageVersion(image string) string {
	versions := config.SupportedImageVersions[image]
	if len(versions) == 0 {
		return ""
	}
	return versions[0]
}

type containerSpec struct {
	ServiceName    string
	Username       string
	Image          string
	ImageVersion   string
	Shell          string
	Aliases        map[string]string
	MountRoot      string
	Devcontainer   bool
	Certificates   bool
	Packages       []string
	Plugins        []string
	PluginVersions map[string]string
	PluginMethods  map[string]string
	Ports          []string
}

// renderWorkspaceToml emits workspace.toml. Inline comments come from
// the i18n catalog so the locale matches the original runner's $LANG
// (re-run with --force under a different LANG to switch).
//
//nolint:funlen // sequence of independent section emits; splitting hides the resulting TOML's top-to-bottom structure.
func renderWorkspaceToml(s containerSpec, cat *i18n.Catalog) string {
	var sb strings.Builder
	sb.WriteString(cat.Msg("init_toml_header"))
	sb.WriteByte('\n')

	sb.WriteString(cat.Msg("init_toml_section_workspace"))
	sb.WriteByte('\n')
	sb.WriteString("[workspace]\n")
	fmt.Fprintf(&sb, "mount_root = %q\n", s.MountRoot)
	fmt.Fprintf(&sb, "devcontainer = %t\n\n", s.Devcontainer)

	sb.WriteString(cat.Msg("init_toml_section_container"))
	sb.WriteByte('\n')
	sb.WriteString("[container]\n")
	fmt.Fprintf(&sb, "service_name = %q\n", s.ServiceName)
	fmt.Fprintf(&sb, "username = %q\n", s.Username)
	fmt.Fprintf(&sb, "image = %q\n", s.Image)
	fmt.Fprintf(&sb, "image_version = %q\n\n", s.ImageVersion)

	// Commented-out templates for [container] opt-in extras. The flat-field
	// templates come first so an uncommented line still belongs to
	// [container]; the [container.*] subtables follow.
	for _, key := range []string{
		"init_toml_template_container_docker_socket",
		"init_toml_template_container_group_add",
		"init_toml_template_container_devices",
		"init_toml_template_container_ipc",
		"init_toml_template_container_gpus",
	} {
		emitTemplate(&sb, cat, key)
	}
	for _, key := range []string{
		"init_toml_template_container_resources",
		"init_toml_template_container_hosts",
		"init_toml_template_container_dns",
		"init_toml_template_container_sysctls",
		"init_toml_template_container_capabilities",
		"init_toml_template_container_security_opt",
		"init_toml_template_container_skel",
	} {
		emitTemplate(&sb, cat, key)
	}

	sb.WriteString(cat.Msg("init_toml_section_container_shell"))
	sb.WriteByte('\n')
	sb.WriteString("[container.shell]\n")
	fmt.Fprintf(&sb, "default = %q\n", s.Shell)
	if len(s.Aliases) > 0 {
		sb.WriteString("aliases = ")
		writeInlineTable(&sb, s.Aliases)
		sb.WriteByte('\n')
	}
	sb.WriteByte('\n')

	sb.WriteString(cat.Msg("init_toml_section_plugins"))
	sb.WriteByte('\n')
	sb.WriteString("[plugins]\n")
	if len(s.Plugins) == 0 {
		sb.WriteString("enable = []\n\n")
	} else {
		sb.WriteString("enable = [\n")
		for _, id := range s.Plugins {
			fmt.Fprintf(&sb, "    %q,\n", id)
		}
		sb.WriteString("]\n\n")
	}

	writePluginMethods(&sb, cat, s.PluginMethods)
	writePluginVersions(&sb, cat, s.PluginVersions)

	sb.WriteString(cat.Msg("init_toml_section_apt"))
	sb.WriteByte('\n')
	sb.WriteString("[apt]\n")
	if len(s.Packages) == 0 {
		sb.WriteString("packages = []\n\n")
	} else {
		sb.WriteString("packages = [\n")
		for _, pkg := range s.Packages {
			fmt.Fprintf(&sb, "    %q,\n", pkg)
		}
		sb.WriteString("]\n\n")
	}

	for _, key := range []string{
		"init_toml_template_apt_mirror",
		"init_toml_template_apt_proxy",
		"init_toml_template_apt_sources",
	} {
		emitTemplate(&sb, cat, key)
	}

	if s.Certificates {
		sb.WriteString(cat.Msg("init_toml_section_certificates"))
		sb.WriteByte('\n')
		sb.WriteString("[certificates]\n")
		sb.WriteString("enable = true\n\n")
	}

	// [ports] is the only top-level extras section that may be emitted as
	// an active block — when the user supplied ports via --ports or the
	// interactive prompt. With no ports the commented-out template still
	// emits so the file remains self-documenting (matches [volumes] /
	// [env] / [mounts] behavior).
	if len(s.Ports) > 0 {
		sb.WriteString(cat.Msg("init_toml_section_ports"))
		sb.WriteByte('\n')
		sb.WriteString("[ports]\nforward = [\n")
		for _, p := range s.Ports {
			fmt.Fprintf(&sb, "    %q,\n", p)
		}
		sb.WriteString("]\n\n")
	} else {
		emitTemplate(&sb, cat, "init_toml_template_ports")
	}

	// Top-level opt-in extras at the end of the file. Order roughly
	// follows "compose runtime knobs first, then host-side persistence,
	// then locale + Dockerfile hooks, then certificates, then sidecars +
	// IDE config".
	templateKeys := []string{
		"init_toml_template_volumes",
		"init_toml_template_env",
		"init_toml_template_mounts",
		"init_toml_template_home_files",
		"init_toml_template_locale",
		"init_toml_template_dockerfile",
	}
	if !s.Certificates {
		templateKeys = append(templateKeys, "init_toml_template_certificates")
	}
	templateKeys = append(templateKeys,
		"init_toml_template_services",
		"init_toml_template_devcontainer",
	)
	for _, key := range templateKeys {
		emitTemplate(&sb, cat, key)
	}

	return strings.TrimRight(sb.String(), "\n") + "\n"
}

// writePluginMethods emits a single `[plugins.methods]` block with one
// `<id> = "<method>"` line per pick, alphabetically sorted by id. When picks
// is empty it falls back to the commented-out template so users discover the
// section without us having to render an empty `[plugins.methods]` table.
func writePluginMethods(sb *strings.Builder, cat *i18n.Catalog, picks map[string]string) {
	if len(picks) == 0 {
		emitTemplate(sb, cat, "init_toml_template_plugins_methods")
		return
	}
	ids := make([]string, 0, len(picks))
	for id := range picks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	sb.WriteString(cat.Msg("init_toml_section_plugins_methods"))
	sb.WriteByte('\n')
	sb.WriteString("[plugins.methods]\n")
	for _, id := range ids {
		fmt.Fprintf(sb, "%s = %q\n", id, picks[id])
	}
	sb.WriteByte('\n')
}

// writePluginVersions emits a single `[plugins.versions]` section with one
// inline-table line per pin, alphabetically sorted by id. When pins is empty
// it falls back to the commented-out example template so the reader still
// discovers the section.
func writePluginVersions(sb *strings.Builder, cat *i18n.Catalog, pins map[string]string) {
	if len(pins) == 0 {
		emitTemplate(sb, cat, "init_toml_template_plugins_versions")
		return
	}
	lines := make([]plugin.PinLine, 0, len(pins))
	for id, ref := range pins {
		lines = append(lines, plugin.PinLine{ID: id, Ref: ref, ChecksumAmd64: "", ChecksumArm64: ""})
	}
	sb.WriteString(cat.Msg("init_toml_section_plugins_versions"))
	sb.WriteByte('\n')
	sb.WriteString(plugin.FormatPinSection(lines))
	sb.WriteByte('\n')
}

// emitTemplate writes a localized commented-out section template to sb,
// followed by exactly one blank line so adjacent templates stay visually
// separated. Each i18n value is the raw `# ...` block (no trailing
// newline) — the `\n\n` here adds the closing newline for that line plus
// the blank-line separator.
func emitTemplate(sb *strings.Builder, cat *i18n.Catalog, key string) {
	sb.WriteString(cat.Msg(key))
	sb.WriteString("\n\n")
}

// writeInlineTable emits a TOML inline-table value (`{ k = "v", ... }`)
// with keys sorted so the output is deterministic across runs. Used for
// `[container.shell] aliases = { ... }` so the generated workspace.toml
// stays diff-friendly when the user re-runs `cocoon init --force`.
func writeInlineTable(sb *strings.Builder, m map[string]string) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	sb.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			sb.WriteString(", ")
		} else {
			sb.WriteByte(' ')
		}
		fmt.Fprintf(sb, "%s = %q", k, m[k])
	}
	sb.WriteString(" }")
}
