package initcli

import (
	"slices"
	"strings"

	"github.com/sukekyo26/cocoon/internal/aliasbundles"
	"github.com/sukekyo26/cocoon/internal/aptcategories"
	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

// sudoChoiceNone is the init-level selection that disables in-container sudo.
// Unlike nopasswd/password it does not map to [container.sudo]; it sets
// [container.security_opt] no_new_privileges = true.
const sudoChoiceNone = "none"

// sudoChoices lists the three init-level sudo selections in prompt order.
//
//nolint:gochecknoglobals // small fixed choice list, file-scoped by design.
var sudoChoices = []string{config.SudoModeNoPasswd, config.SudoModePassword, sudoChoiceNone}

type initFlags struct {
	AutoYes        bool
	ServiceName    string
	Username       string
	Image          string
	ImageVersion   string
	Shell          string
	MountRoot      string
	Dir            string
	Devcontainer   bool
	NoDevcontainer bool
	Certificates   bool
	NoCertificates bool
	Sudo           string
	ImagePathFix   bool
	NoImagePathFix bool
	AptCategories  string
	Plugins        string
	PluginVersions string
	PluginMethods  string
	AliasBundles   string
	Ports          string
	Force          bool
}

// initAnswers is what gets written into cocoon.toml. The *Set companions
// distinguish "not yet provided" from a zero value the user actively chose
// (e.g. devcontainer = false), so the prompt builder doesn't skip groups
// whose value happens to look empty.
type initAnswers struct {
	ServiceName     string
	Username        string
	Image           string
	ImageSet        bool
	ImageVersion    string
	ImageVersionSet bool
	Shell           string
	ShellSet        bool
	MountRoot       string
	MountRootSet    bool
	Dir             string
	DirSet          bool
	Devcontainer    bool
	DevcontainerSet bool
	Certificates    bool
	CertificatesSet bool
	Sudo            string
	SudoSet         bool
	// SudoPassword is collected only in the interactive flow when Sudo ==
	// "password"; it seeds .devcontainer/.env.local. Never sourced from a flag
	// (a password on the command line leaks into shell history / ps / CI logs).
	SudoPassword      string
	ImagePathFix      bool
	ImagePathFixSet   bool
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
	return clihelpers.UsageErr(
		"err_initflags_image_provides_plugin",
		ans.Image, conflict, conflict, conflict,
	)
}

// applyFlags marks *Set on every populated flag. Empty flags leave the field
// zero so the prompt or default layer fills it in. The flag groups are
// independent; applyPluginFlags runs last because its version/method parsers
// read the plugin id list applyPluginFlags itself populates.
func applyFlags(flags *initFlags, plugins map[string]*plugin.Plugin) (initAnswers, error) {
	var ans initAnswers
	if err := applyIdentityFlags(flags, &ans); err != nil {
		return ans, err
	}
	if err := applyImageFlags(flags, &ans); err != nil {
		return ans, err
	}
	if err := applyToggleFlags(flags, &ans); err != nil {
		return ans, err
	}
	if err := applyImagePathFixFlags(flags, &ans); err != nil {
		return ans, err
	}
	if err := applyListFlags(flags, &ans); err != nil {
		return ans, err
	}
	if err := applyPluginFlags(flags, plugins, &ans); err != nil {
		return ans, err
	}
	return ans, nil
}

// applyIdentityFlags validates --service-name / --username against their
// charset regexes.
func applyIdentityFlags(flags *initFlags, ans *initAnswers) error {
	if flags.ServiceName != "" {
		if !rxServiceName.MatchString(flags.ServiceName) {
			return clihelpers.UsageErr("err_initflags_service_name_mismatch",
				flags.ServiceName, rxServiceName)
		}
		ans.ServiceName = flags.ServiceName
	}
	if flags.Username != "" {
		if !rxUsername.MatchString(flags.Username) {
			return clihelpers.UsageErr("err_initflags_username_mismatch",
				flags.Username, rxUsername)
		}
		ans.Username = flags.Username
	}
	return nil
}

// applyImageFlags validates the base-image quartet: --image, --image-version
// (which requires --image), --shell, --mount-root.
func applyImageFlags(flags *initFlags, ans *initAnswers) error {
	if flags.Image != "" {
		if _, ok := config.SupportedImageVersions[flags.Image]; !ok {
			return clihelpers.UsageErr("err_initflags_image_unsupported",
				flags.Image, strings.Join(config.SupportedImages, ", "))
		}
		ans.Image, ans.ImageSet = flags.Image, true
	}
	if flags.ImageVersion != "" {
		if flags.Image == "" {
			return clihelpers.UsageErr(
				"err_initflags_image_version_requires_image",
				flags.ImageVersion)
		}
		if !rxImageVersionInput.MatchString(flags.ImageVersion) {
			return clihelpers.UsageErr(
				"err_initflags_image_version_mismatch",
				flags.ImageVersion, rxImageVersionInput.String())
		}
		ans.ImageVersion, ans.ImageVersionSet = flags.ImageVersion, true
	}
	if flags.Shell != "" {
		if !slices.Contains(config.SupportedShells, flags.Shell) {
			return clihelpers.UsageErr("err_initflags_shell_unsupported",
				flags.Shell, strings.Join(config.SupportedShells, ", "))
		}
		ans.Shell, ans.ShellSet = flags.Shell, true
	}
	if flags.MountRoot != "" {
		if flags.MountRoot != "." && flags.MountRoot != ".." {
			return clihelpers.UsageErr("err_initflags_mount_root_invalid")
		}
		ans.MountRoot, ans.MountRootSet = flags.MountRoot, true
	}
	if flags.Dir != "" {
		if !config.IsValidWorkspaceDir(flags.Dir) {
			return clihelpers.UsageErr(
				"err_initflags_dir_invalid",
				flags.Dir)
		}
		ans.Dir, ans.DirSet = flags.Dir, true
	}
	return nil
}

// applyToggleFlags resolves the --x / --no-x pairs for --devcontainer and
// --certificates, plus the tri-state --sudo. The mutual-exclusion check
// happens earlier in runInit, so at most one of each pair is set.
// (--image-path-fix / --no-image-path-fix is image-gated and lives in
// applyImagePathFixFlags below.)
func applyToggleFlags(flags *initFlags, ans *initAnswers) error {
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
	if flags.Sudo != "" {
		if !slices.Contains(sudoChoices, flags.Sudo) {
			return clihelpers.UsageErr("err_initflags_sudo_invalid",
				flags.Sudo, strings.Join(sudoChoices, ", "))
		}
		ans.Sudo, ans.SudoSet = flags.Sudo, true
	}
	return nil
}

// applyImagePathFixFlags resolves --image-path-fix / --no-image-path-fix.
// The fix is image-specific, so --image must already be set; an
// otherwise-valid --image-path-fix with no --image would silently hit
// the imagePathFixApplies("") false branch and surface as the confusing
// "--image=\"\" has no fix" message — the explicit --image requirement
// here mirrors --image-version's same dependency. After that gate, the
// pair is rejected against images that have no fix entry so a scripted
// invocation against ubuntu/debian cannot silently no-op.
func applyImagePathFixFlags(flags *initFlags, ans *initAnswers) error {
	if !flags.ImagePathFix && !flags.NoImagePathFix {
		return nil
	}
	flag := "--image-path-fix"
	if flags.NoImagePathFix {
		flag = "--no-image-path-fix"
	}
	if !ans.ImageSet {
		return clihelpers.UsageErr(
			"err_initflags_image_path_fix_requires_image",
			flag)
	}
	if !imagePathFixApplies(ans.Image) {
		return imagePathFixFlagUsageErr(flag, ans.Image)
	}
	ans.ImagePathFix, ans.ImagePathFixSet = flags.ImagePathFix, true
	return nil
}

func imagePathFixFlagUsageErr(flag, image string) error {
	return clihelpers.UsageErr(
		"err_initflags_image_path_fix_no_fix",
		flag, image)
}

// applyListFlags parses the comma-separated list flags: --apt-categories,
// --alias-bundles, --ports.
func applyListFlags(flags *initFlags, ans *initAnswers) error {
	if flags.AptCategories != "" {
		ids, err := parseAptCategories(flags.AptCategories)
		if err != nil {
			return err
		}
		ans.AptCategories, ans.AptSet = ids, true
	}
	if flags.AliasBundles != "" {
		ids, err := parseAliasBundles(flags.AliasBundles)
		if err != nil {
			return err
		}
		ans.AliasBundles, ans.AliasBundlesSet = ids, true
	}
	if flags.Ports != "" {
		ports, err := parsePorts(flags.Ports)
		if err != nil {
			return err
		}
		ans.Ports, ans.PortsSet = ports, true
	}
	return nil
}

// applyPluginFlags resolves --plugins first so the --plugin-versions and
// --plugin-methods parsers can cross-check each id against the enabled set.
func applyPluginFlags(flags *initFlags, plugins map[string]*plugin.Plugin, ans *initAnswers) error {
	if flags.Plugins != "" {
		ids, err := parsePlugins(flags.Plugins, plugins)
		if err != nil {
			return err
		}
		if conflictErr := validatePluginConflicts(plugins, ids); conflictErr != nil {
			return conflictErr
		}
		ans.Plugins, ans.PluginsSet = ids, true
	}
	if flags.PluginVersions != "" {
		pins, err := parsePluginVersions(flags.PluginVersions, plugins, ans.Plugins)
		if err != nil {
			return err
		}
		ans.PluginVersions, ans.PluginVersionsSet = pins, true
	}
	if flags.PluginMethods != "" {
		picks, err := parsePluginMethods(flags.PluginMethods, plugins, ans.Plugins)
		if err != nil {
			return err
		}
		ans.PluginMethods, ans.PluginMethodsSet = picks, true
	}
	return nil
}

// applyDefaults fills the still-empty answer fields with sensible
// defaults so --yes can proceed without prompts. service_name and
// username are required and never defaulted; missing them returns
// clihelpers.ErrUsage so CI scripts know to pass the flags.
func applyDefaults(ans initAnswers, plugins map[string]*plugin.Plugin) (initAnswers, error) {
	if ans.ServiceName == "" {
		return ans, clihelpers.UsageErr("err_initflags_yes_requires_service_name")
	}
	if ans.Username == "" {
		return ans, clihelpers.UsageErr("err_initflags_yes_requires_username")
	}
	applyIdentityDefaults(&ans)
	applyWorkspaceDefaults(&ans)
	applyListDefaults(&ans, plugins)
	return ans, nil
}

// applyIdentityDefaults fills the image / shell-related fields. The
// image-path-fix toggle defaults to true for language images so `--yes`
// scripts inherit the same safe-by-default behavior the interactive
// prompt uses; ubuntu/debian leave it false because imagePathFixApplies
// is false there.
func applyIdentityDefaults(ans *initAnswers) {
	if !ans.ImageSet {
		ans.Image, ans.ImageSet = "debian", true
	}
	if !ans.ImageVersionSet {
		ans.ImageVersion, ans.ImageVersionSet = defaultImageVersion(ans.Image), true
	}
	if !ans.ShellSet {
		ans.Shell, ans.ShellSet = "bash", true
	}
	if !ans.ImagePathFixSet {
		ans.ImagePathFix = imagePathFixApplies(ans.Image)
		ans.ImagePathFixSet = true
	}
}

// applyWorkspaceDefaults fills the [workspace]-related fields and the
// devcontainer / certificates / sudo selections.
func applyWorkspaceDefaults(ans *initAnswers) {
	if !ans.MountRootSet {
		ans.MountRoot, ans.MountRootSet = ".", true
	}
	if !ans.DirSet {
		ans.Dir, ans.DirSet = "workspace", true
	}
	if !ans.DevcontainerSet {
		ans.Devcontainer, ans.DevcontainerSet = true, true
	}
	if !ans.CertificatesSet {
		ans.Certificates, ans.CertificatesSet = false, true
	}
	if !ans.SudoSet {
		ans.Sudo, ans.SudoSet = config.SudoModeNoPasswd, true
	}
}

// applyListDefaults fills the multi-select / plugin-related fields.
func applyListDefaults(ans *initAnswers, plugins map[string]*plugin.Plugin) {
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
	if !ans.PluginVersionsSet {
		ans.PluginVersions, ans.PluginVersionsSet = nil, true
	}
}

// defaultImageVersion returns SupportedImageVersions[image][0] — the first
// entry, which is the default / recommended tag cocoon picks when
// --image-version is omitted. Lists are usually newest-first, but the first
// entry is whichever tag is the default (e.g. debian leads with 12, not the
// newer 13).
func defaultImageVersion(image string) string {
	versions := config.SupportedImageVersions[image]
	if len(versions) == 0 {
		return ""
	}
	return versions[0]
}
