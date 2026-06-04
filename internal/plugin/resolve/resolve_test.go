package resolve_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sukekyo26/cocoon/internal/plugin"
	"github.com/sukekyo26/cocoon/internal/plugin/resolve"
)

const (
	shaA = "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2"
	shaB = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
)

// errNoStub stands in for a 404 when a test hits a URL it did not stub.
var errNoStub = errors.New("mapFetcher: no stub")

// mapFetcher is an offline Fetcher: each URL maps to a body or an error.
type mapFetcher struct {
	bodies map[string]string
	errs   map[string]error
}

func (m mapFetcher) Get(_ context.Context, url string) ([]byte, error) {
	if err, ok := m.errs[url]; ok {
		return nil, err
	}
	if b, ok := m.bodies[url]; ok {
		return []byte(b), nil
	}
	return nil, fmt.Errorf("%w for %s", errNoStub, url)
}

func ptr(s string) *string { return &s }

func TestResolve_GitHubReleasePlusShasums(t *testing.T) {
	t.Parallel()
	src := &plugin.VersionSource{
		Latest:   plugin.LatestSpec{Type: plugin.LatestGitHubRelease, Repo: "casey/just", StripPrefix: ""},                                                                                                                    //nolint:exhaustruct // optional fields
		Checksum: plugin.ChecksumSpec{Type: plugin.ChecksumShasumsFile, ManifestURL: "https://github.com/casey/just/releases/download/${version}/SHA256SUMS", AssetName: "just-${version}-${arch}-unknown-linux-musl.tar.gz"}, //nolint:exhaustruct // optional fields
		Arch:     map[string]string{"amd64": "x86_64", "arm64": "aarch64"},
	}
	f := mapFetcher{
		bodies: map[string]string{
			"https://api.github.com/repos/casey/just/releases/latest": `{"tag_name":"1.51.0"}`,
			"https://github.com/casey/just/releases/download/1.51.0/SHA256SUMS": shaA + "  just-1.51.0-x86_64-unknown-linux-musl.tar.gz\n" +
				shaB + "  just-1.51.0-aarch64-unknown-linux-musl.tar.gz\n",
		},
		errs: nil,
	}
	got, err := resolve.New(f).Resolve(context.Background(), resolve.Request{
		ID: "just", Source: src, IsLatest: true, Arches: []string{"amd64", "arm64"}, Version: "",
	})
	require.NoError(t, err)
	require.Equal(t, "1.51.0", got.Version)
	require.Equal(t, ptr(shaA), got.ChecksumAMD64)
	require.Equal(t, ptr(shaB), got.ChecksumARM64)
}

func TestResolve_TextPlusSidecar(t *testing.T) {
	t.Parallel()
	src := &plugin.VersionSource{
		Latest:   plugin.LatestSpec{Type: plugin.LatestText, URL: "https://go.dev/VERSION?m=text", StripPrefix: "go"},                                          //nolint:exhaustruct // optional fields
		Checksum: plugin.ChecksumSpec{Type: plugin.ChecksumSidecar, AssetURL: "https://dl.google.com/go/go${version}.linux-${arch}.tar.gz", Suffix: ".sha256"}, //nolint:exhaustruct // optional fields
		Arch:     map[string]string{"amd64": "amd64", "arm64": "arm64"},
	}
	f := mapFetcher{
		bodies: map[string]string{
			"https://go.dev/VERSION?m=text":                               "go1.23.4\ntime 2024-01-01\n",
			"https://dl.google.com/go/go1.23.4.linux-amd64.tar.gz.sha256": shaA,
			"https://dl.google.com/go/go1.23.4.linux-arm64.tar.gz.sha256": shaB + "  go1.23.4.linux-arm64.tar.gz\n",
		},
		errs: nil,
	}
	got, err := resolve.New(f).Resolve(context.Background(), resolve.Request{
		ID: "go", Source: src, IsLatest: true, Arches: []string{"amd64", "arm64"}, Version: "",
	})
	require.NoError(t, err)
	require.Equal(t, "1.23.4", got.Version)
	require.Equal(t, ptr(shaA), got.ChecksumAMD64)
	require.Equal(t, ptr(shaB), got.ChecksumARM64)
}

