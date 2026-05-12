package release_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sukekyo26/cocoon/internal/release"
)

func TestFetchLatest_OK(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{
			"tag_name": "v9.9.9",
			"assets": [
				{"name": "cocoon-linux-amd64", "browser_download_url": "https://example/dl/bin"},
				{"name": "SHA256SUMS",         "browser_download_url": "https://example/dl/sums"}
			]
		}`)); err != nil {
			t.Errorf("server write: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	// Redirect API URL by serving on srv via a custom client transport that
	// rewrites the host. Simpler: only FetchLatest target is fixed, so we
	// monkey-patch via a RoundTripper.
	tr := &rewritingTransport{base: srv.Client().Transport, host: srv.URL}
	client := &http.Client{Transport: tr}

	rel, err := release.FetchLatest(context.Background(), release.WithHTTPClient(client))
	if err != nil {
		t.Fatalf("FetchLatest: %v", err)
	}
	if rel.TagName != "v9.9.9" {
		t.Errorf("TagName = %q, want v9.9.9", rel.TagName)
	}
	if got := rel.AssetURL("cocoon-linux-amd64"); got != "https://example/dl/bin" {
		t.Errorf("AssetURL = %q", got)
	}
	if got := rel.AssetURL("missing-asset"); got != "" {
		t.Errorf("missing asset URL = %q, want empty", got)
	}
}

func TestFetchLatest_HTTPErrorStatus(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	tr := &rewritingTransport{base: srv.Client().Transport, host: srv.URL}
	client := &http.Client{Transport: tr}

	_, err := release.FetchLatest(context.Background(), release.WithHTTPClient(client))
	if !errors.Is(err, release.ErrHTTPStatus) {
		t.Fatalf("err = %v, want wrap of ErrHTTPStatus", err)
	}
}

func TestFetchLatest_BadJSON(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(`{not-json`)); err != nil {
			t.Errorf("server write: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	tr := &rewritingTransport{base: srv.Client().Transport, host: srv.URL}
	client := &http.Client{Transport: tr}

	_, err := release.FetchLatest(context.Background(), release.WithHTTPClient(client))
	if err == nil {
		t.Fatal("err = nil, want decode error")
	}
}

func TestFetchLatest_ContextCancelled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := release.FetchLatest(ctx)
	if err == nil {
		t.Fatal("err = nil, want cancelled error")
	}
}

// TestFetchLatest_NilHTTPClientUsesDefault asserts WithHTTPClient(nil)
// is a no-op so the default *http.Client is retained. The earlier
// `cfg.client = c` form silently installed nil and panicked inside
// FetchLatest at `cfg.client.Do(req)`.
func TestFetchLatest_NilHTTPClientUsesDefault(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// We cannot exercise the real GitHub API in unit tests, but a
	// cancelled context still drives the request to cfg.client.Do —
	// which must not panic with a nil receiver. A non-panic error
	// return is sufficient.
	_, err := release.FetchLatest(ctx, release.WithHTTPClient(nil))
	if err == nil {
		t.Fatal("err = nil, want cancelled error from default client")
	}
}

// rewritingTransport rewrites api.github.com requests to the test server
// so FetchLatest's hardcoded URL routes through httptest.
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
