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

// TestLoad_RejectsRetiredUserDirsField pins the contract that the
// removed [install].user_dirs field surfaces as a strict-unmarshal error
// rather than being silently ignored, so authors discover the migration
// to [install].volumes immediately.
func TestLoad_RejectsRetiredUserDirsField(t *testing.T) {
	t.Parallel()

	body := []byte(`[metadata]
name = "Retired"
description = "uses removed user_dirs (https://example.com)"

[install]
requires_root = true
user_dirs = ["/home/${USERNAME}/.cache/x"]

[version]
version_capable = false
`)
	dir := t.TempDir()
	path := filepath.Join(dir, "plugin.toml")
	require.NoError(t, os.WriteFile(path, body, 0o600))

	_, err := plugin.Load(path)
	require.Error(t, err)
	if !strings.Contains(err.Error(), "user_dirs") {
		t.Errorf("expected error to mention user_dirs, got %v", err)
	}
}

func TestLoadEnabledFromFS_WarnsOnMissingPlugin(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var warnings bytes.Buffer
	out, err := plugin.LoadEnabledFromFS(os.DirFS(dir), []string{"nonexistent"}, &warnings, dir)
	require.NoError(t, err)
	require.Empty(t, out)
	if !strings.Contains(warnings.String(), "WARNING: Plugin 'nonexistent' not found") {
		t.Errorf("expected warning: %q", warnings.String())
	}
}

func TestLoadEnabledFromFS_LoadsExistingPlugin(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Copy the sample plugin into the temp dir under id "sample". The
	// loader's validateMethodScripts requires install.<method>.sh for
	// every declared method, so the install.installer.sh stub has to
	// land in the temp plugin dir too.
	src := filepath.Join("testdata", "plugin_sample.toml")
	body, err := os.ReadFile(src)
	require.NoError(t, err)
	pluginDir := filepath.Join(dir, "sample")
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))
	//nolint:gosec // pluginDir is built from t.TempDir + literal "sample".
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin.toml"), body, 0o600))
	scriptSrc := filepath.Join("testdata", "install.installer.sh")
	scriptBody, err := os.ReadFile(scriptSrc)
	require.NoError(t, err)
	//nolint:gosec // pluginDir is built from t.TempDir + literal "sample".
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "install.installer.sh"), scriptBody, 0o600))

	out, err := plugin.LoadEnabledFromFS(os.DirFS(dir), []string{"sample", "ghost"}, &bytes.Buffer{}, dir)
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Contains(t, out, "sample")
}

func TestLoadEnabledFromFS_NilWarnings(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// nil warnings writer must not panic; missing plugins still skip.
	_, err := plugin.LoadEnabledFromFS(os.DirFS(dir), []string{"missing"}, nil, dir)
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

const methodTOMLBody = `
[metadata]
name = "x"
description = "y"
url = "https://example.com/x"
default = false

[install]
default_method = "official"

[install.methods.official]
description = "Official installer"

[install.methods.binary]
description = "Direct binary"

[version]
version_capable = false
`

// TestLoad_MethodScriptMissing pins that Load rejects a plugin declaring
// [install.methods.<name>] when the matching install.<name>.sh file is
// missing from the same directory.
func TestLoad_MethodScriptMissing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "plugin.toml")
	require.NoError(t, os.WriteFile(tomlPath, []byte(methodTOMLBody), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "install.official.sh"), []byte("#!/bin/sh\n"), 0o600))
	// install.binary.sh intentionally absent.
	_, err := plugin.Load(tomlPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "install.binary.sh does not exist")
}

// TestLoad_MethodScriptsPresent pins the happy path: all declared
// methods have their install.<name>.sh and Load succeeds.
func TestLoad_MethodScriptsPresent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "plugin.toml")
	require.NoError(t, os.WriteFile(tomlPath, []byte(methodTOMLBody), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "install.official.sh"), []byte("#!/bin/sh\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "install.binary.sh"), []byte("#!/bin/sh\n"), 0o600))
	p, err := plugin.Load(tomlPath)
	require.NoError(t, err)
	require.Equal(t, "official", p.Install.DefaultMethod)
	require.Contains(t, p.Install.Methods, "official")
	require.Contains(t, p.Install.Methods, "binary")
}

// TestLoad_InstallShAlwaysRejected pins the new invariant: a literal
// install.sh is never accepted, regardless of whether [install.methods]
// is declared. The error message points the author at the rename +
// plugin.toml edit so they can migrate without reading the docs.
func TestLoad_InstallShAlwaysRejected(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "plugin.toml")
	require.NoError(t, os.WriteFile(tomlPath, []byte(methodTOMLBody), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "install.official.sh"), []byte("#!/bin/sh\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "install.binary.sh"), []byte("#!/bin/sh\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "install.sh"), []byte("#!/bin/sh\n"), 0o600))
	_, err := plugin.Load(tomlPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "install.sh is no longer supported")
	require.Contains(t, err.Error(), "install.<category>.sh")
}

// TestLoad_RequiresInstallMethods pins the new mandatory invariant:
// every plugin.toml must declare at least one [install.methods.<name>]
// entry. Single-method plugins still need one entry; the legacy
// "no [install.methods] → use install.sh" path is gone.
func TestLoad_RequiresInstallMethods(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "plugin.toml")
	body := `
[metadata]
name = "x"
description = "y"
url = "https://example.com/x"
default = false

[install]
requires_root = false

[version]
version_capable = false
`
	require.NoError(t, os.WriteFile(tomlPath, []byte(body), 0o600))
	_, err := plugin.Load(tomlPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "[install.methods] must declare at least one entry")
	require.Contains(t, err.Error(), "binary / installer / apt / archive")
}
