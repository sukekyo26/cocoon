// Package dockerfile generates the workspace Dockerfile.
package dockerfile

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sukekyo26/cocoon/internal/aptbase"
	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/generate/shellrc"
	"github.com/sukekyo26/cocoon/internal/generate/shellx"
	"github.com/sukekyo26/cocoon/internal/generate/tmplx"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

// entrypointScript is the contents of docker-entrypoint.sh that the generator
// writes alongside the generated Dockerfile (.devcontainer/docker-entrypoint.sh).
// It is embedded so cocoon ships as a single binary with no host-side script
// dependency.
//
//go:embed entrypoint.sh
var entrypointScript string

// EntrypointScript returns the docker-entrypoint.sh contents that should be
// written next to the generated Dockerfile. Callers (cocoon gen) add it to
// the artifact set with mode 0o755.
func EntrypointScript() string { return entrypointScript }

// ErrInvalidVersionOverride is returned when [plugins.versions] references an
// unknown plugin or one whose install method does not allow pinning.
var ErrInvalidVersionOverride = errors.New("dockerfile: invalid version override")

// Options carries inputs Generate needs beyond ctx.
type Options struct {
	WorkspaceRoot string
	// RepoDir overrides the directory name baked into the rc-loader echo
	// lines. When empty, filepath.Base(WorkspaceRoot) is used. Tests set
	// this to a fixed value so snapshot output does not depend on the
	// checkout's directory name.
	RepoDir  string
	Plugins  map[string]*plugin.Plugin
	Warnings io.Writer
}

type templateData struct {
	Image                  string
	ImageVersion           string
	AptMirrorRewritePre    string
	AptProxyConfPre        string
	CertInstallRoot        string
	AptCABootstrap         string
	AptMirrorRewrite       string
	AptThirdParty          string
	AptBasePackages        string
	AptShellPackages       string
	AptPluginPackages      string
	AptExtraPackages       string
	LocaleGenList          string
	LocaleLang             string
	LocaleLanguage         string
	RepoDir                string
	LoginShellPath         string
	RCFilePath             string
	CustomShellRC          string
	ShellCompletionInit    string
	ShellHistoryInit       string
	DockerfilePreUserSetup string
	SkelCopies             string
	GitConfig              string
	CustomCertificates     string
	PluginInstalls         string
	DockerfilePostPlugins  string
}

