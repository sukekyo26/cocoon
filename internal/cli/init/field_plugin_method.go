package initcli

import (
	"fmt"
	"sort"

	"github.com/charmbracelet/huh"

	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

// promptPluginMethodsForMulti shows one method picker per enabled plugin that
// declares two or more [install.methods.<name>] entries. Plugins with zero or
// one declared method are silently skipped — there is no real choice and the
// extra prompt would just add friction. Plugins whose pick is already present
// in picks (typically pre-filled from --plugin-methods on the same interactive
// run) are skipped so the flag wins and the user is not re-prompted. The
// non-interactive (--yes) path bypasses this function entirely via
// applyDefaults. The chosen method is recorded in picks; absent entries leave
// the plugin on its DefaultMethod at install time.
func promptPluginMethodsForMulti(
	cat *i18n.Catalog,
	plugins map[string]*plugin.Plugin,
	enable []string,
	picks map[string]string,
) error {
	for _, id := range enable {
		p, ok := plugins[id]
		if !ok {
			continue
		}
		if len(p.Install.Methods) < 2 {
			continue
		}
		if _, alreadyPicked := picks[id]; alreadyPicked {
			continue
		}
		method, err := promptOnePluginMethod(cat, id, p)
		if err != nil {
			return err
		}
		picks[id] = method
	}
	return nil
}

// promptOnePluginMethod renders a huh.Select listing every declared method
// name, each annotated with its description. The plugin's DefaultMethod is
// pre-selected so pressing Enter on the prompt is a no-op for the common
// case (user accepts the recommendation).
func promptOnePluginMethod(cat *i18n.Catalog, id string, p *plugin.Plugin) (string, error) {
	names := make([]string, 0, len(p.Install.Methods))
	for name := range p.Install.Methods {
		names = append(names, name)
	}
	sort.Strings(names)

	options := make([]huh.Option[string], 0, len(names))
	for _, name := range names {
		label := fmt.Sprintf("%s — %s", name, p.Install.Methods[name].Description)
		options = append(options, huh.NewOption(label, name))
	}

	picked := p.Install.DefaultMethod
	field := huh.NewSelect[string]().
		Title(fmt.Sprintf(cat.Msg("init_prompt_plugin_method"), id)).
		Description(cat.Msg("init_desc_plugin_method")).
		Options(options...).
		Value(&picked)
	if err := runSingleFieldForm(field); err != nil {
		return "", err
	}
	return picked, nil
}
