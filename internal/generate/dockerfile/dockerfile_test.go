package dockerfile_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/generate/dockerfile"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

// updateGolden, when set with `go test -update-golden`, rewrites the
// testdata/*.expected files from the current generator output instead of
// asserting against them. This is the cocoon-wide pattern for regen-ing
// snapshots after intentional generator changes; only this package wires
// it up today, but other generators that grow snapshot suites should
// adopt the same flag name.
//
//nolint:gochecknoglobals // test-only flag scoped to dockerfile_test.
var updateGolden = flag.Bool("update-golden", false, "rewrite testdata/*.expected from current generator output")

// TestGenerate_Snapshot exercises the dockerfile generator against the
// pinned fixture for each supported login shell so per-shell parameterization
// (useradd -s, completion init, history setup, rc-loader, cert env exports,
// plugin RC_FILE/RC_SYNTAX/LOGIN_SHELL) regresses visibly.
func TestGenerate_Snapshot(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		shell    string // "" = no [container.shell] section (defaults to bash)
		expected string
	}{
		{name: "default-bash", shell: "", expected: "snapshot.expected"},
		{name: "zsh", shell: "zsh", expected: "snapshot_zsh.expected"},
		{name: "fish", shell: "fish", expected: "snapshot_fish.expected"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := repoRoot(t)
			wsPath := filepath.Join(root, "tests", "fixtures", "snapshot.workspace.toml")
			pluginsDir := filepath.Join(root, "internal", "plugin", "catalog")

			ws, err := config.LoadWorkspace(wsPath)
			if err != nil {
				t.Fatalf("load workspace: %v", err)
			}
			if tc.shell != "" {
				shell := tc.shell
				ws.Container.Shell = &config.ContainerShellSpec{Default: &shell}
			}

			var warns bytes.Buffer
			plugins, err := plugin.LoadEnabled(pluginsDir, ws.Plugins.Enable, &warns)
			if err != nil {
				t.Fatalf("load plugins: %v", err)
			}

			ctx := &generate.WorkspaceContext{WS: ws, PluginsFS: os.DirFS(pluginsDir), Plugins: plugins, Warnings: &warns}
			got, err := dockerfile.Generate(ctx, dockerfile.Options{
				WorkspaceRoot: root,
				// Pin the embedded repo dir so the snapshot does not drift
				// when the local checkout is named something other than
				// "cocoon" (e.g. a sibling worktree).
				RepoDir:  "cocoon",
				Plugins:  plugins,
				Warnings: &warns,
			})
			if err != nil {
				t.Fatalf("generate: %v", err)
			}

			path := filepath.Join("testdata", tc.expected)
			if *updateGolden {
				if werr := os.WriteFile(path, []byte(got), 0o600); werr != nil {
					t.Fatalf("update golden: %v", werr)
				}
				return
			}
			wantBytes, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read expected: %v", err)
			}
			if got != string(wantBytes) {
				t.Errorf("Dockerfile mismatch (run with -update-golden to refresh)\n--- got ---\n%s\n--- want ---\n%s", got, string(wantBytes))
			}
		})
	}
}

// TestGenerate_FromLineForEachImage walks every supported base image and
// verifies the Dockerfile FROM line matches the expected registry path +
// version pair. The point is to catch a future SupportedImages change
// (new image, dropped image, renamed deno vendor namespace) that would
// otherwise only surface as a runtime error during `docker build`.
//
// Each case picks the first whitelisted version per image (the same
// default cocoon init writes) and asserts on the literal ARG IMAGE,
// ARG IMAGE_VERSION and FROM lines the template emits. deno is the
// only image whose IMAGE arg differs from the user-facing id; the
// remaining six are library/ namespace and round-trip verbatim.
//
//nolint:paralleltest // mutates ws.Container in each iteration
func TestGenerate_FromLineForEachImage(t *testing.T) {
	root := repoRoot(t)
	wsPath := filepath.Join(root, "tests", "fixtures", "snapshot.workspace.toml")
	pluginsDir := filepath.Join(root, "internal", "plugin", "catalog")

	for _, image := range config.SupportedImages {
		image := image
		version := config.SupportedImageVersions[image][0]
		wantRegistry := config.ResolveImageRegistry(image)
		t.Run(image, func(t *testing.T) {
			ws, err := config.LoadWorkspace(wsPath)
			if err != nil {
				t.Fatalf("load workspace: %v", err)
			}
			ws.Container.Image = image
			ws.Container.ImageVersion = version

			var warns bytes.Buffer
			plugins, err := plugin.LoadEnabled(pluginsDir, ws.Plugins.Enable, &warns)
			if err != nil {
				t.Fatalf("load plugins: %v", err)
			}
			ctx := &generate.WorkspaceContext{WS: ws, PluginsFS: os.DirFS(pluginsDir), Plugins: plugins, Warnings: &warns}
			got, err := dockerfile.Generate(ctx, dockerfile.Options{
				WorkspaceRoot: root, RepoDir: "cocoon",
				Plugins: plugins, Warnings: &warns,
			})
			if err != nil {
				t.Fatalf("generate: %v", err)
			}
			wantArgImage := "ARG IMAGE=" + wantRegistry
			wantArgVersion := "ARG IMAGE_VERSION=" + version
			wantFrom := "FROM ${IMAGE}:${IMAGE_VERSION}"
			for _, want := range []string{wantArgImage, wantArgVersion, wantFrom} {
				if !strings.Contains(got, want) {
					t.Errorf("Dockerfile missing %q for image=%s\n--- got (head) ---\n%s",
						want, image, headLines(got, 10))
				}
			}
		})
	}
}

