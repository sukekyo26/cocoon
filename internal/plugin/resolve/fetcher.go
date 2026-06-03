package resolve

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// ErrHTTPStatus lets callers match a non-2xx response via errors.Is rather
// than pattern-matching the wrapped status message.
var ErrHTTPStatus = errors.New("resolve: unexpected http status")

// ErrBodyTooLarge is returned when a response exceeds maxBody. Get reads
// maxBody+1 bytes and checks the length so an oversized or misconfigured
// endpoint fails fast here instead of being silently truncated and surfacing
// as a confusing downstream parse error.
var ErrBodyTooLarge = errors.New("resolve: response body exceeds size limit")

const (
	// defaultTimeout caps a single GET when the caller's context carries no
	// deadline so a stalled upstream cannot freeze `cocoon lock`.
	defaultTimeout = 30 * time.Second
	// maxBody bounds a response read so a hostile or misconfigured endpoint
	// cannot exhaust memory; version indexes and checksum files are tiny.
	maxBody = 8 << 20 // 8 MiB
)

// Fetcher performs an HTTP GET and returns the 2xx response body. Tests
// inject a fake so the resolver runs with zero network.
type Fetcher interface {
	Get(ctx context.Context, url string) ([]byte, error)
}

// HTTPFetcher is the production Fetcher. A nil Client uses http.DefaultClient.
// When ctx has no deadline a per-request defaultTimeout applies. When
// GITHUB_TOKEN is set it is sent as a bearer token to api.github.com to lift
// the unauthenticated rate limit (60 req/hr) that a large `cocoon lock` over
// many github-release plugins could otherwise hit.
type HTTPFetcher struct {
	Client *http.Client
}

// Get issues the request and returns the body, wrapping a non-2xx status in
// ErrHTTPStatus and any transport error with the URL for context.
func (f HTTPFetcher) Get(ctx context.Context, url string) ([]byte, error) {
	client := f.Client
	if client == nil {
		client = http.DefaultClient
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultTimeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request %s: %w", url, err)
	}
	req.Header.Set("Accept", "application/vnd.github+json, */*")
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" && strings.HasPrefix(url, "https://api.github.com/") {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("%w: %s for %s", ErrHTTPStatus, resp.Status, url)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, fmt.Errorf("read body %s: %w", url, err)
	}
	if int64(len(body)) > maxBody {
		return nil, fmt.Errorf("%w: %s (limit %d bytes)", ErrBodyTooLarge, url, maxBody)
	}
	return body, nil
}
