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

	var verr *config.ValidationError
	require.ErrorAs(t, err, &verr, "expected *ValidationError, got %T", err)
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
// [plugins.versions].<id> accepts either a scalar constraint string or an
// inline table whose "version" key carries the constraint and whose extra
// keys (declared by a plugin via [install.extra_versions]) route into
// PluginVersionOverride.Extra. Without the table form, an Android SDK
// plugin declaring api_level / build_tools could not surface the workspace
// override into the install script.
//
// Four input shapes covered: a scalar exact pin (Extra nil), a table with
// version + extras, a table with version = "latest" + an extra, and a
// quoted-key table form ("version" = "…").
func TestLoadWorkspace_PluginVersionsExtra(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		versionLine string
		wantSpec    string
		wantPin     string
		wantExtra   map[string]string
	}{
		{
			name:        "scalar_exact_no_extra",
			versionLine: `go = "=1.23.4"`,
			wantSpec:    "=1.23.4",
			wantPin:     "1.23.4",
			wantExtra:   nil,
		},
		{
			name:        "version_plus_extras",
			versionLine: `android-sdk = { version = "=14742923", api_level = "36", build_tools = "36.0.0" }`,
			wantSpec:    "=14742923",
			wantPin:     "14742923",
			wantExtra:   map[string]string{"api_level": "36", "build_tools": "36.0.0"},
		},
		{
			name:        "latest_plus_extra",
			versionLine: `android-sdk = { version = "latest", api_level = "35" }`,
			wantSpec:    "latest",
			wantPin:     "",
			wantExtra:   map[string]string{"api_level": "35"},
		},
		{
			name:        "quoted_keys",
			versionLine: `android-sdk = { "version" = "=14742923", "api_level" = "36" }`,
			wantSpec:    "=14742923",
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
			require.Equal(t, tc.wantSpec, ov.Spec)
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
android-sdk = { version = "=14742923", api_level = 36 }
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
		// $ and backtick trigger shell parameter / command substitution
		// inside the generated Dockerfile RUN-prefix KEY="..." pair —
		// rejected for the same fail-fast reason as the runes above.
		{"dollar", `api_level = "$HOME/sdk"`},
		{"command_substitution", `api_level = "$(date)"`},
		{"backtick", "api_level = \"`whoami`\""},
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
android-sdk = { version = "=14742923", ` + tc.extraExpr + ` }
`
			tmp := t.TempDir() + "/ws.toml"
			require.NoError(t, os.WriteFile(tmp, []byte(body), 0o600))
			_, err := config.LoadWorkspace(tmp)
			require.Error(t, err)
			require.Contains(t, err.Error(), "value contains unsafe character")
		})
	}
}

// TestLoadWorkspace_PluginVersionsConstraintRejectsUnsafe pins that the
// version constraint string is bounded to the rxImageVersion tag charset: a
// ", \, space, ;, $, or backtick in the version would otherwise break out of
// the Dockerfile RUN-prefix `PIN="..."` env pair, so it must be rejected at
// decode time (well before docker build) with a *ValidationError at
// plugins.versions.<id>.
func TestLoadWorkspace_PluginVersionsConstraintRejectsUnsafe(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		spec       string
		wantUnsafe bool
	}{
		{"plain_exact", `"=14742923"`, false},
		{"latest", `"latest"`, false},
		{"double_quote", `"=1.0.0\" rm -rf / \""`, true},
		{"backslash", `"=1.0.0\\nfoo"`, true},
		{"newline", `"=1.0.0\nfoo"`, true},
		{"carriage_return", `"=1.0.0\rfoo"`, true},
		{"dollar", `"=$(date)"`, true},
		// The exact injection PoC from the evaluation report: a closing quote
		// would let the rest break out of the PIN="..." env pair.
		{"injection_poc", `"=1.0.0\"; echo PWNED > /tmp/pwn; PIN2=\""`, true},
		{"backtick", "\"=`whoami`\"", true},
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
go = ` + tc.spec + "\n"
			tmp := t.TempDir() + "/ws.toml"
			require.NoError(t, os.WriteFile(tmp, []byte(body), 0o600))
			_, err := config.LoadWorkspace(tmp)
			if !tc.wantUnsafe {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			// Assert the failure class and the exact field location, not just
			// a substring: the guard must surface as a decode-time
			// *ValidationError at plugins.versions.<id> (a regression that
			// relaxed the charset or moved the check would still otherwise
			// match the substring).
			var verr *config.ValidationError
			require.ErrorAsf(t, err, &verr, "expected *ValidationError, got %T: %v", err, err)
			var hit *config.FieldError
			for i := range verr.Errors {
				if verr.Errors[i].LocString() == "plugins.versions.go" {
					hit = &verr.Errors[i]
					break
				}
			}
			require.NotNilf(t, hit, "no FieldError at plugins.versions.go; errors = %+v", verr.Errors)
			require.Contains(t, hit.Message, "unsupported characters")
		})
	}
}
