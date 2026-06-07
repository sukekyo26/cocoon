package plugin

import (
	"errors"
	"fmt"
)

// ErrNilPlugin is returned by ResolveMethod when called with a nil
// *Plugin. Exposed as a sentinel so callers (and tests) can match the
// failure class via errors.Is.
var ErrNilPlugin = errors.New("plugin: nil plugin pointer")

// ErrUnknownMethod is returned by ResolveMethod when cocoon.toml's
// [plugins.methods] map names an install method that the plugin does
// not declare in [install.methods]. The wrapped message includes the
// method name but omits the plugin id; callers add their own plugin-id
// context (e.g. via fmt.Errorf("plugin %q: %w", id, err)) to avoid
// duplicate-id wrapping.
var ErrUnknownMethod = errors.New("unknown install method")

// ResolveMethod selects which install.<name>.sh to use for the plugin
// identified by id. methods is the cocoon.toml [plugins.methods]
// map (may be nil or empty). The returned name is:
//   - methods[id] when it names a declared method
//   - p.Install.DefaultMethod when no workspace override is provided
//   - "" when p has no [install.methods] entries at all — this is not a
//     valid loaded-plugin state (the loader's validateMethodScripts
//     rejects empty Methods), so callers should treat "" as a
//     pre-validation / test-data shape rather than a "legacy install.sh"
//     runtime path. The branch is kept so unit tests can build *Plugin
//     literals without filling in Methods.
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
