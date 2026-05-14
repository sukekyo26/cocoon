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