func TestResolve_JSONFieldNoChecksum(t *testing.T) {
	t.Parallel()
	src := &plugin.VersionSource{
		Latest:   plugin.LatestSpec{Type: plugin.LatestJSONField, URL: "https://checkpoint-api.hashicorp.com/v1/check/terraform", Field: "current_version"}, //nolint:exhaustruct // optional fields
		Checksum: plugin.ChecksumSpec{Type: plugin.ChecksumNone},                                                                                            //nolint:exhaustruct // none has no fields
		Arch:     nil,
	}
	f := mapFetcher{
		bodies: map[string]string{
			"https://checkpoint-api.hashicorp.com/v1/check/terraform": `{"product":"terraform","current_version":"1.10.5"}`,
		},
		errs: nil,
	}
	got, err := resolve.New(f).Resolve(context.Background(), resolve.Request{
		ID: "terraform", Source: src, IsLatest: true, Arches: []string{"amd64", "arm64"}, Version: "",
	})
	require.NoError(t, err)
	require.Equal(t, "1.10.5", got.Version)
	require.Nil(t, got.ChecksumAMD64)
	require.Nil(t, got.ChecksumARM64)
}

func TestResolve_TabLTS(t *testing.T) {
	t.Parallel()
	src := &plugin.VersionSource{
		Latest:   plugin.LatestSpec{Type: plugin.LatestTab, URL: "https://nodejs.org/dist/index.tab", LTSOnly: true}, //nolint:exhaustruct // optional fields
		Checksum: plugin.ChecksumSpec{Type: plugin.ChecksumNone},                                                     //nolint:exhaustruct // none
		Arch:     map[string]string{"amd64": "x64", "arm64": "arm64"},
	}
	// header + a non-LTS newest row + the first LTS row.
	tab := "version\tdate\tfiles\tnpm\tv8\tuv\tzlib\topenssl\tmodules\tlts\tsecurity\n" +
		"v23.5.0\t2024-12-01\t-\t-\t-\t-\t-\t-\t-\t-\tfalse\n" +
		"v22.15.0\t2024-11-01\t-\t-\t-\t-\t-\t-\t-\tJod\tfalse\n"
	f := mapFetcher{bodies: map[string]string{"https://nodejs.org/dist/index.tab": tab}, errs: nil}
	got, err := resolve.New(f).Resolve(context.Background(), resolve.Request{
		ID: "node", Source: src, IsLatest: true, Arches: []string{"amd64"}, Version: "",
	})
	require.NoError(t, err)
	require.Equal(t, "22.15.0", got.Version)
}

func TestResolve_ExactPinFetchesChecksum(t *testing.T) {
	t.Parallel()
	src := &plugin.VersionSource{
		Latest:   plugin.LatestSpec{Type: plugin.LatestText, URL: "https://go.dev/VERSION?m=text", StripPrefix: "go"},                                          //nolint:exhaustruct // optional fields
		Checksum: plugin.ChecksumSpec{Type: plugin.ChecksumSidecar, AssetURL: "https://dl.google.com/go/go${version}.linux-${arch}.tar.gz", Suffix: ".sha256"}, //nolint:exhaustruct // optional
		Arch:     map[string]string{"amd64": "amd64"},
	}
	// No latest stub: an exact pin must NOT call the latest endpoint.
	f := mapFetcher{
		bodies: map[string]string{
			"https://dl.google.com/go/go1.22.5.linux-amd64.tar.gz.sha256": shaA,
		},
		errs: nil,
	}
	got, err := resolve.New(f).Resolve(context.Background(), resolve.Request{
		ID: "go", Source: src, IsLatest: false, Version: "1.22.5", Arches: []string{"amd64"},
	})
	require.NoError(t, err)
	require.Equal(t, "1.22.5", got.Version)
	require.Equal(t, ptr(shaA), got.ChecksumAMD64)
}

