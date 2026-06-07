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
// "latest" request cannot be resolved to a reproducible version and is skipped
// by `cocoon lock` (the sourceless degradation case).
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

// extraSourcePluginTOML is demoPluginTOML plus a declared [install.extra_versions]
// knob, so a workspace can set [plugins.options].demo.build_tools and have it
// carried into the lock entry's Extra.
const extraSourcePluginTOML = demoPluginTOML + `
[install.extra_versions]
build_tools = { env = "DEMO_BUILD_TOOLS", default = "35.0.0" }
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
	t.Setenv("WORKSPACE_LANG", "en") // pin locale so the English assertion is deterministic
	seedProject(t, "", nil)          // no plugins enabled → nothing to lock
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

// TestLock_MalformedExistingLock pins the recovery contract: `cocoon lock`
// (write) regenerates over a corrupt existing lock instead of refusing, while
// `cocoon lock --check` reports it as a usage error CI can gate on.
//
//nolint:paralleltest // mutates defaultFetcher global + cwd/HOME.
func TestLock_MalformedExistingLock(t *testing.T) {
	t.Setenv("WORKSPACE_LANG", "en") // pin locale so the English "malformed" assertions are deterministic
	lockPath := seedProject(t, `"demo"`, map[string]string{"demo": demoPluginTOML})
	swapFetcher(t, demoFetcher())

	// A lock with lock_version omitted (decoded as 0) is malformed.
	//nolint:gosec // test path under t.TempDir
	require.NoError(t, os.WriteFile(lockPath, []byte("inputs_hash = \"x\"\n"), 0o600))

	// --check: malformed is a usage error (not silently treated as up to date).
	out, err := runLockCmd(t, "--check")
	require.ErrorIs(t, err, clihelpers.ErrUsage, "out=%s", out)
	require.Contains(t, errOrOut(err, out), "malformed")

	// write: regenerates from scratch (warns, succeeds) and replaces the file.
	out, err = runLockCmd(t)
	require.NoError(t, err, "out=%s", out)
	require.Contains(t, out, "malformed")
	l, lerr := lockfile.Load(lockPath)
	require.NoError(t, lerr, "the regenerated lock must be valid")
	_, ok := l.Find("demo")
	require.True(t, ok)
	_, err = runLockCmd(t, "--check")
	require.NoError(t, err, "after regeneration --check passes")
}

// TestLock_SourcelessLatestIsSkipped pins that a sourceless version_capable
// plugin requesting "latest" is skipped (not a usage error): the lock is
// written with no entry for it and a skip notice is logged.
//
//nolint:paralleltest // mutates defaultFetcher global + cwd/HOME.
func TestLock_SourcelessLatestIsSkipped(t *testing.T) {
	t.Setenv("WORKSPACE_LANG", "en") // pin locale for the English "not lockable" assertion
	lockPath := seedProject(t, `"nosrc"`, map[string]string{"nosrc": noSourcePluginTOML})
	swapFetcher(t, demoFetcher())

	out, err := runLockCmd(t)
	require.NoError(t, err, "out=%s", out)
	require.Contains(t, out, "not lockable")
	l, lerr := lockfile.Load(lockPath)
	require.NoError(t, lerr)
	_, ok := l.Find("nosrc")
	require.False(t, ok, "sourceless latest plugin must not be locked")
}

// TestLock_SourcelessSkippedWhileSourcedLocked pins that one sourceless latest
// plugin does not abort the whole lock: a co-enabled sourced plugin is still
// resolved and recorded.
//
//nolint:paralleltest // mutates defaultFetcher global + cwd/HOME.
func TestLock_SourcelessSkippedWhileSourcedLocked(t *testing.T) {
	lockPath := seedProject(t, `"nosrc", "demo"`, map[string]string{
		"nosrc": noSourcePluginTOML,
		"demo":  demoPluginTOML,
	})
	swapFetcher(t, demoFetcher())

	_, err := runLockCmd(t)
	require.NoError(t, err)

	l, lerr := lockfile.Load(lockPath)
	require.NoError(t, lerr)
	entry, ok := l.Find("demo")
	require.True(t, ok, "sourced plugin must still be locked")
	require.Equal(t, "1.2.3", entry.Version)
	_, ok = l.Find("nosrc")
	require.False(t, ok, "sourceless latest plugin must be skipped")
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

// TestLock_ExtraDriftRelocksWithoutNetwork pins two coupled contracts: --check
// flags a changed [plugins.options] knob even though the version spec (and thus
// inputs_hash) is unchanged, and re-locking refreshes the reused entry's Extra
// from the current workspace without re-resolving (no new network calls).
//
//nolint:paralleltest // mutates defaultFetcher global + cwd/HOME.
func TestLock_ExtraDriftRelocksWithoutNetwork(t *testing.T) {
	opt := func(v string) string {
		return "\n[plugins.options]\ndemo = { build_tools = \"" + v + "\" }\n"
	}
	proj := seedProjectExtra(t, `"demo=1.2.3"`, opt("34.0.0"),
		map[string]string{"demo": extraSourcePluginTOML})
	lockPath := filepath.Join(proj, lockfile.FileName)
	f := demoFetcher()
	swapFetcher(t, f)

	// Initial lock records the [plugins.options] knob in Extra; --check passes.
	_, err := runLockCmd(t)
	require.NoError(t, err)
	firstCalls := f.calls
	require.Positive(t, firstCalls)
	l, err := lockfile.Load(lockPath)
	require.NoError(t, err)
	entry, ok := l.Find("demo")
	require.True(t, ok)
	require.Equal(t, "34.0.0", entry.Extra["build_tools"])
	_, err = runLockCmd(t, "--check")
	require.NoError(t, err)

	// Change only the knob (version spec unchanged → inputs_hash unaffected):
	// --check must still flag the drift.
	wsPath := filepath.Join(proj, "workspace.toml")
	body, rerr := os.ReadFile(wsPath) //nolint:gosec // test path under t.TempDir
	require.NoError(t, rerr)
	mutated := bytes.Replace(body, []byte(`"34.0.0"`), []byte(`"35.0.0"`), 1)
	require.NotEqual(t, string(body), string(mutated), "options rewrite must change the file")
	//nolint:gosec // test path under t.TempDir
	require.NoError(t, os.WriteFile(wsPath, mutated, 0o600))
	_, err = runLockCmd(t, "--check")
	require.ErrorIs(t, err, clihelpers.ErrUsage)

	// Re-locking reuses the resolved version + checksums (no new network) but
	// refreshes Extra from the current workspace; --check then passes again.
	_, err = runLockCmd(t)
	require.NoError(t, err)
	require.Equal(t, firstCalls, f.calls, "Extra-only drift must not trigger re-resolution")
	l, err = lockfile.Load(lockPath)
	require.NoError(t, err)
	entry, ok = l.Find("demo")
	require.True(t, ok)
	require.Equal(t, "35.0.0", entry.Extra["build_tools"])
	_, err = runLockCmd(t, "--check")
	require.NoError(t, err)
}
