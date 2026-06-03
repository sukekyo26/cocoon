package resolve_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sukekyo26/cocoon/internal/plugin/resolve"
)

// TestHTTPFetcher_Get covers the body contract: a small 2xx body is returned
// verbatim, an oversized body fails fast with ErrBodyTooLarge (rather than
// being silently truncated), and a non-2xx status maps to ErrHTTPStatus.
func TestHTTPFetcher_Get(t *testing.T) {
	t.Parallel()

	t.Run("small_body_ok", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("v1.2.3\n")) //nolint:errcheck // test mock server response
		}))
		t.Cleanup(srv.Close)
		body, err := resolve.HTTPFetcher{}.Get(context.Background(), srv.URL)
		require.NoError(t, err)
		require.Equal(t, "v1.2.3\n", string(body))
	})

	t.Run("oversized_body_fails_fast", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(make([]byte, 9<<20)) //nolint:errcheck // 9 MiB > maxBody; client closes early so a broken pipe is expected
		}))
		t.Cleanup(srv.Close)
		_, err := resolve.HTTPFetcher{}.Get(context.Background(), srv.URL)
		require.ErrorIs(t, err, resolve.ErrBodyTooLarge)
	})

	t.Run("non_2xx_is_ErrHTTPStatus", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		t.Cleanup(srv.Close)
		_, err := resolve.HTTPFetcher{}.Get(context.Background(), srv.URL)
		require.ErrorIs(t, err, resolve.ErrHTTPStatus)
	})
}
