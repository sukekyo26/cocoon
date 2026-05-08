// Package dockersock locates the Docker daemon's Unix socket on the host.
//
// Linux puts the socket at /var/run/docker.sock (system Docker) or under
// $XDG_RUNTIME_DIR (rootless Docker). macOS Docker Desktop 4.13+ defaults to
// $HOME/.docker/run/docker.sock and only exposes /var/run/docker.sock when
// the user enables the "Allow the default Docker socket to be used" toggle
// in Settings → Advanced.
//
// [CandidatePaths] returns every path we look at, in priority order;
// [First] returns the first one that actually exists as a Unix socket.
package dockersock

import (
	"fmt"
	"os"
	"path/filepath"
)

// CandidatePaths returns the candidate Docker socket paths in priority order.
// Paths that cannot be expanded (e.g. ~ when $HOME is unset) are silently
// skipped.
func CandidatePaths() []string {
	paths := []string{"/var/run/docker.sock"}

	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		paths = append(paths, filepath.Join(xdg, "docker.sock"))
	} else if uid := os.Getuid(); uid > 0 {
		paths = append(paths, fmt.Sprintf("/run/user/%d/docker.sock", uid))
	}

	if home, err := os.UserHomeDir(); err == nil && home != "" {
		paths = append(paths, filepath.Join(home, ".docker", "run", "docker.sock"))
	}

	return paths
}

// First returns the first candidate path that exists as a Unix socket file,
// or "" when none do.
func First() string {
	for _, p := range CandidatePaths() {
		if isSocket(p) {
			return p
		}
	}
	return ""
}

func isSocket(p string) bool {
	info, err := os.Stat(p)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSocket != 0
}
