package plugincli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

// ErrWorkspaceNotFound is returned by projectPluginsDir when no workspace.toml
// is reachable from cwd (the user is outside a cocoon project). System
// failures during discovery (e.g. os.Getwd, filesystem stat errors) return a
// wrapped error instead, so callers can map "not found" to ErrUsage and other
// failures to ErrFailure.
var ErrWorkspaceNotFound = errors.New("workspace.toml not found in tree")

// resolveLayered builds the layered plugin FS from the embedded catalog plus
// the project (.cocoon/plugins under the discovered workspace.toml) and user
// (~/.cocoon/plugins) overlays. The project layer is best-effort: if no
// workspace.toml is discoverable, just the embedded + user view is returned.
func resolveLayered() (*plugin.LayeredFS, error) {
	embedded, err := plugin.CatalogFS()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailure, err)
	}
	userDir, err := userPluginsDir()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailure, err)
	}
	projectDir, projErr := projectPluginsDir()
	if projErr != nil {
		// Discovery failure (no workspace.toml in tree) is expected outside
		// a cocoon project and must not fail plugin commands; drop the layer
		// silently, the user/embedded view stays valid.
		projectDir = ""
	}
	return plugin.NewLayeredFS(embedded, userDir, projectDir), nil
}

// userPluginsDir is the user-scope LayeredFS layer root: ~/.cocoon/plugins.
func userPluginsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".cocoon", "plugins"), nil
}

// projectPluginsDir locates the project-scope layer root by discovering
// workspace.toml from cwd and joining .cocoon/plugins next to it.
//
// Error semantics:
//   - (path, nil)                   on success
//   - ("", ErrWorkspaceNotFound)   when discovery walks all the way up
//     without finding workspace.toml (= caller is outside a cocoon project)
//   - ("", wrapped err)             on filesystem-level failures (Getwd or
//     Discover errored)
//
// Callers map ErrWorkspaceNotFound to ErrUsage with an actionable message,
// and other errors to ErrFailure with the underlying context preserved.
func projectPluginsDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	wsPath, err := config.Discover(cwd)
	if err != nil {
		return "", fmt.Errorf("discover workspace.toml: %w", err)
	}
	if wsPath == "" {
		return "", ErrWorkspaceNotFound
	}
	return filepath.Join(filepath.Dir(wsPath), ".cocoon", "plugins"), nil
}

// scopeDir returns the absolute directory where `cocoon plugin add <id>`
// places the copy for the requested scope. A missing workspace.toml when
// --scope project is requested is a usage error; other discovery failures
// (Getwd, filesystem stat) bubble up as ErrFailure with context preserved.
func scopeDir(scope string) (string, error) {
	switch scope {
	case plugin.SourceUser:
		return userPluginsDir()
	case plugin.SourceProject:
		dir, err := projectPluginsDir()
		switch {
		case errors.Is(err, ErrWorkspaceNotFound):
			return "", fmt.Errorf("%w: --scope project requires a discoverable workspace.toml", ErrUsage)
		case err != nil:
			return "", fmt.Errorf("%w: %w", ErrFailure, err)
		}
		return dir, nil
	default:
		return "", fmt.Errorf("%w: --scope must be %q or %q", ErrUsage, plugin.SourceUser, plugin.SourceProject)
	}
}

// loadPluginFromLayer reads <id>/plugin.toml from layered for parsing /
// inspection. Returns fs.ErrNotExist (wrapped) when the id is unknown.
func loadPluginFromLayer(layered fs.FS, id string) (*plugin.Plugin, error) {
	body, err := fs.ReadFile(layered, id+"/plugin.toml")
	if err != nil {
		return nil, fmt.Errorf("read %s/plugin.toml: %w", id, err)
	}
	var p plugin.Plugin
	if uerr := config.StrictUnmarshal(id+"/plugin.toml", body, &p); uerr != nil {
		return nil, fmt.Errorf("parse %s/plugin.toml: %w", id, uerr)
	}
	return &p, nil
}
