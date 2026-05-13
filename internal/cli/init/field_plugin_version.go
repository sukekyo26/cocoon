package initcli

import (
	"fmt"
	"strings"

	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

// promptPluginVersionsForCapable walks the user-enabled plugins in order and,
// for each one whose plugin.toml declares version_capable = true, shows a
// single-screen picker: a LATEST row followed by an editable free-text row
// ("Other (manual input)"). The chosen value is merged into pins.
//
// Plugins already pinned via --plugin-versions are skipped: the flag value
// takes precedence so non-interactive flows stay deterministic. Plugins
// without version_capable = true are also skipped because their install.sh
// cannot consume a $PIN.
//
// "LATEST" is encoded as the absence of an entry in pins. The writer
// emits one inline-table line per pin under a single [plugins.versions]
// section, so a missing entry means the `<id> = { ... }` line is simply
// not emitted and install.sh resolves latest at container build time.
//
// The picker does not verify whether the typed string actually exists
// upstream — that is the user's responsibility (the i18n description
// points them at the upstream URL). The format validator only enforces
// a TOML-safe character set so the inline-table line stays parseable.
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

// pluginVersionLatestSentinel is the row label shown for the "no pin,
// resolve latest at build time" option. Picking it is encoded as
// *absence* of the plugin id in the pins map (promptPluginVersionsForCapable
// skips the assignment when the returned pin is ""), which downstream
// writePluginVersions treats as the LATEST case. Keep the label short
// so the picker line stays scannable.
const pluginVersionLatestSentinel = "LATEST"

// promptOnePluginVersion runs the single-screen LATEST / manual-input
// picker for one plugin. Returns "" when the user kept LATEST.
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