//nolint:lll // Dockerfile RUN/echo lines cannot be wrapped without changing semantics.
var tmpl = tmplx.MustParse("dockerfile", `# syntax=docker/dockerfile:1.7
# Auto-generated from workspace.toml — do not edit directly.
ARG IMAGE={{ .Image }}
ARG IMAGE_VERSION={{ .ImageVersion }}
FROM ${IMAGE}:${IMAGE_VERSION}

ARG USERNAME
ARG UID
ARG GID

{{ with .AptMirrorRewritePre -}}
{{ . }}

{{ end -}}
{{ with .AptProxyConfPre -}}
{{ . }}

{{ end -}}
{{ with .CertInstallRoot -}}
{{ . }}

{{ end -}}
{{ with .AptCABootstrap -}}
{{ . }}

{{ end -}}
{{ with .AptMirrorRewrite -}}
{{ . }}

{{ end -}}
{{ with .AptThirdParty -}}
{{ . }}

{{ end -}}
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    rm -f /etc/apt/apt.conf.d/docker-clean && \
    apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
{{ .AptBasePackages }}
{{- with .AptShellPackages }}
{{ . }}
{{- end }}
{{- with .AptPluginPackages }}
{{ . }}
{{- end }}
{{- with .AptExtraPackages }}
{{ . }}
{{- end }}
    && locale-gen {{ .LocaleGenList }} \
    && rm -rf /tmp/* /var/tmp/*

{{ with .DockerfilePreUserSetup -}}
{{ . }}

{{ end -}}
{{ with .SkelCopies -}}
{{ . }}

{{ end -}}
RUN existing_user="$(getent passwd ${UID} | cut -d: -f1)" && \
    if [ -n "$existing_user" ] && [ "$existing_user" != "${USERNAME}" ]; then \
        userdel -r "$existing_user" 2>/dev/null || true; \
    fi && \
    existing_group="$(getent group ${GID} | cut -d: -f1)" && \
    if [ -n "$existing_group" ] && [ "$existing_group" != "${USERNAME}" ]; then \
        groupdel "$existing_group" 2>/dev/null || true; \
    fi && \
    groupadd -g ${GID} ${USERNAME} && \
    useradd -m -s {{ .LoginShellPath }} -u ${UID} -g ${GID} ${USERNAME} && \
    usermod -aG sudo ${USERNAME} && \
    echo "${USERNAME} ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/${USERNAME} && \
    chmod 0440 /etc/sudoers.d/${USERNAME}

USER ${USERNAME}
WORKDIR /home/${USERNAME}

# Set locale environment variables
ENV LANG={{ .LocaleLang }}
ENV LANGUAGE={{ .LocaleLanguage }}
ENV LC_ALL={{ .LocaleLang }}

# Ensure the active login shell's rc file exists before any later RUN
# (plugin install scripts, cert env exports, completion init) appends to it
# via >> "$RC_FILE". fish in particular keeps its config under
# ~/.config/fish/, a path useradd does not pre-create.
RUN mkdir -p "$(dirname "$HOME/{{ .RCFilePath }}")" && touch "$HOME/{{ .RCFilePath }}"

# Seed ~/.cocoon with placeholder rc files so the cocoon named volume
# (mounted at /home/${USERNAME}/.cocoon at runtime) inherits these comment
# headers on first start. Edits inside the container persist across
# 'docker compose down' + 'up --build' because the named volume keeps the
# writable layer; only 'down -v' resets it.
RUN mkdir -p "$HOME/.cocoon" && \
    printf '# cocoon: persisted shell rc (bash/zsh). Edit freely; survives container rebuild.\n' \
        > "$HOME/.cocoon/.shellrc" && \
    printf '# cocoon: persisted shell rc (fish). Edit freely; survives container rebuild.\n' \
        > "$HOME/.cocoon/.shellrc.fish"

{{ with .ShellCompletionInit -}}
{{ . }}

{{ end -}}
{{ with .GitConfig -}}
{{ . }}

{{ end -}}
{{ with .CustomCertificates -}}
{{ . }}

{{ end -}}
{{ with .PluginInstalls -}}
{{ . }}

{{ end -}}
{{ with .DockerfilePostPlugins -}}
{{ . }}

{{ end -}}
# Backup image-installed files for volume-mount resilience
RUN if [ -d ~/.local ]; then cp -a ~/.local ~/.image-local; else mkdir -p ~/.image-local; fi

# create volume mount directories for permission
RUN mkdir -p ~/.local

# Setup persistent shell history
{{ .ShellHistoryInit }}

{{ with .CustomShellRC -}}
{{ . }}

{{ end -}}
# Entrypoint: sync image files to volume-mounted ~/.local
USER root
COPY .devcontainer/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh
USER ${USERNAME}

ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["sleep", "infinity"]

WORKDIR /home/${USERNAME}/workspace
`, nil)

