package plugin_test

import (
	"bytes"
	"errors"
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

func TestLoad_MissingFile(t *testing.T) {
	t.Parallel()
	_, err := plugin.Load(filepath.Join(t.TempDir(), "missing.toml"))
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

// TestLoadEnabledFromFS_NilSrcReturnsSentinel pins the contract
// documented on LoadEnabledFromFS: a nil source surfaces ErrNilPluginsFS
// instead of panicking inside fs.Stat. Both an empty and a non-empty
// enable list are exercised so the early return short-circuits in both
// cases (a nil fs is a programming error regardless of what was asked).
func TestLoadEnabledFromFS_NilSrcReturnsSentinel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		enabled []string
	}{
		{name: "empty_enable_list", enabled: nil},
		{name: "non_empty_enable_list", enabled: []string{"foo"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out, err := plugin.LoadEnabledFromFS(nil, tc.enabled, nil, "")
			if !errors.Is(err, plugin.ErrNilPluginsFS) {
				t.Fatalf("err = %v, want errors.Is(.., ErrNilPluginsFS)", err)
			}
			if out != nil {
				t.Errorf("out = %v, want nil on error", out)
			}
		})
	}
}
