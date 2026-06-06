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

// pluginsTestWorkspace wraps a [plugins] body in the minimal container
// preamble so the enable-array + [plugins.options] parsing can be exercised
// in isolation.
func pluginsTestWorkspace(pluginsBody string) string {
	return `
[container]
service_name = "dev"
username = "developer"
image = "ubuntu"
image_version = "24.04"

` + pluginsBody + "\n"
}

// TestLoadWorkspace_EnableArrayConstraints pins that a [plugins].enable entry
// carries the version constraint inline (uv/pip-style): a bare "<id>" enables
// the plugin unpinned (no Versions entry), "<id>=<version>" seeds an exact
// pin, and "<id>=latest" / "<id>=*" stay floating. Array order is the install
// order, so it is preserved into Enable.
func TestLoadWorkspace_EnableArrayConstraints(t *testing.T) {
	t.Parallel()
	body := pluginsTestWorkspace(`[plugins]
enable = ["go=1.23.4", "node=latest", "deno=*", "docker-cli"]`)
	tmp := t.TempDir() + "/ws.toml"
	require.NoError(t, os.WriteFile(tmp, []byte(body), 0o600))
	ws, err := config.LoadWorkspace(tmp)
	require.NoError(t, err)

	require.Equal(t, []string{"go", "node", "deno", "docker-cli"}, ws.Plugins.Enable)

	require.Equal(t, "=1.23.4", ws.Plugins.Versions["go"].Spec)
	require.Equal(t, "1.23.4", ws.Plugins.Versions["go"].Pin)
	require.Equal(t, "latest", ws.Plugins.Versions["node"].Spec)
	require.Equal(t, "latest", ws.Plugins.Versions["deno"].Spec)
	// A bare id leaves no Versions entry (zero override → "latest" downstream).
	_, hasDocker := ws.Plugins.Versions["docker-cli"]
	require.False(t, hasDocker)
}

// TestLoadWorkspace_EnableRejectsBadID pins the two id-rejection messages: an
// empty id (a stray leading "=") points the author at the "<id>=..." form and
// echoes the typed version, while a charset-violating id reports the pattern.
func TestLoadWorkspace_EnableRejectsBadID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, entry, want string
	}{
		{"empty_id_with_spec", `"=1.2.3"`, `write "<id>=1.2.3"`},
		{"empty_id_bare", `"="`, "has no plugin id"},
		{"bad_charset", `"Go=1.2"`, "plugin id does not match"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := pluginsTestWorkspace("[plugins]\nenable = [" + tc.entry + "]")
			tmp := t.TempDir() + "/ws.toml"
			require.NoError(t, os.WriteFile(tmp, []byte(body), 0o600))
			_, err := config.LoadWorkspace(tmp)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

// TestLoadWorkspace_OptionsExtras pins that [plugins.options].<id> folds a
// plugin's [install.extra_versions] knobs into Versions[id].Extra while the
// main version stays in the enable array. Without it, an Android SDK plugin
// declaring api_level / build_tools could not surface the workspace override.
func TestLoadWorkspace_OptionsExtras(t *testing.T) {
	t.Parallel()
	body := pluginsTestWorkspace(`[plugins]
enable = ["android-sdk=14742923"]

[plugins.options]
android-sdk = { api_level = "36", build_tools = "36.0.0" }`)
	tmp := t.TempDir() + "/ws.toml"
	require.NoError(t, os.WriteFile(tmp, []byte(body), 0o600))
	ws, err := config.LoadWorkspace(tmp)
	require.NoError(t, err)

	ov := ws.Plugins.Versions["android-sdk"]
	require.Equal(t, "=14742923", ov.Spec)
	require.Equal(t, "14742923", ov.Pin)
	require.Equal(t, map[string]string{"api_level": "36", "build_tools": "36.0.0"}, ov.Extra)
}

// TestLoadWorkspace_OptionsReservedKeys pins that the keys carrying the main
// version (version / pin) are rejected under [plugins.options] with a hint
// pointing at the enable array.
func TestLoadWorkspace_OptionsReservedKeys(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, line, want string
	}{
		{"version", `android-sdk = { version = "=14742923" }`, "version belongs in the enable array"},
		{"pin", `go = { pin = "1.23.4" }`, `"pin" key was removed`},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := pluginsTestWorkspace("[plugins]\nenable = []\n\n[plugins.options]\n" + tc.line)
			tmp := t.TempDir() + "/ws.toml"
			require.NoError(t, os.WriteFile(tmp, []byte(body), 0o600))
			_, err := config.LoadWorkspace(tmp)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

// TestLoadWorkspace_OptionsManualChecksum pins that a [plugins.options] manual
// checksum parses into the override's checksum fields (validated as 64
// lowercase hex chars). Whether the plugin may carry one is gated later by the
// generator, so the loader accepts it regardless of plugin type.
func TestLoadWorkspace_OptionsManualChecksum(t *testing.T) {
	t.Parallel()
	const hex64 = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		body := pluginsTestWorkspace(`[plugins]
enable = ["codex=0.5.0"]

[plugins.options]
codex = { checksum_amd64 = "` + hex64 + `", checksum_arm64 = "` + hex64 + `" }`)
		tmp := t.TempDir() + "/ws.toml"
		require.NoError(t, os.WriteFile(tmp, []byte(body), 0o600))
		ws, err := config.LoadWorkspace(tmp)
		require.NoError(t, err)
		ov := ws.Plugins.Versions["codex"]
		require.NotNil(t, ov.ChecksumAmd64)
		require.Equal(t, hex64, *ov.ChecksumAmd64)
		require.NotNil(t, ov.ChecksumArm64)
		require.Equal(t, hex64, *ov.ChecksumArm64)
	})
	t.Run("rejects_non_hex", func(t *testing.T) {
		t.Parallel()
		body := pluginsTestWorkspace("[plugins]\nenable = [\"codex=0.5.0\"]\n\n[plugins.options]\ncodex = { checksum_amd64 = \"abc\" }")
		tmp := t.TempDir() + "/ws.toml"
		require.NoError(t, os.WriteFile(tmp, []byte(body), 0o600))
		_, err := config.LoadWorkspace(tmp)
		require.Error(t, err)
		require.Contains(t, err.Error(), "64 lowercase hex")
	})
}

// TestLoadWorkspace_OptionsExtraNonString pins that a non-string value under
// [plugins.options].<id>.<key> is rejected as a *ValidationError rather than
// silently dropped.
func TestLoadWorkspace_OptionsExtraNonString(t *testing.T) {
	t.Parallel()
	body := pluginsTestWorkspace(`[plugins]
enable = ["android-sdk=14742923"]

[plugins.options]
android-sdk = { api_level = 36 }`)
	tmp := t.TempDir() + "/ws.toml"
	require.NoError(t, os.WriteFile(tmp, []byte(body), 0o600))
	_, err := config.LoadWorkspace(tmp)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be a string")
	// %T must report the value's real type (int64 here), not the type of a
	// pre-formatted string arg.
	require.Contains(t, err.Error(), "got int64")
}

// TestLoadWorkspace_OptionsExtraUnsafeValue covers the rune classes
// UnsafeExtraVersionRune rejects on a workspace override value. A bare ", \,
// \n, or \r would break the Dockerfile RUN-prefix `KEY="..."` env pair the
// value is later embedded into, so they have to be rejected at decode time
// (well before docker build).
func TestLoadWorkspace_OptionsExtraUnsafeValue(t *testing.T) {
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
			body := pluginsTestWorkspace(`[plugins]
enable = ["android-sdk=14742923"]

[plugins.options]
android-sdk = { ` + tc.extraExpr + ` }`)
			tmp := t.TempDir() + "/ws.toml"
			require.NoError(t, os.WriteFile(tmp, []byte(body), 0o600))
			_, err := config.LoadWorkspace(tmp)
			require.Error(t, err)
			require.Contains(t, err.Error(), "value contains unsafe character")
		})
	}
}

