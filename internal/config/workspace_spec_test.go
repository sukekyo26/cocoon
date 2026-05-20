package config_test

import (
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/config"
)

func TestWorkspaceSpec_MountRootOrDefault(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		spec *config.WorkspaceSpec
		want string
	}{
		{"nil section", nil, "."},
		{"empty mount_root", &config.WorkspaceSpec{}, "."},
		{"explicit dot", &config.WorkspaceSpec{MountRoot: "."}, "."},
		{"explicit dotdot", &config.WorkspaceSpec{MountRoot: ".."}, ".."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.spec.MountRootOrDefault(); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWorkspaceSpec_DevContainerOrDefault(t *testing.T) {
	t.Parallel()

	yes := true
	no := false
	cases := []struct {
		name string
		spec *config.WorkspaceSpec
		want bool
	}{
		{"nil section", nil, true},
		{"omitted field", &config.WorkspaceSpec{}, true},
		{"explicit true", &config.WorkspaceSpec{DevContainer: &yes}, true},
		{"explicit false", &config.WorkspaceSpec{DevContainer: &no}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.spec.DevContainerOrDefault(); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestWorkspace_ValidateRejectsBadMountRoot(t *testing.T) {
	t.Parallel()

	ws := &config.Workspace{
		Workspace: &config.WorkspaceSpec{MountRoot: "../.."},
		Container: config.ContainerSpec{
			ServiceName:  "dev",
			Username:     "shogo",
			Image:        "ubuntu",
			ImageVersion: "24.04",
		},
	}
	err := ws.Validate("test.toml")
	if err == nil {
		t.Fatal("expected validation error for mount_root = '../..'")
	}
	if !strings.Contains(err.Error(), "mount_root") {
		t.Fatalf("error did not mention mount_root: %v", err)
	}
}

func TestWorkspace_ValidateAcceptsValidMountRoot(t *testing.T) {
	t.Parallel()

	for _, mountRoot := range []string{"", ".", ".."} {
		ws := &config.Workspace{
			Workspace: &config.WorkspaceSpec{MountRoot: mountRoot},
			Container: config.ContainerSpec{
				ServiceName:  "dev",
				Username:     "shogo",
				Image:        "ubuntu",
				ImageVersion: "24.04",
			},
		}
		if err := ws.Validate("test.toml"); err != nil {
			t.Fatalf("mount_root=%q rejected unexpectedly: %v", mountRoot, err)
		}
	}
}

func TestWorkspaceSpec_DirOrDefault(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		spec *config.WorkspaceSpec
		want string
	}{
		{"nil section", nil, "workspace"},
		{"empty dir", &config.WorkspaceSpec{}, "workspace"},
		{"explicit single", &config.WorkspaceSpec{Dir: "myapp"}, "myapp"},
		{"explicit nested", &config.WorkspaceSpec{Dir: "work/myapp"}, "work/myapp"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.spec.DirOrDefault(); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWorkspace_ValidateAcceptsValidDir(t *testing.T) {
	t.Parallel()

	for _, dir := range []string{"", "workspace", "myapp", "work/myapp", "a/b/c", "with-dash_and.dot"} {
		ws := &config.Workspace{
			Workspace: &config.WorkspaceSpec{Dir: dir},
			Container: config.ContainerSpec{
				ServiceName:  "dev",
				Username:     "shogo",
				Image:        "ubuntu",
				ImageVersion: "24.04",
			},
		}
		if err := ws.Validate("test.toml"); err != nil {
			t.Fatalf("dir=%q rejected unexpectedly: %v", dir, err)
		}
	}
}

func TestWorkspace_ValidateRejectsBadDir(t *testing.T) {
	t.Parallel()

	bads := []string{
		"/abs",    // leading slash
		"trail/",  // trailing slash
		"a//b",    // empty segment
		"a/../b",  // .. segment
		"a/./b",   // . segment
		"..",      // bare ..
		".",       // bare .
		"a b",     // whitespace
		"~",       // tilde
		"foo$bar", // shell-special
		"foo;rm",  // shell metachar
		"日本語",     // multibyte
	}
	for _, dir := range bads {
		t.Run(dir, func(t *testing.T) {
			t.Parallel()
			ws := &config.Workspace{
				Workspace: &config.WorkspaceSpec{Dir: dir},
				Container: config.ContainerSpec{
					ServiceName:  "dev",
					Username:     "shogo",
					Image:        "ubuntu",
					ImageVersion: "24.04",
				},
			}
			err := ws.Validate("test.toml")
			if err == nil {
				t.Fatalf("expected validation error for dir=%q", dir)
			}
			if !strings.Contains(err.Error(), "dir") {
				t.Fatalf("error did not mention dir for %q: %v", dir, err)
			}
		})
	}
}

func TestIsValidWorkspaceDir(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want bool
	}{
		{"workspace", true},
		{"work/myproject", true},
		{"a", true},
		{"with-dash_and.dot", true},
		{"", false},
		{"/abs", false},
		{"trail/", false},
		{"a//b", false},
		{"a/../b", false},
		{"..", false},
		{".", false},
		{"a b", false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			if got := config.IsValidWorkspaceDir(tc.in); got != tc.want {
				t.Fatalf("IsValidWorkspaceDir(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