// Generate produces Dockerfile contents from ctx and opts.
func Generate(ctx *generate.WorkspaceContext, opts Options) (string, error) {
	root := opts.WorkspaceRoot
	if root == "" {
		root = ctx.ProjectDir
	}
	configDir := filepath.Join(root, "config")

	customVolPaths := sortedValues(ctx.CustomVolumes())
	overrides := ctx.PluginVersionOverrides()
	enabled := ctx.EnabledPlugins()

	pluginInstalls, err := generatePluginInstalls(
		opts.Plugins, enabled, ctx.PluginsFS, customVolPaths, overrides, opts.Warnings,
		shellEnv{
			rcFileAbs:  ctx.RCFileAbs(),
			rcSyntax:   ctx.RCSyntax(),
			loginShell: ctx.LoginShell(),
		})
	if err != nil {
		return "", err
	}
	var certInstallRoot, certInstallEnv string
	if ctx.CertificatesEnabled() {
		certInstallRoot, certInstallEnv = generateCertificateInstall()
	}
	aptCABootstrap := buildAptCABootstrap(ctx)

	mirrorRewritePre, mirrorRewrite, proxyConfPre := splitAptSetupForBootstrap(ctx)

	aptBase := readAptPackages(configDir)
	basePkgNames := parseBasePackages(aptBase)
	aptPlugin := collectPluginAptPackages(opts.Plugins, enabled, basePkgNames)

	aptExtraPkgs := ctx.AptExtraPackages()
	for _, pkg := range aptExtraPkgs {
		if _, dup := basePkgNames[pkg]; dup && opts.Warnings != nil {
			fmt.Fprintf(opts.Warnings,
				"WARNING: [apt] packages contains '%s', which is already in "+
					"apt-base-packages.conf. Remove duplicates from [apt] packages "+
					"in workspace.toml to avoid redundant installs.\n", pkg)
		}
	}
	aptExtra := formatAptContinuations(aptExtraPkgs)

	gitConfig := buildGitConfig(ctx)
	preUser, postPlugins := buildDockerfileHooks(ctx, opts.Warnings)
	genList, lang, language := ctx.ResolveLocale()

	loginShell := ctx.LoginShell()
	rcPath := ctx.RCFilePath()
	aptShellList := filterShellPackages(ctx.LoginShellAptPackages(), basePkgNames)

	customShellRC, err := shellrc.RenderDockerfileBlock(ctx)
	if err != nil {
		return "", fmt.Errorf("dockerfile: %w", err)
	}

	data := templateData{
		Image:                  config.ResolveImageRegistry(ctx.WS.Container.Image),
		ImageVersion:           ctx.WS.Container.ImageVersion,
		AptMirrorRewritePre:    mirrorRewritePre,
		AptProxyConfPre:        proxyConfPre,
		CertInstallRoot:        certInstallRoot,
		AptCABootstrap:         aptCABootstrap,
		AptMirrorRewrite:       mirrorRewrite,
		AptThirdParty:          buildAptThirdParty(ctx),
		AptBasePackages:        strings.TrimRight(aptBase, "\n"),
		AptShellPackages:       strings.TrimRight(formatAptContinuations(aptShellList), "\n"),
		AptPluginPackages:      strings.TrimRight(aptPlugin, "\n"),
		AptExtraPackages:       strings.TrimRight(aptExtra, "\n"),
		LocaleGenList:          genList,
		LocaleLang:             lang,
		LocaleLanguage:         language,
		RepoDir:                pickRepoDir(opts.RepoDir, root),
		LoginShellPath:         ctx.LoginShellPath(),
		RCFilePath:             rcPath,
		CustomShellRC:          customShellRC,
		ShellCompletionInit:    buildShellCompletionInit(loginShell, rcPath),
		ShellHistoryInit:       buildShellHistoryInit(loginShell, rcPath),
		DockerfilePreUserSetup: preUser,
		SkelCopies:             buildSkelCopies(ctx),
		GitConfig:              gitConfig,
		CustomCertificates:     certInstallEnv,
		PluginInstalls:         pluginInstalls,
		DockerfilePostPlugins:  postPlugins,
	}

	out, err := tmplx.Render(tmpl, data)
	if err != nil {
		return "", fmt.Errorf("dockerfile: %w", err)
	}
	return out, nil
}

// pickRepoDir returns the explicit override when set, otherwise the basename
// of the workspace root. Tests rely on this so the Dockerfile snapshot does
// not depend on the local checkout's directory name.
func pickRepoDir(override, root string) string {
	if override != "" {
		return override
	}
	return filepath.Base(root)
}

// filterShellPackages drops any LoginShellAptPackages() entry that's already
// in the base package set (e.g. when a user adds bash-completion to base).
func filterShellPackages(pkgs []string, base map[string]struct{}) []string {
	out := make([]string, 0, len(pkgs))
	for _, p := range pkgs {
		if _, dup := base[p]; dup {
			continue
		}
		out = append(out, p)
	}
	return out
}

