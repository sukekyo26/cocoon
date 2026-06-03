package lockfile_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

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
