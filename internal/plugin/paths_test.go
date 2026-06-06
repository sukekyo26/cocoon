package plugin_test

import (
	"path/filepath"
	"testing"

	"github.com/sukekyo26/cocoon/internal/plugin"
)

// TestProjectPluginsDir pins the contract: ProjectPluginsDir resolves to the
// <dir>/.cocoon/plugins sibling of the workspace.toml at wsPath.
func TestProjectPluginsDir(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		wsPath string
		want   string
	}{
		{"nested", "/a/b/workspace.toml", filepath.FromSlash("/a/b/.cocoon/plugins")},
		{"root_level", "/proj/workspace.toml", filepath.FromSlash("/proj/.cocoon/plugins")},
		{"relative", filepath.FromSlash("rel/workspace.toml"), filepath.FromSlash("rel/.cocoon/plugins")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := plugin.ProjectPluginsDir(tc.wsPath); got != tc.want {
				t.Errorf("ProjectPluginsDir(%q) = %q, want %q", tc.wsPath, got, tc.want)
			}
		})
	}
}

// TestUserPluginsDir pins the contract: UserPluginsDir returns the absolute
// ~/.cocoon/plugins root under the resolved home directory.
//
//nolint:paralleltest // t.Setenv pins HOME process-wide; cannot run in parallel.
func TestUserPluginsDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	got, err := plugin.UserPluginsDir()
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	want := filepath.Join(home, ".cocoon", "plugins")
	if got != want {
		t.Errorf("UserPluginsDir() = %q, want %q", got, want)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("UserPluginsDir() = %q, want absolute path", got)
	}
}
