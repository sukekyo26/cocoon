// Package hostguard reports whether the current process appears to be
// running inside a Docker/containerd container, so host-only tooling can
// refuse to proceed.
//
// The check mirrors the bash idiom used across rebuild-container.sh,
// clean-docker.sh, and clean-volumes.sh:
//
//	[[ -f /.dockerenv ]] || grep -qsE 'docker|containerd' /proc/1/cgroup
package hostguard

import (
	"os"
	"regexp"
)

//nolint:gochecknoglobals // test seam — overridden by tests, never by callers.
var (
	dockerEnvPath = "/.dockerenv"
	cgroupPath    = "/proc/1/cgroup"
	containerRE   = regexp.MustCompile(`docker|containerd`)
)

// InsideContainer returns true when the process appears to be running
// inside a container.
func InsideContainer() bool {
	if _, err := os.Stat(dockerEnvPath); err == nil {
		return true
	}
	data, err := os.ReadFile(cgroupPath)
	if err != nil {
		return false
	}
	return containerRE.Match(data)
}
