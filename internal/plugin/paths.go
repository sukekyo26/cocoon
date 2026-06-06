package plugin

import (
	"fmt"
	"os"
	"path/filepath"
)

// UserPluginsDir is the LayeredFS user-layer root (~/.cocoon/plugins).
func UserPluginsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".cocoon", "plugins"), nil
}

// ProjectPluginsDir is the LayeredFS project-layer root for the workspace
// whose workspace.toml lives at wsPath: the .cocoon/plugins directory nested
// under the workspace directory (filepath.Dir(wsPath)). It sits inside a
// WorkspaceContext's ProjectDir (the directory holding workspace.toml), not
// alongside it.
func ProjectPluginsDir(wsPath string) string {
	return filepath.Join(filepath.Dir(wsPath), ".cocoon", "plugins")
}
