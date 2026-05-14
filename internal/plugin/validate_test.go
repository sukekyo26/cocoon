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
