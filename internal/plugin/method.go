package plugin

import (
	"errors"
	"fmt"
)

// ErrNilPlugin is returned by ResolveMethod when called with a nil
// *Plugin. Exposed as a sentinel so callers (and tests) can match the
// failure class via errors.Is.
var ErrNilPlugin = errors.New("plugin: nil plugin pointer")

// ErrUnknownMethod is returned by ResolveMethod when workspace.toml's
// [plugins.methods] map names an install method that the plugin does
// not declare in [install.methods]. The package returns only the
// method name in its message; callers add their own plugin-id context
// to avoid duplicate-id wrapping.
var ErrUnknownMethod = errors.New("unknown install method")

// ResolveMethod selects which install.<name>.sh to use for the plugin
// identified by id. methods is the workspace.toml [plugins.methods]
// map (may be nil or empty). The returned name is:
//   - "" when p has no [install.methods] section (legacy install.sh path)
//   - methods[id] when it names a declared method
//   - p.Install.DefaultMethod when no workspace override is provided
//
// Returns ErrNilPlugin when p is nil and ErrUnknownMethod when
// methods[id] points at an undeclared method. Callers can distinguish
// the failure class via errors.Is.
func ResolveMethod(p *Plugin, id string, methods map[string]string) (string, error) {
	if p == nil {
		return "", fmt.Errorf("%w: id %q", ErrNilPlugin, id)
	}
	if len(p.Install.Methods) == 0 {
		return "", nil
	}
	if chosen := methods[id]; chosen != "" {
		if _, ok := p.Install.Methods[chosen]; !ok {
			return "", fmt.Errorf("%w: %q", ErrUnknownMethod, chosen)
		}
		return chosen, nil
	}
	return p.Install.DefaultMethod, nil
}
