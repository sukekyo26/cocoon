package plugin_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sukekyo26/cocoon/internal/plugin"
)

func TestValidate_DuplicateVolumes(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir() + "/plugin.toml"
	body := `
[metadata]
name = "x"
description = "y"
url = "https://example.com/x"
default = false
[install]
requires_root = false
volumes = ["/home/${USERNAME}/a", "/home/${USERNAME}/a"]
[version]
version_capable = false
`
	require.NoError(t, os.WriteFile(tmp, []byte(body), 0o600))
	_, err := plugin.Load(tmp)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate")
}

func TestValidate_DuplicateBuildArgs(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir() + "/plugin.toml"
	body := `
[metadata]
name = "x"
description = "y"
url = "https://example.com/x"
default = false
[install]
requires_root = false
build_args = ["FOO", "FOO"]
[version]
version_capable = false
`
	require.NoError(t, os.WriteFile(tmp, []byte(body), 0o600))
	_, err := plugin.Load(tmp)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate")
}

func TestValidate_InstallEnvKey(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir() + "/plugin.toml"
	body := `
[metadata]
name = "x"
description = "y"
url = "https://example.com/x"
default = false
[install]
requires_root = false
env = { "1BAD" = "v" }
[version]
version_capable = false
`
	require.NoError(t, os.WriteFile(tmp, []byte(body), 0o600))
	_, err := plugin.Load(tmp)
	require.Error(t, err)
}

func TestValidate_URLRequired(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir() + "/plugin.toml"
	body := `
[metadata]
name = "x"
description = "y"
default = false
[install]
requires_root = false
[version]
version_capable = false
`
	require.NoError(t, os.WriteFile(tmp, []byte(body), 0o600))
	_, err := plugin.Load(tmp)
	require.Error(t, err)
	require.Contains(t, err.Error(), "url must not be empty")
}

func TestValidate_URLBadShape(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		url  string
	}{
		{"http_not_https", "http://example.com"},
		{"contains_space", "https://example.com /a"},
		{"contains_tab", "https://example.com\t/a"},
		{"scheme_relative", "//example.com"},
		{"bare_word", "example.com"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmp := t.TempDir() + "/plugin.toml"
			body := `
[metadata]
name = "x"
description = "y"
url = "` + tc.url + `"
default = false
[install]
requires_root = false
[version]
version_capable = false
`
			require.NoError(t, os.WriteFile(tmp, []byte(body), 0o600))
			_, err := plugin.Load(tmp)
			require.Error(t, err)
			require.Contains(t, err.Error(), "url must start with https://")
		})
	}
}

func TestValidate_DuplicateConflicts(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir() + "/plugin.toml"
	body := `
[metadata]
name = "x"
description = "y"
url = "https://example.com/x"
default = false
conflicts = ["a", "a"]
[install]
requires_root = false
[version]
version_capable = false
`
	require.NoError(t, os.WriteFile(tmp, []byte(body), 0o600))
	_, err := plugin.Load(tmp)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate")
}

func validMetadata() plugin.Metadata {
	return plugin.Metadata{Name: "x", Description: "y", URL: "https://example.com/x"}
}

// TestValidate_MethodsAccepted pins the contract that a plugin declaring
// multiple [install.methods.<name>] entries with a matching default_method
// passes validation. Asserts the optional-methods path is a no-op for
// well-formed input.
func TestValidate_MethodsAccepted(t *testing.T) {
	t.Parallel()
	p := &plugin.Plugin{
		Metadata: validMetadata(),
		Install: plugin.Install{
			DefaultMethod: "official",
			Methods: map[string]plugin.InstallMethod{
				"official": {Description: "Official installer"},
				"binary":   {Description: "Direct binary"},
			},
		},
	}
	require.NoError(t, p.Validate("test/plugin.toml"))
}

// TestValidate_MethodsBadDefault pins that default_method must reference
// an existing method.
func TestValidate_MethodsBadDefault(t *testing.T) {
	t.Parallel()
	p := &plugin.Plugin{
		Metadata: validMetadata(),
		Install: plugin.Install{
			DefaultMethod: "missing",
			Methods: map[string]plugin.InstallMethod{
				"official": {Description: "x"},
			},
		},
	}
	err := p.Validate("test/plugin.toml")
	require.Error(t, err)
	require.Contains(t, err.Error(), `default_method "missing" is not declared`)
}

