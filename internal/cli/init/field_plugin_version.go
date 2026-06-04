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
// encoded as absence from pins (the enable entry is then written bare, with
// no "=<version>" suffix). Upstream-existence is not verified; the format
// validator only enforces a TOML-safe charset.
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
		pin, err := promptOnePluginVersion(cat, id, p.Metadata.URL)
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
// the plugin absent from pins, so its enable entry is written bare (LATEST).
const pluginVersionLatestSentinel = "LATEST"

// promptOnePluginVersion returns "" when the user kept LATEST. A non-empty
// url is rendered under the description so the user can look up valid
// version strings on the upstream release page before typing one.
func promptOnePluginVersion(cat *i18n.Catalog, id, url string) (string, error) {
	var picked string
	field := newSelectOrInputField(
		"plugin_version_"+id,
		&picked,
		[]string{pluginVersionLatestSentinel},
		cat.Msg("init_option_other_manual_input"),
	).
		Title(fmt.Sprintf(cat.Msg("init_prompt_plugin_version"), id)).
		Description(cat.Msg("init_desc_plugin_version")).
		URLLine(url).
		Validate(versionStringValidator(cat, "init_err_plugin_pin_fmt", pluginVersionLatestSentinel))
	if err := runSingleFieldForm(field); err != nil {
		return "", err
	}
	if picked == pluginVersionLatestSentinel {
		return "", nil
	}
	return strings.TrimSpace(picked), nil
}