// buildShellCompletionInit returns the RUN block that wires up shell
// completion in the chosen login shell's rc file. Empty for fish (its native
// completions auto-load with no rc tweak).
func buildShellCompletionInit(shell, rcFile string) string {
	switch shell {
	case "bash":
		rc := `"$HOME/` + rcFile + `"`
		return "# Enable bash completion\n" +
			"RUN echo 'if [ -f /usr/share/bash-completion/bash_completion ]; then' >> " + rc + " && \\\n" +
			"    echo '  . /usr/share/bash-completion/bash_completion' >> " + rc + " && \\\n" +
			"    echo 'fi' >> " + rc
	case "zsh":
		rc := `"$HOME/` + rcFile + `"`
		return "# Enable zsh completion + autosuggestions\n" +
			"RUN echo 'autoload -Uz compinit && compinit' >> " + rc + " && \\\n" +
			"    echo 'source /usr/share/zsh-autosuggestions/zsh-autosuggestions.zsh 2>/dev/null || true' >> " + rc
	case "fish":
		return ""
	}
	return ""
}

// buildShellHistoryInit returns the RUN block that configures shell history
// for the chosen login shell. Each shell uses a separate state file.
func buildShellHistoryInit(shell, rcFile string) string {
	switch shell {
	case "bash":
		rc := `"$HOME/` + rcFile + `"`
		return "RUN echo 'export HISTFILE=~/.local/state/.bash_history_docker' >> " + rc + " && \\\n" +
			"    echo 'export HISTSIZE=10000' >> " + rc + " && \\\n" +
			"    echo 'export HISTFILESIZE=20000' >> " + rc + " && \\\n" +
			"    mkdir -p ~/.local/state && touch ~/.local/state/.bash_history_docker"
	case "zsh":
		rc := `"$HOME/` + rcFile + `"`
		return "RUN echo 'export HISTFILE=~/.local/state/.zsh_history_docker' >> " + rc + " && \\\n" +
			"    echo 'export HISTSIZE=10000' >> " + rc + " && \\\n" +
			"    echo 'export SAVEHIST=20000' >> " + rc + " && \\\n" +
			"    mkdir -p ~/.local/state && touch ~/.local/state/.zsh_history_docker"
	case "fish":
		// fish manages history natively under ~/.local/share/fish; just
		// pre-create the state dir so first-run doesn't warn.
		return "RUN mkdir -p ~/.local/share/fish && touch ~/.local/share/fish/fish_history"
	}
	return ""
}

// formatAptContinuations renders one Dockerfile RUN-continuation line per
// package: "    pkg \\\n". Returns "" when packages is empty.
func formatAptContinuations(packages []string) string {
	if len(packages) == 0 {
		return ""
	}
	var b strings.Builder
	for _, p := range packages {
		b.WriteString("    " + p + " \\\n")
	}
	return b.String()
}

func sortedValues(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, m[k])
	}
	return out
}

// splitAptSetupForBootstrap places [apt.mirror] / [apt.proxy] relative
// to the bootstrap apt activity (cert install RUN / AptCABootstrap):
// HTTP mirrors and proxy go pre-bootstrap (the bootstrap's apt-get
// update must reach the archive); HTTPS mirrors go post-bootstrap (TLS
// to them needs ca-certificates first).
func splitAptSetupForBootstrap(
	ctx *generate.WorkspaceContext,
) (mirrorPre, mirrorPost, proxyPre string) {
	proxyPre = buildAptProxyConf(ctx)
	mirror := buildAptMirrorRewrite(ctx)
	if mirror != "" {
		if strings.HasPrefix(ctx.AptMirrorURL(), "https://") {
			mirrorPost = mirror
		} else {
			mirrorPre = mirror
		}
	}
	return mirrorPre, mirrorPost, proxyPre
}

