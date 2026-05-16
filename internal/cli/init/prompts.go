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
