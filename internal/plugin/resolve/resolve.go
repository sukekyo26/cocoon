// Package resolve turns a plugin's [version.source] declaration into a
// concrete version and per-arch SHA256 checksums for `cocoon lock`. It is the
// only place cocoon performs network I/O at lock time; `cocoon gen` stays
// offline and consumes the lock the resolver produces. All network access
// goes through an injected Fetcher so the package is unit-testable offline.
package resolve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/sukekyo26/cocoon/internal/plugin"
)

// Resolution sentinels (exported, errors.Is-matchable).
var (
	// ErrLatestUnsupported is returned when "latest" is requested for a plugin
	// with no [version.source] to resolve it.
	ErrLatestUnsupported = errors.New(
		"resolve: cannot resolve 'latest' without a [version.source] (pin an exact version)")
	// ErrChecksumNotFound is returned when the artifact is absent from the
	// fetched checksum manifest.
	ErrChecksumNotFound = errors.New("resolve: artifact not found in checksum manifest")
	// ErrUnknownLatestType / ErrUnknownChecksumType guard the closed kind sets.
	ErrUnknownLatestType   = errors.New("resolve: unknown [version.source.latest] type")
	ErrUnknownChecksumType = errors.New("resolve: unknown [version.source.checksum] type")
	// ErrBadChecksum is returned when a fetched checksum is not 64 hex chars
	// (e.g. a CDN served HTML on a 200) — fail loud, never lock garbage.
	ErrBadChecksum = errors.New("resolve: fetched value is not a 64-char hex sha256")
	// ErrEmptyVersion is returned when the upstream yields no version string.
	ErrEmptyVersion = errors.New("resolve: upstream returned an empty version")
)

var rxSha256 = regexp.MustCompile(`^[a-f0-9]{64}$`)

// Resolved is the outcome for one plugin: a concrete version plus per-arch
// checksums (nil when the source records none — pgp plugins, installer-only
// plugins, or arches the source could not produce).
type Resolved struct {
	Version       string
	ChecksumAMD64 *string
	ChecksumARM64 *string
}

// Request is one plugin's resolution ask. When IsLatest is false, Version is
// the exact pin: its source strip_prefix is applied (so "v1.2.3" and "1.2.3"
// resolve identically and never double up against a "v${version}" template),
// and the checksum is still fetched for that version when the source declares
// one.
type Request struct {
	ID       string
	Source   *plugin.VersionSource
	Version  string
	IsLatest bool
	Arches   []string
}

// Resolver performs lock-time resolution against an injected Fetcher.
type Resolver struct {
	fetch Fetcher
}

// New returns a Resolver backed by fetch.
func New(fetch Fetcher) *Resolver {
	return &Resolver{fetch: fetch}
}

// get wraps the injected Fetcher so callers return its error directly while
// still attaching the URL for context (and satisfying the wrap-once contract).
func (r *Resolver) get(ctx context.Context, url string) ([]byte, error) {
	body, err := r.fetch.Get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	return body, nil
}

// Resolve produces the concrete version and checksums for req. When
// req.IsLatest it discovers the newest version via the source's latest kind;
// otherwise req.Version is used. Checksums are fetched per arch unless the
// source's checksum kind is "none" (or the plugin has no source).
func (r *Resolver) Resolve(ctx context.Context, req Request) (Resolved, error) {
	version := req.Version
	if req.IsLatest {
		if req.Source == nil {
			return Resolved{}, fmt.Errorf("%s: %w", req.ID, ErrLatestUnsupported) //nolint:exhaustruct // error path
		}
		v, err := r.resolveLatest(ctx, req.Source.Latest)
		if err != nil {
			return Resolved{}, fmt.Errorf("%s: %w", req.ID, err) //nolint:exhaustruct // error path
		}
		version = v
	} else if req.Source != nil {
		// Normalize an exact pin the same way a discovered "latest" is, so a
		// user-supplied prefix (e.g. "v1.2.3") does not double up against a
		// template that already encodes it ("v${version}").
		version = stripPrefix(req.Version, req.Source.Latest.StripPrefix)
	}
	res := Resolved{Version: version} //nolint:exhaustruct // checksums filled below
	if req.Source == nil || req.Source.Checksum.Type == "" || req.Source.Checksum.Type == plugin.ChecksumNone {
		return res, nil
	}
	for _, arch := range req.Arches {
		sum, err := r.resolveChecksum(ctx, req.Source, version, arch)
		if err != nil {
			return Resolved{}, fmt.Errorf("%s/%s: %w", req.ID, arch, err) //nolint:exhaustruct // error path
		}
		switch arch {
		case "amd64":
			res.ChecksumAMD64 = &sum
		case "arm64":
			res.ChecksumARM64 = &sum
		}
	}
	return res, nil
}

func (r *Resolver) resolveLatest(ctx context.Context, l plugin.LatestSpec) (string, error) {
	switch l.Type {
	case plugin.LatestGitHubRelease:
		return r.latestGitHub(ctx, l)
	case plugin.LatestText:
		return r.latestText(ctx, l)
	case plugin.LatestJSONField:
		return r.latestJSON(ctx, l)
	case plugin.LatestTab:
		return r.latestTab(ctx, l)
	default:
		return "", fmt.Errorf("%q: %w", l.Type, ErrUnknownLatestType)
	}
}

