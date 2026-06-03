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

// TestValidate_BuildArgsRejectsReservedEnv covers the cases where a
// plugin would silently shadow a framework-provided env variable by
// declaring its name in install.build_args. Every entry of
// reservedExtraVersionEnvs is exercised so adding a new reserved name
// to that set automatically gets caught by this test if the validator
// drift away.
func TestValidate_BuildArgsRejectsReservedEnv(t *testing.T) {
	t.Parallel()
	cases := []string{
		"PIN",
		"CHECKSUM_AMD64",
		"CHECKSUM_ARM64",
		"RC_FILE",
		"RC_SYNTAX",
		"LOGIN_SHELL",
		"COCOON_INSTALL_METHOD",
		"USERNAME",
	}
	for _, name := range cases {
		name := name
		t.Run(name, func(t *testing.T) {
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
build_args = ["` + name + `"]
[version]
version_capable = false
`
			require.NoError(t, os.WriteFile(tmp, []byte(body), 0o600))
			_, err := plugin.Load(tmp)
			require.Error(t, err)
			require.Contains(t, err.Error(), "collides with a cocoon-reserved variable")
		})
	}
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

func TestValidate_InstallEnvValueRejectsUnsafe(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		val  string
	}{
		{"newline", "a\nRUN echo pwned"},
		{"carriage return", "a\rb"},
		{"double quote", `a"b`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := &plugin.Plugin{
				Metadata: validMetadata(),
				Install: plugin.Install{
					DefaultMethod: "binary",
					Methods:       map[string]plugin.InstallMethod{"binary": {Description: "Direct binary"}},
					Env:           map[string]string{"GOOD": tc.val},
				},
			}
			err := p.Validate("test/plugin.toml")
			require.Error(t, err)
			require.Contains(t, err.Error(), "unsafe character")
		})
	}
}

// TestValidate_InstallEnvValueAcceptsExpansion pins that $-references stay
// legal in install.env values — only newline / double-quote are rejected.
func TestValidate_InstallEnvValueAcceptsExpansion(t *testing.T) {
	t.Parallel()
	p := &plugin.Plugin{
		Metadata: validMetadata(),
		Install: plugin.Install{
			DefaultMethod: "binary",
			Methods:       map[string]plugin.InstallMethod{"binary": {Description: "Direct binary"}},
			Env:           map[string]string{"PATH": "/usr/local/go/bin:$PATH"},
		},
	}
	require.NoError(t, p.Validate("test/plugin.toml"))
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

// versionSourcePlugin builds a Plugin with a valid Install and the given
// [version] block so [version.source] validation runs without unrelated
// failures masking the result.
func versionSourcePlugin(versionCapable bool, verify string, src *plugin.VersionSource) *plugin.Plugin {
	return &plugin.Plugin{
		Metadata: validMetadata(),
		Install: plugin.Install{
			DefaultMethod: "official",
			Methods:       map[string]plugin.InstallMethod{"official": {Description: "x"}},
		},
		Version: plugin.Version{VersionCapable: versionCapable, Verify: verify, Source: src},
	}
}

// TestValidate_VersionSourceAccepted pins the well-formed [version.source]
// shapes for the latest × checksum kind combinations cocoon ships.
func TestValidate_VersionSourceAccepted(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		src  plugin.VersionSource
	}{
		{
			name: "github_release_shasums",
			src: plugin.VersionSource{
				Latest:   plugin.LatestSpec{Type: plugin.LatestGitHubRelease, Repo: "casey/just"},
				Checksum: plugin.ChecksumSpec{Type: plugin.ChecksumShasumsFile, ManifestURL: "https://github.com/casey/just/releases/download/${version}/SHA256SUMS", AssetName: "just-${version}-${arch}.tar.gz"},
				Arch:     map[string]string{"amd64": "x86_64", "arm64": "aarch64"},
			},
		},
		{
			name: "text_sidecar",
			src: plugin.VersionSource{
				Latest:   plugin.LatestSpec{Type: plugin.LatestText, URL: "https://go.dev/VERSION?m=text", StripPrefix: "go"},
				Checksum: plugin.ChecksumSpec{Type: plugin.ChecksumSidecar, AssetURL: "https://dl.google.com/go/go${version}.linux-${arch}.tar.gz", Suffix: ".sha256"},
				Arch:     map[string]string{"amd64": "amd64", "arm64": "arm64"},
			},
		},
		{
			name: "json_field_none",
			src: plugin.VersionSource{
				Latest:   plugin.LatestSpec{Type: plugin.LatestJSONField, URL: "https://x.test/v", Field: "current_version"},
				Checksum: plugin.ChecksumSpec{Type: plugin.ChecksumNone},
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			src := tc.src
			require.NoError(t, versionSourcePlugin(true, "", &src).Validate("test/plugin.toml"))
		})
	}
}

