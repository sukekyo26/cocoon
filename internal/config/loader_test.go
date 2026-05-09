package config_test

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/sukekyo26/cocoon/internal/config"
)

func TestLoadWorkspace_Minimal(t *testing.T) {
	t.Parallel()

	ws, err := config.LoadWorkspace(filepath.Join("testdata", "config", "workspace_minimal.toml"))
	require.NoError(t, err)
	require.Equal(t, "dev", ws.Container.ServiceName)
	require.Equal(t, "developer", ws.Container.Username)
	require.Equal(t, "ubuntu", ws.Container.Os)
	require.Equal(t, "24.04", ws.Container.OsVersion)
	require.Empty(t, ws.Plugins.Enable)
	require.Nil(t, ws.Ports)
	require.False(t, ws.HasDevcontainer())
}

func TestLoadWorkspace_Full(t *testing.T) {
	t.Parallel()

	ws, err := config.LoadWorkspace(filepath.Join("testdata", "config", "workspace_full.toml"))
	require.NoError(t, err)

	require.Equal(t, []string{"go", "uv"}, ws.Plugins.Enable)
	require.NotNil(t, ws.Ports)
	require.Len(t, ws.Ports.Forward, 3)
	require.Equal(t, "3000:3000", ws.Ports.Forward[0])
	require.Equal(t, "127.0.0.1:8080:8080/tcp", ws.Ports.Forward[1])
	long, ok := ws.Ports.Forward[2].(map[string]any)
	require.True(t, ok, "third entry should be a long-form table")
	require.Equal(t, int64(9000), long["target"])
	require.Equal(t, "127.0.0.1", long["host_ip"])
	require.Equal(t, "udp", long["protocol"])
	require.NotNil(t, ws.Apt)
	require.Equal(t, []string{"ripgrep"}, ws.Apt.Packages)
	require.Equal(t, map[string]string{"cache": "/home/developer/.cache"}, ws.Volumes)
	require.Equal(t, map[string]string{"EDITOR": "vim"}, ws.Env)
	require.Len(t, ws.Mounts, 1)
	require.True(t, ws.Mounts[0].Readonly)
	require.NotNil(t, ws.Repositories)
	require.Len(t, ws.Repositories.Clone, 2)
	require.True(t, ws.HasDevcontainer())
	require.Equal(t, "echo hi", ws.Devcontainer["postCreateCommand"])
}

func TestLoadWorkspace_UnknownFieldRejected(t *testing.T) {
	t.Parallel()

	_, err := config.LoadWorkspace(filepath.Join("testdata", "config", "workspace_unknown_field.toml"))
	require.Error(t, err)

	verr, ok := config.AsValidationError(err)
	require.True(t, ok, "expected *ValidationError, got %T", err)
	require.Len(t, verr.Errors, 1)
	require.Contains(t, verr.Errors[0].Message, "unknown")
}

func TestLoadWorkspace_FileNotFound(t *testing.T) {
	t.Parallel()

	_, err := config.LoadWorkspace(filepath.Join("testdata", "config", "does_not_exist.toml"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "does_not_exist.toml")
}

func TestBasenameFromGitURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"https with .git", "https://github.com/foo/bar.git", "bar"},
		{"https without .git", "https://github.com/foo/bar", "bar"},
		{"trailing slash", "https://github.com/foo/bar/", "bar"},
		{"ssh scp", "git@github.com:foo/bar.git", "bar"},
		{"empty", "", ""},
		{"with query", "https://github.com/foo/bar.git?ref=main", "bar"},
		{"with fragment", "https://github.com/foo/bar.git#tag", "bar"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := config.BasenameFromGitURL(tc.in)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestResolveRepoPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		path, url string
		want      string
	}{
		{"path empty falls back to basename", "", "https://github.com/foo/bar.git", "bar"},
		{"explicit path", "vendor/bar", "https://github.com/foo/bar.git", "vendor/bar"},
		{"trailing slash appends basename", "vendor/", "https://github.com/foo/bar.git", "vendor/bar"},
		{"slashes-only path", "///", "https://github.com/foo/bar.git", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := config.ResolveRepoPath(tc.path, tc.url)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	t.Parallel()

	v := &config.ValidationError{
		Path: "/tmp/x.toml",
		Errors: []config.FieldError{
			{Loc: []string{"container", "username"}, Message: "must be lowercase"},
			{Loc: []string{"plugins", "enable"}, Message: "duplicate"},
		},
	}
	require.Contains(t, v.Error(), "/tmp/x.toml")
	require.Contains(t, v.Error(), "container.username")

	sorted := v.Sort()
	require.Equal(t, "container.username", sorted.Errors[0].LocString())
	require.Equal(t, "plugins.enable", sorted.Errors[1].LocString())
}

func TestValidationError_LocStringEmpty(t *testing.T) {
	t.Parallel()

	fe := config.FieldError{Loc: nil, Message: "x"}
	require.Equal(t, "(root)", fe.LocString())
}
