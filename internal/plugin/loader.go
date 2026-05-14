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
	p, err := parsePluginTOML(tomlPath, data)
	if err != nil {
		return nil, err
	}
	if err := validateMethodScripts(tomlPath, p, os.DirFS(filepath.Dir(tomlPath)), "."); err != nil {
		return nil, err //nolint:wrapcheck // already a *config.ValidationError.
	}
	return p, nil
}

// loadFromFS uses label for error wrapping so messages mirror Load.
func loadFromFS(src fs.FS, id, label string) (*Plugin, error) {
	data, err := fs.ReadFile(src, path.Join(id, "plugin.toml"))
	if err != nil {
		return nil, config.WrapIO(label, err) //nolint:wrapcheck // wrapper preserves the renderer-friendly format.
	}
	p, err := parsePluginTOML(label, data)
	if err != nil {
		return nil, err
	}
	if err := validateMethodScripts(label, p, src, id); err != nil {
		return nil, err //nolint:wrapcheck // already a *config.ValidationError.
	}
	return p, nil
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

// validateMethodScripts enforces that every declared install.methods.<name>
// entry has a matching install.<name>.sh file in scriptDir, and that the
// legacy install.sh is absent when methods are declared (exclusivity).
// Returns nil when [install.methods] is empty so legacy plugins are
// unaffected.
func validateMethodScripts(label string, p *Plugin, fsys fs.FS, scriptDir string) error {
	if len(p.Install.Methods) == 0 {
		return nil
	}
	a := newAccumulator()
	for name := range p.Install.Methods {
		scriptPath := path.Join(scriptDir, "install."+name+".sh")
		if _, err := fs.Stat(fsys, scriptPath); err != nil {
			a.add("install."+name+".sh does not exist", "install", "methods", name)
		}
	}
	if _, err := fs.Stat(fsys, path.Join(scriptDir, "install.sh")); err == nil {
		a.add("install.sh must not exist when [install.methods] is declared; use install.<name>.sh instead", "install")
	}
	if len(*a.errs) == 0 {
		return nil
	}
	return &config.ValidationError{Path: label, Errors: *a.errs}
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
