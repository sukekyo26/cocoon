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
			ServiceName: "dev",
			Username:    "shogo",
			Os:          "ubuntu",
			OsVersion:   "24.04",
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
				ServiceName: "dev",
				Username:    "shogo",
				Os:          "ubuntu",
				OsVersion:   "24.04",
			},
		}
		if err := ws.Validate("test.toml"); err != nil {
			t.Fatalf("mount_root=%q rejected unexpectedly: %v", mountRoot, err)
		}
	}
}
