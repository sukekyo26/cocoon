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

// defaultImageVersion returns SupportedImageVersions[image][0], which is
// ordered newest-first.
func defaultImageVersion(image string) string {
	versions := config.SupportedImageVersions[image]
	if len(versions) == 0 {
		return ""
	}
	return versions[0]
}
