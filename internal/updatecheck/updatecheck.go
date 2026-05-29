// Package updatecheck runs cocoon's once-per-day GitHub Releases check.
// Distinct from selfupdate (the binary replacement) because this call is
// cross-cutting: it runs in root PersistentPreRun, must never error out,
// and caches its answer to avoid hammering api.github.com.
//
// All failures are silent — Check returns nil. Update notification must
// never get in the way of the user's command.
package updatecheck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sukekyo26/cocoon/internal/release"
)

// CacheTTL caps how long a fetched latest-version answer is reused.
const CacheTTL = 24 * time.Hour

const cacheFile = "update_check.json"

// cacheSchemaVersion bumps on incompatible JSON layout changes.
const cacheSchemaVersion = 1

// Notice signals the running binary is older than the latest published
// release. Coloring is left to the caller (Logger.Noticef).
type Notice struct {
	Current, Latest string
}

// Format returns the one-line announcement to print on stderr.
func (n *Notice) Format() string {
	return fmt.Sprintf(
		"A new version v%s is available (current: v%s). Run `cocoon self-update` to upgrade.",
		n.Latest, n.Current,
	)
}

// Options configures Check. All fields are optional.
type Options struct {
	// Now overrides time.Now (for TTL boundary tests).
	Now func() time.Time
	// CacheDir overrides the default <UserCacheDir>/cocoon path.
	CacheDir string
	// HTTPClient overrides the http.Client passed to release.FetchLatest.
	HTTPClient *http.Client
}

// Check swallows every error (network, cache I/O, parse) and returns nil.
// It runs on every invocation and must not block or noise-spam the user.
func Check(ctx context.Context, currentVersion string, opts Options) *Notice {
	// Nil ctx mirrors the silent-fail philosophy; would otherwise panic in
	// http.NewRequestWithContext.
	if ctx == nil {
		return nil
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}

	cachePath, cacheOK := resolveCachePath(opts.CacheDir)

	if cacheOK {
		// A future CheckedAt (clock skew at write time, then a wall-clock
		// rollback) would make now().Sub(...) negative and always satisfy
		// `< CacheTTL`, effectively freezing the notifier until the wall
		// clock caught up. Reject negatives so the next call refetches.
		if c, ok := readCache(cachePath); ok {
			since := now().Sub(c.CheckedAt)
			if since >= 0 && since < CacheTTL {
				return buildNotice(currentVersion, c.LatestVersion)
			}
		}
	}

	var fetchOpts []release.Option
	if opts.HTTPClient != nil {
		fetchOpts = append(fetchOpts, release.WithHTTPClient(opts.HTTPClient))
	}
	rel, err := release.FetchLatest(ctx, fetchOpts...)
	if err != nil {
		return nil
	}
	latest := strings.TrimPrefix(rel.TagName, "v")

	if cacheOK {
		// Cache write errors are silent — retry tomorrow.
		_ = writeCache(cachePath, cacheState{ //nolint:errcheck // silent fail by design (see package doc)
			CheckedAt:     now(),
			LatestVersion: latest,
			SchemaVersion: cacheSchemaVersion,
		})
	}

	return buildNotice(currentVersion, latest)
}

func buildNotice(currentVersion, latestVersion string) *Notice {
	current := strings.TrimPrefix(strings.TrimSpace(currentVersion), "v")
	latest := strings.TrimPrefix(strings.TrimSpace(latestVersion), "v")
	if current == "" || latest == "" {
		return nil
	}
	if release.Compare(latest, current) <= 0 {
		return nil
	}
	return &Notice{Current: current, Latest: latest}
}

type cacheState struct {
	CheckedAt     time.Time `json:"checked_at"`
	LatestVersion string    `json:"latest_version"`
	SchemaVersion int       `json:"schema_version"`
}

func resolveCachePath(override string) (string, bool) {
	if override != "" {
		return filepath.Join(override, cacheFile), true
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", false
	}
	return filepath.Join(base, "cocoon", cacheFile), true
}

func readCache(path string) (cacheState, bool) {
	var zero cacheState
	data, err := os.ReadFile(path) //nolint:gosec // path resolved via os.UserCacheDir or test-supplied tempdir.
	if err != nil {
		return zero, false
	}
	var c cacheState
	if err := json.Unmarshal(data, &c); err != nil {
		return zero, false
	}
	if c.SchemaVersion != cacheSchemaVersion {
		return zero, false
	}
	if c.LatestVersion == "" {
		return zero, false
	}
	return c, true
}

func writeCache(path string, c cacheState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir cache: %w", err)
	}
	data, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal cache: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write cache: %w", err)
	}
	return nil
}
