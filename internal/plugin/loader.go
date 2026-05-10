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

// Load parses and validates a single plugin TOML file. The parameter
// is named tomlPath (not path) so it does not shadow the imported
// "path" package — `path.Join` calls inside this function would
// otherwise refer to the string argument and stop compiling.
func Load(tomlPath string) (*Plugin, error) {
	data, err := os.ReadFile(tomlPath) //nolint:gosec // path provided by trusted caller.
	if err != nil {
		return nil, config.WrapIO(tomlPath, err) //nolint:wrapcheck // wrapper preserves the renderer-friendly format.
	}
	return parsePluginTOML(tomlPath, data)
}

// loadFromFS reads <id>/plugin.toml out of src and validates it. The
// label argument is used purely for error wrapping so messages mirror
// the on-disk Load behaviour.
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

// ErrNilPluginsFS is returned by LoadEnabledFromFS when called with a
// nil source. Callers can identify it via errors.Is to distinguish a
// programming error (forgot to wire the LayeredFS) from a missing
// plugin on a real filesystem.
var ErrNilPluginsFS = errors.New("plugin: source fs is nil")

// LoadEnabled loads plugin.toml for every id in `enabled` from `pluginsDir`.
// Missing plugins emit a stderr-style warning to `warnings` (mirroring the
// Python `_load_plugin_data` warning) and are skipped.
func LoadEnabled(pluginsDir string, enabled []string, warnings io.Writer) (map[string]*Plugin, error) {
	return LoadEnabledFromFS(os.DirFS(pluginsDir), enabled, warnings, pluginsDir)
}

// LoadEnabledFromFS is the fs.FS-backed counterpart of LoadEnabled. The
// optional pathPrefix is only used to decorate the warning message for
// missing plugins so on-disk callers can keep their absolute-path
// diagnostic; pass "" for embedded sources.
//
// Returns ErrNilPluginsFS when src is nil. (fs.Stat / fs.ReadFile would
// otherwise panic on the nil interface, masking the actual configuration
// bug — a caller built a WorkspaceContext without wiring PluginsFS.)
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