// TestValidate_MethodsNoDefault pins that declaring [install.methods]
// without default_method is an error.
func TestValidate_MethodsNoDefault(t *testing.T) {
	t.Parallel()
	p := &plugin.Plugin{
		Metadata: validMetadata(),
		Install: plugin.Install{
			Methods: map[string]plugin.InstallMethod{
				"official": {Description: "x"},
			},
		},
	}
	err := p.Validate("test/plugin.toml")
	require.Error(t, err)
	require.Contains(t, err.Error(), "default_method must be set")
}

// TestValidate_MethodsBadName covers method-name regex rejection across
// input categories (uppercase / digit-prefix / punctuation / whitespace).
func TestValidate_MethodsBadName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		method string
	}{
		{"uppercase", "Official"},
		{"starts_with_digit", "1method"},
		{"contains_dot", "a.b"},
		{"contains_slash", "a/b"},
		{"contains_space", "a b"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := &plugin.Plugin{
				Metadata: validMetadata(),
				Install: plugin.Install{
					DefaultMethod: tc.method,
					Methods: map[string]plugin.InstallMethod{
						tc.method: {Description: "x"},
					},
				},
			}
			err := p.Validate("test/plugin.toml")
			require.Error(t, err)
			require.Contains(t, err.Error(), "method name does not match")
		})
	}
}

// TestValidate_MethodsEmptyName isolates the empty-key case: an
// unnamed method coexists with a valid default, so only the regex
// failure is reported (default_method points to a valid name).
func TestValidate_MethodsEmptyName(t *testing.T) {
	t.Parallel()
	p := &plugin.Plugin{
		Metadata: validMetadata(),
		Install: plugin.Install{
			DefaultMethod: "official",
			Methods: map[string]plugin.InstallMethod{
				"":         {Description: "anonymous"},
				"official": {Description: "Official"},
			},
		},
	}
	err := p.Validate("test/plugin.toml")
	require.Error(t, err)
	require.Contains(t, err.Error(), "method name does not match")
}

// TestValidate_MethodsEmptyDescription pins that every declared method
// requires a non-empty description.
func TestValidate_MethodsEmptyDescription(t *testing.T) {
	t.Parallel()
	p := &plugin.Plugin{
		Metadata: validMetadata(),
		Install: plugin.Install{
			DefaultMethod: "official",
			Methods: map[string]plugin.InstallMethod{
				"official": {Description: ""},
			},
		},
	}
	err := p.Validate("test/plugin.toml")
	require.Error(t, err)
	require.Contains(t, err.Error(), "description must not be empty")
}

// TestValidate_DefaultMethodWithoutMethods pins that default_method
// cannot be set when [install.methods] is absent.
func TestValidate_DefaultMethodWithoutMethods(t *testing.T) {
	t.Parallel()
	p := &plugin.Plugin{
		Metadata: validMetadata(),
		Install: plugin.Install{
			DefaultMethod: "official",
		},
	}
	err := p.Validate("test/plugin.toml")
	require.Error(t, err)
	require.Contains(t, err.Error(), "default_method requires at least one")
}

// TestValidate_MethodsEmptyMapIgnored pins backward compatibility: a
// plugin with no [install.methods] section validates exactly as before.
func TestValidate_MethodsEmptyMapIgnored(t *testing.T) {
	t.Parallel()
	p := &plugin.Plugin{
		Metadata: validMetadata(),
		Install:  plugin.Install{RequiresRoot: false},
	}
	require.NoError(t, p.Validate("test/plugin.toml"))
}

// TestValidate_VerifyAccepted pins that the recognised [version].verify
// values pass validation, including the omitted (empty) default.
func TestValidate_VerifyAccepted(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		verify string
	}{
		{"omitted", ""},
		{"checksum", "checksum"},
		{"pgp", "pgp"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := &plugin.Plugin{
				Metadata: validMetadata(),
				Version:  plugin.Version{VersionCapable: true, Verify: tc.verify},
			}
			require.NoError(t, p.Validate("test/plugin.toml"))
		})
	}
}

