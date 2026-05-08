package hostguard_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sukekyo26/cocoon/internal/hostguard"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

//nolint:paralleltest // tests mutate package-level path globals via SetTestPaths.
func TestInsideContainer(t *testing.T) {
	cases := []struct {
		name      string
		dockerEnv bool
		cgroup    string
		want      bool
	}{
		{name: "no_markers", dockerEnv: false, cgroup: "1:cpu:/", want: false},
		{name: "dockerenv_present", dockerEnv: true, cgroup: "1:cpu:/", want: true},
		{name: "cgroup_without_marker", dockerEnv: false, cgroup: "0::/init.scope", want: false},
		{name: "cgroup_with_docker", dockerEnv: false, cgroup: "0::/docker/abc", want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { //nolint:paralleltest // see parent
			dir := t.TempDir()
			cgroup := filepath.Join(dir, "cgroup")
			writeFile(t, cgroup, tc.cgroup)
			docker := filepath.Join(dir, "absent")
			if tc.dockerEnv {
				docker = filepath.Join(dir, "dockerenv")
				writeFile(t, docker, "")
			}
			restore := hostguard.SetTestPaths(docker, cgroup)
			defer restore()
			if got := hostguard.InsideContainer(); got != tc.want {
				t.Fatalf("InsideContainer() = %v, want %v", got, tc.want)
			}
		})
	}
}