func (r *Resolver) latestGitHub(ctx context.Context, l plugin.LatestSpec) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", l.Repo)
	body, err := r.get(ctx, url)
	if err != nil {
		return "", err
	}
	var rel struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &rel); err != nil {
		return "", fmt.Errorf("decode github release: %w", err)
	}
	return nonEmpty(stripPrefix(rel.TagName, l.StripPrefix))
}

func (r *Resolver) latestText(ctx context.Context, l plugin.LatestSpec) (string, error) {
	body, err := r.get(ctx, l.URL)
	if err != nil {
		return "", err
	}
	return nonEmpty(stripPrefix(firstLine(body), l.StripPrefix))
}

func (r *Resolver) latestJSON(ctx context.Context, l plugin.LatestSpec) (string, error) {
	body, err := r.get(ctx, l.URL)
	if err != nil {
		return "", err
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", fmt.Errorf("decode json: %w", err)
	}
	v, ok := jsonField(doc, l.Field)
	if !ok {
		return "", fmt.Errorf("field %q: %w", l.Field, ErrEmptyVersion)
	}
	return nonEmpty(stripPrefix(v, l.StripPrefix))
}

func (r *Resolver) latestTab(ctx context.Context, l plugin.LatestSpec) (string, error) {
	body, err := r.get(ctx, l.URL)
	if err != nil {
		return "", err
	}
	return nonEmpty(stripPrefix(firstTabVersion(body, l.LTSOnly), l.StripPrefix))
}

func (r *Resolver) resolveChecksum(
	ctx context.Context, src *plugin.VersionSource, version, arch string,
) (string, error) {
	archToken, ok := src.Arch[arch]
	if !ok {
		archToken = arch
	}
	switch src.Checksum.Type {
	case plugin.ChecksumSidecar:
		return r.checksumSidecar(ctx, src.Checksum, version, archToken)
	case plugin.ChecksumShasumsFile:
		return r.checksumShasums(ctx, src.Checksum, version, archToken)
	default:
		return "", fmt.Errorf("%q: %w", src.Checksum.Type, ErrUnknownChecksumType)
	}
}

func (r *Resolver) checksumSidecar(
	ctx context.Context, c plugin.ChecksumSpec, version, arch string,
) (string, error) {
	body, err := r.get(ctx, expand(c.AssetURL, version, arch)+c.Suffix)
	if err != nil {
		return "", err
	}
	// The sidecar body is a bare hash, optionally followed by a filename.
	fields := strings.Fields(string(body))
	if len(fields) == 0 {
		return "", ErrChecksumNotFound
	}
	return validateHash(fields[0])
}

func (r *Resolver) checksumShasums(
	ctx context.Context, c plugin.ChecksumSpec, version, arch string,
) (string, error) {
	body, err := r.get(ctx, expand(c.ManifestURL, version, arch))
	if err != nil {
		return "", err
	}
	want := expand(c.AssetName, version, arch)
	for _, ln := range strings.Split(string(body), "\n") {
		fields := strings.Fields(ln)
		if len(fields) < 2 {
			continue
		}
		// Strip the binary-mode "*" marker some manifests prepend to the name.
		if strings.TrimPrefix(fields[1], "*") == want {
			return validateHash(fields[0])
		}
	}
	return "", fmt.Errorf("%q: %w", want, ErrChecksumNotFound)
}

// expand substitutes the ${version} / ${arch} placeholders literally (no
// shell), so a malformed template cannot inject anything beyond a 404.
func expand(tmpl, version, arch string) string {
	return strings.NewReplacer("${version}", version, "${arch}", arch).Replace(tmpl)
}

func stripPrefix(v, prefix string) string {
	if prefix != "" {
		v = strings.TrimPrefix(v, prefix)
	}
	return strings.TrimSpace(v)
}

func nonEmpty(v string) (string, error) {
	if v == "" {
		return "", ErrEmptyVersion
	}
	return v, nil
}

func validateHash(h string) (string, error) {
	h = strings.ToLower(strings.TrimSpace(h))
	if !rxSha256.MatchString(h) {
		return "", fmt.Errorf("%q: %w", h, ErrBadChecksum)
	}
	return h, nil
}

func firstLine(body []byte) string {
	s := string(body)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// firstTabVersion parses a Node-style dist/index.tab: a header row followed
// by tab-separated rows newest-first. Column 0 is the version; column 9 is
// the LTS codename ("-" for non-LTS lines). With ltsOnly it returns the first
// LTS row; otherwise the first data row. The leading "v" is stripped.
func firstTabVersion(body []byte, ltsOnly bool) string {
	for i, ln := range strings.Split(string(body), "\n") {
		if i == 0 || strings.TrimSpace(ln) == "" {
			continue
		}
		cols := strings.Split(ln, "\t")
		if len(cols) == 0 {
			continue
		}
		if ltsOnly && (len(cols) < 10 || cols[9] == "-") {
			continue
		}
		return strings.TrimPrefix(strings.TrimSpace(cols[0]), "v")
	}
	return ""
}

// jsonField walks a dotted path (e.g. "current_version" or "a.b.c") through a
// decoded JSON object and returns the string leaf, or ok=false if any segment
// is missing or the leaf is not a string.
func jsonField(doc map[string]any, dotted string) (string, bool) {
	var cur any = doc
	for _, part := range strings.Split(dotted, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return "", false
		}
		cur, ok = m[part]
		if !ok {
			return "", false
		}
	}
	s, ok := cur.(string)
	return s, ok
}
