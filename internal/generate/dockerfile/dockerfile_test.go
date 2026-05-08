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
			pluginsDir := filepath.Join(root, "plugins")

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

			ctx := &generate.WorkspaceContext{WS: ws, PluginsDir: pluginsDir, Plugins: plugins, Warnings: &warns}
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
				t.Errorf("Dockerfile mismatch\n--- got ---\n%s\n--- want ---\n%s", got, string(wantBytes))
			}
		})
	}
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
	pluginsDir := filepath.Join(root, "plugins")

	ws, err := config.LoadWorkspace(wsPath)
	if err != nil {
		t.Fatalf("load workspace: %v", err)
	}
	if ws.Apt == nil || ws.Apt.Mirror == nil {
		t.Fatalf("snapshot fixture missing [apt.mirror]; this test relies on the fixture's mirror block")
	}
	ws.Container.Os = "debian"
	ws.Container.OsVersion = "12"
	ws.Apt.Mirror.URL = mirrorURL

	var warns bytes.Buffer
	plugins, err := plugin.LoadEnabled(pluginsDir, ws.Plugins.Enable, &warns)
	if err != nil {
		t.Fatalf("load plugins: %v", err)
	}

	ctx := &generate.WorkspaceContext{WS: ws, PluginsDir: pluginsDir, Plugins: plugins, Warnings: &warns}
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
	pluginsDir := filepath.Join(root, "plugins")

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

	ctx := &generate.WorkspaceContext{WS: ws, PluginsDir: pluginsDir, Plugins: plugins, Warnings: &warns}
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

// TestGenerate_CertInstallBeforeAptInstall verifies that when certs/*.crt
// is present, the COPY + update-ca-certificates RUN is emitted before the
// main apt install RUN. This is the core CERT-01 regression guard.
func TestGenerate_CertInstallBeforeAptInstall(t *testing.T) {
	t.Parallel()

	root := stagingRootWithCert(t)
	got := generateInStagingRoot(t, root, "https://archive.ubuntu.com/ubuntu/")

	const certCopy = "COPY certs/example-corp.crt /tmp/certs/example-corp.crt"
	const aptInstallMarker = "rm -f /etc/apt/apt.conf.d/docker-clean"

	certIdx := strings.Index(got, certCopy)
	aptIdx := strings.Index(got, aptInstallMarker)
	if certIdx < 0 {
		t.Fatalf("cert COPY missing from output:\n%s", got)
	}
	if aptIdx < 0 {
		t.Fatalf("main apt install RUN missing from output:\n%s", got)
	}
	if certIdx > aptIdx {
		t.Errorf("cert install must precede apt install (certIdx=%d aptIdx=%d)", certIdx, aptIdx)
	}

	// AptCABootstrap is suppressed when cert install root stage is present
	// (the cert install RUN already brings in ca-certificates).
	if strings.Contains(got, "# Pre-install ca-certificates from the default HTTP archive") {
		t.Errorf("AptCABootstrap should be suppressed when CertInstallRoot is non-empty:\n%s", got)
	}

	// Env declarations and rc echos must remain on the post-USER stage so
	// they apply to the unprivileged user's shells.
	envIdx := strings.Index(got, "ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt")
	userSwitchIdx := strings.Index(got, "USER ${USERNAME}\nWORKDIR")
	if envIdx < 0 || userSwitchIdx < 0 || envIdx < userSwitchIdx {
		t.Errorf("cert ENV exports must follow the USER ${USERNAME} switch (envIdx=%d userSwitchIdx=%d)", envIdx, userSwitchIdx)
	}
}

// TestGenerate_NoCertsLeavesNoCertBlock verifies that an empty certs
// directory produces no cert-related Dockerfile content (regression guard
// against accidental empty-block emission from the new template).
func TestGenerate_NoCertsLeavesNoCertBlock(t *testing.T) {
	t.Parallel()

	root := stagingRoot(t)
	got := generateInStagingRoot(t, root, "http://archive.ubuntu.com/ubuntu/")

	if strings.Contains(got, "/usr/local/share/ca-certificates") {
		t.Errorf("empty certs/ should not emit cert install block:\n%s", got)
	}
	if strings.Contains(got, "ENV SSL_CERT_FILE=") {
		t.Errorf("empty certs/ should not emit cert ENV exports:\n%s", got)
	}
}

