package generate

import "os"

// ContainerSentinelPath is the file used to detect whether cocoon is
// running inside a container. Exported as a variable so tests can swap it
// for a tempfile without racing on the real /.dockerenv.
var ContainerSentinelPath = "/.dockerenv"

// InContainer reports whether cocoon appears to be running inside a
// container by checking for the existence of ContainerSentinelPath.
func InContainer() bool {
	_, err := os.Stat(ContainerSentinelPath)
	return err == nil
}
