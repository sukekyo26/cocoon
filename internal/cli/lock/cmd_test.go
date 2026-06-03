//nolint:testpackage // internal test swaps the unexported defaultFetcher seam.
package lockcli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/lockfile"
	"github.com/sukekyo26/cocoon/internal/plugin/resolve"
)

const (
	hexA = "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2"
	hexB = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
)

// countingFetcher is an offline Fetcher that records how many GETs it served
// so a test can assert the idempotent-reuse path made zero network calls.
type countingFetcher struct {
	mu     sync.Mutex
	bodies map[string]string
	calls  int
}

func (f *countingFetcher) Get(_ context.Context, url string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if b, ok := f.bodies[url]; ok {
		return []byte(b), nil
	}
	return nil, fmt.Errorf("countingFetcher: no stub for %s", url)
}

func demoFetcher() *countingFetcher {
	return &countingFetcher{bodies: map[string]string{
		"https://api.github.com/repos/acme/demo/releases/latest": `{"tag_name":"1.2.3"}`,
		"https://dl.acme.test/demo-1.2.3-amd64.bin.sha256":       hexA,
		"https://dl.acme.test/demo-1.2.3-arm64.bin.sha256":       hexB,
	}}
}

func swapFetcher(t *testing.T, f resolve.Fetcher) {
	t.Helper()
	prev := defaultFetcher
	defaultFetcher = f
	t.Cleanup(func() { defaultFetcher = prev })
}

const demoPluginTOML = `[metadata]
name = "demo"
description = "lock test fixture"
url = "https://example.test"
default = false

[install]
requires_root = false
default_method = "binary"

[install.methods.binary]
description = "binary download"

[version]
version_capable = true

[version.source.latest]
type = "github-release"
repo = "acme/demo"

[version.source.checksum]
type = "sidecar"
asset_url = "https://dl.acme.test/demo-${version}-${arch}.bin"
suffix = ".sha256"

[version.source.arch]
amd64 = "amd64"
arm64 = "arm64"
`

// noSourcePluginTOML is version_capable but declares no [version.source], so a
// "latest" request cannot be resolved (the exact-only degradation case).
const noSourcePluginTOML = `[metadata]
name = "nosrc"
description = "no source fixture"
url = "https://example.test"
default = false

[install]
requires_root = false
default_method = "binary"

[install.methods.binary]
description = "binary download"

[version]
version_capable = true
`

// seedProject writes an isolated HOME, a workspace.toml enabling the named
// plugins, and a project-overlay plugin per (id → plugin.toml) entry. It
// chdirs into the project so config.Discover finds the workspace. Returns the
// default cocoon.lock path.
func seedProject(t *testing.T, enable string, plugins map[string]string) string {
	t.Helper()
	return filepath.Join(seedProjectExtra(t, enable, "", plugins), lockfile.FileName)
}

// seedProjectExtra is seedProject with an extra block of top-level TOML
// appended after [plugins] (e.g. a [lockfile] section). It returns the project
// directory so callers can assert on arbitrary output paths.
func seedProjectExtra(t *testing.T, enable, extraTOML string, plugins map[string]string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	proj := t.TempDir()
	ws := "[container]\nservice_name = \"dev\"\nusername = \"dev\"\nimage = \"ubuntu\"\nimage_version = \"24.04\"\n\n" +
		"[plugins]\nenable = [" + enable + "]\n" + extraTOML
	require.NoError(t, os.WriteFile(filepath.Join(proj, "workspace.toml"), []byte(ws), 0o600))
	for id, toml := range plugins {
		dir := filepath.Join(proj, ".cocoon", "plugins", id)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "plugin.toml"), []byte(toml), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "install.binary.sh"), []byte("#!/usr/bin/env bash\necho "+id+"\n"), 0o600))
	}
	t.Chdir(proj)
	return proj
}

// TestLock_RespectsCustomLockFileName pins that `cocoon lock` writes the
// configured [lockfile].name (and not the default cocoon.lock).
//
//nolint:paralleltest // mutates defaultFetcher global + cwd/HOME.
func TestLock_RespectsCustomLockFileName(t *testing.T) {
	proj := seedProjectExtra(t, `"demo"`, "\n[lockfile]\nname = \"custom.lock\"\n",
		map[string]string{"demo": demoPluginTOML})
	swapFetcher(t, demoFetcher())

	_, err := runLockCmd(t)
	require.NoError(t, err)

	l, err := lockfile.Load(filepath.Join(proj, "custom.lock"))
	require.NoError(t, err, "custom.lock should be written and valid")
	_, ok := l.Find("demo")
	require.True(t, ok)
	_, defErr := os.Stat(filepath.Join(proj, "cocoon.lock"))
	require.True(t, os.IsNotExist(defErr), "default cocoon.lock must not be written")
}

// TestLock_CheckPassesWithNoVersionCapablePlugins pins that `cocoon lock --check`
// succeeds (no lock required) when the workspace enables no version-capable
// plugins, so it stays usable as a generic CI gate.
//
//nolint:paralleltest // mutates cwd/HOME via seedProject.
func TestLock_CheckPassesWithNoVersionCapablePlugins(t *testing.T) {
	seedProject(t, "", nil) // no plugins enabled → nothing to lock
	out, err := runLockCmd(t, "--check")
	require.NoError(t, err, "out=%s", out)
	require.Contains(t, out, "nothing to lock")
}

func runLockCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := NewCommand(&stdout, &stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String() + stderr.String(), err
}