// buildAptCABootstrap returns a RUN block that installs ca-certificates
// from the default Ubuntu HTTP archive before any HTTPS mirror rewrite or
// third-party HTTPS source takes effect. ubuntu:24.04 ships without a CA
// bundle, so an apt-get update / curl against an HTTPS endpoint would fail
// TLS handshake. Returns "" unless [apt.mirror].url is https:// or any
// [[apt.sources]] entry uses an https:// URL or key_url, keeping the HTTP
// and no-mirror paths byte-identical to today's output.
func buildAptCABootstrap(ctx *generate.WorkspaceContext) string {
	if !strings.HasPrefix(ctx.AptMirrorURL(), "https://") && !hasHTTPSAptSource(ctx) {
		return ""
	}
	return "# Pre-install ca-certificates from the default HTTP archive so the\n" +
		"# subsequent HTTPS [apt.mirror] can complete its TLS handshake.\n" +
		"RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \\\n" +
		"    --mount=type=cache,target=/var/lib/apt,sharing=locked \\\n" +
		"    apt-get update && \\\n" +
		"    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends ca-certificates"
}

// hasHTTPSAptSource reports whether any [[apt.sources]] entry has an
// https:// URL or key_url, in which case the build needs ca-certificates
// installed before apt-get update or the keyring curl can run.
func hasHTTPSAptSource(ctx *generate.WorkspaceContext) bool {
	for _, src := range ctx.AptSources() {
		if strings.HasPrefix(src.URL, "https://") || strings.HasPrefix(src.KeyURL, "https://") {
			return true
		}
	}
	return false
}

// buildAptMirrorRewrite returns a RUN block that rewrites the default
// upstream archive URLs to the configured mirror URL, touching both the
// legacy sources.list path and the deb822 files under sources.list.d. The
// pattern set is OS-specific: Ubuntu has three archive hosts
// (archive.ubuntu.com / security.ubuntu.com / ports.ubuntu.com) while
// Debian 12+ publishes from a single host with two paths
// (deb.debian.org/debian for the main archive and deb.debian.org/debian-security
// for security updates). Returns "" when no mirror is configured. The URL is
// validated to contain neither `'` nor `|`, so single-quoting the sed
// expressions is safe without further escaping.
func buildAptMirrorRewrite(ctx *generate.WorkspaceContext) string {
	url := ctx.AptMirrorURL()
	if url == "" {
		return ""
	}
	originHosts := aptMirrorOriginHosts(ctx.WS.Container.Image)
	var sedLines strings.Builder
	for _, host := range originHosts {
		sedLines.WriteString("    -e 's|" + host + "|" + url + "|g' \\\n")
	}
	return "# Rewrite upstream apt archive URLs to the configured [apt.mirror].url\n" +
		"RUN sed -i \\\n" +
		sedLines.String() +
		"    /etc/apt/sources.list /etc/apt/sources.list.d/*.list /etc/apt/sources.list.d/*.sources 2>/dev/null || true"
}

// aptMirrorOriginHosts returns the set of upstream archive URL prefixes the
// generator rewrites when [apt.mirror].url is set. Ubuntu and Debian publish
// from disjoint hosts; the list is keyed off [container].image so a Debian
// build does not emit useless Ubuntu sed expressions (and vice versa).
//
// Only "ubuntu" maps to the Ubuntu archive hosts; every other supported
// image (debian, node, python, go, rust, deno) is Debian-based and uses
// deb.debian.org regardless of which language-runtime layer sits on top.
//
// Order matters. The slice is consumed top-down by sed -e expressions, and
// each expression sees the line as already-rewritten by every earlier one.
// On Debian, "http://deb.debian.org/debian" is a strict prefix of
// "http://deb.debian.org/debian-security", so the security entry must be
// rewritten first — otherwise sed would replace the prefix and leave a
// nonsensical "<mirror>-security" tail. The Ubuntu entries do not overlap
// (different hostnames), but they are listed longest-first too so that the
// invariant "more specific patterns precede their prefixes" stays uniform.
func aptMirrorOriginHosts(image string) []string {
	if image == "ubuntu" {
		return []string{
			"http://archive.ubuntu.com/ubuntu/",
			"http://security.ubuntu.com/ubuntu/",
			"http://ports.ubuntu.com/ubuntu-ports/",
		}
	}
	return []string{
		"http://deb.debian.org/debian-security",
		"http://deb.debian.org/debian",
	}
}

