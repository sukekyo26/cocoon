package hostguard

// SetTestPaths overrides the file paths used by InsideContainer for testing.
// It returns a restore function. Not safe for parallel callers; tests using
// it must not call t.Parallel().
func SetTestPaths(dockerEnv, cgroup string) func() {
	prevDocker, prevCgroup := dockerEnvPath, cgroupPath
	dockerEnvPath = dockerEnv
	cgroupPath = cgroup
	return func() {
		dockerEnvPath = prevDocker
		cgroupPath = prevCgroup
	}
}