// stagingRoot creates a temp workspace root with an empty certs/ and an
// empty config/ directory; the apt base set now lives in
// internal/aptbase.MinimalBasePackages and no longer needs a copied
// apt-base-packages.conf for Generate to run end-to-end.
func stagingRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "certs"), 0o755); err != nil {
		t.Fatalf("mkdir certs: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	return root
}

// stagingRootWithCert returns stagingRoot plus a PEM dummy cert at
// certs/example-corp.crt.
func stagingRootWithCert(t *testing.T) string {
	t.Helper()
	root := stagingRoot(t)
	pem := "-----BEGIN CERTIFICATE-----\nMIIBdummyCertificateContentForTesting==\n-----END CERTIFICATE-----\n"
	if err := os.WriteFile(filepath.Join(root, "certs", "example-corp.crt"), []byte(pem), 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
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
	pluginsDir := filepath.Join(repoRoot(t), "plugins")

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

	ctx := &generate.WorkspaceContext{WS: ws, PluginsDir: pluginsDir, Plugins: plugins, Warnings: &warns}
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
// HTTP [apt.mirror] is rewritten before the cert install RUN. The cert install
// itself runs apt-get update against archive.ubuntu.com, so on air-gapped
// hosts the mirror swap must happen first or the build fails before reaching
// update-ca-certificates.
func TestGenerate_CertWithHTTPMirror_RewriteBeforeCertInstall(t *testing.T) {
	t.Parallel()

	root := stagingRootWithCert(t)
	got := generateInStagingRoot(t, root, "http://internal.mirror.invalid/ubuntu/")

	const rewriteHeader = "# Rewrite upstream apt archive URLs to the configured [apt.mirror].url"
	const certCopy = "COPY certs/example-corp.crt /tmp/certs/example-corp.crt"

	rewriteIdx := strings.Index(got, rewriteHeader)
	certIdx := strings.Index(got, certCopy)
	if rewriteIdx < 0 {
		t.Fatalf("mirror rewrite block missing from output:\n%s", got)
	}
	if certIdx < 0 {
		t.Fatalf("cert COPY missing from output:\n%s", got)
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

	root := stagingRootWithCert(t)
	got := generateInStagingRoot(t, root, "https://internal.mirror.invalid/ubuntu/")

	const rewriteHeader = "# Rewrite upstream apt archive URLs to the configured [apt.mirror].url"
	const certCopy = "COPY certs/example-corp.crt /tmp/certs/example-corp.crt"

	rewriteIdx := strings.Index(got, rewriteHeader)
	certIdx := strings.Index(got, certCopy)
	if rewriteIdx < 0 {
		t.Fatalf("mirror rewrite block missing from output:\n%s", got)
	}
	if certIdx < 0 {
		t.Fatalf("cert COPY missing from output:\n%s", got)
	}
	if certIdx > rewriteIdx {
		t.Errorf("cert install must precede HTTPS mirror rewrite (certIdx=%d rewriteIdx=%d)", certIdx, rewriteIdx)
	}
}

// TestGenerate_CertWithAptProxy_ProxyConfBeforeCertInstall verifies that the
// 95proxy file is written before the cert install RUN. Cert install's
// apt-get update must go through the configured proxy to reach the archive
// at all in proxied corporate networks; the previous ordering hit the network
// directly and broke those builds.
func TestGenerate_CertWithAptProxy_ProxyConfBeforeCertInstall(t *testing.T) {
	t.Parallel()

	root := stagingRootWithCert(t)
	got := generateInStagingRootWithProxy(t, root, "http://archive.ubuntu.com/ubuntu/", "http://proxy.invalid:3128")

	const proxyHeader = "# Configure apt HTTP(S) proxy from [apt.proxy]"
	const certCopy = "COPY certs/example-corp.crt /tmp/certs/example-corp.crt"

	proxyIdx := strings.Index(got, proxyHeader)
	certIdx := strings.Index(got, certCopy)
	if proxyIdx < 0 {
		t.Fatalf("apt proxy block missing from output:\n%s", got)
	}
	if certIdx < 0 {
		t.Fatalf("cert COPY missing from output:\n%s", got)
	}
	if proxyIdx > certIdx {
		t.Errorf("apt proxy conf must precede cert install (proxyIdx=%d certIdx=%d)", proxyIdx, certIdx)
	}
}

// TestGenerate_CertWithHTTPMirrorAndProxy_BothPreCert verifies the directly
// orthogonal case: certs + HTTP mirror + apt proxy. Both rewrite blocks must
// land in the pre-cert slot, in mirror→proxy order, so cert install's own
// apt-get update can use the internal mirror through the proxy.
func TestGenerate_CertWithHTTPMirrorAndProxy_BothPreCert(t *testing.T) {
	t.Parallel()

	root := stagingRootWithCert(t)
	got := generateInStagingRootWithProxy(t, root, "http://internal.mirror.invalid/ubuntu/", "http://proxy.invalid:3128")

	const rewriteHeader = "# Rewrite upstream apt archive URLs to the configured [apt.mirror].url"
	const proxyHeader = "# Configure apt HTTP(S) proxy from [apt.proxy]"
	const certCopy = "COPY certs/example-corp.crt /tmp/certs/example-corp.crt"

	rewriteIdx := strings.Index(got, rewriteHeader)
	proxyIdx := strings.Index(got, proxyHeader)
	certIdx := strings.Index(got, certCopy)
	if rewriteIdx < 0 || proxyIdx < 0 || certIdx < 0 {
		t.Fatalf("expected mirror, proxy, and cert COPY blocks; got:\n%s", got)
	}
	if rewriteIdx >= proxyIdx || proxyIdx >= certIdx {
		t.Errorf("expected mirror→proxy→cert order (rewriteIdx=%d proxyIdx=%d certIdx=%d)", rewriteIdx, proxyIdx, certIdx)
	}
}

// TestGenerate_CertWithHTTPSMirrorAndProxy_ProxyPreMirrorPost verifies the
// split case: certs + HTTPS mirror + apt proxy. The proxy conf must move into
// the pre-cert slot (cert install needs it to reach the archive) while the
// HTTPS mirror rewrite must stay in the post-cert slot (cert install can't TLS
// to the HTTPS mirror without the CA bundle yet).
func TestGenerate_CertWithHTTPSMirrorAndProxy_ProxyPreMirrorPost(t *testing.T) {
	t.Parallel()

	root := stagingRootWithCert(t)
	got := generateInStagingRootWithProxy(t, root, "https://internal.mirror.invalid/ubuntu/", "http://proxy.invalid:3128")

	const proxyHeader = "# Configure apt HTTP(S) proxy from [apt.proxy]"
	const certCopy = "COPY certs/example-corp.crt /tmp/certs/example-corp.crt"
	const rewriteHeader = "# Rewrite upstream apt archive URLs to the configured [apt.mirror].url"

	proxyIdx := strings.Index(got, proxyHeader)
	certIdx := strings.Index(got, certCopy)
	rewriteIdx := strings.Index(got, rewriteHeader)
	if proxyIdx < 0 || certIdx < 0 || rewriteIdx < 0 {
		t.Fatalf("expected proxy, cert COPY, and mirror rewrite blocks; got:\n%s", got)
	}
	if proxyIdx >= certIdx || certIdx >= rewriteIdx {
		t.Errorf("expected proxy→cert→mirror order (proxyIdx=%d certIdx=%d rewriteIdx=%d)", proxyIdx, certIdx, rewriteIdx)
	}
}

// TestGenerate_NoCerts_KeepsAptOrderingStable verifies the byte-exact field
// positions for the no-certs case: AptMirrorRewrite and AptProxyConf must
// stay in their post-CertInstallRoot slots so an empty certs/ tree produces
// the same Dockerfile as the v8.5.0 baseline.
func TestGenerate_NoCerts_KeepsAptOrderingStable(t *testing.T) {
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
