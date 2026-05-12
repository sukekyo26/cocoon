package updatecheck_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sukekyo26/cocoon/internal/updatecheck"
)

func serverServing(t *testing.T, tag string) *http.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"tag_name":"` + tag + `","assets":[]}`)); err != nil {
			t.Errorf("server write: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	tr := &rewritingTransport{base: srv.Client().Transport, host: srv.URL}
	return &http.Client{Transport: tr}
}

func TestCheck_FreshFetchNotifies(t *testing.T) {
	t.Parallel()
	client := serverServing(t, "v9.9.9")
	dir := t.TempDir()

	n := updatecheck.Check(context.Background(), "0.1.0", updatecheck.Options{
		CacheDir:   dir,
		HTTPClient: client,
		Now:        func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) },
	})
	if n == nil {
		t.Fatal("expected notice, got nil")
	}
	if n.Latest != "9.9.9" || n.Current != "0.1.0" {
		t.Errorf("notice = %+v", n)
	}
	if got := n.Format(); !contains(got, "v9.9.9") || !contains(got, "v0.1.0") {
		t.Errorf("Format() = %q", got)
	}

	cachePath := filepath.Join(dir, "update_check.json")
	if _, err := os.Stat(cachePath); err != nil {
		t.Errorf("cache file not written: %v", err)
	}
}

func TestCheck_SameVersionReturnsNil(t *testing.T) {
	t.Parallel()
	client := serverServing(t, "v0.2.0")
	dir := t.TempDir()

	n := updatecheck.Check(context.Background(), "0.2.0", updatecheck.Options{
		CacheDir:   dir,
		HTTPClient: client,
	})
	if n != nil {
		t.Errorf("expected nil for same version, got %+v", n)
	}
}

func TestCheck_OlderRemoteReturnsNil(t *testing.T) {
	t.Parallel()
	client := serverServing(t, "v0.1.0")
	dir := t.TempDir()

	n := updatecheck.Check(context.Background(), "0.2.0", updatecheck.Options{
		CacheDir:   dir,
		HTTPClient: client,
	})
	if n != nil {
		t.Errorf("expected nil for older remote, got %+v", n)
	}
}

func TestCheck_NetworkErrorIsSilent(t *testing.T) {
	t.Parallel()
	client := &http.Client{Transport: failingTransport{}}
	dir := t.TempDir()

	n := updatecheck.Check(context.Background(), "0.1.0", updatecheck.Options{
		CacheDir:   dir,
		HTTPClient: client,
	})
	if n != nil {
		t.Errorf("expected nil on network error, got %+v", n)
	}
	if _, err := os.Stat(filepath.Join(dir, "update_check.json")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("cache should not be written on error; stat = %v", err)
	}
}

func TestCheck_CacheHitSkipsFetch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "update_check.json")
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.WriteFile(cachePath, mustJSON(t, map[string]any{
		"checked_at":     time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		"latest_version": "9.9.9",
		"schema_version": 1,
	}), 0o600))

	// HTTPClient is a failing transport; if Check tried to fetch, it
	// would return nil. The cache hit must short-circuit it.
	client := &http.Client{Transport: failingTransport{}}
	n := updatecheck.Check(context.Background(), "0.1.0", updatecheck.Options{
		CacheDir:   dir,
		HTTPClient: client,
		Now:        func() time.Time { return time.Date(2026, 1, 1, 23, 59, 0, 0, time.UTC) },
	})
	if n == nil {
		t.Fatal("expected notice from cache, got nil")
	}
	if n.Latest != "9.9.9" {
		t.Errorf("Latest = %q", n.Latest)
	}
}

func TestCheck_CacheExpiredRefetches(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "update_check.json")
	if err := os.WriteFile(cachePath, mustJSON(t, map[string]any{
		"checked_at":     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		"latest_version": "0.1.0", // stale: older than current
		"schema_version": 1,
	}), 0o600); err != nil {
		t.Fatal(err)
	}

	client := serverServing(t, "v9.9.9")
	n := updatecheck.Check(context.Background(), "0.5.0", updatecheck.Options{
		CacheDir:   dir,
		HTTPClient: client,
		// 24h + 1s past stored checked_at — cache is stale.
		Now: func() time.Time { return time.Date(2026, 1, 2, 0, 0, 1, 0, time.UTC) },
	})
	if n == nil || n.Latest != "9.9.9" {
		t.Fatalf("expected refetched notice 9.9.9, got %+v", n)
	}
}

func TestCheck_FutureTimestampRefetches(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "update_check.json")
	// CheckedAt 1h in the "future" relative to Now(). A naive
	// now().Sub(CheckedAt) returns a negative duration that is always
	// `< CacheTTL`, so without the >=0 guard the stale cache would win
	// indefinitely. Verify the network path runs instead.
	if err := os.WriteFile(cachePath, mustJSON(t, map[string]any{
		"checked_at":     time.Date(2026, 1, 1, 13, 0, 0, 0, time.UTC),
		"latest_version": "0.1.0", // stale: older than current.
		"schema_version": 1,
	}), 0o600); err != nil {
		t.Fatal(err)
	}
	client := serverServing(t, "v9.9.9")
	n := updatecheck.Check(context.Background(), "0.5.0", updatecheck.Options{
		CacheDir:   dir,
		HTTPClient: client,
		Now:        func() time.Time { return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC) },
	})
	if n == nil || n.Latest != "9.9.9" {
		t.Fatalf("expected refetched notice 9.9.9, got %+v", n)
	}
}

func TestCheck_MalformedCacheRefetches(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "update_check.json")
	if err := os.WriteFile(cachePath, []byte(`{not-json`), 0o600); err != nil {
		t.Fatal(err)
	}
	client := serverServing(t, "v9.9.9")
	n := updatecheck.Check(context.Background(), "0.1.0", updatecheck.Options{
		CacheDir:   dir,
		HTTPClient: client,
	})
	if n == nil {
		t.Fatal("expected refetched notice on malformed cache")
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && indexOf(s, sub) >= 0 }

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// rewritingTransport reroutes api.github.com to the test server.
type rewritingTransport struct {
	base http.RoundTripper
	host string
}

func (t *rewritingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = "http"
	clone.URL.Host = trimScheme(t.host)
	clone.Host = clone.URL.Host
	return t.base.RoundTrip(clone) //nolint:wrapcheck // test transport
}

func trimScheme(u string) string {
	for _, p := range []string{"http://", "https://"} {
		if len(u) > len(p) && u[:len(p)] == p {
			return u[len(p):]
		}
	}
	return u
}

type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("network down")
}
