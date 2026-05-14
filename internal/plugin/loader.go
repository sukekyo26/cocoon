package plugin

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"

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

// validateMethodScripts enforces three invariants on every plugin:
//
//  1. [install.methods] is non-empty — every plugin declares at least one
//     install method by category name (binary / installer / apt / archive
//     per the docs/plugins.md convention; arbitrary names also accepted).
//  2. each declared method has a matching install.<name>.sh file in scriptDir.
//  3. a literal install.sh file must NOT exist. The legacy single-method
//     shape is dropped — all plugins use the same install.<name>.sh layout
//     so the loader / generator / scaffold / docs don't have to branch on
//     "declared methods" vs "implicit install.sh".
//
// The error message for a stray install.sh names the rename + plugin.toml
// edit the author needs (rather than asking them to read the docs), so a
// user-overlay plugin that pre-dates this rule gets actionable migration
// guidance the first time `cocoon gen` rejects it.
func validateMethodScripts(label string, p *Plugin, fsys fs.FS, scriptDir string) error {
	a := newAccumulator()
	if len(p.Install.Methods) == 0 {
		a.add("[install.methods] must declare at least one entry "+
			"(category convention: binary / installer / apt / archive — see docs/plugins.md). "+
			"Single-method plugins still need one entry; pick the category that matches your script.",
			"install", "methods")
	}
	// Sort method names so the accumulator's FieldError ordering is
	// stable across runs — ValidationError.Error() summarises the first
	// entry, and a map-iteration order would make the "leading" error
	// flicker between runs.
	names := make([]string, 0, len(p.Install.Methods))
	for name := range p.Install.Methods {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		scriptPath := path.Join(scriptDir, "install."+name+".sh")
		st, statErr := fs.Stat(fsys, scriptPath)
		switch {
		case errors.Is(statErr, fs.ErrNotExist):
			a.add("install."+name+".sh does not exist", "install", "methods", name)
		case statErr != nil:
			// Permission / I/O failures surface as themselves so the
			// author can fix the real cause; collapsing them into
			// "does not exist" would send them on a wild-goose chase.
			a.add(fmt.Sprintf("install.%s.sh: %v", name, statErr), "install", "methods", name)
		case st.IsDir():
			a.add("install."+name+".sh must be a regular file (got a directory)", "install", "methods", name)
		}
	}
	// A stat failure that is not fs.ErrNotExist (e.g. permission denied,
	// I/O error) must also block the plugin — otherwise a legacy
	// install.sh could slip through whenever its directory entry can't be
	// stat'd, silently rendering a Dockerfile that runs forbidden
	// content. Treat anything other than "definitively absent" as a
	// validation failure.
	switch _, err := fs.Stat(fsys, path.Join(scriptDir, "install.sh")); {
	case err == nil:
		a.add("install.sh is no longer supported; rename it to install.<category>.sh "+
			"(binary / installer / apt / archive) and declare a matching [install.methods.<category>] "+
			"entry in plugin.toml. See docs/plugins.md for the category convention.",
			"install")
	case !errors.Is(err, fs.ErrNotExist):
		a.add(fmt.Sprintf(
			"install.sh: cannot rule out a legacy file (%v); "+
				"resolve the stat failure before retrying — the loader rejects "+
				"install.sh as a plugin script, so a missed check would let it run silently",
			err), "install")
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