// TestResolve_ExactPinStripsPrefix pins the Request.Version contract: an exact
// pin carrying the source's prefix ("v1.30.0") is normalized exactly like a
// discovered "latest", so the checksum URL gets a single prefix ("v1.30.0")
// rather than the double-prefix ("vv1.30.0") a verbatim substitution produces.
func TestResolve_ExactPinStripsPrefix(t *testing.T) {
	t.Parallel()
	src := &plugin.VersionSource{
		Latest:   plugin.LatestSpec{Type: plugin.LatestText, URL: "https://example.test/VERSION", StripPrefix: "v"},                                           //nolint:exhaustruct // optional fields
		Checksum: plugin.ChecksumSpec{Type: plugin.ChecksumSidecar, AssetURL: "https://dl.k8s.io/release/v${version}/bin/${arch}/kubectl", Suffix: ".sha256"}, //nolint:exhaustruct // optional
		Arch:     map[string]string{"amd64": "amd64"},
	}
	// Only the single-prefix URL is stubbed: a double-prefix "vv1.30.0" request
	// would miss and fail, so a passing test proves the prefix was stripped.
	f := mapFetcher{
		bodies: map[string]string{
			"https://dl.k8s.io/release/v1.30.0/bin/amd64/kubectl.sha256": shaA,
		},
		errs: nil,
	}
	got, err := resolve.New(f).Resolve(context.Background(), resolve.Request{
		ID: "kubectl", Source: src, IsLatest: false, Version: "v1.30.0", Arches: []string{"amd64"},
	})
	require.NoError(t, err)
	require.Equal(t, "1.30.0", got.Version) // recorded clean, no leading "v"
	require.Equal(t, ptr(shaA), got.ChecksumAMD64)
}

func TestResolve_NoSourceLatestUnsupported(t *testing.T) {
	t.Parallel()
	_, err := resolve.New(mapFetcher{bodies: nil, errs: nil}).Resolve(context.Background(), resolve.Request{
		ID: "aws-cli", Source: nil, IsLatest: true, Arches: []string{"amd64"}, Version: "",
	})
	require.ErrorIs(t, err, resolve.ErrLatestUnsupported)
}