// TestLoadWorkspace_EnableConstraintRejectsUnsafe pins that the enable-entry
// version constraint is bounded to the rxImageVersion tag charset and rejects
// range operators: a ", \, space, $, or backtick in the version would
// otherwise break out of the Dockerfile RUN-prefix `PIN="..."` env pair, so it
// must be rejected at decode time with a *ValidationError at plugins.enable.<i>.
func TestLoadWorkspace_EnableConstraintRejectsUnsafe(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		entry      string
		wantReject bool
	}{
		{"plain_exact", `go=14742923`, false},
		{"latest", `go=latest`, false},
		{"bare_id", `go`, false},
		{"range_gte", `go=>=1.0`, true},
		{"double_quote", `go=1.0.0\" rm -rf / \"`, true},
		{"dollar", `go=$(date)`, true},
		// The injection PoC from the evaluation report: a closing quote would
		// let the rest break out of the PIN="..." env pair.
		{"injection_poc", `go=1.0.0\"; echo PWNED > /tmp/pwn; PIN2=\"`, true},
		{"backtick", "go=`whoami`", true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := pluginsTestWorkspace("[plugins]\nenable = [\"" + tc.entry + "\"]")
			tmp := t.TempDir() + "/ws.toml"
			require.NoError(t, os.WriteFile(tmp, []byte(body), 0o600))
			_, err := config.LoadWorkspace(tmp)
			if !tc.wantReject {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			// Assert the failure class and the field location: the guard must
			// surface as a decode-time *ValidationError at plugins.enable.0.
			var verr *config.ValidationError
			require.ErrorAsf(t, err, &verr, "expected *ValidationError, got %T: %v", err, err)
			var hit *config.FieldError
			for i := range verr.Errors {
				if verr.Errors[i].LocString() == "plugins.enable.0" {
					hit = &verr.Errors[i]
					break
				}
			}
			require.NotNilf(t, hit, "no FieldError at plugins.enable.0; errors = %+v", verr.Errors)
		})
	}
}

// TestLoadWorkspace_LegacyPluginVersionsRejected pins that the removed
// [plugins.versions] table (any inline form) is rejected at strict-decode time
// with the migration hint pointing at the enable array + [plugins.options].
func TestLoadWorkspace_LegacyPluginVersionsRejected(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, line string }{
		{"string_form", `go = "=1.23.4"`},
		{"pin_table", `go = { pin = "1.23.4" }`},
		{"checksum_table", `go = { version = "=1.23.4", checksum_amd64 = "abc" }`},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := pluginsTestWorkspace("[plugins]\nenable = []\n\n[plugins.versions]\n" + tc.line)
			tmp := t.TempDir() + "/ws.toml"
			require.NoError(t, os.WriteFile(tmp, []byte(body), 0o600))
			_, err := config.LoadWorkspace(tmp)
			require.Error(t, err)
			require.Contains(t, err.Error(), "[plugins.versions] was removed")
		})
	}
}
