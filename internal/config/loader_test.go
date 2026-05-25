package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sukekyo26/cocoon/internal/config"
)

func TestLoadWorkspace_Minimal(t *testing.T) {
	t.Parallel()

	ws, err := config.LoadWorkspace(filepath.Join("testdata", "config", "workspace_minimal.toml"))
	require.NoError(t, err)
	require.Equal(t, "dev", ws.Container.ServiceName)
	require.Equal(t, "developer", ws.Container.Username)
	require.Equal(t, "ubuntu", ws.Container.Image)
	require.Equal(t, "24.04", ws.Container.ImageVersion)
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

// TestLoadWorkspace_PluginVersionsExtra pins that workspace.toml's
// [plugins.versions].<id> inline-table accepts keys beyond the reserved
// triple (pin / checksum_amd64 / checksum_arm64) and routes them into
// PluginVersionOverride.Extra. Without this, an Android SDK plugin
// declaring api_level / build_tools under [install.extra_versions]
// could not surface the workspace override into the install script.
//
// Four input shapes covered: pin only (Extra nil), pin + extras, pin +
// checksum + extras, and a quoted bare-key form ("api_level" = "36").
func TestLoadWorkspace_PluginVersionsExtra(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		versionLine string
		wantPin     string
		wantExtra   map[string]string
	}{
		{
			name:        "pin_only_no_extra",
			versionLine: `go = { pin = "1.23.4" }`,
			wantPin:     "1.23.4",
			wantExtra:   nil,
		},
		{
			name:        "pin_plus_extras",
			versionLine: `android-sdk = { pin = "14742923", api_level = "36", build_tools = "36.0.0" }`,
			wantPin:     "14742923",
			wantExtra:   map[string]string{"api_level": "36", "build_tools": "36.0.0"},
		},
		{
			name:        "pin_with_checksum_and_extra",
			versionLine: `android-sdk = { pin = "14742923", checksum_amd64 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", api_level = "35" }`,
			wantPin:     "14742923",
			wantExtra:   map[string]string{"api_level": "35"},
		},
		{
			name:        "quoted_keys",
			versionLine: `android-sdk = { "pin" = "14742923", "api_level" = "36" }`,
			wantPin:     "14742923",
			wantExtra:   map[string]string{"api_level": "36"},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := `
[container]
service_name = "dev"
username = "developer"
image = "ubuntu"
image_version = "24.04"

[plugins]
enable = []

[plugins.versions]
` + tc.versionLine + "\n"
			tmp := t.TempDir() + "/ws.toml"
			require.NoError(t, os.WriteFile(tmp, []byte(body), 0o600))
			ws, err := config.LoadWorkspace(tmp)
			require.NoError(t, err)
			require.NotNil(t, ws.Plugins.Versions)
			var id string
			for k := range ws.Plugins.Versions {
				id = k
				break
			}
			ov := ws.Plugins.Versions[id]
			require.Equal(t, tc.wantPin, ov.Pin)
			require.Equal(t, tc.wantExtra, ov.Extra)
		})
	}
}

// TestLoadWorkspace_PluginVersionsExtraNonString pins that a non-string
// value under [plugins.versions].<id>.<key> is rejected as a
// *ValidationError rather than silently dropped.
func TestLoadWorkspace_PluginVersionsExtraNonString(t *testing.T) {
	t.Parallel()
	body := `
[container]
service_name = "dev"
username = "developer"
image = "ubuntu"
image_version = "24.04"

[plugins]
enable = []

[plugins.versions]
android-sdk = { pin = "14742923", api_level = 36 }
`
	tmp := t.TempDir() + "/ws.toml"
	require.NoError(t, os.WriteFile(tmp, []byte(body), 0o600))
	_, err := config.LoadWorkspace(tmp)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be a string")
}

// TestLoadWorkspace_PluginVersionsExtraUnsafeValue covers the rune
// classes UnsafeExtraVersionRune rejects on a workspace override value.
// A bare ", \, \n, or \r would break the Dockerfile RUN-prefix
// `KEY="..."` env pair the value is later embedded into, so they have
// to be rejected at decode time (well before docker build).
func TestLoadWorkspace_PluginVersionsExtraUnsafeValue(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		extraExpr string
	}{
		// TOML basic strings forbid raw control chars but accept escapes;
		// \" / \\ / \n / \r all decode to the literal rune the validator rejects.
		{"double_quote", `api_level = "36\" rm -rf / \""`},
		{"backslash", `api_level = "36\\nfoo"`},
		{"newline", `api_level = "36\nfoo"`},
		{"carriage_return", `api_level = "36\rfoo"`},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := `
[container]
service_name = "dev"
username = "developer"
image = "ubuntu"
image_version = "24.04"

[plugins]
enable = []

[plugins.versions]
android-sdk = { pin = "14742923", ` + tc.extraExpr + ` }
`
			tmp := t.TempDir() + "/ws.toml"
			require.NoError(t, os.WriteFile(tmp, []byte(body), 0o600))
			_, err := config.LoadWorkspace(tmp)
			require.Error(t, err)
			require.Contains(t, err.Error(), "value contains unsafe character")
		})
	}
}
