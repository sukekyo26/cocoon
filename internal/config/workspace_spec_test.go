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
		{"chain", &config.WorkspaceSpec{MountRoot: "../.."}, "../.."},
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

// TestWorkspaceSpec_IsNestedMount pins the doc claim that only mount_root "."
// nests the project under <dir>/<service>; every parent mount maps flat.
func TestWorkspaceSpec_IsNestedMount(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		spec *config.WorkspaceSpec
		want bool
	}{
		{"nil section defaults to nested", nil, true},
		{"empty defaults to nested", &config.WorkspaceSpec{}, true},
		{"dot nests", &config.WorkspaceSpec{MountRoot: "."}, true},
		{"parent is flat", &config.WorkspaceSpec{MountRoot: ".."}, false},
		{"chain is flat", &config.WorkspaceSpec{MountRoot: "../.."}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.spec.IsNestedMount(); got != tc.want {
				t.Fatalf("IsNestedMount() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestWorkspaceSpec_HostMountPath pins the doc claim that the compose-relative
// source carries exactly one extra ".." over mount_root (compose resolves bind
// paths against .devcontainer/).
func TestWorkspaceSpec_HostMountPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		spec *config.WorkspaceSpec
		want string
	}{
		{"nil section", nil, ".."},
		{"empty", &config.WorkspaceSpec{}, ".."},
		{"dot", &config.WorkspaceSpec{MountRoot: "."}, ".."},
		{"parent", &config.WorkspaceSpec{MountRoot: ".."}, "../.."},
		{"grandparent", &config.WorkspaceSpec{MountRoot: "../.."}, "../../.."},
		{"three levels", &config.WorkspaceSpec{MountRoot: "../../.."}, "../../../.."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.spec.HostMountPath(); got != tc.want {
				t.Fatalf("HostMountPath() = %q, want %q", got, tc.want)
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

	for _, mountRoot := range []string{"../foo", "foo", "/abs", "./..", "...", "../"} {
		t.Run(mountRoot, func(t *testing.T) {
			t.Parallel()
			ws := &config.Workspace{
				Workspace: &config.WorkspaceSpec{MountRoot: mountRoot},
				Container: config.ContainerSpec{
					ServiceName:  "dev",
					Username:     "shogo",
					Image:        "ubuntu",
					ImageVersion: "24.04",
				},
			}
			err := ws.Validate("test.toml")
			if err == nil {
				t.Fatalf("expected validation error for mount_root = %q", mountRoot)
			}
			if !strings.Contains(err.Error(), "mount_root") {
				t.Fatalf("error did not mention mount_root: %v", err)
			}
		})
	}
}

func TestWorkspace_ValidateAcceptsValidMountRoot(t *testing.T) {
	t.Parallel()

	for _, mountRoot := range []string{"", ".", "..", "../..", "../../.."} {
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

func TestIsValidMountRoot(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want bool
	}{
		{"", true},
		{".", true},
		{"..", true},
		{"../..", true},
		{"../../..", true},
		{"../foo", false},
		{"foo", false},
		{"/abs", false},
		{"./..", false},
		{"...", false},
		{"../", false},
		{"..//..", false},
		{"../.", false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			if got := config.IsValidMountRoot(tc.in); got != tc.want {
				t.Fatalf("IsValidMountRoot(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestIsValidCodeWorkspaceName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want bool
	}{
		// Single-segment names with portable filename chars are accepted.
		{"workspace", true},
		{"my-stack", true},
		{"with.dot_under-dash", true},
		{"A1", true},
		// Empty, traversal, and path separators must all be rejected.
		{"", false},
		{".", false},
		{"..", false},
		{"a/b", false},
		{"a\\b", false},
		{"a:b", false},
		{"with space", false},
		{"~tilde", false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			if got := config.IsValidCodeWorkspaceName(tc.in); got != tc.want {
				t.Fatalf("IsValidCodeWorkspaceName(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestWorkspace_ValidateRejectsBadCodeWorkspaceName runs the [code_workspace]
// validator end-to-end through Workspace.Validate so the FieldError loc
// path ("code_workspace.name") is also exercised, not just the name regex.
func TestWorkspace_ValidateRejectsBadCodeWorkspaceName(t *testing.T) {
	t.Parallel()

	bads := []string{"a/b", "..", ".", "with space", "name:colon"}
	for _, name := range bads {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ws := &config.Workspace{
				Container: config.ContainerSpec{
					ServiceName:  "dev",
					Username:     "shogo",
					Image:        "ubuntu",
					ImageVersion: "24.04",
				},
				CodeWorkspace: &config.CodeWorkspaceSpec{Name: name},
			}
			err := ws.Validate("test.toml")
			if err == nil {
				t.Fatalf("expected validation error for name=%q", name)
			}
			if !strings.Contains(err.Error(), "code_workspace") {
				t.Fatalf("error did not mention code_workspace for %q: %v", name, err)
			}
		})
	}
}

// TestWorkspace_ValidateRejectsEmptyFolderPath pins the structural
// folders[].path == "" check. Empty paths would otherwise reach the
// generator where they fail with ErrInvalidFolderPath; catching at
// validate-time gives a cleaner FieldError trail.
func TestWorkspace_ValidateRejectsEmptyFolderPath(t *testing.T) {
	t.Parallel()

	ws := &config.Workspace{
		Container: config.ContainerSpec{
			ServiceName:  "dev",
			Username:     "shogo",
			Image:        "ubuntu",
			ImageVersion: "24.04",
		},
		CodeWorkspace: &config.CodeWorkspaceSpec{
			Folders: []config.CodeWorkspaceFolder{{Path: ""}},
		},
	}
	err := ws.Validate("test.toml")
	if err == nil {
		t.Fatal("expected validation error for empty folder path")
	}
	if !strings.Contains(err.Error(), "folders") {
		t.Fatalf("error did not mention folders: %v", err)
	}
}

// TestWorkspace_ValidateAcceptsValidCodeWorkspace ensures the full set of
// supported shapes — empty section, name only, folders only, all features
// — passes validation. Mirrors the AcceptsValid pattern for Dir / MountRoot.
func TestWorkspace_ValidateAcceptsValidCodeWorkspace(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		spec *config.CodeWorkspaceSpec
	}{
		{"omitted", nil},
		{"empty section", &config.CodeWorkspaceSpec{}},
		{"name only", &config.CodeWorkspaceSpec{Name: "my-stack"}},
		{"folders only", &config.CodeWorkspaceSpec{
			Folders: []config.CodeWorkspaceFolder{{Path: "."}, {Path: "~/.claude"}},
		}},
		{"all features", &config.CodeWorkspaceSpec{
			Name: "stack",
			Folders: []config.CodeWorkspaceFolder{
				{Path: "."},
				{Path: "~/.config/nvim", Name: "Neovim"},
			},
			Settings:   map[string]any{"editor.tabSize": int64(2)},
			Extensions: &config.CodeWorkspaceExtSpec{Recommendations: []string{"golang.go"}},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ws := &config.Workspace{
				Container: config.ContainerSpec{
					ServiceName:  "dev",
					Username:     "shogo",
					Image:        "ubuntu",
					ImageVersion: "24.04",
				},
				CodeWorkspace: tc.spec,
			}
			if err := ws.Validate("test.toml"); err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}
