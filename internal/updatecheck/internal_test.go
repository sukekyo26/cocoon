//nolint:testpackage // white-box tests for unexported resolveCachePath / readCache / writeCache.
package updatecheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestResolveCachePath_Override pins that a non-empty override is joined
// with the cache filename and reported usable without consulting the OS.
func TestResolveCachePath_Override(t *testing.T) {
	t.Parallel()
	got, ok := resolveCachePath(filepath.FromSlash("/tmp/somewhere"))
	if !ok {
		t.Fatal("ok = false, want true for an explicit override")
	}
	if want := filepath.Join(filepath.FromSlash("/tmp/somewhere"), cacheFile); got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
}

// TestResolveCachePath_DefaultFromEnv pins the empty-override branch that
// derives the path from os.UserCacheDir. Expected base is computed via the
// same call so the assertion stays portable across platforms.
//
//nolint:paralleltest // mutates process env via t.Setenv
func TestResolveCachePath_DefaultFromEnv(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	wantBase, err := os.UserCacheDir()
	if err != nil {
		t.Skipf("os.UserCacheDir unavailable: %v", err)
	}
	got, ok := resolveCachePath("")
	if !ok {
		t.Fatal("ok = false, want true when a cache dir resolves")
	}
	if want := filepath.Join(wantBase, "cocoon", cacheFile); got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
}

// TestResolveCachePath_NoCacheDir pins the failure branch: when neither an
// override nor a resolvable user cache dir exists, resolveCachePath reports
// the path is unusable so Check proceeds without caching.
//
//nolint:paralleltest // mutates process env via t.Setenv
func TestResolveCachePath_NoCacheDir(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")
	if _, err := os.UserCacheDir(); err == nil {
		t.Skip("os.UserCacheDir resolves without HOME/XDG on this platform")
	}
	got, ok := resolveCachePath("")
	if ok || got != "" {
		t.Errorf("resolveCachePath = (%q, %v), want (\"\", false)", got, ok)
	}
}

// TestReadCache covers every rejection branch and the happy path. A
// rejected read must report ok=false and a zero state so Check refetches.
func TestReadCache(t *testing.T) {
	t.Parallel()
	mustJSON := func(c cacheState) string {
		b, err := json.Marshal(c)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return string(b)
	}
	valid := cacheState{CheckedAt: time.Now(), LatestVersion: "1.2.3", SchemaVersion: cacheSchemaVersion}
	cases := []struct {
		name    string
		write   bool
		content string
		wantOK  bool
	}{
		{"missing", false, "", false},
		{"malformed", true, "{not json", false},
		{"wrong_schema", true, mustJSON(cacheState{CheckedAt: time.Now(), LatestVersion: "1.2.3", SchemaVersion: 99}), false},
		{"empty_latest", true, mustJSON(cacheState{CheckedAt: time.Now(), LatestVersion: "", SchemaVersion: cacheSchemaVersion}), false},
		{"valid", true, mustJSON(valid), true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "update_check.json")
			if tc.write {
				if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
					t.Fatalf("write: %v", err)
				}
			}
			got, ok := readCache(path)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !tc.wantOK {
				if got.LatestVersion != "" || got.SchemaVersion != 0 || !got.CheckedAt.IsZero() {
					t.Errorf("rejected read returned non-zero state %+v", got)
				}
				return
			}
			if got.LatestVersion != "1.2.3" {
				t.Errorf("LatestVersion = %q, want 1.2.3", got.LatestVersion)
			}
		})
	}
}

// TestWriteCache_RoundTrip pins that writeCache creates missing parent
// directories and that readCache reads the value back.
func TestWriteCache_RoundTrip(t *testing.T) {
	t.Parallel()
	// "sub" does not exist yet — exercises the MkdirAll path.
	path := filepath.Join(t.TempDir(), "sub", "update_check.json")
	want := cacheState{
		CheckedAt:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		LatestVersion: "1.2.3",
		SchemaVersion: cacheSchemaVersion,
	}
	if err := writeCache(path, want); err != nil {
		t.Fatalf("writeCache: %v", err)
	}
	got, ok := readCache(path)
	if !ok {
		t.Fatal("readCache after write: ok = false")
	}
	if got.LatestVersion != want.LatestVersion {
		t.Errorf("LatestVersion = %q, want %q", got.LatestVersion, want.LatestVersion)
	}
}

// TestWriteCache_MkdirError pins the MkdirAll error branch: a parent path
// component that is a regular file cannot be turned into a directory.
func TestWriteCache_MkdirError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fileAsDir := filepath.Join(dir, "afile")
	if err := os.WriteFile(fileAsDir, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	path := filepath.Join(fileAsDir, "sub", "update_check.json")
	if err := writeCache(path, cacheState{SchemaVersion: cacheSchemaVersion}); err == nil {
		t.Fatal("expected mkdir error, got nil")
	}
}

// TestWriteCache_WriteError pins the WriteFile error branch: the target
// path is itself an existing directory, so the file write fails after the
// parent MkdirAll succeeds.
func TestWriteCache_WriteError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := writeCache(dir, cacheState{SchemaVersion: cacheSchemaVersion}); err == nil {
		t.Fatal("expected write error when target is a directory, got nil")
	}
}