// TestValidate_VersionSourceRejected covers each [version.source] reject
// path: missing version_capable, unknown/blank kinds, missing per-kind
// required fields, an insecure URL, a pgp/checksum contradiction, and a
// ${arch} template without an arch map.
func TestValidate_VersionSourceRejected(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name           string
		versionCapable bool
		verify         string
		src            plugin.VersionSource
		wantContains   string
	}{
		{
			name:           "source_without_version_capable",
			versionCapable: false,
			src:            plugin.VersionSource{Latest: plugin.LatestSpec{Type: plugin.LatestText, URL: "https://x.test/v"}, Checksum: plugin.ChecksumSpec{Type: plugin.ChecksumNone}},
			wantContains:   "[version.source] requires version_capable = true",
		},
		{
			name:           "unknown_latest_type",
			versionCapable: true,
			src:            plugin.VersionSource{Latest: plugin.LatestSpec{Type: "wat"}, Checksum: plugin.ChecksumSpec{Type: plugin.ChecksumNone}},
			wantContains:   "latest.type",
		},
		{
			name:           "github_missing_repo",
			versionCapable: true,
			src:            plugin.VersionSource{Latest: plugin.LatestSpec{Type: plugin.LatestGitHubRelease}, Checksum: plugin.ChecksumSpec{Type: plugin.ChecksumNone}},
			wantContains:   "repo is required",
		},
		{
			name:           "json_missing_field",
			versionCapable: true,
			src:            plugin.VersionSource{Latest: plugin.LatestSpec{Type: plugin.LatestJSONField, URL: "https://x.test/v"}, Checksum: plugin.ChecksumSpec{Type: plugin.ChecksumNone}},
			wantContains:   "field is required",
		},
		{
			name:           "text_insecure_url",
			versionCapable: true,
			src:            plugin.VersionSource{Latest: plugin.LatestSpec{Type: plugin.LatestText, URL: "http://insecure.test/v"}, Checksum: plugin.ChecksumSpec{Type: plugin.ChecksumNone}},
			wantContains:   "must start with https://",
		},
		{
			name:           "unknown_checksum_type",
			versionCapable: true,
			src:            plugin.VersionSource{Latest: plugin.LatestSpec{Type: plugin.LatestText, URL: "https://x.test/v"}, Checksum: plugin.ChecksumSpec{Type: "wat"}},
			wantContains:   "checksum.type",
		},
		{
			name:           "shasums_missing_asset_name",
			versionCapable: true,
			src:            plugin.VersionSource{Latest: plugin.LatestSpec{Type: plugin.LatestText, URL: "https://x.test/v"}, Checksum: plugin.ChecksumSpec{Type: plugin.ChecksumShasumsFile, ManifestURL: "https://x.test/SHA"}},
			wantContains:   "asset_name is required",
		},
		{
			name:           "pgp_with_checksum",
			versionCapable: true,
			verify:         plugin.VerifyPGP,
			src:            plugin.VersionSource{Latest: plugin.LatestSpec{Type: plugin.LatestText, URL: "https://x.test/v"}, Checksum: plugin.ChecksumSpec{Type: plugin.ChecksumSidecar, AssetURL: "https://x.test/${version}.tar.gz", Suffix: ".sha256"}, Arch: map[string]string{"amd64": "amd64"}},
			wantContains:   `checksum.type must be "none" when verify = "pgp"`,
		},
		{
			name:           "arch_template_without_map",
			versionCapable: true,
			src:            plugin.VersionSource{Latest: plugin.LatestSpec{Type: plugin.LatestText, URL: "https://x.test/${arch}/v"}, Checksum: plugin.ChecksumSpec{Type: plugin.ChecksumNone}},
			wantContains:   "arch map is required",
		},
		{
			name:           "unknown_placeholder",
			versionCapable: true,
			src:            plugin.VersionSource{Latest: plugin.LatestSpec{Type: plugin.LatestText, URL: "https://x.test/${foo}/v"}, Checksum: plugin.ChecksumSpec{Type: plugin.ChecksumNone}},
			wantContains:   "unknown placeholder ${foo}",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			src := tc.src
			err := versionSourcePlugin(tc.versionCapable, tc.verify, &src).Validate("test/plugin.toml")
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantContains)
		})
	}
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