//nolint:paralleltest // mutates defaultFetcher global + cwd/HOME.
func TestLock_WritesResolvedLock(t *testing.T) {
	lockPath := seedProject(t, `"demo"`, map[string]string{"demo": demoPluginTOML})
	swapFetcher(t, demoFetcher())

	_, err := runLockCmd(t)
	require.NoError(t, err)

	l, err := lockfile.Load(lockPath)
	require.NoError(t, err)
	entry, ok := l.Find("demo")
	require.True(t, ok)
	require.Equal(t, "1.2.3", entry.Version)
	require.Equal(t, "latest", entry.Requested)
	require.Equal(t, hexA, entry.ChecksumAMD64)
	require.Equal(t, hexB, entry.ChecksumARM64)
}

//nolint:paralleltest // mutates defaultFetcher global + cwd/HOME.
func TestLock_IdempotentReuseNoNetwork(t *testing.T) {
	seedProject(t, `"demo"`, map[string]string{"demo": demoPluginTOML})
	f := demoFetcher()
	swapFetcher(t, f)

	_, err := runLockCmd(t)
	require.NoError(t, err)
	firstCalls := f.calls
	require.Positive(t, firstCalls)

	// A second run with an up-to-date lock must reuse entries: zero new GETs.
	_, err = runLockCmd(t)
	require.NoError(t, err)
	require.Equal(t, firstCalls, f.calls, "second lock run should make no network calls")
}

//nolint:paralleltest // mutates defaultFetcher global + cwd/HOME.
func TestLock_UpgradeReResolves(t *testing.T) {
	seedProject(t, `"demo"`, map[string]string{"demo": demoPluginTOML})
	f := demoFetcher()
	swapFetcher(t, f)

	_, err := runLockCmd(t)
	require.NoError(t, err)
	firstCalls := f.calls

	_, err = runLockCmd(t, "--upgrade")
	require.NoError(t, err)
	require.Greater(t, f.calls, firstCalls, "--upgrade should re-resolve the latest constraint")
}

//nolint:paralleltest // mutates defaultFetcher global + cwd/HOME.
func TestLock_CheckUpToDateAndDrift(t *testing.T) {
	lockPath := seedProject(t, `"demo"`, map[string]string{"demo": demoPluginTOML})
	swapFetcher(t, demoFetcher())

	// --check before any lock exists: missing → usage error.
	_, err := runLockCmd(t, "--check")
	require.ErrorIs(t, err, clihelpers.ErrUsage)

	// Write the lock, then --check passes.
	_, err = runLockCmd(t)
	require.NoError(t, err)
	_, err = runLockCmd(t, "--check")
	require.NoError(t, err)

	// Pin demo inline in the enable array (changes inputs_hash) → drift.
	wsPath := filepath.Join(filepath.Dir(lockPath), "workspace.toml")
	body, rerr := os.ReadFile(wsPath) //nolint:gosec // test path under t.TempDir
	require.NoError(t, rerr)
	mutated := bytes.Replace(body, []byte(`["demo"]`), []byte(`["demo=9.9.9"]`), 1)
	require.NotEqual(t, string(body), string(mutated), "enable rewrite must change the file")
	//nolint:gosec // test path under t.TempDir
	require.NoError(t, os.WriteFile(wsPath, mutated, 0o600))
	_, err = runLockCmd(t, "--check")
	require.ErrorIs(t, err, clihelpers.ErrUsage)
}

//nolint:paralleltest // mutates defaultFetcher global + cwd/HOME.
func TestLock_LatestUnsupportedIsUsageError(t *testing.T) {
	seedProject(t, `"nosrc"`, map[string]string{"nosrc": noSourcePluginTOML})
	swapFetcher(t, demoFetcher())

	out, err := runLockCmd(t)
	require.ErrorIs(t, err, clihelpers.ErrUsage)
	require.Contains(t, errOrOut(err, out), "cannot resolve 'latest'")
}

func errOrOut(err error, out string) string {
	if err != nil {
		return err.Error() + out
	}
	return out
}

//nolint:paralleltest // mutates defaultFetcher global + cwd/HOME.
func TestLock_ExactPinSkipsLatestFetch(t *testing.T) {
	lockPath := seedProject(t, `"nosrc"`, map[string]string{"nosrc": noSourcePluginTOML})
	// nosrc has no source, but an exact pin needs no latest resolution and no
	// checksum (checksum kind absent → none), so lock succeeds offline.
	wsPath := filepath.Join(filepath.Dir(lockPath), "workspace.toml")
	body, rerr := os.ReadFile(wsPath) //nolint:gosec // test path under t.TempDir
	require.NoError(t, rerr)
	mutated := bytes.Replace(body, []byte(`["nosrc"]`), []byte(`["nosrc=2.0.0"]`), 1)
	require.NotEqual(t, string(body), string(mutated), "enable rewrite must change the file")
	//nolint:gosec // test path under t.TempDir
	require.NoError(t, os.WriteFile(wsPath, mutated, 0o600))
	swapFetcher(t, &countingFetcher{bodies: map[string]string{}})

	_, err := runLockCmd(t)
	require.NoError(t, err)
	l, err := lockfile.Load(lockPath)
	require.NoError(t, err)
	entry, ok := l.Find("nosrc")
	require.True(t, ok)
	require.Equal(t, "2.0.0", entry.Version)
	require.Empty(t, entry.ChecksumAMD64)
}
