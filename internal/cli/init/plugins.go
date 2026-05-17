package initcli

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

// validatePluginConflicts reports the first incompatible pair in the
// enabled list. Conflicts are declared on plugin.toml's metadata.conflicts
// field; every enabled plugin's list is scanned, so an asymmetric
// declaration (only one side names the other) is still caught.
func validatePluginConflicts(plugins map[string]*plugin.Plugin, enabled []string) error {
	enabledSet := make(map[string]struct{}, len(enabled))
	for _, id := range enabled {
		enabledSet[id] = struct{}{}
	}
	// Iterate enabled in a deterministic order so the first-failure
	// message is stable when more than one conflict exists.
	sorted := make([]string, len(enabled))
	copy(sorted, enabled)
	sort.Strings(sorted)
	for _, id := range sorted {
		p, ok := plugins[id]
		if !ok {
			continue
		}
		for _, other := range p.Metadata.Conflicts {
			if _, hit := enabledSet[other]; hit {
				return fmt.Errorf("%w: %s conflicts with %s — pick one",
					clihelpers.ErrUsage, id, other)
			}
		}
	}
	return nil
}

// defaultPluginIDs returns the ids of plugins whose plugin.toml metadata
// has `default = true`. Order is sorted by id for determinism.
func defaultPluginIDs(plugins map[string]*plugin.Plugin) []string {
	var ids []string
	for id, p := range plugins {
		if p.Metadata.Default {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

// filterPluginIDs returns ids with excludeID removed. Empty excludeID
// returns ids verbatim; the caller does not need to special-case the
// "no exclusion" path.
func filterPluginIDs(ids []string, excludeID string) []string {
	if excludeID == "" {
		return ids
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == excludeID {
			continue
		}
		out = append(out, id)
	}
	return out
}

// formatPluginLabel shows conflicts up front so the user does not have to
// dig into plugin.toml.
func formatPluginLabel(id string, p *plugin.Plugin) string {
	name := p.Metadata.Name
	if name == "" {
		name = id
	}
	hint := p.Metadata.Description
	if len(p.Metadata.Conflicts) > 0 {
		// Conflicts is the actionable signal; drop description if both
		// are set so the warning is what the user sees.
		hint = "conflicts: " + strings.Join(p.Metadata.Conflicts, ", ")
	}
	if hint == "" {
		return name
	}
	return fmt.Sprintf("%s (%s)", name, hint)
}

// loadEmbeddedPlugins reads only the embedded catalog (no project /
// user overlays); init bootstraps a fresh project where overlays are
// not meaningful and could not be tampered with by stray files.
func loadEmbeddedPlugins() (map[string]*plugin.Plugin, error) {
	fsys, err := plugin.CatalogFS()
	if err != nil {
		return nil, fmt.Errorf("plugin catalog: %w", err)
	}
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read catalog dir: %w", err)
	}
	out := make(map[string]*plugin.Plugin, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		body, readErr := fs.ReadFile(fsys, id+"/plugin.toml")
		if readErr != nil {
			return nil, fmt.Errorf("read %s/plugin.toml: %w", id, readErr)
		}
		var p plugin.Plugin
		if uerr := config.StrictUnmarshal(id+"/plugin.toml", body, &p); uerr != nil {
			return nil, fmt.Errorf("parse %s/plugin.toml: %w", id, uerr)
		}
		out[id] = &p
	}
	return out, nil
}
