package dockerfile_test

import (
	"bytes"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/generate/dockerfile"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

//nolint:gochecknoglobals // test-only flag scoped to dockerfile_test.
var updateGolden = flag.Bool("update-golden", false, "rewrite testdata/*.expected from current generator output")

// TestGenerate_Snapshot pins per-shell parameterization (useradd -s,
// completion init, history setup, rc-loader, cert env exports, plugin
// RC_FILE/RC_SYNTAX/LOGIN_SHELL) against the snapshot fixture.
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
			plugins, err := plugin.LoadEnabledFromFS(os.DirFS(pluginsDir), ws.Plugins.Enable, &warns, pluginsDir)
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

// TestGenerate_FromLineForEachImage catches future SupportedImages drift
// (new image, dropped image, renamed deno vendor namespace) that would
// otherwise only surface as a `docker build` runtime error.
//
//nolint:paralleltest // mutates ws.Container in each iteration
func TestGenerate_FromLineForEachImage(t *testing.T) {
	root := repoRoot(t)
	wsPath := filepath.Join(root, "tests", "fixtures", "snapshot.workspace.toml")
	pluginsDir := filepath.Join(root, "internal", "plugin", "catalog")

	for _, image := range config.SupportedImages {
		image := image
		version := config.SupportedImageVersions[image][0]
		t.Run(image, func(t *testing.T) {
			ws, err := config.LoadWorkspace(wsPath)
			if err != nil {
				t.Fatalf("load workspace: %v", err)
			}
			ws.Container.Image = image
			ws.Container.ImageVersion = version
			// Drop image-conflict plugins (and their orphan version pins)
			// so the test never feeds an invalid workspace into Generate.
			if conflict, hit := config.ImageProvidesPlugin[image]; hit {
				filtered := ws.Plugins.Enable[:0]
				for _, id := range ws.Plugins.Enable {
					if id == conflict {
						continue
					}
					filtered = append(filtered, id)
				}
				ws.Plugins.Enable = filtered
				delete(ws.Plugins.Versions, conflict)
			}

			var warns bytes.Buffer
			plugins, err := plugin.LoadEnabledFromFS(os.DirFS(pluginsDir), ws.Plugins.Enable, &warns, pluginsDir)
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
			wantArgImage := "ARG IMAGE=" + image
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

// headLines returns the first n lines, keeping error output compact.
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
// fixture to image = "debian" / image_version = "12" before generating,
// so the rewrite block emits the Debian host set.
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
	plugins, err := plugin.LoadEnabledFromFS(os.DirFS(pluginsDir), ws.Plugins.Enable, &warns, pluginsDir)
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

// keepExistingSources is a sentinel signalling "leave fixture sources alone".
var keepExistingSources = []config.AptSource{{Name: "__keep__"}}

// generateWithMirrorURLAndSources overrides [apt.mirror].url; nil sources
// clears, keepExistingSources preserves the fixture's entries.
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
	plugins, err := plugin.LoadEnabledFromFS(os.DirFS(pluginsDir), ws.Plugins.Enable, &warns, pluginsDir)
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
	return filepath.Clean(filepath.Join(wd, "..", "..", ".."))
}

// certInstallHeader anchors ordering assertions against a stable line
// instead of the shell-conditional body that drifts with formatting tweaks.
const certInstallHeader = "# Install custom CA certificates from ~/.cocoon/certs/"

// TestGenerate_CertInstallBeforeAptInstall is the CERT-01 regression guard:
// HTTPS apt operations in the main install must see user CAs already in the
// trust store.
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

	// ENV exports must land post-USER so they apply to the unprivileged shell.
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

// TestGenerate_CertInstallIngestsCerAndCrt pins the .cer support contract:
// the cert block must scan for both extensions, and .cer files must be
// renamed to .crt on copy (update-ca-certificates only ingests *.crt, so a
// verbatim .cer copy would be silently dropped from the trust store).
func TestGenerate_CertInstallIngestsCerAndCrt(t *testing.T) {
	t.Parallel()

	root := stagingRoot(t)
	got := generateInStagingRoot(t, root, "http://archive.ubuntu.com/ubuntu/")

	if !strings.Contains(got, `\( -name '*.crt' -o -name '*.cer' \)`) {
		t.Errorf("cert existence check must match both *.crt and *.cer:\n%s", got)
	}
	if !strings.Contains(got, `find /tmp/cocoon-user-certs -maxdepth 1 -name '*.cer'`) {
		t.Errorf("cert install must scan for *.cer files:\n%s", got)
	}
	if !strings.Contains(got, `basename "$1" .cer).crt`) {
		t.Errorf("*.cer files must be copied renamed to *.crt:\n%s", got)
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
		"*.cer",
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

// generateWithCertificatesDisabled drops [certificates] so the default-off
// path applies.
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
	plugins, err := plugin.LoadEnabledFromFS(os.DirFS(pluginsDir), ws.Plugins.Enable, &warns, pluginsDir)
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
	plugins, err := plugin.LoadEnabledFromFS(os.DirFS(pluginsDir), ws.Plugins.Enable, &warns, pluginsDir)
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

// generateFromSnapshotFixture renders the Dockerfile from the parent-mount
// snapshot fixture with the embedded catalog plugins.
func generateFromSnapshotFixture(t *testing.T) string {
	t.Helper()
	root := repoRoot(t)
	wsPath := filepath.Join(root, "tests", "fixtures", "snapshot.workspace.toml")
	pluginsDir := filepath.Join(root, "internal", "plugin", "catalog")

	ws, err := config.LoadWorkspace(wsPath)
	if err != nil {
		t.Fatalf("load workspace: %v", err)
	}
	var warns bytes.Buffer
	plugins, err := plugin.LoadEnabledFromFS(os.DirFS(pluginsDir), ws.Plugins.Enable, &warns, pluginsDir)
	if err != nil {
		t.Fatalf("load plugins: %v", err)
	}
	ctx := &generate.WorkspaceContext{WS: ws, PluginsFS: os.DirFS(pluginsDir), Plugins: plugins, Warnings: &warns}
	got, err := dockerfile.Generate(ctx, dockerfile.Options{
		WorkspaceRoot: root, RepoDir: "cocoon", Plugins: plugins, Warnings: &warns,
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return got
}

// TestGenerate_HostIndependentImage pins the host-independent contract: the
// Dockerfile takes no UID/GID build args, creates the user at a fixed uid/gid
// 1000, exposes the docker-entrypoint.sh inputs as ENV, and keeps the final
// stage as root so the entrypoint can remap before dropping privileges.
func TestGenerate_HostIndependentImage(t *testing.T) {
	t.Parallel()

	got := generateFromSnapshotFixture(t)
	for _, bad := range []string{"ARG UID", "ARG GID", "ARG DOCKER_GID", "DOCKER_GID"} {
		if strings.Contains(got, bad) {
			t.Errorf("host-independent Dockerfile must not contain %q:\n%s", bad, got)
		}
	}
	for _, want := range []string{
		"useradd -m -s /bin/bash -u 1000 -g 1000 ${USERNAME}",
		"ENV COCOON_USER=${USERNAME}",
		`ENV COCOON_WORKSPACE="`,
		`ENV COCOON_BIND_PATHS="`,
		"RUN chmod +x /usr/local/bin/docker-entrypoint.sh\n\nENTRYPOINT",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in Dockerfile:\n%s", want, got)
		}
	}
}

// TestGenerate_WorkspaceDirOverride pins that [workspace].dir flows into
// the Dockerfile's WORKDIR line and the entrypoint's COCOON_WORKSPACE /
// COCOON_BIND_PATHS env vars. Multi-segment dirs (e.g. "work/myapp") must
// survive verbatim — they describe the in-container parent directory, not
// the host bind path.
func TestGenerate_WorkspaceDirOverride(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	wsPath := filepath.Join(root, "tests", "fixtures", "snapshot-cwd.workspace.toml")
	pluginsDir := filepath.Join(root, "internal", "plugin", "catalog")
	ws, err := config.LoadWorkspace(wsPath)
	if err != nil {
		t.Fatalf("load workspace: %v", err)
	}
	if ws.Workspace == nil {
		ws.Workspace = &config.WorkspaceSpec{}
	}
	ws.Workspace.Dir = "work/myapp"

	var warns bytes.Buffer
	plugins, err := plugin.LoadEnabledFromFS(os.DirFS(pluginsDir), ws.Plugins.Enable, &warns, pluginsDir)
	if err != nil {
		t.Fatalf("load plugins: %v", err)
	}
	ctx := &generate.WorkspaceContext{WS: ws, PluginsFS: os.DirFS(pluginsDir), Plugins: plugins, Warnings: &warns}
	got, err := dockerfile.Generate(ctx, dockerfile.Options{
		WorkspaceRoot: root, RepoDir: "cocoon", Plugins: plugins, Warnings: &warns,
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	for _, want := range []string{
		"WORKDIR /home/${USERNAME}/work/myapp",
		`ENV COCOON_WORKSPACE="/home/testuser/work/myapp/snapshot-test"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in Dockerfile:\n%s", want, got)
		}
	}
	if strings.Contains(got, "WORKDIR /home/${USERNAME}/workspace\n") {
		t.Errorf("Dockerfile still contains the default WORKDIR after dir override:\n%s", got)
	}
}

// TestGenerate_BindPathsIncludeHomeRootMount pins that a [[mounts]] target at
// exactly the user's home directory is recorded in COCOON_BIND_PATHS, so the
// entrypoint's chown sweep prunes it instead of recursively re-owning the
// host-mounted tree.
func TestGenerate_BindPathsIncludeHomeRootMount(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	wsPath := filepath.Join(root, "tests", "fixtures", "snapshot.workspace.toml")
	pluginsDir := filepath.Join(root, "internal", "plugin", "catalog")
	ws, err := config.LoadWorkspace(wsPath)
	if err != nil {
		t.Fatalf("load workspace: %v", err)
	}
	ws.Mounts = []config.Mount{{Source: "/host/x", Target: "/home/${USERNAME}"}}

	var warns bytes.Buffer
	plugins, err := plugin.LoadEnabledFromFS(os.DirFS(pluginsDir), ws.Plugins.Enable, &warns, pluginsDir)
	if err != nil {
		t.Fatalf("load plugins: %v", err)
	}
	ctx := &generate.WorkspaceContext{WS: ws, PluginsFS: os.DirFS(pluginsDir), Plugins: plugins, Warnings: &warns}
	got, err := dockerfile.Generate(ctx, dockerfile.Options{
		WorkspaceRoot: root, RepoDir: "cocoon", Plugins: plugins, Warnings: &warns,
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "ENV COCOON_BIND_PATHS=") {
			if !strings.Contains(line, "/home/testuser\"") && !strings.Contains(line, "/home/testuser:") {
				t.Errorf("home-root mount missing from COCOON_BIND_PATHS: %s", line)
			}
			return
		}
	}
	t.Fatalf("ENV COCOON_BIND_PATHS not found in:\n%s", got)
}

// TestEntrypointScript pins the contract of the embedded entrypoint: a bash
// script that branches on root and drops privileges via setpriv.
func TestEntrypointScript(t *testing.T) {
	t.Parallel()

	got := dockerfile.EntrypointScript()
	for _, want := range []string{
		"#!/bin/bash\n",
		"COCOON_WORKSPACE",
		"COCOON_BIND_PATHS",
		`[ "$(id -u)" -ne 0 ]`,
		"setpriv",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("EntrypointScript missing %q", want)
		}
	}
}

// TestEntrypointScript_BashSyntax runs `bash -n` over the embedded
// docker-entrypoint.sh. The string-contains test above cannot catch a
// parse error, and a broken entrypoint only fails at container start —
// while running as root mid-uid/gid-remap.
func TestEntrypointScript_BashSyntax(t *testing.T) {
	t.Parallel()
	assertBashSyntax(t, dockerfile.EntrypointScript())
}

// assertBashSyntax writes script to a temp file and runs `bash -n` over it,
// failing on any parse error. Skips when bash is not on PATH.
func assertBashSyntax(t *testing.T, script string) {
	t.Helper()
	bashPath, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not on PATH")
	}
	scriptPath := filepath.Join(t.TempDir(), "script.sh")
	if writeErr := os.WriteFile(scriptPath, []byte(script), 0o600); writeErr != nil {
		t.Fatalf("write script: %v", writeErr)
	}
	cmd := exec.CommandContext(t.Context(), bashPath, "-n", scriptPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if runErr := cmd.Run(); runErr != nil {
		t.Fatalf("bash -n rejected the embedded script: %v\n%s", runErr, stderr.String())
	}
}

// TestGenerate_PasswordSudo pins the password-sudo RUN block: it mounts the
// sudo_password build secret, fails the build with distinct causes for a
// missing/empty secret vs an absent SUDO_PASSWORD line, and rewrites the
// sudoers entry to require a password. It is emitted AFTER the plugin installs
// (the snapshot fixture enables aws-cli, which runs `sudo ./aws/install` at
// build time) so build-time sudo stays passwordless until the final lock-down.
// The base passwordless user-creation line is untouched; nopasswd output
// carries neither the secret mount nor chpasswd.
//
//nolint:paralleltest // loads the shared snapshot fixture twice; cheap, no shared state
func TestGenerate_PasswordSudo(t *testing.T) {
	root := repoRoot(t)
	wsPath := filepath.Join(root, "tests", "fixtures", "snapshot.workspace.toml")
	pluginsDir := filepath.Join(root, "internal", "plugin", "catalog")
	gen := func(mode string) string {
		ws, err := config.LoadWorkspace(wsPath)
		if err != nil {
			t.Fatalf("load workspace: %v", err)
		}
		if mode != "" {
			m := mode
			ws.Container.Sudo = &config.SudoSpec{Mode: &m}
		}
		var warns bytes.Buffer
		plugins, err := plugin.LoadEnabledFromFS(os.DirFS(pluginsDir), ws.Plugins.Enable, &warns, pluginsDir)
		if err != nil {
			t.Fatalf("load plugins: %v", err)
		}
		ctx := &generate.WorkspaceContext{WS: ws, PluginsFS: os.DirFS(pluginsDir), Plugins: plugins, Warnings: &warns}
		got, gerr := dockerfile.Generate(ctx, dockerfile.Options{
			WorkspaceRoot: root, RepoDir: "cocoon", Plugins: plugins, Warnings: &warns,
		})
		if gerr != nil {
			t.Fatalf("generate: %v", gerr)
		}
		return got
	}

	pw := gen(config.SudoModePassword)
	for _, want := range []string{
		"RUN --mount=type=secret,id=sudo_password",
		"SUDO_PASSWORD",
		`echo "${USERNAME} ALL=(ALL) ALL" > /etc/sudoers.d/${USERNAME}`,
		"| chpasswd",
		// Distinct fail-fast causes (commit 7f400db): the pre-parse guard and
		// the two separate messages must all survive — collapsing them would
		// re-introduce the ambiguity, and dropping the [ -s ] check would let
		// an empty secret fall through to an empty password.
		`if [ ! -s "$secret" ]`,
		"the sudo_password build secret is missing or empty",
		"requires a non-empty SUDO_PASSWORD line",
	} {
		if !strings.Contains(pw, want) {
			t.Errorf("password Dockerfile missing %q\n--- got ---\n%s", want, pw)
		}
	}
	// The base user-creation RUN still writes the passwordless line first
	// (the password RUN overwrites it).
	if !strings.Contains(pw, `echo "${USERNAME} ALL=(ALL) NOPASSWD:ALL"`) {
		t.Error("password Dockerfile dropped the base user-creation NOPASSWD line")
	}
	// The password RUN must come AFTER plugin installs so build-time sudo is
	// still passwordless when an installer calls it (aws-cli runs
	// `sudo ./aws/install`); tightening sudoers before that breaks the build.
	awsIdx := strings.Index(pw, "sudo ./aws/install")
	pwIdx := strings.Index(pw, "id=sudo_password")
	if awsIdx < 0 {
		t.Fatal("snapshot fixture expected to enable aws-cli (sudo ./aws/install not found)")
	}
	if pwIdx < 0 || awsIdx > pwIdx {
		t.Errorf("password setup must come after plugin installs that use sudo (awsIdx=%d pwIdx=%d)", awsIdx, pwIdx)
	}
	// And after the USER switch (it is the final root-context step).
	if pwIdx < strings.Index(pw, "\nUSER ${USERNAME}") {
		t.Error("password setup RUN must come after the USER ${USERNAME} switch")
	}

	nopw := gen("")
	if strings.Contains(nopw, "sudo_password") || strings.Contains(nopw, "chpasswd") {
		t.Errorf("nopasswd Dockerfile must not emit the secret mount or chpasswd\n--- got ---\n%s", nopw)
	}
}