// TestValidate_VerifyBadValue covers rejection of unrecognised
// [version].verify values across input categories (wrong algorithm /
// wrong case / near-miss spelling).
func TestValidate_VerifyBadValue(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		verify string
	}{
		{"wrong_algorithm", "sha256"},
		{"uppercase", "PGP"},
		{"near_miss", "gpg"},
		{"arbitrary", "none"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := &plugin.Plugin{
				Metadata: validMetadata(),
				Version:  plugin.Version{VersionCapable: true, Verify: tc.verify},
			}
			err := p.Validate("test/plugin.toml")
			require.Error(t, err)
			require.Contains(t, err.Error(), "is not one of")
		})
	}
}

// TestValidate_VerifyWithoutVersionCapable pins that a verify value is
// rejected unless version_capable = true, so a misleading declaration
// cannot sit silently in plugin.toml.
func TestValidate_VerifyWithoutVersionCapable(t *testing.T) {
	t.Parallel()
	p := &plugin.Plugin{
		Metadata: validMetadata(),
		Version:  plugin.Version{VersionCapable: false, Verify: "pgp"},
	}
	err := p.Validate("test/plugin.toml")
	require.Error(t, err)
	require.Contains(t, err.Error(), "verify requires version_capable = true")
}

// TestVersion_VerifiesByChecksum pins that the omitted and "checksum"
// values mean checksum verification, and "pgp" does not.
func TestVersion_VerifiesByChecksum(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		verify string
		want   bool
	}{
		{"omitted", "", true},
		{"checksum", "checksum", true},
		{"pgp", "pgp", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			v := plugin.Version{VersionCapable: true, Verify: tc.verify}
			require.Equal(t, tc.want, v.VerifiesByChecksum())
		})
	}
}

// TestValidate_ExtraVersionsAccepted pins that a well-formed
// [install.extra_versions] block passes validation: two declarations
// with distinct lowercase keys, uppercase env names, non-empty defaults.
func TestValidate_ExtraVersionsAccepted(t *testing.T) {
	t.Parallel()
	p := &plugin.Plugin{
		Metadata: validMetadata(),
		Install: plugin.Install{
			DefaultMethod: "archive",
			Methods:       map[string]plugin.InstallMethod{"archive": {Description: "x"}},
			ExtraVersions: map[string]plugin.ExtraVersionSpec{
				"api_level":   {Env: "ANDROID_SDK_API_LEVEL", Default: "35"},
				"build_tools": {Env: "ANDROID_SDK_BUILD_TOOLS", Default: "35.0.0"},
			},
		},
	}
	require.NoError(t, p.Validate("test/plugin.toml"))
}

