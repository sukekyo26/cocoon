package initcli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

// promptPluginVersionsForCapable walks the user-enabled plugins in order and,
// for each one whose plugin.toml declares version_capable = true, asks the
// user whether to pin a specific version or keep LATEST (resolved at
// container build time). The result is merged into pins.
//
// Plugins already pinned via --plugin-versions are skipped: the flag value
// takes precedence so non-interactive flows stay deterministic. Plugins
// without version_capable = true are also skipped because their install.sh
// cannot consume a $PIN.
//
// "LATEST" is encoded as the absence of an entry in pins, so the writer's
// fallback (omit [plugins.versions.<id>], install.sh resolves latest at
// build time) is reused unchanged.
func promptPluginVersionsForCapable(
	cat *i18n.Catalog,
	plugins map[string]*plugin.Plugin,
	enable []string,
	pins map[string]string,
) error {
	for _, id := range enable {
		p, ok := plugins[id]
		if !ok || !p.Version.VersionCapable {
			continue
		}
		if _, alreadyPinned := pins[id]; alreadyPinned {
			continue
		}
		pin, err := promptOnePluginVersion(cat, id)
		if err != nil {
			return err
		}
		if pin != "" {
			pins[id] = pin
		}
	}
	return nil
}

// promptOnePluginVersion runs the two-step Yes/No → optional input prompt
// for one plugin. Returns "" when the user kept LATEST.
func promptOnePluginVersion(cat *i18n.Catalog, id string) (string, error) {
	wantPin := false
	confirm := huh.NewConfirm().
		Title(fmt.Sprintf(cat.Msg("init_prompt_plugin_version"), id)).
		Description(cat.Msg("init_desc_plugin_version")).
		Affirmative(cat.Msg("init_plugin_version_pin_label")).
		Negative(cat.Msg("init_plugin_version_latest_label")).
		Value(&wantPin)
	if err := runSingleFieldForm(confirm); err != nil {
		return "", err
	}
	if !wantPin {
		return "", nil
	}

	var pin string
	input := huh.NewInput().
		Title(fmt.Sprintf(cat.Msg("init_prompt_plugin_version_pin"), id)).
		Description(cat.Msg("init_desc_plugin_version_pin")).
		Validate(pluginPinValidator(cat)).
		Value(&pin)
	if err := runSingleFieldForm(input); err != nil {
		return "", err
	}
	return strings.TrimSpace(pin), nil
}

// pluginPinValidator enforces the same character set as image_version
// (`rxImageVersionInput`) so a pin accepted here cannot be rejected by the
// loader's [plugins.versions] regex (`rxImageVersion`). The error string is
// localized via the catalog because huh prints it verbatim in the form footer.
func pluginPinValidator(cat *i18n.Catalog) func(string) error {
	return func(s string) error {
		s = strings.TrimSpace(s)
		if s == "" {
			return errors.New(cat.Msg("init_err_required")) //nolint:err113 // user-facing prompt
		}
		if !rxImageVersionInput.MatchString(s) {
			return errors.New(cat.Msg("init_err_plugin_pin_fmt")) //nolint:err113 // user-facing prompt
		}
		return nil
	}
}