// buildAptProxyConf writes /etc/apt/apt.conf.d/95proxy with the configured
// HTTP/HTTPS proxies. Returns "" when [apt.proxy] is unset or both fields
// are nil.
func buildAptProxyConf(ctx *generate.WorkspaceContext) string {
	p := ctx.AptProxy()
	if p == nil || (p.HTTP == nil && p.HTTPS == nil) {
		return ""
	}
	var lines []string
	if p.HTTP != nil {
		lines = append(lines, fmt.Sprintf("Acquire::http::Proxy %q;", *p.HTTP))
	}
	if p.HTTPS != nil {
		lines = append(lines, fmt.Sprintf("Acquire::https::Proxy %q;", *p.HTTPS))
	}
	cmd := "RUN { "
	for i, l := range lines {
		if i > 0 {
			cmd += " "
		}
		cmd += "echo " + shellx.ShellQuote(l) + ";"
	}
	cmd += " } > /etc/apt/apt.conf.d/95proxy"
	return "# Configure apt HTTP(S) proxy from [apt.proxy]\n" + cmd
}

// buildAptThirdParty installs ca-certificates / curl / gpg, then writes one
// keyring + sources.list.d entry per [[apt.sources]] block, so subsequent
// apt-get update sees the third-party repository. Returns "" when no
// sources are declared.
func buildAptThirdParty(ctx *generate.WorkspaceContext) string {
	sources := ctx.AptSources()
	if len(sources) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("# Add third-party apt repositories from [[apt.sources]]\n")
	b.WriteString("RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \\\n")
	b.WriteString("    --mount=type=cache,target=/var/lib/apt,sharing=locked \\\n")
	b.WriteString("    apt-get update && \\\n")
	b.WriteString("    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \\\n")
	b.WriteString("        ca-certificates curl gnupg && \\\n")
	b.WriteString("    install -d -m 0755 /etc/apt/keyrings")
	for _, src := range sources {
		archAttr := ""
		if src.Arch != nil {
			archAttr = "arch=" + *src.Arch + " "
		}
		debLine := fmt.Sprintf("deb [%ssigned-by=/etc/apt/keyrings/%s.gpg] %s %s %s",
			archAttr, src.Name, src.URL, src.Suite, strings.Join(src.Components, " "))
		fmt.Fprintf(&b, " && \\\n    curl -fsSL %s | gpg --dearmor -o /etc/apt/keyrings/%s.gpg",
			shellx.ShellQuote(src.KeyURL), src.Name)
		fmt.Fprintf(&b, " && \\\n    echo %s > /etc/apt/sources.list.d/%s.list",
			shellx.ShellQuote(debLine), src.Name)
	}
	return b.String()
}

// buildSkelCopies emits one Dockerfile COPY per [[container.skel]] entry.
// Files land under /etc/skel and are picked up automatically by the
// subsequent `useradd -m` (Linux's standard skeleton-copy behaviour, which
// also chowns to the new user — no explicit chown needed here). The
// generator inserts this block between the pre_user_setup hook and the
// useradd RUN, where the build is still executing as root.
func buildSkelCopies(ctx *generate.WorkspaceContext) string {
	entries := ctx.SkelEntries()
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("# Dotfiles seeded into the new user's home via /etc/skel\n")
	for i, e := range entries {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "COPY %s /etc/skel/%s", e.Source, e.Target)
	}
	return b.String()
}

func buildGitConfig(ctx *generate.WorkspaceContext) string {
	name := ctx.GitUserName()
	email := ctx.GitUserEmail()
	var lines []string
	if name != "" {
		lines = append(lines, "git config --system user.name  "+shellx.ShellQuote(name))
	}
	if email != "" {
		lines = append(lines, "git config --system user.email "+shellx.ShellQuote(email))
	}
	if len(lines) == 0 {
		return ""
	}
	joined := strings.Join(lines, " \\\n && ")
	return "# Git identity from [git] section of workspace.toml\n" +
		"USER root\n" +
		"RUN " + joined + "\n" +
		"USER ${USERNAME}"
}

