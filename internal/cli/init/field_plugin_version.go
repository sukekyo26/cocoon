package initcli

import (
	"fmt"
	"strings"

	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

// promptPluginVersionsForCapable shows a LATEST + free-text picker per
// version_capable plugin. Plugins already pinned via --plugin-versions
// are skipped so flag values win in non-interactive flows. "LATEST" is
// encoded as absence from pins (writePluginVersions then omits the line).
// Upstream-existence is not verified; the format validator only enforces
// a TOML-safe charset.
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

// pluginVersionLatestSentinel labels the "no pin" row. Picking it leaves
// the plugin absent from pins, which writePluginVersions treats as LATEST.
const pluginVersionLatestSentinel = "LATEST"

// promptOnePluginVersion returns "" when the user kept LATEST.
func promptOnePluginVersion(cat *i18n.Catalog, id string) (string, error) {
	var picked string
	field := newSelectOrInputField(
		"plugin_version_"+id,
		&picked,
		[]string{pluginVersionLatestSentinel},
		cat.Msg("init_option_other_manual_input"),
	).
		Title(fmt.Sprintf(cat.Msg("init_prompt_plugin_version"), id)).
		Description(cat.Msg("init_desc_plugin_version")).
		Validate(versionStringValidator(cat, "init_err_plugin_pin_fmt", pluginVersionLatestSentinel))
	if err := runSingleFieldForm(field); err != nil {
		return "", err
	}
	if picked == pluginVersionLatestSentinel {
		return "", nil
	}
	return strings.TrimSpace(picked), nil
}