// headLines returns the first n lines of s, suitable for compact error
// output that does not dump 300 lines of generated Dockerfile.
func headLines(s string, n int) string {
	lines := strings.SplitN(s, "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

// TestGenerate_AptMirrorHTTPS_BootstrapsCACerts verifies that an HTTPS
// [apt.mirror].url injects a ca-certificates pre-install RUN block, placed
// before the mirror rewrite, so the subsequent apt-get update against the
// HTTPS mirror succeeds on a stock ubuntu:24.04 (which ships with no CA
// bundle).
func TestGenerate_AptMirrorHTTPS_BootstrapsCACerts(t *testing.T) {
	t.Parallel()

	got := generateWithMirrorURL(t, "https://example.invalid/ubuntu/")

	const bootstrapHeader = "# Pre-install ca-certificates from the default HTTP archive"
	const bootstrapInstall = "apt-get install -y --no-install-recommends ca-certificates"
	const rewriteHeader = "# Rewrite upstream apt archive URLs to the configured [apt.mirror].url"

	bootIdx := strings.Index(got, bootstrapHeader)
	if bootIdx < 0 {
		t.Fatalf("bootstrap RUN block missing from output:\n%s", got)
	}
	if !strings.Contains(got, bootstrapInstall) {
		t.Errorf("bootstrap apt-get install line missing:\n%s", got)
	}
	rewriteIdx := strings.Index(got, rewriteHeader)
	if rewriteIdx < 0 {
		t.Fatalf("mirror rewrite block missing from output:\n%s", got)
	}
	if bootIdx > rewriteIdx {
		t.Errorf("bootstrap block must precede mirror rewrite (bootIdx=%d rewriteIdx=%d)", bootIdx, rewriteIdx)
	}
}

// TestGenerate_AptMirrorHTTP_NoBootstrap verifies that an HTTP mirror with
// no HTTPS [[apt.sources]] keeps the generated Dockerfile free of the
// ca-certificates pre-install block — the bootstrap is HTTPS-only.
func TestGenerate_AptMirrorHTTP_NoBootstrap(t *testing.T) {
	t.Parallel()

	got := generateWithMirrorURLAndSources(t, "http://jp.archive.ubuntu.com/ubuntu/", nil)

	if strings.Contains(got, "# Pre-install ca-certificates from the default HTTP archive") {
		t.Errorf("HTTP mirror with no HTTPS sources should not emit ca-cert bootstrap block:\n%s", got)
	}
}

// TestGenerate_AptMirrorDebian_LongerPatternFirst pins the Debian sed-pattern
// order so the rewrite block cannot regress to the broken layout where
// "deb.debian.org/debian" runs before "deb.debian.org/debian-security". With
// the broken order, sed rewrites the prefix first and produces "<mirror>-security"
// — an invalid URL. This test loads the snapshot fixture, switches it to
// Debian, sets a mirror, and asserts the security pattern appears before the
// shorter prefix in the generated RUN block.
func TestGenerate_AptMirrorDebian_LongerPatternFirst(t *testing.T) {
	t.Parallel()

	got := generateDebianWithMirrorURL(t, "http://ftp.jp.debian.org/debian/")

	const securityHost = "http://deb.debian.org/debian-security"
	const mainHost = "http://deb.debian.org/debian|"
	secIdx := strings.Index(got, securityHost)
	if secIdx < 0 {
		t.Fatalf("debian-security sed pattern missing from output:\n%s", got)
	}
	mainIdx := strings.Index(got, mainHost)
	if mainIdx < 0 {
		t.Fatalf("debian (main) sed pattern missing from output:\n%s", got)
	}
	if secIdx > mainIdx {
		t.Errorf("debian-security pattern must precede debian (secIdx=%d mainIdx=%d) — sed runs top-down and the shorter prefix would otherwise win", secIdx, mainIdx)
	}
	if strings.Contains(got, "security.debian.org") {
		t.Errorf("Debian rewrite must not target security.debian.org (Debian 12+ uses deb.debian.org/debian-security):\n%s", got)
	}
}

// generateDebianWithMirrorURL is generateWithMirrorURL but switches the
// fixture to os = "debian" / os_version = "12" before generating, so the
// rewrite block emits the Debian host set.
func generateDebianWithMirrorURL(t *testing.T, mirrorURL string) string {
	t.Helper()

	root := repoRoot(t)
	wsPath := filepath.Join(root, "tests", "fixtures", "snapshot.workspace.toml")
	pluginsDir := filepath.Join(root, "internal", "plugin", "catalog")

	ws, err := config.LoadWorkspace(wsPath)
	if err != nil {
		t.Fatalf("load workspace: %v", err)
	}
	if ws.Apt == nil || ws.Apt.Mirror == nil {
		t.Fatalf("snapshot fixture missing [apt.mirror]; this test relies on the fixture's mirror block")
	}
	ws.Container.Image = "debian"
	ws.Container.ImageVersion = "12"
	ws.Apt.Mirror.URL = mirrorURL

	var warns bytes.Buffer
	plugins, err := plugin.LoadEnabled(pluginsDir, ws.Plugins.Enable, &warns)
	if err != nil {
		t.Fatalf("load plugins: %v", err)
	}

	ctx := &generate.WorkspaceContext{WS: ws, PluginsFS: os.DirFS(pluginsDir), Plugins: plugins, Warnings: &warns}
	got, err := dockerfile.Generate(ctx, dockerfile.Options{
		WorkspaceRoot: root,
		RepoDir:       "cocoon",
		Plugins:       plugins,
		Warnings:      &warns,
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return got
}

// TestGenerate_AptSourceHTTPS_BootstrapsCACerts verifies that an HTTP mirror
// combined with an HTTPS [[apt.sources]] still triggers the ca-certificates
// pre-install — the bootstrap fires whenever any apt operation (mirror or
// third-party source) reaches an HTTPS endpoint.
func TestGenerate_AptSourceHTTPS_BootstrapsCACerts(t *testing.T) {
	t.Parallel()

	httpsSource := []config.AptSource{{
		Name:       "example-corp",
		Suite:      "noble",
		Components: []string{"main"},
		URL:        "https://apt.example.invalid/ubuntu/",
		KeyURL:     "https://apt.example.invalid/keyring.gpg",
	}}
	got := generateWithMirrorURLAndSources(t, "http://jp.archive.ubuntu.com/ubuntu/", httpsSource)

	const bootstrapHeader = "# Pre-install ca-certificates from the default HTTP archive"
	if !strings.Contains(got, bootstrapHeader) {
		t.Errorf("HTTPS apt source should emit ca-cert bootstrap block:\n%s", got)
	}
}

// TestGenerate_AptSourceHTTPSKeyURLOnly_BootstrapsCACerts checks that an
// HTTP source URL with an HTTPS key_url alone is enough to trigger the
// bootstrap (the keyring curl needs ca-certificates either way).
func TestGenerate_AptSourceHTTPSKeyURLOnly_BootstrapsCACerts(t *testing.T) {
	t.Parallel()

	mixed := []config.AptSource{{
		Name:       "example-corp",
		Suite:      "noble",
		Components: []string{"main"},
		URL:        "http://apt.example.invalid/ubuntu/",
		KeyURL:     "https://apt.example.invalid/keyring.gpg",
	}}
	got := generateWithMirrorURLAndSources(t, "http://jp.archive.ubuntu.com/ubuntu/", mixed)

	const bootstrapHeader = "# Pre-install ca-certificates from the default HTTP archive"
	if !strings.Contains(got, bootstrapHeader) {
		t.Errorf("HTTPS key_url should emit ca-cert bootstrap block:\n%s", got)
	}
}

// TestGenerate_AptCABootstrap_HTTPMirrorAndProxy_AllPreBootstrap pins the
// no-cert path: when AptCABootstrap fires (HTTPS [[apt.sources]]) and an HTTP
// [apt.mirror] / [apt.proxy] are configured, both rewrite blocks must land
// before the bootstrap RUN — the bootstrap's own apt-get update needs the
// mirror / proxy to reach archive.ubuntu.com on air-gapped hosts.
func TestGenerate_AptCABootstrap_HTTPMirrorAndProxy_AllPreBootstrap(t *testing.T) {
	t.Parallel()

	got := generateWithMirrorURL(t, "http://internal.mirror.invalid/ubuntu/")

	const rewriteHeader = "# Rewrite upstream apt archive URLs to the configured [apt.mirror].url"
	const proxyHeader = "# Configure apt HTTP(S) proxy from [apt.proxy]"
	const bootstrapHeader = "# Pre-install ca-certificates from the default HTTP archive"

	rewriteIdx := strings.Index(got, rewriteHeader)
	proxyIdx := strings.Index(got, proxyHeader)
	bootIdx := strings.Index(got, bootstrapHeader)
	if rewriteIdx < 0 || proxyIdx < 0 || bootIdx < 0 {
		t.Fatalf("expected mirror, proxy, and bootstrap blocks; got:\n%s", got)
	}
	if rewriteIdx >= proxyIdx || proxyIdx >= bootIdx {
		t.Errorf("expected mirror→proxy→bootstrap order (rewriteIdx=%d proxyIdx=%d bootIdx=%d)", rewriteIdx, proxyIdx, bootIdx)
	}
}

// TestGenerate_AptCABootstrap_HTTPSMirrorAndProxy_ProxyPreMirrorPost pins the
// split case: an HTTPS [apt.mirror] forces the mirror rewrite into the
// post-bootstrap slot (bootstrap can't TLS to the HTTPS mirror without the CA
// bundle yet), while [apt.proxy] still moves into the pre-bootstrap slot
// (bootstrap needs the proxy to reach archive.ubuntu.com).
func TestGenerate_AptCABootstrap_HTTPSMirrorAndProxy_ProxyPreMirrorPost(t *testing.T) {
	t.Parallel()

	got := generateWithMirrorURL(t, "https://internal.mirror.invalid/ubuntu/")

	const proxyHeader = "# Configure apt HTTP(S) proxy from [apt.proxy]"
	const bootstrapHeader = "# Pre-install ca-certificates from the default HTTP archive"
	const rewriteHeader = "# Rewrite upstream apt archive URLs to the configured [apt.mirror].url"

	proxyIdx := strings.Index(got, proxyHeader)
	bootIdx := strings.Index(got, bootstrapHeader)
	rewriteIdx := strings.Index(got, rewriteHeader)
	if proxyIdx < 0 || bootIdx < 0 || rewriteIdx < 0 {
		t.Fatalf("expected proxy, bootstrap, and mirror blocks; got:\n%s", got)
	}
	if proxyIdx >= bootIdx || bootIdx >= rewriteIdx {
		t.Errorf("expected proxy→bootstrap→mirror order (proxyIdx=%d bootIdx=%d rewriteIdx=%d)", proxyIdx, bootIdx, rewriteIdx)
	}
}

// generateWithMirrorURL loads the snapshot fixture, overrides
// [apt.mirror].url, and returns the generated Dockerfile. The fixture's
// existing [[apt.sources]] entries are kept.
func generateWithMirrorURL(t *testing.T, mirrorURL string) string {
	t.Helper()
	return generateWithMirrorURLAndSources(t, mirrorURL, keepExistingSources)
}

// keepExistingSources is a sentinel passed to
// generateWithMirrorURLAndSources to indicate the caller wants the
// fixture's [[apt.sources]] left untouched.
var keepExistingSources = []config.AptSource{{Name: "__keep__"}}

// generateWithMirrorURLAndSources loads the snapshot fixture, overrides
// [apt.mirror].url, replaces [[apt.sources]] (nil clears, keepExistingSources
// preserves the fixture's entries), and returns the generated Dockerfile.
func generateWithMirrorURLAndSources(t *testing.T, mirrorURL string, sources []config.AptSource) string {
	t.Helper()

	root := repoRoot(t)
	wsPath := filepath.Join(root, "tests", "fixtures", "snapshot.workspace.toml")
	pluginsDir := filepath.Join(root, "internal", "plugin", "catalog")

	ws, err := config.LoadWorkspace(wsPath)
	if err != nil {
		t.Fatalf("load workspace: %v", err)
	}
	if ws.Apt == nil || ws.Apt.Mirror == nil {
		t.Fatalf("snapshot fixture missing [apt.mirror]; this test relies on the fixture's mirror block")
	}
	ws.Apt.Mirror.URL = mirrorURL
	if len(sources) != 1 || sources[0].Name != "__keep__" {
		ws.Apt.Sources = sources
	}

	var warns bytes.Buffer
	plugins, err := plugin.LoadEnabled(pluginsDir, ws.Plugins.Enable, &warns)
	if err != nil {
		t.Fatalf("load plugins: %v", err)
	}

	ctx := &generate.WorkspaceContext{WS: ws, PluginsFS: os.DirFS(pluginsDir), Plugins: plugins, Warnings: &warns}
	got, err := dockerfile.Generate(ctx, dockerfile.Options{
		WorkspaceRoot: root,
		RepoDir:       "cocoon",
		Plugins:       plugins,
		Warnings:      &warns,
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return got
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// internal/generate/dockerfile -> repo root
	return filepath.Clean(filepath.Join(wd, "..", "..", ".."))
}

// certInstallHeader is the comment that opens the always-emitted cert
// install RUN block. Tests use it to anchor ordering assertions instead
// of pinning the inner shell-conditional body, which would couple them
// to incidental formatting tweaks.
const certInstallHeader = "# Install custom CA certificates from ~/.cocoon/certs/"

// TestGenerate_CertInstallBeforeAptInstall verifies that on the
// enabled cert path (the snapshot fixture sets [certificates]
// enable = true), the cert install RUN block lands before the main
// apt install RUN. This is the core CERT-01 regression guard: HTTPS
// apt operations performed by the main install must see any user-
// provided corporate CAs already in the trust store. The disabled
// branch is covered separately by
// TestGenerate_CertInstallSuppressedWhenDisabled.
func TestGenerate_CertInstallBeforeAptInstall(t *testing.T) {
	t.Parallel()

	root := stagingRoot(t)
	got := generateInStagingRoot(t, root, "https://archive.ubuntu.com/ubuntu/")

	const aptInstallMarker = "rm -f /etc/apt/apt.conf.d/docker-clean"

	certIdx := strings.Index(got, certInstallHeader)
	aptIdx := strings.Index(got, aptInstallMarker)
	if certIdx < 0 {
		t.Fatalf("cert install block missing from output:\n%s", got)
	}
	if aptIdx < 0 {
		t.Fatalf("main apt install RUN missing from output:\n%s", got)
	}
	if certIdx > aptIdx {
		t.Errorf("cert install must precede apt install (certIdx=%d aptIdx=%d)", certIdx, aptIdx)
	}

	// Env declarations live on the post-USER stage so they apply to
	// the unprivileged user's shells. On the enabled path the four
	// ENV exports always land regardless of whether the host actually
	// has *.crt files in ~/.cocoon/certs/ (the bundle they reference
	// is the merged system bundle, which exists either way). On the
	// disabled path they are suppressed entirely — covered by
	// TestGenerate_CertInstallSuppressedWhenDisabled.
	envIdx := strings.Index(got, "ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt")
	userSwitchIdx := strings.Index(got, "USER ${USERNAME}\nWORKDIR")
	if envIdx < 0 || userSwitchIdx < 0 || envIdx < userSwitchIdx {
		t.Errorf("cert ENV exports must follow the USER ${USERNAME} switch (envIdx=%d userSwitchIdx=%d)", envIdx, userSwitchIdx)
	}
}

// TestGenerate_CertInstallEmittedWhenEnabled verifies the cert install
// block and ENV declarations land in the Dockerfile when the workspace
// opts in via [certificates] enable=true. The fixture used by all
// staging-root tests now sets that flag through generateInStagingRoot's
// path, so this test is the positive case for the gated emit.
func TestGenerate_CertInstallEmittedWhenEnabled(t *testing.T) {
	t.Parallel()

	root := stagingRoot(t)
	got := generateInStagingRoot(t, root, "http://archive.ubuntu.com/ubuntu/")

	if !strings.Contains(got, certInstallHeader) {
		t.Errorf("cert install block must be emitted when [certificates] enable=true:\n%s", got)
	}
	if !strings.Contains(got, "RUN --mount=type=bind,from=cocoon_user_certs") {
		t.Errorf("cert install RUN must use the cocoon_user_certs build context:\n%s", got)
	}
	if !strings.Contains(got, "ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt") {
		t.Errorf("ENV SSL_CERT_FILE must be emitted when certificates are enabled:\n%s", got)
	}
}

// TestGenerate_CertInstallSuppressedWhenDisabled verifies that omitting
// the [certificates] section (or setting enable=false) yields a
// Dockerfile with zero cert-related content: no RUN block, no ENV
// declarations, no /usr/local/share/ca-certificates/cocoon-user
// references. This is the core opt-out invariant for cert-free teams.
func TestGenerate_CertInstallSuppressedWhenDisabled(t *testing.T) {
	t.Parallel()

	got := generateWithCertificatesDisabled(t)

	for _, mustNot := range []string{
		certInstallHeader,
		"cocoon_user_certs",
		"/usr/local/share/ca-certificates/cocoon-user",
		"ENV SSL_CERT_FILE",
		"ENV CURL_CA_BUNDLE",
		"ENV REQUESTS_CA_BUNDLE",
		"ENV NODE_EXTRA_CA_CERTS",
	} {
		if strings.Contains(got, mustNot) {
			t.Errorf("Dockerfile with [certificates] disabled must not contain %q\n--- got ---\n%s", mustNot, got)
		}
	}
}

// generateWithCertificatesDisabled loads the snapshot fixture, drops
// any [certificates] section so the default-off path applies, and
// returns the generated Dockerfile. Mirrors generateInStagingRoot's
// shape but does not seed a staging root since the disabled branch
// does not depend on host-side cert state.
func generateWithCertificatesDisabled(t *testing.T) string {
	t.Helper()
	root := stagingRoot(t)
	wsPath := filepath.Join(repoRoot(t), "tests", "fixtures", "snapshot.workspace.toml")
	pluginsDir := filepath.Join(repoRoot(t), "internal", "plugin", "catalog")

	ws, err := config.LoadWorkspace(wsPath)
	if err != nil {
		t.Fatalf("load workspace: %v", err)
	}
	ws.Apt.Sources = nil
	ws.Certificates = nil

	var warns bytes.Buffer
	plugins, err := plugin.LoadEnabled(pluginsDir, ws.Plugins.Enable, &warns)
	if err != nil {
		t.Fatalf("load plugins: %v", err)
	}

	ctx := &generate.WorkspaceContext{WS: ws, PluginsFS: os.DirFS(pluginsDir), Plugins: plugins, Warnings: &warns}
	got, err := dockerfile.Generate(ctx, dockerfile.Options{
		WorkspaceRoot: root,
		RepoDir:       "cocoon",
		Plugins:       plugins,
		Warnings:      &warns,
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return got
}

// stagingRoot creates a temp workspace root with an empty config/ directory.
// (Earlier versions of cocoon also scanned <root>/certs/ for *.crt files,
// but the cert install block is now sourced from ~/.cocoon/certs/ via
// docker-compose's additional_contexts and no longer reads the project
// tree at gen time.)
func stagingRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	return root
}

// generateInStagingRoot runs Generate against the snapshot fixture using
// `root` as the WorkspaceRoot (so its certs/ + config/ are picked up). The
// fixture's [[apt.sources]] is cleared to keep assertions focused on the
// cert install path.
func generateInStagingRoot(t *testing.T, root, mirrorURL string) string {
	t.Helper()
	return generateInStagingRootWithProxy(t, root, mirrorURL, "")
}

// generateInStagingRootWithProxy is like generateInStagingRoot but also sets
// [apt.proxy].http when httpProxy is non-empty, so cert/proxy ordering can be
// asserted without rebuilding the whole staging dance per test.
func generateInStagingRootWithProxy(t *testing.T, root, mirrorURL, httpProxy string) string {
	t.Helper()

	wsPath := filepath.Join(repoRoot(t), "tests", "fixtures", "snapshot.workspace.toml")
	pluginsDir := filepath.Join(repoRoot(t), "internal", "plugin", "catalog")

	ws, err := config.LoadWorkspace(wsPath)
	if err != nil {
		t.Fatalf("load workspace: %v", err)
	}
	ws.Apt.Mirror.URL = mirrorURL
	ws.Apt.Sources = nil
	if httpProxy != "" {
		http := httpProxy
		ws.Apt.Proxy = &config.AptProxy{HTTP: &http}
	}

	var warns bytes.Buffer
	plugins, err := plugin.LoadEnabled(pluginsDir, ws.Plugins.Enable, &warns)
	if err != nil {
		t.Fatalf("load plugins: %v", err)
	}

	ctx := &generate.WorkspaceContext{WS: ws, PluginsFS: os.DirFS(pluginsDir), Plugins: plugins, Warnings: &warns}
	got, err := dockerfile.Generate(ctx, dockerfile.Options{
		WorkspaceRoot: root,
		RepoDir:       "cocoon",
		Plugins:       plugins,
		Warnings:      &warns,
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return got
}

// TestGenerate_CertWithHTTPMirror_RewriteBeforeCertInstall verifies that an
// HTTP [apt.mirror] is rewritten before the cert install RUN. The cert
// install conditionally runs apt-get update against archive.ubuntu.com, so
// on air-gapped hosts the mirror swap must happen first or the build fails
// before reaching update-ca-certificates.
func TestGenerate_CertWithHTTPMirror_RewriteBeforeCertInstall(t *testing.T) {
	t.Parallel()

	root := stagingRoot(t)
	got := generateInStagingRoot(t, root, "http://internal.mirror.invalid/ubuntu/")

	const rewriteHeader = "# Rewrite upstream apt archive URLs to the configured [apt.mirror].url"

	rewriteIdx := strings.Index(got, rewriteHeader)
	certIdx := strings.Index(got, certInstallHeader)
	if rewriteIdx < 0 {
		t.Fatalf("mirror rewrite block missing from output:\n%s", got)
	}
	if certIdx < 0 {
		t.Fatalf("cert install block missing from output:\n%s", got)
	}
	if rewriteIdx > certIdx {
		t.Errorf("HTTP mirror rewrite must precede cert install (rewriteIdx=%d certIdx=%d)", rewriteIdx, certIdx)
	}
}

// TestGenerate_CertWithHTTPSMirror_RewriteAfterCertInstall verifies that an
// HTTPS [apt.mirror] is rewritten after the cert install RUN. Cert install's
// own apt-get update can't TLS to the HTTPS mirror without the corporate CA
// bundle, so the rewrite must wait until update-ca-certificates has run.
func TestGenerate_CertWithHTTPSMirror_RewriteAfterCertInstall(t *testing.T) {
	t.Parallel()

	root := stagingRoot(t)
	got := generateInStagingRoot(t, root, "https://internal.mirror.invalid/ubuntu/")

	const rewriteHeader = "# Rewrite upstream apt archive URLs to the configured [apt.mirror].url"

	rewriteIdx := strings.Index(got, rewriteHeader)
	certIdx := strings.Index(got, certInstallHeader)
	if rewriteIdx < 0 {
		t.Fatalf("mirror rewrite block missing from output:\n%s", got)
	}
	if certIdx < 0 {
		t.Fatalf("cert install block missing from output:\n%s", got)
	}
	if certIdx > rewriteIdx {
		t.Errorf("cert install must precede HTTPS mirror rewrite (certIdx=%d rewriteIdx=%d)", certIdx, rewriteIdx)
	}
}

// TestGenerate_CertWithAptProxy_ProxyConfBeforeCertInstall verifies that the
// 95proxy file is written before the cert install RUN. Cert install's
// apt-get update must go through the configured proxy to reach the archive
// at all in proxied corporate networks.
func TestGenerate_CertWithAptProxy_ProxyConfBeforeCertInstall(t *testing.T) {
	t.Parallel()

	root := stagingRoot(t)
	got := generateInStagingRootWithProxy(t, root, "http://archive.ubuntu.com/ubuntu/", "http://proxy.invalid:3128")

	const proxyHeader = "# Configure apt HTTP(S) proxy from [apt.proxy]"

	proxyIdx := strings.Index(got, proxyHeader)
	certIdx := strings.Index(got, certInstallHeader)
	if proxyIdx < 0 {
		t.Fatalf("apt proxy block missing from output:\n%s", got)
	}
	if certIdx < 0 {
		t.Fatalf("cert install block missing from output:\n%s", got)
	}
	if proxyIdx > certIdx {
		t.Errorf("apt proxy conf must precede cert install (proxyIdx=%d certIdx=%d)", proxyIdx, certIdx)
	}
}

// TestGenerate_CertWithHTTPMirrorAndProxy_BothPreCert verifies the directly
// orthogonal case: HTTP mirror + apt proxy. Both rewrite blocks must land
// in the pre-cert slot, in mirror→proxy order, so cert install's own
// apt-get update can use the internal mirror through the proxy.
func TestGenerate_CertWithHTTPMirrorAndProxy_BothPreCert(t *testing.T) {
	t.Parallel()

	root := stagingRoot(t)
	got := generateInStagingRootWithProxy(t, root, "http://internal.mirror.invalid/ubuntu/", "http://proxy.invalid:3128")

	const rewriteHeader = "# Rewrite upstream apt archive URLs to the configured [apt.mirror].url"
	const proxyHeader = "# Configure apt HTTP(S) proxy from [apt.proxy]"

	rewriteIdx := strings.Index(got, rewriteHeader)
	proxyIdx := strings.Index(got, proxyHeader)
	certIdx := strings.Index(got, certInstallHeader)
	if rewriteIdx < 0 || proxyIdx < 0 || certIdx < 0 {
		t.Fatalf("expected mirror, proxy, and cert install blocks; got:\n%s", got)
	}
	if rewriteIdx >= proxyIdx || proxyIdx >= certIdx {
		t.Errorf("expected mirror→proxy→cert order (rewriteIdx=%d proxyIdx=%d certIdx=%d)", rewriteIdx, proxyIdx, certIdx)
	}
}

// TestGenerate_CertWithHTTPSMirrorAndProxy_ProxyPreMirrorPost verifies the
// split case: HTTPS mirror + apt proxy. The proxy conf must move into the
// pre-cert slot (cert install needs it to reach the archive) while the HTTPS
// mirror rewrite must stay in the post-cert slot (cert install can't TLS to
// the HTTPS mirror without the CA bundle yet).
func TestGenerate_CertWithHTTPSMirrorAndProxy_ProxyPreMirrorPost(t *testing.T) {
	t.Parallel()

	root := stagingRoot(t)
	got := generateInStagingRootWithProxy(t, root, "https://internal.mirror.invalid/ubuntu/", "http://proxy.invalid:3128")

	const proxyHeader = "# Configure apt HTTP(S) proxy from [apt.proxy]"
	const rewriteHeader = "# Rewrite upstream apt archive URLs to the configured [apt.mirror].url"

	proxyIdx := strings.Index(got, proxyHeader)
	certIdx := strings.Index(got, certInstallHeader)
	rewriteIdx := strings.Index(got, rewriteHeader)
	if proxyIdx < 0 || certIdx < 0 || rewriteIdx < 0 {
		t.Fatalf("expected proxy, cert install, and mirror rewrite blocks; got:\n%s", got)
	}
	if proxyIdx >= certIdx || certIdx >= rewriteIdx {
		t.Errorf("expected proxy→cert→mirror order (proxyIdx=%d certIdx=%d rewriteIdx=%d)", proxyIdx, certIdx, rewriteIdx)
	}
}

// TestGenerate_AptOrderingWithMirrorAndProxy_StableShape pins the byte-exact
// relative positions of AptMirrorRewrite, AptProxyConf, and the main apt
// install RUN. The cert install block now sits between the proxy conf and
// the main apt install (always emitted), but the mirror→proxy→apt-install
// chain must remain monotonic.
func TestGenerate_AptOrderingWithMirrorAndProxy_StableShape(t *testing.T) {
	t.Parallel()

	root := stagingRoot(t)
	got := generateInStagingRootWithProxy(t, root, "http://internal.mirror.invalid/ubuntu/", "http://proxy.invalid:3128")

	const rewriteHeader = "# Rewrite upstream apt archive URLs to the configured [apt.mirror].url"
	const proxyHeader = "# Configure apt HTTP(S) proxy from [apt.proxy]"
	const aptInstallMarker = "rm -f /etc/apt/apt.conf.d/docker-clean"

	rewriteIdx := strings.Index(got, rewriteHeader)
	proxyIdx := strings.Index(got, proxyHeader)
	aptIdx := strings.Index(got, aptInstallMarker)
	if rewriteIdx < 0 || proxyIdx < 0 || aptIdx < 0 {
		t.Fatalf("expected rewrite, proxy, and main apt install blocks; got:\n%s", got)
	}
	if rewriteIdx >= proxyIdx || proxyIdx >= aptIdx {
		t.Errorf("expected mirror→proxy→apt-install order (rewriteIdx=%d proxyIdx=%d aptIdx=%d)", rewriteIdx, proxyIdx, aptIdx)
	}
}
