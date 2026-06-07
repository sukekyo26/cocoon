package plugincli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

// ErrWorkspaceNotFound lets callers map "outside a cocoon project" to
// clihelpers.ErrUsage and genuine system failures to clihelpers.ErrFailure.
var ErrWorkspaceNotFound = errors.New("config file not found in tree")

// resolveLayered drops the project layer silently when no config file
// is discoverable (read-only views like list/show still work from
// embedded + user alone).
func resolveLayered() (*plugin.LayeredFS, error) {
	embedded, err := plugin.CatalogFS()
	if err != nil {
		return nil, clihelpers.FailureWrap(err, "")
	}
	userDir, err := plugin.UserPluginsDir()
	if err != nil {
		return nil, clihelpers.FailureWrap(err, "")
	}
	projectDir, projErr := projectPluginsDir()
	switch {
	case errors.Is(projErr, ErrWorkspaceNotFound):
		projectDir = ""
	case projErr != nil:
		// Surface system errors (Getwd / stat); don't pretend the layer
		// is just absent.
		return nil, clihelpers.FailureWrap(projErr, "err_pluginsrc_project_plugins_dir")
	}
	return plugin.NewLayeredFS(embedded, userDir, projectDir), nil
}

// projectPluginsDir returns ErrWorkspaceNotFound when discovery walks all
// the way up without finding the config file (caller is outside a cocoon
// project), and a wrapped error for genuine system failures.
func projectPluginsDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	wsPath, err := config.Discover(cwd)
	if err != nil {
		return "", fmt.Errorf("discover the config file: %w", err)
	}
	if wsPath == "" {
		return "", ErrWorkspaceNotFound
	}
	return plugin.ProjectPluginsDir(wsPath), nil
}

// loadPluginFromLayer returns fs.ErrNotExist (wrapped) for unknown ids.
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
