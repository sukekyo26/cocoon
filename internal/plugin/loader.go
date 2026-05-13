package plugin

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/sukekyo26/cocoon/internal/config"
)

// Load parses and validates a single plugin TOML file. The parameter is
// named tomlPath so it does not shadow the imported "path" package.
func Load(tomlPath string) (*Plugin, error) {
	data, err := os.ReadFile(tomlPath) //nolint:gosec // path provided by trusted caller.
	if err != nil {
		return nil, config.WrapIO(tomlPath, err) //nolint:wrapcheck // wrapper preserves the renderer-friendly format.
	}
	return parsePluginTOML(tomlPath, data)
}

// loadFromFS uses label for error wrapping so messages mirror Load.
func loadFromFS(src fs.FS, id, label string) (*Plugin, error) {
	data, err := fs.ReadFile(src, path.Join(id, "plugin.toml"))
	if err != nil {
		return nil, config.WrapIO(label, err) //nolint:wrapcheck // wrapper preserves the renderer-friendly format.
	}
	return parsePluginTOML(label, data)
}

func parsePluginTOML(label string, data []byte) (*Plugin, error) {
	var p Plugin
	if err := config.StrictUnmarshal(label, data, &p); err != nil {
		return nil, err //nolint:wrapcheck // already a *config.ValidationError.
	}
	if err := p.Validate(label); err != nil {
		return nil, err //nolint:wrapcheck // already a *config.ValidationError.
	}
	return &p, nil
}

// ErrNilPluginsFS lets callers distinguish "forgot to wire PluginsFS" from
// missing plugins via errors.Is.
var ErrNilPluginsFS = errors.New("plugin: source fs is nil")

// LoadEnabled emits one stderr warning to `warnings` per missing plugin and
// skips it.
func LoadEnabled(pluginsDir string, enabled []string, warnings io.Writer) (map[string]*Plugin, error) {
	return LoadEnabledFromFS(os.DirFS(pluginsDir), enabled, warnings, pluginsDir)
}

// LoadEnabledFromFS uses pathPrefix purely to decorate the missing-plugin
// warning (pass "" for embedded sources). Returns ErrNilPluginsFS so a nil
// src surfaces as a clear config bug rather than a fs.Stat panic.
func LoadEnabledFromFS(src fs.FS, enabled []string, warnings io.Writer, pathPrefix string) (map[string]*Plugin, error) {
	if src == nil {
		return nil, ErrNilPluginsFS
	}
	out := make(map[string]*Plugin, len(enabled))
	for _, id := range enabled {
		rel := path.Join(id, "plugin.toml")
		if _, err := fs.Stat(src, rel); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				if warnings != nil {
					where := rel
					if pathPrefix != "" {
						where = filepath.Join(pathPrefix, id, "plugin.toml")
					}
					fmt.Fprintf(warnings, "WARNING: Plugin '%s' not found at %s\n", id, where)
				}
				continue
			}
			return nil, fmt.Errorf("stat plugin %q: %w", id, err)
		}
		label := rel
		if pathPrefix != "" {
			label = filepath.Join(pathPrefix, id, "plugin.toml")
		}
		p, err := loadFromFS(src, id, label)
		if err != nil {
			return nil, fmt.Errorf("load plugin %q: %w", id, err)
		}
		out[id] = p
	}
	return out, nil
}
