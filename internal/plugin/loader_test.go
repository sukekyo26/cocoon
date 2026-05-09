package plugin_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sukekyo26/cocoon/internal/plugin"
)

func TestLoad_Sample(t *testing.T) {
	t.Parallel()

	p, err := plugin.Load(filepath.Join("testdata", "plugin_sample.toml"))
	require.NoError(t, err)
	require.Equal(t, "Sample", p.Metadata.Name)
	require.False(t, p.Metadata.Default)
	require.True(t, p.Install.RequiresRoot)
	require.Equal(t, []string{"/home/${USERNAME}/.sample"}, p.Install.Volumes)
	require.Equal(t, map[string]string{"SAMPLE_HOME": "/home/${USERNAME}/.sample"}, p.Install.Env)
	require.False(t, p.Version.VersionCapable)
}

func TestLoadDir(t *testing.T) {
	t.Parallel()

	plugins, err := plugin.LoadDir("../../internal/plugin/catalog")
	require.NoError(t, err)
	require.NotEmpty(t, plugins)
	// Sanity: a few well-known plugins must be present.
	require.Contains(t, plugins, "go")
	require.Contains(t, plugins, "uv")
}

func TestLoad_MissingFile(t *testing.T) {
	t.Parallel()
	_, err := plugin.Load(filepath.Join(t.TempDir(), "missing.toml"))
	require.Error(t, err)
}

func TestLoadDir_MissingDir(t *testing.T) {
	t.Parallel()
	_, err := plugin.LoadDir(filepath.Join(t.TempDir(), "no-such"))
	require.Error(t, err)
}

func TestLoadEnabled_WarnsOnMissingPlugin(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var warnings bytes.Buffer
	out, err := plugin.LoadEnabled(dir, []string{"nonexistent"}, &warnings)
	require.NoError(t, err)
	require.Empty(t, out)
	if !strings.Contains(warnings.String(), "WARNING: Plugin 'nonexistent' not found") {
		t.Errorf("expected warning: %q", warnings.String())
	}
}

func TestLoadEnabled_LoadsExistingPlugin(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Copy the sample plugin into the temp dir under id "sample".
	src := filepath.Join("testdata", "plugin_sample.toml")
	body, err := os.ReadFile(src)
	require.NoError(t, err)
	pluginDir := filepath.Join(dir, "sample")
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))
	//nolint:gosec // pluginDir is built from t.TempDir + literal "sample".
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin.toml"), body, 0o600))

	out, err := plugin.LoadEnabled(dir, []string{"sample", "ghost"}, &bytes.Buffer{})
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Contains(t, out, "sample")
}

func TestLoadEnabled_NilWarnings(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// nil warnings writer must not panic; missing plugins still skip.
	_, err := plugin.LoadEnabled(dir, []string{"missing"}, nil)
	require.NoError(t, err)
}
