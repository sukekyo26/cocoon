package plugin

import (
	"errors"
	"fmt"
)

// ErrUnknownMethod is returned by ResolveMethod when workspace.toml's
// [plugins.methods] map names an install method that the plugin does
// not declare in [install.methods].
var ErrUnknownMethod = errors.New("plugin: unknown install method")

// ResolveMethod selects which install.<name>.sh to use for the plugin
// identified by id. methods is the workspace.toml [plugins.methods]
// map (may be nil or empty). The returned name is:
//   - "" when p has no [install.methods] section (legacy install.sh path)
//   - methods[id] when it names a declared method
//   - p.Install.DefaultMethod when no workspace override is provided
//   - ErrUnknownMethod when methods[id] points at an undeclared method
//
// Callers can distinguish the failure class via errors.Is.
func ResolveMethod(p *Plugin, id string, methods map[string]string) (string, error) {
	if len(p.Install.Methods) == 0 {
		return "", nil
	}
	if chosen := methods[id]; chosen != "" {
		if _, ok := p.Install.Methods[chosen]; !ok {
			return "", fmt.Errorf("%w: %q for plugin %q", ErrUnknownMethod, chosen, id)
		}
		return chosen, nil
	}
	return p.Install.DefaultMethod, nil
}
