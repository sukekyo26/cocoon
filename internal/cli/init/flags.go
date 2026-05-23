package initcli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/sukekyo26/cocoon/internal/aliasbundles"
	"github.com/sukekyo26/cocoon/internal/aptcategories"
	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

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
	Dir               string
	DirSet            bool
	Devcontainer      bool
	DevcontainerSet   bool
	Certificates      bool
	CertificatesSet   bool
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
	return fmt.Errorf(
		"%w: image=%q already provides %s; drop %q from --plugins, "+
			"or pick --image=ubuntu/debian to pin a custom %s via the plugin",
		clihelpers.ErrUsage, ans.Image, conflict, conflict, conflict,
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
	applyToggleFlags(flags, &ans)
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
			return fmt.Errorf("%w: --service-name %q does not match %s",
				clihelpers.ErrUsage, flags.ServiceName, rxServiceName)
		}
		ans.ServiceName = flags.ServiceName
	}
	if flags.Username != "" {
		if !rxUsername.MatchString(flags.Username) {
			return fmt.Errorf("%w: --username %q does not match %s",
				clihelpers.ErrUsage, flags.Username, rxUsername)
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
			return fmt.Errorf("%w: --image %q not in %s",
				clihelpers.ErrUsage, flags.Image, strings.Join(config.SupportedImages, ", "))
		}
		ans.Image, ans.ImageSet = flags.Image, true
	}
	if flags.ImageVersion != "" {
		if flags.Image == "" {
			return fmt.Errorf(
				"%w: --image-version %q requires --image (so the registry path is known)",
				clihelpers.ErrUsage, flags.ImageVersion)
		}
		if !rxImageVersionInput.MatchString(flags.ImageVersion) {
			return fmt.Errorf(
				"%w: --image-version %q must match %s",
				clihelpers.ErrUsage, flags.ImageVersion, rxImageVersionInput.String())
		}
		ans.ImageVersion, ans.ImageVersionSet = flags.ImageVersion, true
	}
	if flags.Shell != "" {
		if !slices.Contains(config.SupportedShells, flags.Shell) {
			return fmt.Errorf("%w: --shell %q not in %s",
				clihelpers.ErrUsage, flags.Shell, strings.Join(config.SupportedShells, ", "))
		}
		ans.Shell, ans.ShellSet = flags.Shell, true
	}
	if flags.MountRoot != "" {
		if flags.MountRoot != "." && flags.MountRoot != ".." {
			return fmt.Errorf(`%w: --mount-root must be "." or ".."`, clihelpers.ErrUsage)
		}
		ans.MountRoot, ans.MountRootSet = flags.MountRoot, true
	}
	if flags.Dir != "" {
		if !config.IsValidWorkspaceDir(flags.Dir) {
			return fmt.Errorf(
				`%w: --dir %q must be one or more path segments of [A-Za-z0-9._-] joined by "/" `+
					`(no leading/trailing slash, no "." or ".." segments)`,
				clihelpers.ErrUsage, flags.Dir)
		}
		ans.Dir, ans.DirSet = flags.Dir, true
	}
	return nil
}

// applyToggleFlags resolves the --x / --no-x pairs for --devcontainer
// and --certificates. The mutual-exclusion check happens earlier in
// runInit, so at most one of each pair is set.
// (--image-path-fix / --no-image-path-fix is image-gated and lives in
// applyImagePathFixFlags below.)
func applyToggleFlags(flags *initFlags, ans *initAnswers) {
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
		return fmt.Errorf(
			"%w: %s requires --image (the fix is image-specific)",
			clihelpers.ErrUsage, flag)
	}
	if !imagePathFixApplies(ans.Image) {
		return imagePathFixFlagUsageErr(flag, ans.Image)
	}
	ans.ImagePathFix, ans.ImagePathFixSet = flags.ImagePathFix, true
	return nil
}

func imagePathFixFlagUsageErr(flag, image string) error {
	return fmt.Errorf(
		"%w: %s only applies to language images "+
			"(node, python, golang, rust, denoland/deno); --image=%q has no fix",
		clihelpers.ErrUsage, flag, image)
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
		return ans, fmt.Errorf("%w: --yes requires --service-name", clihelpers.ErrUsage)
	}
	if ans.Username == "" {
		return ans, fmt.Errorf("%w: --yes requires --username", clihelpers.ErrUsage)
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
		ans.Image, ans.ImageSet = "ubuntu", true
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
// devcontainer / certificates toggles.
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

// defaultImageVersion returns SupportedImageVersions[image][0], which is
// ordered newest-first.
func defaultImageVersion(image string) string {
	versions := config.SupportedImageVersions[image]
	if len(versions) == 0 {
		return ""
	}
	return versions[0]
}
