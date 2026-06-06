// Package dockerfile generates the workspace Dockerfile.
package dockerfile

import (
	_ "embed"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/sukekyo26/cocoon/internal/aptbase"
	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/generate/shellrc"
	"github.com/sukekyo26/cocoon/internal/generate/shellx"
	"github.com/sukekyo26/cocoon/internal/generate/tmplx"
	"github.com/sukekyo26/cocoon/internal/plugin"
	"github.com/sukekyo26/cocoon/internal/warn"
)

// entrypointScript embeds docker-entrypoint.sh so cocoon ships as a single
// binary with no host-side script dependency.
//
//go:embed entrypoint.sh
var entrypointScript string

// EntrypointScript is written next to the Dockerfile with mode 0o755.
func EntrypointScript() string { return entrypointScript }

// ErrInvalidVersionOverride is returned when the enable array pins an unknown
// plugin or one whose install method does not allow pinning.
var ErrInvalidVersionOverride = errors.New("dockerfile: invalid version override")

// ErrUnknownExtraVersion is returned when [plugins.options].<id> sets an
// extra key that the plugin's plugin.toml does not declare under
// [install.extra_versions]. Surfaced as a sentinel so callers and tests can
// match the typo-detection failure class with errors.Is.
var ErrUnknownExtraVersion = errors.New("dockerfile: unknown extra version key")

// Options carries inputs Generate needs beyond ctx.
type Options struct {
	WorkspaceRoot string
	// RepoDir overrides the directory name baked into the rc-loader echo
	// lines. When empty, filepath.Base(WorkspaceRoot) is used. Tests set
	// this to a fixed value so snapshot output does not depend on the
	// checkout's directory name.
	RepoDir  string
	Plugins  map[string]*plugin.Plugin
	Warnings *warn.Sink
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
	LocaleSedScript        string
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
	CustomCertificates     string
	PluginInstalls         string
	DockerfilePostPlugins  string
	CocoonWorkspace        string
	CocoonBindPaths        string
	WorkspaceDir           string
	SudoPasswordSetup      string
}

//nolint:lll // Dockerfile RUN/echo lines cannot be wrapped without changing semantics.
var tmpl = tmplx.MustParse("dockerfile", `# syntax=docker/dockerfile:1.7
# Auto-generated from workspace.toml — do not edit directly.
ARG IMAGE={{ .Image }}
ARG IMAGE_VERSION={{ .ImageVersion }}
FROM ${IMAGE}:${IMAGE_VERSION}

ARG USERNAME

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
    && sed -i -E {{ .LocaleSedScript }} /etc/locale.gen \
    && locale-gen \
    && rm -rf /tmp/* /var/tmp/*

{{ with .DockerfilePreUserSetup -}}
{{ . }}

{{ end -}}
{{ with .SkelCopies -}}
{{ . }}

{{ end -}}
# The container user is created with a FIXED uid/gid (1000) so the committed
# .devcontainer/ stays host-independent; docker-entrypoint.sh remaps it to the
# host owner at container start. A base-image account already on 1000
# (e.g. ubuntu:24.04 ships 'ubuntu') is removed first.
RUN existing_user="$(getent passwd 1000 | cut -d: -f1)" && \
    if [ -n "$existing_user" ] && [ "$existing_user" != "${USERNAME}" ]; then \
        userdel -r "$existing_user" 2>/dev/null || true; \
    fi && \
    existing_group="$(getent group 1000 | cut -d: -f1)" && \
    if [ -n "$existing_group" ] && [ "$existing_group" != "${USERNAME}" ]; then \
        groupdel "$existing_group" 2>/dev/null || true; \
    fi && \
    groupadd -g 1000 ${USERNAME} && \
    useradd -m -s {{ .LoginShellPath }} -u 1000 -g 1000 ${USERNAME} && \
    usermod -aG sudo ${USERNAME} && \
    echo "${USERNAME} ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/${USERNAME} && \
    chmod 0440 /etc/sudoers.d/${USERNAME}

USER ${USERNAME}
WORKDIR /home/${USERNAME}

# Set locale environment variables
ENV LANG={{ .LocaleLang }}
ENV LANGUAGE={{ .LocaleLanguage }}
ENV LC_ALL={{ .LocaleLang }}

# Inputs read by docker-entrypoint.sh, which remaps the fixed-uid user above
# to the host owner at container start.
ENV COCOON_USER=${USERNAME}
ENV COCOON_WORKSPACE="{{ .CocoonWorkspace }}"
ENV COCOON_BIND_PATHS="{{ .CocoonBindPaths }}"

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
# Entrypoint: runs as root to remap UID/GID to the host owner, then drops
# privileges to ${USERNAME} and re-execs itself. The unprivileged re-entry
# syncs image files into the volume-mounted ~/.local before exec'ing CMD.
USER root
COPY .devcontainer/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

{{ with .SudoPasswordSetup -}}
{{ . }}

{{ end -}}
ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["sleep", "infinity"]

WORKDIR /home/${USERNAME}/{{ .WorkspaceDir }}
`, nil)