func buildDockerfileHooks(ctx *generate.WorkspaceContext, warnings io.Writer) (preUser, postPlugins string) {
	wrap := func(content, label string) string {
		text := strings.TrimSpace(content)
		if text == "" {
			return ""
		}
		if warnings != nil {
			fmt.Fprintf(warnings,
				"WARNING: Custom Dockerfile instructions from [dockerfile].%s "+
					"are being injected verbatim. You are responsible for their safety.\n",
				label)
		}
		return fmt.Sprintf("# === user-defined dockerfile.%s (from workspace.toml) ===\n%s\n# === end dockerfile.%s ===",
			label, text, label)
	}
	return wrap(ctx.DockerfilePreUserSetup(), "pre_user_setup"),
		wrap(ctx.DockerfilePostPlugins(), "post_plugins")
}

// readAptPackages reads config/apt-base-packages.conf and formats each
// non-blank, non-comment line as a Dockerfile continuation.
//
// The configDir argument is retained for compatibility with the v1
// caller signature; cocoon ignores it because the base apt set is
// now hardcoded in internal/setup.MinimalBasePackages. Users add the
// rest through [apt] packages (cocoon init's AptCategories picker
// pre-fills common groups).
func readAptPackages(configDir string) string {
	_ = configDir
	pkgs := aptbase.MinimalBasePackages
	if len(pkgs) == 0 {
		return ""
	}
	lines := make([]string, 0, len(pkgs))
	for _, p := range pkgs {
		lines = append(lines, "    "+p+" \\")
	}
	return strings.Join(lines, "\n") + "\n"
}

func parseBasePackages(aptBase string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, line := range strings.Split(aptBase, "\n") {
		s := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(line), `\`))
		if s != "" {
			out[s] = struct{}{}
		}
	}
	return out
}

func pluginAptPackages(p *plugin.Plugin) []string {
	if p == nil || p.Apt == nil {
		return nil
	}
	return p.Apt.Packages
}

func collectPluginAptPackages(
	plugins map[string]*plugin.Plugin, enabled []string, basePackages map[string]struct{},
) string {
	seen := map[string]struct{}{}
	var packages []string
	for _, id := range enabled {
		p, ok := plugins[id]
		if !ok {
			continue
		}
		for _, pkg := range pluginAptPackages(p) {
			if _, dup := seen[pkg]; dup {
				continue
			}
			if _, base := basePackages[pkg]; base {
				continue
			}
			seen[pkg] = struct{}{}
			packages = append(packages, pkg)
		}
	}
	return formatAptContinuations(packages)
}

// generateCertificateInstall returns the cert install RUN block + the
// post-USER SSL_CERT_FILE / etc. ENV block. Generate() calls this only
// when CertificatesEnabled() is true; on the disabled branch neither
// block lands in the Dockerfile.
func generateCertificateInstall() (rootBlock, envBlock string) {
	return certInstallRootBlock, certInstallEnvBlock
}

//nolint:lll // Dockerfile RUN/ENV lines embed shell semantics verbatim.
var certInstallRootBlock = `# Install custom CA certificates from ~/.cocoon/certs/ (no-op when the
# host directory is empty). Runs before the main apt install so HTTPS
# mirrors signed by a corporate CA can complete TLS handshake.
RUN --mount=type=bind,from=` + generate.CertsBuildContextName + `,target=/tmp/cocoon-user-certs \
    --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    if [ -n "$(find /tmp/cocoon-user-certs -maxdepth 1 -name '*.crt' -print -quit)" ]; then \
        apt-get update && \
        DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends ca-certificates && \
        mkdir -p /usr/local/share/ca-certificates/cocoon-user && \
        find /tmp/cocoon-user-certs -maxdepth 1 -name '*.crt' \
            -exec cp {} /usr/local/share/ca-certificates/cocoon-user/ \; && \
        update-ca-certificates; \
    fi`

//nolint:lll // ENV declarations are single Dockerfile lines.
const certInstallEnvBlock = `# CA bundle env vars for tools that read them directly.
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
ENV CURL_CA_BUNDLE=/etc/ssl/certs/ca-certificates.crt
ENV REQUESTS_CA_BUNDLE=/etc/ssl/certs/ca-certificates.crt
ENV NODE_EXTRA_CA_CERTS=/etc/ssl/certs/ca-certificates.crt`
