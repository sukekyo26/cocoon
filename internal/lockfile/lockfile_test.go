package lockfile_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/lockfile"
)

const hexA = "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2"

func sampleLock() *lockfile.Lock {
	return &lockfile.Lock{
		LockVersion: lockfile.Version,
		InputsHash:  "deadbeef",
		Plugins: []lockfile.LockPlugin{
			// Intentionally unsorted to prove Marshal sorts by ID.
			{ID: "node", Requested: "latest", Version: "22.15.0", ChecksumAMD64: hexA, ChecksumARM64: hexA},               //nolint:exhaustruct // no extras
			{ID: "go", Requested: "=1.23.4", Version: "1.23.4"},                                                           //nolint:exhaustruct // no checksum
			{ID: "android-sdk", Requested: "=14742923", Version: "14742923", Extra: map[string]string{"api_level": "35"}}, //nolint:exhaustruct // no checksum
		},
	}
}

// TestMarshal_SortedAndOmitEmpty pins that Marshal emits the header, sorts
// plugins by id (deterministic diffs), and omits empty checksums.
func TestMarshal_SortedAndOmitEmpty(t *testing.T) {
	t.Parallel()
	got, err := lockfile.Marshal(sampleLock())
	require.NoError(t, err)
	s := string(got)
	require.True(t, strings.HasPrefix(s, "# cocoon.lock"), "missing header:\n%s", s)
	// Sorted by id: android-sdk < go < node.
	iAndroid := strings.Index(s, "id = 'android-sdk'")
	iGo := strings.Index(s, "id = 'go'")
	iNode := strings.Index(s, "id = 'node'")
	require.Positive(t, iAndroid)
	require.Less(t, iAndroid, iGo)
	require.Less(t, iGo, iNode)
	// go has no checksum (omitempty); node carries both.
	require.NotContains(t, s[iGo:iNode], "checksum_")
	require.Contains(t, s, "checksum_amd64 = '"+hexA+"'")
	require.Contains(t, s, "checksum_arm64 = '"+hexA+"'")
	require.Contains(t, s, "[plugins.extra]")
}

// TestMarshal_StableAcrossRuns pins byte-for-byte determinism (the whole
// reason cocoon.lock is committed and diffed).
func TestMarshal_StableAcrossRuns(t *testing.T) {
	t.Parallel()
	a, err := lockfile.Marshal(sampleLock())
	require.NoError(t, err)
	b, err := lockfile.Marshal(sampleLock())
	require.NoError(t, err)
	require.Equal(t, string(a), string(b))
}

// TestSaveLoad_RoundTrip pins that Save → Load preserves the (sorted) lock.
func TestSaveLoad_RoundTrip(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), lockfile.FileName)
	require.NoError(t, lockfile.Save(path, sampleLock()))

	got, err := lockfile.Load(path)
	require.NoError(t, err)
	require.Equal(t, lockfile.Version, got.LockVersion)
	require.Equal(t, "deadbeef", got.InputsHash)
	require.Len(t, got.Plugins, 3)
	require.Equal(t, "android-sdk", got.Plugins[0].ID) // sorted on Save
	node, ok := got.Find("node")
	require.True(t, ok)
	require.Equal(t, hexA, node.ChecksumAMD64)
	require.Equal(t, map[string]string{"api_level": "35"}, got.Plugins[0].Extra)
}

func TestLoad_Errors(t *testing.T) {
	t.Parallel()
	t.Run("missing_is_not_exist", func(t *testing.T) {
		t.Parallel()
		_, err := lockfile.Load(filepath.Join(t.TempDir(), "absent.lock"))
		require.Error(t, err)
		require.True(t, lockfile.IsNotExist(err), "want IsNotExist, got %v", err)
	})
	cases := []struct {
		name, body, wantContains string
	}{
		{
			name:         "unsupported_version",
			body:         "lock_version = 999\ninputs_hash = 'x'\n",
			wantContains: "unsupported lock_version",
		},
		{
			name:         "bad_checksum",
			body:         "lock_version = 1\ninputs_hash = 'x'\n\n[[plugins]]\nid = 'go'\nrequested = '=1.0'\nversion = '1.0'\nchecksum_amd64 = 'NOPE'\n",
			wantContains: "64 lowercase hex",
		},
		{
			name:         "empty_version",
			body:         "lock_version = 1\ninputs_hash = 'x'\n\n[[plugins]]\nid = 'go'\nrequested = 'latest'\nversion = ''\n",
			wantContains: "empty version",
		},
		{
			name:         "missing_lock_version",
			body:         "inputs_hash = 'x'\n",
			wantContains: "missing lock_version",
		},
		{
			name:         "duplicate_plugin_id",
			body:         "lock_version = 1\ninputs_hash = 'x'\n\n[[plugins]]\nid = 'go'\nrequested = '=1.0'\nversion = '1.0'\n\n[[plugins]]\nid = 'go'\nrequested = '=1.1'\nversion = '1.1'\n",
			wantContains: "duplicate plugin id",
		},
		{
			name:         "unknown_field",
			body:         "lock_version = 1\ninputs_hash = 'x'\nbogus = true\n",
			wantContains: "unknown field",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), lockfile.FileName)
			require.NoError(t, os.WriteFile(path, []byte(tc.body), 0o600))
			_, err := lockfile.Load(path)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantContains)
		})
	}
}