func TestResolve_Errors(t *testing.T) {
	t.Parallel()
	goSrc := func(latest plugin.LatestSpec, checksum plugin.ChecksumSpec) *plugin.VersionSource {
		return &plugin.VersionSource{Latest: latest, Checksum: checksum, Arch: map[string]string{"amd64": "amd64"}}
	}
	sidecar := plugin.ChecksumSpec{Type: plugin.ChecksumSidecar, AssetURL: "https://x.test/${version}-${arch}.tar.gz", Suffix: ".sha256"} //nolint:exhaustruct // optional
	textLatest := plugin.LatestSpec{Type: plugin.LatestText, URL: "https://x.test/VERSION"}                                               //nolint:exhaustruct // optional

	t.Run("unknown_latest_type", func(t *testing.T) {
		t.Parallel()
		src := goSrc(plugin.LatestSpec{Type: "wat"}, plugin.ChecksumSpec{Type: plugin.ChecksumNone}) //nolint:exhaustruct // optional
		_, err := resolve.New(mapFetcher{bodies: nil, errs: nil}).Resolve(context.Background(), resolve.Request{
			ID: "p", Source: src, IsLatest: true, Arches: []string{"amd64"}, Version: "",
		})
		require.ErrorIs(t, err, resolve.ErrUnknownLatestType)
	})
	t.Run("unknown_checksum_type", func(t *testing.T) {
		t.Parallel()
		src := goSrc(textLatest, plugin.ChecksumSpec{Type: "wat"}) //nolint:exhaustruct // optional
		f := mapFetcher{bodies: map[string]string{"https://x.test/VERSION": "1.0.0"}, errs: nil}
		_, err := resolve.New(f).Resolve(context.Background(), resolve.Request{
			ID: "p", Source: src, IsLatest: true, Arches: []string{"amd64"}, Version: "",
		})
		require.ErrorIs(t, err, resolve.ErrUnknownChecksumType)
	})
	t.Run("checksum_not_found", func(t *testing.T) {
		t.Parallel()
		src := goSrc(textLatest, plugin.ChecksumSpec{Type: plugin.ChecksumShasumsFile, ManifestURL: "https://x.test/SHA", AssetName: "p-${version}.tgz"}) //nolint:exhaustruct // optional
		f := mapFetcher{bodies: map[string]string{
			"https://x.test/VERSION": "1.0.0",
			"https://x.test/SHA":     shaA + "  some-other-file.tgz\n",
		}, errs: nil}
		_, err := resolve.New(f).Resolve(context.Background(), resolve.Request{
			ID: "p", Source: src, IsLatest: true, Arches: []string{"amd64"}, Version: "",
		})
		require.ErrorIs(t, err, resolve.ErrChecksumNotFound)
	})
	t.Run("bad_checksum_html_on_200", func(t *testing.T) {
		t.Parallel()
		src := goSrc(textLatest, sidecar)
		f := mapFetcher{bodies: map[string]string{
			"https://x.test/VERSION":                   "1.0.0",
			"https://x.test/1.0.0-amd64.tar.gz.sha256": "<!DOCTYPE html><html>404</html>",
		}, errs: nil}
		_, err := resolve.New(f).Resolve(context.Background(), resolve.Request{
			ID: "p", Source: src, IsLatest: true, Arches: []string{"amd64"}, Version: "",
		})
		require.ErrorIs(t, err, resolve.ErrBadChecksum)
	})
	t.Run("transport_error", func(t *testing.T) {
		t.Parallel()
		boom := errors.New("connection refused")
		src := goSrc(textLatest, plugin.ChecksumSpec{Type: plugin.ChecksumNone}) //nolint:exhaustruct // none
		f := mapFetcher{bodies: nil, errs: map[string]error{"https://x.test/VERSION": boom}}
		_, err := resolve.New(f).Resolve(context.Background(), resolve.Request{
			ID: "p", Source: src, IsLatest: true, Arches: []string{"amd64"}, Version: "",
		})
		require.ErrorIs(t, err, boom)
	})
	t.Run("empty_version", func(t *testing.T) {
		t.Parallel()
		src := goSrc(textLatest, plugin.ChecksumSpec{Type: plugin.ChecksumNone}) //nolint:exhaustruct // none
		f := mapFetcher{bodies: map[string]string{"https://x.test/VERSION": "\n"}, errs: nil}
		_, err := resolve.New(f).Resolve(context.Background(), resolve.Request{
			ID: "p", Source: src, IsLatest: true, Arches: []string{"amd64"}, Version: "",
		})
		require.ErrorIs(t, err, resolve.ErrEmptyVersion)
	})
}

// TestResolve_ShasumsBinaryMarker pins that a "*"-prefixed binary-mode entry
// in a SHASUMS manifest matches the asset name with the marker stripped.
func TestResolve_ShasumsBinaryMarker(t *testing.T) {
	t.Parallel()
	src := &plugin.VersionSource{
		Latest:   plugin.LatestSpec{Type: plugin.LatestText, URL: "https://x.test/VERSION"},                                                          //nolint:exhaustruct // optional
		Checksum: plugin.ChecksumSpec{Type: plugin.ChecksumShasumsFile, ManifestURL: "https://x.test/SHASUMS", AssetName: "node-${version}-${arch}"}, //nolint:exhaustruct // optional
		Arch:     map[string]string{"amd64": "x64"},
	}
	f := mapFetcher{bodies: map[string]string{
		"https://x.test/VERSION": "1.0.0",
		"https://x.test/SHASUMS": shaA + " *node-1.0.0-x64\n",
	}, errs: nil}
	got, err := resolve.New(f).Resolve(context.Background(), resolve.Request{
		ID: "node", Source: src, IsLatest: true, Arches: []string{"amd64"}, Version: "",
	})
	require.NoError(t, err)
	require.Equal(t, ptr(shaA), got.ChecksumAMD64)
}

