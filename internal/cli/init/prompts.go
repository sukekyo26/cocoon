package initcli

import (
	"errors"
	"fmt"
	"os"
	"regexp"

	"github.com/charmbracelet/huh"

	"github.com/sukekyo26/cocoon/internal/aliasbundles"
	"github.com/sukekyo26/cocoon/internal/aptcategories"
	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

// promptForMissing uses single-field huh.Forms (one prompt per screen).
// Multi-Group + OptionsFunc had a viewport race vs. async TitleFunc that
// left the cursor stuck under scrolling options; independent forms side-step
// it. Tradeoff: no shift+tab back-nav; re-run `cocoon init` to fix.
//
// The three groups run in screen order: identity + base image, then the
// workspace options, then the plugin selection.
func promptForMissing(ans initAnswers, cat *i18n.Catalog, plugins map[string]*plugin.Plugin) (initAnswers, error) {
	if err := promptIdentityAndImage(&ans, cat); err != nil {
		return ans, err
	}
	if err := promptWorkspaceOptions(&ans, cat); err != nil {
		return ans, err
	}
	if err := promptPluginSelection(&ans, cat, plugins); err != nil {
		return ans, err
	}
	return ans, nil
}

// promptIdentityAndImage prompts for service-name, username, base image,
// image version, and login shell.
func promptIdentityAndImage(ans *initAnswers, cat *i18n.Catalog) error {
	if ans.ServiceName == "" {
		if err := runStrictIdentForm(cat, "init_prompt_service_name",
			"init_desc_service_name", "init_err_service_name_fmt",
			rxServiceName, &ans.ServiceName); err != nil {
			return err
		}
	}
	if ans.Username == "" {
		if err := runStrictIdentForm(cat, "init_prompt_username",
			"init_desc_username", "init_err_username_fmt",
			rxUsername, &ans.Username); err != nil {
			return err
		}
	}
	if !ans.ImageSet {
		if ans.Image == "" {
			ans.Image = "debian"
		}
		if err := runSingleFieldForm(imageSelect(cat, &ans.Image)); err != nil {
			return err
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
			return err
		}
		ans.ImageVersionSet = true
	}
	if err := promptImagePathFix(ans, cat); err != nil {
		return err
	}
	if !ans.ShellSet {
		ans.Shell = "bash"
		if err := runSingleFieldForm(shellSelect(cat, &ans.Shell)); err != nil {
			return err
		}
		ans.ShellSet = true
	}
	return nil
}

// promptImagePathFix asks the image-path-fix confirm only when the
// chosen image has a fix entry; for other images (ubuntu, debian) it
// silently marks the answer as set so subsequent --force re-runs do not
// re-prompt against a non-applicable image.
func promptImagePathFix(ans *initAnswers, cat *i18n.Catalog) error {
	if ans.ImagePathFixSet {
		return nil
	}
	if !imagePathFixApplies(ans.Image) {
		ans.ImagePathFixSet = true
		return nil
	}
	ans.ImagePathFix = true
	if err := runSingleFieldForm(imagePathFixConfirm(cat, ans.Image, &ans.ImagePathFix)); err != nil {
		return err
	}
	ans.ImagePathFixSet = true
	return nil
}

// promptWorkspaceOptions prompts for alias bundles, mount root, the
// devcontainer / certificates toggles, ports, and apt categories.
func promptWorkspaceOptions(ans *initAnswers, cat *i18n.Catalog) error {
	if err := promptMountAndDir(ans, cat); err != nil {
		return err
	}
	if !ans.DevcontainerSet {
		ans.Devcontainer = true
		if err := runSingleFieldForm(devcontainerConfirm(cat, &ans.Devcontainer)); err != nil {
			return err
		}
		ans.DevcontainerSet = true
	}
	if !ans.CertificatesSet {
		ans.Certificates = false
		if err := runSingleFieldForm(certificatesConfirm(cat, &ans.Certificates)); err != nil {
			return err
		}
		ans.CertificatesSet = true
	}
	if !ans.PortsSet {
		ports, err := promptForPorts(cat)
		if err != nil {
			return err
		}
		ans.Ports, ans.PortsSet = ports, true
	}
	if !ans.AptSet {
		ans.AptCategories = aptcategories.DefaultAptCategoryIDs()
		if err := runSingleFieldForm(aptMultiSelect(cat, &ans.AptCategories)); err != nil {
			return err
		}
		ans.AptSet = true
	}
	return nil
}

// promptPluginSelection prompts for the enabled plugins, then their install
// methods, then their version pins. Method prompts run BEFORE version prompts
// because picking a method may change the upstream URL shown beside the
// version picker (e.g. official installer page vs. GitHub Releases).
func promptPluginSelection(ans *initAnswers, cat *i18n.Catalog, plugins map[string]*plugin.Plugin) error {
	if !ans.PluginsSet {
		// Hide plugins whose toolchain duplicates the chosen base image so the
		// user cannot accidentally pick a combination assertNoImagePluginConflict
		// would later reject.
		excludeID := config.ImageProvidesPlugin[ans.Image]
		ans.Plugins = filterPluginIDs(defaultPluginIDs(plugins), excludeID)
		if err := promptPluginsWithRetry(cat, plugins, excludeID, &ans.Plugins); err != nil {
			return err
		}
		ans.PluginsSet = true
	}
	// Allocate the map up front: an empty map is the "use default_method
	// everywhere" signal, indistinguishable from nil for downstream consumers.
	if ans.PluginMethods == nil {
		ans.PluginMethods = make(map[string]string)
	}
	if err := promptPluginMethodsForMulti(cat, plugins, ans.Plugins, ans.PluginMethods); err != nil {
		return err
	}
	ans.PluginMethodsSet = true
	// Allocate the map even if the prompt produces no picks; an empty map
	// is equivalent to nil in writePluginVersions (both hit the len==0
	// fallback that emits the commented-out template).
	if ans.PluginVersions == nil {
		ans.PluginVersions = make(map[string]string)
	}
	if err := promptPluginVersionsForCapable(cat, plugins, ans.Plugins, ans.PluginVersions); err != nil {
		return err
	}
	ans.PluginVersionsSet = true
	return nil
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

// runSingleFieldForm wraps a single huh.Field into a one-Group, one-
// Field Form and runs it. Centralises the ErrUserAborted / generic-
// failure error wrapping so the call sites stay readable.
//
// Every prompt is its own form (see initLong), so Shift+Tab has no
// previous field to land on. The form keymap below blanks the Prev
// binding's help text in every field profile so huh's help bar stops
// advertising a "shift+tab back" affordance that does nothing.
//
// runSingleFieldForm is a package-level var rather than a func so tests
// can replace it with a no-op (or counter) and exercise the surrounding
// prompt flow's branching logic without a real TTY.
var runSingleFieldForm = func(field huh.Field) error {
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

// promptMountAndDir prompts for alias bundles, mount_root and [workspace].dir
// as one step so the parent promptWorkspaceOptions stays under gocognit.
func promptMountAndDir(ans *initAnswers, cat *i18n.Catalog) error {
	if !ans.AliasBundlesSet {
		ans.AliasBundles = aliasbundles.DefaultAliasBundleIDs()
		if err := runSingleFieldForm(aliasBundlesMultiSelect(cat, &ans.AliasBundles)); err != nil {
			return err
		}
		ans.AliasBundlesSet = true
	}
	if !ans.MountRootSet {
		ans.MountRoot = "."
		if err := runSingleFieldForm(mountRootSelect(cat, &ans.MountRoot)); err != nil {
			return err
		}
		ans.MountRootSet = true
	}
	if !ans.DirSet {
		if err := promptDir(ans, cat); err != nil {
			return err
		}
	}
	return nil
}

// promptDir reads [workspace].dir from a free-text Input. Blank input
// falls back to the "workspace" default; non-blank values are validated
// in dirInput so init never accepts a string `cocoon gen` would reject.
func promptDir(ans *initAnswers, cat *i18n.Catalog) error {
	var raw string
	if err := runSingleFieldForm(dirInput(cat, &raw)); err != nil {
		return err
	}
	if raw == "" {
		raw = "workspace"
	}
	ans.Dir, ans.DirSet = raw, true
	return nil
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