// Generate produces Dockerfile contents.
func Generate(ctx *generate.WorkspaceContext, opts Options) (string, error) {
	root := opts.WorkspaceRoot
	if root == "" {
		root = ctx.ProjectDir
	}

	customVols := ctx.CustomVolumes()
	customVolPaths := make([]string, 0, len(customVols))
	for _, k := range slices.Sorted(maps.Keys(customVols)) {
		customVolPaths = append(customVolPaths, customVols[k])
	}
	overrides := ctx.EffectivePluginVersions()
	enabled := ctx.EnabledPlugins()

	// Gate the [plugins.options] manual checksum hatch on the pre-lock base
	// overrides, where a present checksum is necessarily user-typed.
	if err := validateManualChecksums(opts.Plugins, ctx.PluginVersionOverrides(), ctx.PluginMethods()); err != nil {
		return "", err
	}

	pluginInstalls, err := generatePluginInstalls(
		opts.Plugins, enabled, ctx.PluginsFS, customVolPaths, overrides, ctx.PluginMethods(), opts.Warnings,
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

	aptBase := baseAptPackagesBlock()
	basePkgNames := parseBasePackages(aptBase)
	aptPlugin := collectPluginAptPackages(opts.Plugins, enabled, basePkgNames)

	aptExtraPkgs := ctx.AptExtraPackages()
	warnDuplicateAptExtras(aptExtraPkgs, basePkgNames, opts.Warnings)
	aptExtra := formatAptContinuations(aptExtraPkgs)

	preUser, postPlugins := buildDockerfileHooks(ctx, opts.Warnings)
	_, lang, language := ctx.ResolveLocale()
	localeSed, err := ctx.LocaleSedScript()
	if err != nil {
		return "", fmt.Errorf("dockerfile: %w", err)
	}

	loginShell := ctx.LoginShell()
	rcPath := ctx.RCFilePath()
	aptShellList := filterShellPackages(ctx.LoginShellAptPackages(), basePkgNames)

	customShellRC, err := shellrc.RenderDockerfileBlock(ctx)
	if err != nil {
		return "", fmt.Errorf("dockerfile: %w", err)
	}

	cocoonWorkspace, cocoonBindPaths := cocoonEntrypointPaths(ctx)

	data := templateData{
		Image:                  ctx.WS.Container.Image,
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
		LocaleSedScript:        localeSed,
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
		CustomCertificates:     certInstallEnv,
		PluginInstalls:         pluginInstalls,
		DockerfilePostPlugins:  postPlugins,
		CocoonWorkspace:        cocoonWorkspace,
		CocoonBindPaths:        cocoonBindPaths,
		WorkspaceDir:           ctx.WS.Workspace.DirOrDefault(),
		SudoPasswordSetup:      buildSudoPasswordSetup(ctx),
	}

	out, err := tmplx.Render(tmpl, data)
	if err != nil {
		return "", fmt.Errorf("dockerfile: %w", err)
	}
	return out, nil
}

// buildSudoPasswordSetup returns the RUN block that rewrites the sudoers entry
// to require a password and sets that password from the .env.local build
// secret, or "" when password sudo is off. The base user-creation RUN wrote the
// passwordless entry; this overwrites it. It is emitted as the FINAL root step
// (after every plugin install), NOT next to the user-creation RUN: build-time
// plugin installs run as the unprivileged user and some (e.g. aws-cli,
// aws-sam-cli) call `sudo` for a privileged step, which relies on the
// passwordless entry still being active — tightening it earlier would break
// those builds with an opaque "a terminal is required" error. The build FAILS
// with a distinct cause when the secret file is missing/empty vs when the
// SUDO_PASSWORD key is absent, so password mode never silently degrades to
// passwordless (the whole point of the mode). The plaintext password arrives
// via a secret mount (not an ARG/ENV), so it never lands in an image layer, the
// build cache, or `docker inspect`; chpasswd writes only the derived hash to
// /etc/shadow (an image layer, as for any Unix account with a password).
//
//nolint:lll // shell RUN lines cannot be wrapped without changing semantics.
func buildSudoPasswordSetup(ctx *generate.WorkspaceContext) string {
	if !ctx.PasswordSudoEnabled() {
		return ""
	}
	// The secret file is checked before parsing so "secret missing/empty" and
	// "SUDO_PASSWORD key absent" fail with distinct messages; no `2>/dev/null`
	// masks read errors. A single sed (no head/tr pipeline) takes the first
	// matching line, strips the SUDO_PASSWORD= prefix and any trailing CR, then
	// quits — $(…) drops the trailing newline.
	return fmt.Sprintf(`# Set a password-required sudoers entry and the user's password from the %[1]s
# build secret. Emitted last (after every plugin install) so build-time sudo
# stays passwordless for installers that call it. The build fails with a clear
# cause when the secret is missing or empty, so password mode never silently
# degrades to passwordless. chpasswd writes the derived hash to /etc/shadow (an
# image layer, as for any Unix account); only the plaintext %[2]s — via the
# secret mount, not an ARG/ENV — stays out of image layers, the build cache,
# and docker inspect.
RUN --mount=type=secret,id=%[3]s \
    secret=/run/secrets/%[3]s; \
    if [ ! -s "$secret" ]; then \
        echo "cocoon: [container.sudo] mode=password but the %[3]s build secret is missing or empty" >&2; \
        exit 1; \
    fi; \
    pass="$(sed -n '/^%[2]s=/{s/^%[2]s=//;s/\r$//;p;q}' "$secret")"; \
    if [ -z "$pass" ]; then \
        echo "cocoon: [container.sudo] mode=password requires a non-empty %[2]s line in .devcontainer/%[1]s" >&2; \
        exit 1; \
    fi; \
    echo "${USERNAME} ALL=(ALL) ALL" > /etc/sudoers.d/${USERNAME} && \
    chmod 0440 /etc/sudoers.d/${USERNAME} && \
    printf '%%s:%%s\n' "${USERNAME}" "$pass" | chpasswd`,
		generate.SudoPasswordSecretFile, // %[1]s = .env.local
		generate.SudoPasswordEnvKey,     // %[2]s = SUDO_PASSWORD
		generate.SudoPasswordSecretName, // %[3]s = sudo_password
	)
}

// warnDuplicateAptExtras flags packages listed in workspace.toml [apt]
// that cocoon already installs as a MinimalBasePackage. A nil warnings
// sink silently drops the diagnostics, matching the rest of Generate.
func warnDuplicateAptExtras(extras []string, basePkgNames map[string]struct{}, warnings *warn.Sink) {
	for _, pkg := range extras {
		if _, dup := basePkgNames[pkg]; dup {
			warnings.Warn(warn.AptRedundant, pkg)
		}
	}
}

// pickRepoDir falls back to filepath.Base(WorkspaceRoot). Tests pass an
// explicit override so the snapshot does not depend on the checkout name.
func pickRepoDir(override, root string) string {
	if override != "" {
		return override
	}
	return filepath.Base(root)
}

// cocoonEntrypointPaths returns the resolved container workspace path and the
// colon-joined set of bind-mount paths at or under the user's home. docker-entrypoint.sh
// stats the workspace to detect the host uid/gid, and must never chown a bind
// mount (that rewrites ownership on the host). The mount-root branch mirrors
// compose.workspaceBindMount.
func cocoonEntrypointPaths(ctx *generate.WorkspaceContext) (workspace, bindPaths string) {
	user := ctx.Username()
	home := "/home/" + user
	workspace = home + "/" + ctx.WS.Workspace.DirOrDefault()
	if ctx.WS.Workspace.MountRootOrDefault() != ".." {
		workspace += "/" + ctx.ServiceName()
	}
	resolve := func(p string) string {
		return strings.TrimRight(strings.ReplaceAll(p, "${USERNAME}", user), "/")
	}
	paths := []string{workspace}
	for _, m := range ctx.HomeFileMounts() {
		paths = append(paths, resolve(m.Target))
	}
	for _, m := range ctx.Mounts() {
		if t := resolve(m.Target); t == home || strings.HasPrefix(t, home+"/") {
			paths = append(paths, t)
		}
	}
	return workspace, strings.Join(paths, ":")
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

// buildShellCompletionInit wires shell completion into the rc-file. Empty
// for fish (its native completions auto-load).
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

// buildShellHistoryInit configures shell history; each shell uses a separate
// state file.
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

// formatAptContinuations renders one RUN-continuation line per package
// ("    pkg \\\n"). Returns "" when packages is empty.
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
// for security updates). Returns "" when no mirror is configured, or when
// aptMirrorOriginHosts returns no hosts — that branch is unreachable in
// practice (validateImage rejects images outside SupportedImages upstream)
// but guards against a silent half-baked rewrite if validation regresses.
// The URL passes containsUnsafeForSed validation (no whitespace, control
// chars, `'`, `|`, `&`, or `\` — see internal/config/validate.go), so
// single-quoting the sed expressions is safe without further escaping.
func buildAptMirrorRewrite(ctx *generate.WorkspaceContext) string {
	url := ctx.AptMirrorURL()
	if url == "" {
		return ""
	}
	originHosts := aptMirrorOriginHosts(ctx.WS.Container.Image)
	if len(originHosts) == 0 {
		return ""
	}
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
// from disjoint hosts; the family classification in config.ImageOSFamily
// drives the branch so a Debian build does not emit useless Ubuntu sed
// expressions (and vice versa).
//
// Returns nil when the image id has no row in ImageOSFamily — which
// validateImage prevents upstream, and TestImageOSFamilyLockstep pins
// against accidental desync — so any future regression skips the rewrite
// block entirely instead of silently falling through to the Debian host
// list. Adding an Ubuntu-derived image (e.g. eclipse-temurin) is therefore
// just a one-line ImageOSFamily edit; this function needs no change.
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
	switch config.ImageOSFamily[image] {
	case "ubuntu":
		return []string{
			"http://archive.ubuntu.com/ubuntu/",
			"http://security.ubuntu.com/ubuntu/",
			"http://ports.ubuntu.com/ubuntu-ports/",
		}
	case "debian":
		return []string{
			"http://deb.debian.org/debian-security",
			"http://deb.debian.org/debian",
		}
	default:
		return nil
	}
}

// buildAptProxyConf writes /etc/apt/apt.conf.d/95proxy. Returns "" when
// [apt.proxy] is unset or both fields are nil.
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
// keyring + sources.list.d entry per [[apt.sources]] block so subsequent
// apt-get update sees the third-party repository.
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

// buildSkelCopies emits COPY directives that land under /etc/skel; the
// subsequent `useradd -m` copies and chowns them to the new user (no
// explicit chown needed). Inserted between pre_user_setup and useradd
// while the build is still executing as root.
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

func buildDockerfileHooks(ctx *generate.WorkspaceContext, warnings *warn.Sink) (preUser, postPlugins string) {
	wrap := func(content, label string) string {
		text := strings.TrimSpace(content)
		if text == "" {
			return ""
		}
		warnings.Warn(warn.DockerfileVerbatim, label)
		return fmt.Sprintf("# === user-defined dockerfile.%s (from workspace.toml) ===\n%s\n# === end dockerfile.%s ===",
			label, text, label)
	}
	return wrap(ctx.DockerfilePreUserSetup(), "pre_user_setup"),
		wrap(ctx.DockerfilePostPlugins(), "post_plugins")
}

// baseAptPackagesBlock formats aptbase.MinimalBasePackages as indented Dockerfile continuation lines.
func baseAptPackagesBlock() string {
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
// post-USER SSL_CERT_FILE ENV block. Only called when CertificatesEnabled().
func generateCertificateInstall() (rootBlock, envBlock string) {
	return certInstallRootBlock, certInstallEnvBlock
}

//nolint:lll // Dockerfile RUN/ENV lines embed shell semantics verbatim.
var certInstallRootBlock = `# Install custom CA certificates from ~/.cocoon/certs/ (no-op when the
# host directory is empty). Runs before the main apt install so HTTPS
# mirrors signed by a corporate CA can complete TLS handshake. *.cer files
# are copied in renamed to *.crt because update-ca-certificates only ingests
# the *.crt extension.
RUN --mount=type=bind,from=` + generate.CertsBuildContextName + `,target=/tmp/cocoon-user-certs \
    --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    if [ -n "$(find /tmp/cocoon-user-certs -maxdepth 1 \( -name '*.crt' -o -name '*.cer' \) -print -quit)" ]; then \
        apt-get update && \
        DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends ca-certificates && \
        mkdir -p /usr/local/share/ca-certificates/cocoon-user && \
        find /tmp/cocoon-user-certs -maxdepth 1 -name '*.crt' \
            -exec cp {} /usr/local/share/ca-certificates/cocoon-user/ \; && \
        find /tmp/cocoon-user-certs -maxdepth 1 -name '*.cer' \
            -exec sh -c 'cp "$1" "/usr/local/share/ca-certificates/cocoon-user/$(basename "$1" .cer).crt"' _ {} \; && \
        update-ca-certificates; \
    fi`

//nolint:lll // ENV declarations are single Dockerfile lines.
const certInstallEnvBlock = `# CA bundle env vars for tools that read them directly.
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
ENV CURL_CA_BUNDLE=/etc/ssl/certs/ca-certificates.crt
ENV REQUESTS_CA_BUNDLE=/etc/ssl/certs/ca-certificates.crt
ENV NODE_EXTRA_CA_CERTS=/etc/ssl/certs/ca-certificates.crt`