// TestHTTPFetcher exercises the production Fetcher against a local server so
// the 2xx-body and non-2xx-status paths are covered without real network.
func TestHTTPFetcher(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok" {
			_, _ = w.Write([]byte("body-here")) //nolint:errcheck // test mock server response
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	f := resolve.HTTPFetcher{Client: srv.Client()}

	body, err := f.Get(context.Background(), srv.URL+"/ok")
	require.NoError(t, err)
	require.Equal(t, "body-here", string(body))

	_, err = f.Get(context.Background(), srv.URL+"/missing")
	require.ErrorIs(t, err, resolve.ErrHTTPStatus)
}

// TestResolve_TabNewestNonLTS pins the non-LTS path: with LTSOnly = false the
// first data row (newest) is returned regardless of the LTS column.
func TestResolve_TabNewestNonLTS(t *testing.T) {
	t.Parallel()
	src := &plugin.VersionSource{
		Latest:   plugin.LatestSpec{Type: plugin.LatestTab, URL: "https://x.test/index.tab"}, //nolint:exhaustruct // optional
		Checksum: plugin.ChecksumSpec{Type: plugin.ChecksumNone},                             //nolint:exhaustruct // none
		Arch:     nil,
	}
	tab := "version\tlts\nv23.5.0\t-\nv22.15.0\tJod\n"
	f := mapFetcher{bodies: map[string]string{"https://x.test/index.tab": tab}, errs: nil}
	got, err := resolve.New(f).Resolve(context.Background(), resolve.Request{
		ID: "node", Source: src, IsLatest: true, Arches: []string{"amd64"}, Version: "",
	})
	require.NoError(t, err)
	require.Equal(t, "23.5.0", got.Version)
}

// TestResolve_DecodeAndFieldErrors covers malformed-upstream paths: invalid
// GitHub JSON and a json-field whose dotted path is absent.
func TestResolve_DecodeAndFieldErrors(t *testing.T) {
	t.Parallel()
	t.Run("github_bad_json", func(t *testing.T) {
		t.Parallel()
		src := &plugin.VersionSource{
			Latest:   plugin.LatestSpec{Type: plugin.LatestGitHubRelease, Repo: "a/b"}, //nolint:exhaustruct // optional
			Checksum: plugin.ChecksumSpec{Type: plugin.ChecksumNone},                   //nolint:exhaustruct // none
			Arch:     nil,
		}
		f := mapFetcher{bodies: map[string]string{"https://api.github.com/repos/a/b/releases/latest": "not-json"}, errs: nil}
		_, err := resolve.New(f).Resolve(context.Background(), resolve.Request{
			ID: "x", Source: src, IsLatest: true, Arches: []string{"amd64"}, Version: "",
		})
		require.Error(t, err)
	})
	t.Run("jsonfield_absent", func(t *testing.T) {
		t.Parallel()
		src := &plugin.VersionSource{
			Latest:   plugin.LatestSpec{Type: plugin.LatestJSONField, URL: "https://x.test/v", Field: "nested.version"}, //nolint:exhaustruct // optional
			Checksum: plugin.ChecksumSpec{Type: plugin.ChecksumNone},                                                    //nolint:exhaustruct // none
			Arch:     nil,
		}
		f := mapFetcher{bodies: map[string]string{"https://x.test/v": `{"other":"1.0"}`}, errs: nil}
		_, err := resolve.New(f).Resolve(context.Background(), resolve.Request{
			ID: "x", Source: src, IsLatest: true, Arches: []string{"amd64"}, Version: "",
		})
		require.ErrorIs(t, err, resolve.ErrEmptyVersion)
	})
}
