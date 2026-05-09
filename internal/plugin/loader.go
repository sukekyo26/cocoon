package plugin

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sukekyo26/cocoon/internal/config"
)

// Load parses and validates a single plugin TOML file.
func Load(path string) (*Plugin, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path provided by trusted caller.
	if err != nil {
		return nil, config.WrapIO(path, err) //nolint:wrapcheck // wrapper preserves the renderer-friendly format.
	}
	var p Plugin
	if err := config.StrictUnmarshal(path, data, &p); err != nil {
		return nil, err //nolint:wrapcheck // already a *config.ValidationError.
	}
	if err := p.Validate(path); err != nil {
		return nil, err //nolint:wrapcheck // already a *config.ValidationError.
	}
	return &p, nil
}

// LoadEnabled loads plugin.toml for every id in `enabled` from `pluginsDir`.
// Missing plugins emit a stderr-style warning to `warnings` (mirroring the
// Python `_load_plugin_data` warning) and are skipped.
func LoadEnabled(pluginsDir string, enabled []string, warnings io.Writer) (map[string]*Plugin, error) {
	out := make(map[string]*Plugin, len(enabled))
	for _, id := range enabled {
		path := filepath.Join(pluginsDir, id, "plugin.toml")
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				if warnings != nil {
					fmt.Fprintf(warnings, "WARNING: Plugin '%s' not found at %s\n", id, path)
				}
				continue
			}
			return nil, fmt.Errorf("stat plugin %q: %w", id, err)
		}
		p, err := Load(path)
		if err != nil {
			return nil, fmt.Errorf("load plugin %q: %w", id, err)
		}
		out[id] = p
	}
	return out, nil
}
