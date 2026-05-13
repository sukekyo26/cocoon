// Package release is the single GitHub Releases lookup shared by both
// `cocoon self-update` and the per-invocation update notifier so the API
// call cannot drift out of sync.
package release

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const (
	// RepoSlug is the GitHub repo cocoon releases are published to.
	RepoSlug = "sukekyo26/cocoon"
	// DefaultTimeout caps a single API roundtrip when no explicit timeout
	// is configured in the request's context.
	DefaultTimeout = 30 * time.Second
)

// ErrHTTPStatus lets callers match non-2xx via errors.Is rather than
// pattern-matching the wrapped status message.
var ErrHTTPStatus = errors.New("unexpected http status")

// Asset is a single uploaded artifact attached to a GitHub release.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// Release is the subset of the GitHub Releases payload cocoon needs.
type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

// AssetURL returns the download URL for the named asset, or "" when no
// asset with that name is attached to the release.
func (r *Release) AssetURL(name string) string {
	for _, a := range r.Assets {
		if a.Name == name {
			return a.URL
		}
	}
	return ""
}

// Option configures FetchLatest. Pass WithHTTPClient in tests to inject
// an httptest.Server's client.
type Option func(*fetchConfig)

type fetchConfig struct {
	client *http.Client
}

// WithHTTPClient treats nil as "use the default" so callers can forward
// an optional client without a nil check.
func WithHTTPClient(c *http.Client) Option {
	return func(cfg *fetchConfig) {
		if c == nil {
			return
		}
		cfg.client = c
	}
}

// FetchLatest applies DefaultTimeout when ctx has no deadline so a stalled
// GitHub API cannot freeze interactive paths.
func FetchLatest(ctx context.Context, opts ...Option) (*Release, error) {
	cfg := fetchConfig{client: http.DefaultClient}
	for _, o := range opts {
		o(&cfg)
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultTimeout)
		defer cancel()
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", RepoSlug)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := cfg.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("%w: github releases api: %s", ErrHTTPStatus, resp.Status)
	}
	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}
	return &rel, nil
}