// TestValidate_ExtraVersionsRejects sweeps the rejection cases for
// [install.extra_versions]: bad key shape, bad env shape, empty env,
// reserved env collision, build_args collision, and duplicate env
// across two declarations.
func TestValidate_ExtraVersionsRejects(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		buildArgs   []string
		extras      map[string]plugin.ExtraVersionSpec
		wantSnippet string
	}{
		{
			name: "key_uppercase",
			extras: map[string]plugin.ExtraVersionSpec{
				"API_LEVEL": {Env: "ANDROID_SDK_API_LEVEL", Default: "35"},
			},
			wantSnippet: "extra_versions key does not match",
		},
		{
			name: "key_starts_with_digit",
			extras: map[string]plugin.ExtraVersionSpec{
				"1level": {Env: "ANDROID_SDK_API_LEVEL", Default: "35"},
			},
			wantSnippet: "extra_versions key does not match",
		},
		{
			name: "env_empty",
			extras: map[string]plugin.ExtraVersionSpec{
				"api_level": {Env: "", Default: "35"},
			},
			wantSnippet: "env must not be empty",
		},
		{
			name: "env_lowercase",
			extras: map[string]plugin.ExtraVersionSpec{
				"api_level": {Env: "android_sdk_api_level", Default: "35"},
			},
			wantSnippet: "env does not match",
		},
		{
			name: "env_reserved_pin",
			extras: map[string]plugin.ExtraVersionSpec{
				"api_level": {Env: "PIN", Default: "35"},
			},
			wantSnippet: "collides with a cocoon-reserved variable",
		},
		{
			name: "env_reserved_checksum_amd64",
			extras: map[string]plugin.ExtraVersionSpec{
				"api_level": {Env: "CHECKSUM_AMD64", Default: "35"},
			},
			wantSnippet: "collides with a cocoon-reserved variable",
		},
		{
			name:      "env_collides_with_build_args",
			buildArgs: []string{"ANDROID_SDK_API_LEVEL"},
			extras: map[string]plugin.ExtraVersionSpec{
				"api_level": {Env: "ANDROID_SDK_API_LEVEL", Default: "35"},
			},
			wantSnippet: "collides with an install.build_args entry",
		},
		{
			name: "env_duplicate_across_extras",
			extras: map[string]plugin.ExtraVersionSpec{
				"api_level":   {Env: "ANDROID_X", Default: "35"},
				"build_tools": {Env: "ANDROID_X", Default: "35.0.0"},
			},
			wantSnippet: "is also used by extra_versions.",
		},
		{
			name: "default_empty",
			extras: map[string]plugin.ExtraVersionSpec{
				"api_level": {Env: "ANDROID_SDK_API_LEVEL", Default: ""},
			},
			wantSnippet: "default must not be empty",
		},
		{
			name: "key_reserved_pin",
			extras: map[string]plugin.ExtraVersionSpec{
				"pin": {Env: "ANDROID_PIN", Default: "35"},
			},
			wantSnippet: `extra_versions key "pin" is reserved`,
		},
		{
			name: "key_reserved_checksum_amd64",
			extras: map[string]plugin.ExtraVersionSpec{
				"checksum_amd64": {Env: "ANDROID_CSUM_AMD", Default: "35"},
			},
			wantSnippet: `"checksum_amd64" is reserved`,
		},
		{
			name: "key_reserved_checksum_arm64",
			extras: map[string]plugin.ExtraVersionSpec{
				"checksum_arm64": {Env: "ANDROID_CSUM_ARM", Default: "35"},
			},
			wantSnippet: `"checksum_arm64" is reserved`,
		},
		{
			name: "default_contains_double_quote",
			extras: map[string]plugin.ExtraVersionSpec{
				"api_level": {Env: "ANDROID_SDK_API_LEVEL", Default: `36" rm -rf / "`},
			},
			wantSnippet: "default contains unsafe character",
		},
		{
			name: "default_contains_backslash",
			extras: map[string]plugin.ExtraVersionSpec{
				"api_level": {Env: "ANDROID_SDK_API_LEVEL", Default: `36\nfoo`},
			},
			wantSnippet: "default contains unsafe character",
		},
		{
			name: "default_contains_newline",
			extras: map[string]plugin.ExtraVersionSpec{
				"api_level": {Env: "ANDROID_SDK_API_LEVEL", Default: "36\nfoo"},
			},
			wantSnippet: "default contains unsafe character",
		},
		{
			name: "default_contains_cr",
			extras: map[string]plugin.ExtraVersionSpec{
				"api_level": {Env: "ANDROID_SDK_API_LEVEL", Default: "36\rfoo"},
			},
			wantSnippet: "default contains unsafe character",
		},
		{
			name: "default_contains_dollar",
			extras: map[string]plugin.ExtraVersionSpec{
				"api_level": {Env: "ANDROID_SDK_API_LEVEL", Default: "$HOME/sdk"},
			},
			wantSnippet: "default contains unsafe character",
		},
		{
			name: "default_contains_command_substitution",
			extras: map[string]plugin.ExtraVersionSpec{
				"api_level": {Env: "ANDROID_SDK_API_LEVEL", Default: "$(date)"},
			},
			wantSnippet: "default contains unsafe character",
		},
		{
			name: "default_contains_backtick",
			extras: map[string]plugin.ExtraVersionSpec{
				"api_level": {Env: "ANDROID_SDK_API_LEVEL", Default: "`whoami`"},
			},
			wantSnippet: "default contains unsafe character",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := &plugin.Plugin{
				Metadata: validMetadata(),
				Install: plugin.Install{
					DefaultMethod: "archive",
					Methods:       map[string]plugin.InstallMethod{"archive": {Description: "x"}},
					BuildArgs:     tc.buildArgs,
					ExtraVersions: tc.extras,
				},
			}
			err := p.Validate("test/plugin.toml")
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantSnippet)
		})
	}
}