// TestLoad_RejectsUnsafeVersion pins that Load rejects a hand-tampered lock
// whose version carries a character that would break out of the generated
// Dockerfile's PIN="..." env pair (the build-time injection vector). The
// equivalent enable-array pin is already bounded by config.rxImageVersion, so
// this is the lock-side half of the same guard.
func TestLoad_RejectsUnsafeVersion(t *testing.T) {
	t.Parallel()
	const head = "lock_version = 1\ninputs_hash = 'x'\n\n[[plugins]]\nid = 'go'\nrequested = 'latest'\n"
	cases := []struct{ name, versionLine string }{
		// TOML basic string: \n decodes to a real newline (the PoC payload that
		// splits PIN="..." and injects standalone RUN instructions).
		{"newline", `version = "1.0\nRUN touch /tmp/pwned"`},
		// TOML literal string: the double quote is preserved verbatim and would
		// close PIN="..." early.
		{"double_quote", `version = '1.0" CHECKSUM_AMD64="x'`},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), lockfile.FileName)
			require.NoError(t, os.WriteFile(path, []byte(head+tc.versionLine+"\n"), 0o600))
			_, err := lockfile.Load(path)
			require.ErrorIs(t, err, lockfile.ErrUnsafeVersion)
		})
	}
}

// TestSave_RejectsUnsafeVersion pins that Save refuses to record a version
// resolved from a malicious upstream (the second injection vector) and never
// writes the file when it does. Covers every rune the sink treats as unsafe.
func TestSave_RejectsUnsafeVersion(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, version string }{
		{"newline", "1.0\nRUN touch /tmp/pwned"},
		{"carriage_return", "1.0\rRUN touch /tmp/pwned"},
		{"double_quote", `1.0" CHECKSUM_AMD64="x`},
		{"backslash", `1.0\x`},
		{"dollar", "1.0$(touch /tmp/pwned)"},
		{"backtick", "1.0`touch /tmp/pwned`"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l := &lockfile.Lock{
				LockVersion: lockfile.Version,
				InputsHash:  "x",
				Plugins: []lockfile.LockPlugin{
					{ID: "go", Requested: "latest", Version: tc.version}, //nolint:exhaustruct // no checksum/extra
				},
			}
			path := filepath.Join(t.TempDir(), lockfile.FileName)
			err := lockfile.Save(path, l)
			require.ErrorIs(t, err, lockfile.ErrUnsafeVersion)
			_, statErr := os.Stat(path)
			require.Truef(t, os.IsNotExist(statErr), "Save must not write an invalid lock; stat err = %v", statErr)
		})
	}
}

func TestComputeInputsHash(t *testing.T) {
	t.Parallel()
	base := map[string]string{"go": "=1.23.4", "node": "latest"}
	// Order-independent: a different map literal with the same pairs hashes equal.
	reordered := map[string]string{"node": "latest", "go": "=1.23.4"}
	require.Equal(t, lockfile.ComputeInputsHash(base), lockfile.ComputeInputsHash(reordered))
	// A changed spec changes the hash.
	changed := map[string]string{"go": "=1.24.0", "node": "latest"}
	require.NotEqual(t, lockfile.ComputeInputsHash(base), lockfile.ComputeInputsHash(changed))
	// An added plugin changes the hash.
	added := map[string]string{"go": "=1.23.4", "node": "latest", "uv": "latest"}
	require.NotEqual(t, lockfile.ComputeInputsHash(base), lockfile.ComputeInputsHash(added))
}

func TestFind_NilSafe(t *testing.T) {
	t.Parallel()
	var l *lockfile.Lock
	_, ok := l.Find("go")
	require.False(t, ok)
}

// TestPathFor pins the lock path resolution: with no [lockfile] section the
// basename defaults to FileName; with a configured name it wins. Both join to
// the workspace.toml directory.
func TestPathFor(t *testing.T) {
	t.Parallel()
	wsPath := filepath.Join("proj", "workspace.toml")
	require.Equal(t,
		filepath.Join("proj", lockfile.FileName),
		lockfile.PathFor(wsPath, &config.Workspace{}))
	name := "custom.lock"
	require.Equal(t,
		filepath.Join("proj", "custom.lock"),
		lockfile.PathFor(wsPath, &config.Workspace{Lockfile: &config.LockFileSpec{Name: &name}}))
}
