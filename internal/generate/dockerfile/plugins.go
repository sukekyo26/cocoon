package dockerfile

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"sort"
	"strings"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/generate/tmplx"
	"github.com/sukekyo26/cocoon/internal/plugin"
	"github.com/sukekyo26/cocoon/internal/warn"
)

// installHeredocDelim must match the literal in installRunTmpl. A plugin
// script line equal to this would silently truncate the heredoc, hence
// the up-front check in checkScriptBody.
const installHeredocDelim = "COCOON_PLUGIN_EOF"

// ErrHeredocCollision is returned when a plugin install script
// contains a line that exactly matches installHeredocDelim. Exposed as
// a sentinel so external callers (and tests) can match the failure
// class with errors.Is without scraping the message.
var ErrHeredocCollision = errors.New("dockerfile: plugin install script collides with heredoc delimiter")

// ErrCRLFScript is returned when a plugin install script uses CRLF (or
// bare CR) line endings. Exposed as a sentinel so callers (and tests)
// can match the failure class with errors.Is without scraping the message.
var ErrCRLFScript = errors.New("dockerfile: plugin install script has CRLF line endings")

// fileExistsInFS returns (false, nil) for missing entries / directories,
// (false, err) for permission / I/O failures so the caller surfaces them
// instead of silently dropping the plugin. fsys must be non-nil.
func fileExistsInFS(fsys fs.FS, name string) (bool, error) {
	st, err := fs.Stat(fsys, name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("stat %s: %w", name, err)
	}
	return !st.IsDir(), nil
}

// checkScriptBody rejects a plugin install script that cannot be embedded
// safely in the install heredoc. CRLF (or bare CR) line endings leave a
// stray \r on every command and silently corrupt the build; a line equal
// to installHeredocDelim would terminate the `bash <<'COCOON_PLUGIN_EOF'`
// heredoc early. CRLF is checked first: a \r-terminated delimiter line
// would otherwise slip past the exact-line comparison below.
func checkScriptBody(pluginID string, scriptBody []byte) error {
	body := string(scriptBody)
	if strings.Contains(body, "\r") {
		return fmt.Errorf("%w: plugin %q contains a carriage return; re-save the install script with LF line endings",
			ErrCRLFScript, pluginID)
	}
	for _, line := range strings.Split(body, "\n") {
		if line == installHeredocDelim {
			return fmt.Errorf("%w: plugin %q contains the literal %s; rename it in the script",
				ErrHeredocCollision, pluginID, installHeredocDelim)
		}
	}
	return nil
}

// readFileFromFS wraps errors with the fs-relative path so renderer
// failures point at the offending file.
func readFileFromFS(fsys fs.FS, name string) ([]byte, error) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	return data, nil
}

// readEnvKeyOrderBytes parses plugin.toml bytes and returns the key
// declaration order of the [install.env] table. Order matters because
// Dockerfile output mirrors Python's dict insertion order, but
// go-toml/v2 decodes into Go maps which are unordered.
func readEnvKeyOrderBytes(data []byte) []string {
	var keys []string
	inEnv := false
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inEnv = line == "[install.env]"
			continue
		}
		if !inEnv || line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		key = strings.Trim(key, `"'`)
		if key != "" {
			keys = append(keys, key)
		}
	}
	return keys
}

// pluginSnippets describes the install/install_user snippets generated for one
// plugin. Empty strings mean nothing to emit for that phase.
type pluginSnippets struct {
	install      string
	userInstall  string
	requiresRoot bool
}

// shellEnv carries per-shell context that the plugin install scripts can use
// to write to the right rc file (RC_FILE) and emit the right syntax
// (RC_SYNTAX), with LOGIN_SHELL provided for shell-specific init commands
// (e.g. `starship init <shell>`).
type shellEnv struct {
	rcFileAbs  string
	rcSyntax   string
	loginShell string
}

// resolveMethodScript picks install.<method>.sh for plugins that pass
// the loader's validateMethodScripts (every loaded plugin declares at
// least one [install.methods] entry). The install.sh fallback below is
// only reachable when ResolveMethod returns "" for *Plugin literals
// built directly in tests — install.sh itself is rejected by the
// loader for plugins read from disk. Returns a wrapped
// ErrUnknownMethod when the workspace names a method the plugin does
// not declare.
func resolveMethodScript(p *plugin.Plugin, id string, methods map[string]string) (script, method string, err error) {
	m, err := plugin.ResolveMethod(p, id, methods)
	if err != nil {
		return "", "", fmt.Errorf("plugin %q: %w", id, err)
	}
	if m == "" {
		return "install.sh", "", nil
	}
	return "install." + m + ".sh", m, nil
}

func buildPluginSnippets(
	id string,
	p *plugin.Plugin,
	pluginsFS fs.FS,
	overrides map[string]config.PluginVersionOverride,
	methods map[string]string,
	sh shellEnv,
) (pluginSnippets, bool, error) {
	install := p.Install
	override, hasOverride := resolveOverride(id, p.Version.VersionCapable, overrides)

	installScript, method, err := resolveMethodScript(p, id, methods)
	if err != nil {
		return pluginSnippets{}, false, err
	}
	installPath := path.Join(id, installScript)
	userPath := path.Join(id, "install_user.sh")
	hasInstall, err := fileExistsInFS(pluginsFS, installPath)
	if err != nil {
		return pluginSnippets{}, false, err
	}
	hasUserInstall, err := fileExistsInFS(pluginsFS, userPath)
	if err != nil {
		return pluginSnippets{}, false, err
	}
	if !hasInstall && !hasUserInstall && len(install.BuildArgs) == 0 && len(install.Env) == 0 {
		return pluginSnippets{install: "", userInstall: "", requiresRoot: false}, false, nil
	}

	comment := id
	if p.Metadata.Name != "" {
		comment = p.Metadata.Name
	}
	argLines := make([]string, 0, len(install.BuildArgs))
	for _, a := range install.BuildArgs {
		argLines = append(argLines, "ARG "+a)
	}

	envBlock, err := renderEnvBlock(install.Env, pluginsFS, id)
	if err != nil {
		return pluginSnippets{}, false, err
	}
	rs := runSpec{
		id:             id,
		comment:        comment,
		argLines:       argLines,
		pluginsFS:      pluginsFS,
		envBlock:       envBlock,
		versionCapable: p.Version.VersionCapable,
		checksumVerify: p.Version.VerifiesByChecksum(),
		hasOverride:    hasOverride,
		override:       override,
		buildArgs:      install.BuildArgs,
		extraVersions:  install.ExtraVersions,
		method:         method,
		sh:             sh,
	}

	out := pluginSnippets{install: "", userInstall: "", requiresRoot: install.RequiresRoot}
	out.install, err = renderInstallSnippet(rs, hasInstall, installPath)
	if err != nil {
		return pluginSnippets{}, false, err
	}
	if hasUserInstall {
		// ARG scope is stage-wide: emit declarations only when install.sh
		// did not, so install_user.sh's per-RUN env prefix resolves
		// without a duplicate ARG line.
		var userArgLines []string
		if !hasInstall {
			userArgLines = rs.argLines
		}
		out.userInstall, err = renderUserInstallSnippet(rs, userPath, userArgLines)
		if err != nil {
			return pluginSnippets{}, false, err
		}
	}
	return out, true, nil
}

// runSpec bundles the per-plugin context so buildPluginSnippets does not
// have to thread a dozen positional args through each helper. method is
// "" for legacy single-install.sh plugins; non-empty when the plugin
// declared [install.methods] and a winner was resolved.
type runSpec struct {
	id             string
	comment        string
	argLines       []string
	pluginsFS      fs.FS
	envBlock       string
	versionCapable bool
	checksumVerify bool
	hasOverride    bool
	override       config.PluginVersionOverride
	buildArgs      []string
	extraVersions  map[string]plugin.ExtraVersionSpec
	method         string
	sh             shellEnv
}

func resolveOverride(
	id string,
	versionCapable bool,
	overrides map[string]config.PluginVersionOverride,
) (config.PluginVersionOverride, bool) {
	if !versionCapable {
		return config.PluginVersionOverride{}, false //nolint:exhaustruct // zero value signals "no override"
	}
	o, ok := overrides[id]
	if !ok {
		return config.PluginVersionOverride{}, false //nolint:exhaustruct // zero value signals "no override"
	}
	return o, true
}

// renderInstallSnippet falls back to an env-only block when install.sh
// is absent; ENV is USER-agnostic so the entry can land in either bucket.
func renderInstallSnippet(rs runSpec, hasInstall bool, installPath string) (string, error) {
	if hasInstall {
		body, err := readFileFromFS(rs.pluginsFS, installPath)
		if err != nil {
			return "", err
		}
		if err = checkScriptBody(rs.id, body); err != nil {
			return "", err
		}
		snippet, err := renderInstallRun(rs.id, "# Install "+rs.comment, rs.argLines, body,
			rs.versionCapable, rs.checksumVerify, rs.hasOverride, rs.override, rs.buildArgs, rs.extraVersions, rs.method, rs.sh)
		if err != nil {
			return "", err
		}
		if rs.envBlock != "" {
			snippet = snippet + "\n" + rs.envBlock
		}
		return snippet, nil
	}
	if rs.envBlock != "" {
		return "# Configure " + rs.comment + " (env)\n" + rs.envBlock, nil
	}
	return "", nil
}

// renderUserInstallSnippet renders the install_user.sh heredoc with
// the "# Configure … (user)" comment used in the non-root bucket.
// renderUserInstallSnippet expects argLines nil when install.sh already
// emitted the ARGs (stage-wide scope makes a second declaration redundant).
func renderUserInstallSnippet(rs runSpec, userPath string, argLines []string) (string, error) {
	body, err := readFileFromFS(rs.pluginsFS, userPath)
	if err != nil {
		return "", err
	}
	if err := checkScriptBody(rs.id, body); err != nil {
		return "", err
	}
	return renderInstallRun(rs.id, "# Configure "+rs.comment+" (user)", argLines, body,
		rs.versionCapable, rs.checksumVerify, rs.hasOverride, rs.override, rs.buildArgs, rs.extraVersions, rs.method, rs.sh)
}

func renderEnvBlock(env map[string]string, pluginsFS fs.FS, id string) (string, error) {
	if len(env) == 0 {
		return "", nil
	}
	tomlBytes, err := readFileFromFS(pluginsFS, path.Join(id, "plugin.toml"))
	if err != nil {
		return "", err
	}
	keys := readEnvKeyOrderBytes(tomlBytes)
	envLines := make([]string, 0, len(keys))
	for _, k := range keys {
		if v, ok := env[k]; ok {
			envLines = append(envLines, fmt.Sprintf(`ENV %s="%s"`, k, v))
		}
	}
	return strings.Join(envLines, "\n"), nil
}

// collectAllUserDirs returns the dirs the user-dirs mkdir block must own:
// plugin [install].volumes (under /home/${USERNAME}/...) plus `extra`
// (workspace.toml [volumes] targets, which can point anywhere). Order
// follows `enabled`. Missing plugin entries are skipped (warned elsewhere).
func collectAllUserDirs(plugins map[string]*plugin.Plugin, enabled, extra []string) []string {
	out := make([]string, 0)
	for _, id := range enabled {
		p, ok := plugins[id]
		if !ok {
			continue
		}
		out = append(out, p.Install.Volumes...)
	}
	out = append(out, extra...)
	return out
}

type snippetGroups struct {
	root, nonRoot, userPost []string
}

func collectPluginSnippets(
	plugins map[string]*plugin.Plugin,
	enabled []string,
	pluginsFS fs.FS,
	overrides map[string]config.PluginVersionOverride,
	methods map[string]string,
	sh shellEnv,
) (snippetGroups, error) {
	g := snippetGroups{root: nil, nonRoot: nil, userPost: nil}
	for _, id := range enabled {
		p, ok := plugins[id]
		if !ok {
			continue
		}
		snips, emit, err := buildPluginSnippets(id, p, pluginsFS, overrides, methods, sh)
		if err != nil {
			return snippetGroups{root: nil, nonRoot: nil, userPost: nil}, err
		}
		if !emit {
			continue
		}
		if snips.install != "" {
			if snips.requiresRoot {
				g.root = append(g.root, snips.install)
			} else {
				g.nonRoot = append(g.nonRoot, snips.install)
			}
		}
		if snips.userInstall != "" {
			g.userPost = append(g.userPost, snips.userInstall)
		}
	}
	return g, nil
}

func generatePluginInstalls(
	plugins map[string]*plugin.Plugin,
	enabled []string,
	pluginsFS fs.FS,
	extraUserDirs []string,
	overrides map[string]config.PluginVersionOverride,
	methods map[string]string,
	warnings *warn.Sink,
	sh shellEnv,
) (string, error) {
	// Fail fast when any enabled plugin loaded but the caller forgot to
	// wire PluginsFS — otherwise fileExistsInFS would silently emit a
	// Dockerfile that ignores the plugin set entirely.
	if pluginsFS == nil {
		for _, id := range enabled {
			if _, ok := plugins[id]; ok {
				return "", fmt.Errorf("dockerfile: %w", plugin.ErrNilPluginsFS)
			}
		}
	}

	if len(overrides) > 0 {
		if err := validateVersionOverrides(plugins, overrides, methods, warnings); err != nil {
			return "", err
		}
	}

	dirBlock, err := generateUserDirsBlock(collectAllUserDirs(plugins, enabled, extraUserDirs))
	if err != nil {
		return "", err
	}
	g, err := collectPluginSnippets(plugins, enabled, pluginsFS, overrides, methods, sh)
	if err != nil {
		return "", err
	}
	// Track the prior USER so we emit a `USER` directive only on real
	// transitions, leaving the surrounding template's `USER ${USERNAME}`
	// baseline unchanged.
	var parts []string
	currentUser := "${USERNAME}"
	if dirBlock != "" {
		// userDirsBlockTmpl ends in `USER root`; we defer the `USER ${USERNAME}`
		// trailer here so a root-requiring install runs without a toggle pair.
		parts = append(parts, dirBlock)
		currentUser = "root"
	}
	if len(g.root) > 0 {
		if currentUser != "root" {
			parts = append(parts, "USER root")
		}
		parts = append(parts, g.root...)
		currentUser = "root"
	}
	if currentUser != "${USERNAME}" {
		parts = append(parts, "USER ${USERNAME}")
	}
	parts = append(parts, g.nonRoot...)
	parts = append(parts, g.userPost...)
	return strings.Join(parts, "\n"), nil
}

type installRunData struct {
	Comment    string
	ArgLines   []string
	EnvPairs   []string
	ScriptBody string
}

// installRunTmpl inlines the install body via a quoted heredoc. The
// single-quoted 'COCOON_PLUGIN_EOF' suppresses parameter/command
// substitution so the script lands verbatim, and per-RUN env vars
// (PIN / RC_FILE / etc.) stay scoped to that step. The apt cache
// mounts mirror the apt-related RUN blocks elsewhere in the generator
// so install.apt.sh plugins (and any other plugin that touches apt
// internally) reuse /var/cache/apt + /var/lib/apt across builds —
// without them, `apt-get update` re-fetches the index each build and
// the lists land in the image layer.
var installRunTmpl = tmplx.MustParse("dockerfile-plugin-install", `{{ .Comment }}
{{- range .ArgLines }}
{{ . }}
{{- end }}
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    {{ range .EnvPairs }}{{ . }} {{ end }}bash <<'COCOON_PLUGIN_EOF'
{{ .ScriptBody }}COCOON_PLUGIN_EOF`, nil)

// renderInstallRun normalises the trailing newline so the closing delim
// lands on its own line regardless of input shape. method is the
// selected install method name (empty for legacy single-install.sh
// plugins); when non-empty it surfaces as the COCOON_INSTALL_METHOD
// per-RUN env var so the script can branch on it.
func renderInstallRun(
	pluginID, comment string,
	argLines []string,
	scriptBody []byte,
	versionCapable, checksumVerify, hasOverride bool,
	override config.PluginVersionOverride,
	buildArgs []string,
	extraVersions map[string]plugin.ExtraVersionSpec,
	method string,
	sh shellEnv,
) (string, error) {
	envPairs := buildInstallEnvPairs(
		versionCapable, checksumVerify, hasOverride, override,
		buildArgs, extraVersions, method, sh,
	)
	body := string(scriptBody)
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	out, err := tmplx.Render(installRunTmpl, installRunData{
		Comment:    comment,
		ArgLines:   argLines,
		EnvPairs:   envPairs,
		ScriptBody: body,
	})
	if err != nil {
		return "", fmt.Errorf("dockerfile: render install run for plugin %q: %w", pluginID, err)
	}
	return out, nil
}

func buildInstallEnvPairs(
	versionCapable, checksumVerify, hasOverride bool,
	override config.PluginVersionOverride,
	buildArgs []string,
	extraVersions map[string]plugin.ExtraVersionSpec,
	method string,
	sh shellEnv,
) []string {
	// RC_FILE / RC_SYNTAX / LOGIN_SHELL are passed unconditionally so plugin
	// install scripts can append shell-aware lines to the right rc file
	// without hardcoding ~/.bashrc. See docs/plugins.md for the contract.
	pairs := []string{
		fmt.Sprintf(`RC_FILE="%s"`, sh.rcFileAbs),
		fmt.Sprintf(`RC_SYNTAX="%s"`, sh.rcSyntax),
		fmt.Sprintf(`LOGIN_SHELL="%s"`, sh.loginShell),
	}
	if method != "" {
		pairs = append(pairs, fmt.Sprintf(`COCOON_INSTALL_METHOD="%s"`, method))
	}
	if versionCapable {
		pin := ""
		if hasOverride {
			pin = override.Pin
		}
		pairs = append(pairs, fmt.Sprintf(`PIN="%s"`, pin))
		// CHECKSUM_* is injected only for checksum-verified plugins whose
		// install method consumes it (binary / archive). pgp plugins verify
		// in-script, and installer / apt methods ignore $CHECKSUM_* entirely.
		if checksumVerify && methodVerifiesByChecksum(method) {
			amd64 := ""
			arm64 := ""
			if hasOverride {
				if override.ChecksumAmd64 != nil {
					amd64 = *override.ChecksumAmd64
				}
				if override.ChecksumArm64 != nil {
					arm64 = *override.ChecksumArm64
				}
			}
			pairs = append(pairs,
				fmt.Sprintf(`CHECKSUM_AMD64="%s"`, amd64),
				fmt.Sprintf(`CHECKSUM_ARM64="%s"`, arm64),
			)
		}
	}
	pairs = appendExtraVersionPairs(pairs, hasOverride, override, extraVersions)
	for _, a := range buildArgs {
		pairs = append(pairs, fmt.Sprintf(`%s="${%s}"`, a, a))
	}
	return pairs
}

// appendExtraVersionPairs adds one env pair per [install.extra_versions]
// entry, with the workspace.toml override (if any) taking precedence
// over the plugin.toml default. Keys are sorted so the env order is
// stable across builds (Go map iteration is randomised, which would
// otherwise drift the generated Dockerfile snapshot).
func appendExtraVersionPairs(
	pairs []string,
	hasOverride bool,
	override config.PluginVersionOverride,
	extraVersions map[string]plugin.ExtraVersionSpec,
) []string {
	if len(extraVersions) == 0 {
		return pairs
	}
	keys := slices.Sorted(maps.Keys(extraVersions))
	for _, k := range keys {
		spec := extraVersions[k]
		val := spec.Default
		if hasOverride {
			if v, ok := override.Extra[k]; ok {
				val = v
			}
		}
		pairs = append(pairs, fmt.Sprintf(`%s="%s"`, spec.Env, val))
	}
	return pairs
}

func validateVersionOverrides(
	plugins map[string]*plugin.Plugin,
	overrides map[string]config.PluginVersionOverride,
	methods map[string]string,
	warnings *warn.Sink,
) error {
	ids := slices.Sorted(maps.Keys(overrides))

	for _, id := range ids {
		override := overrides[id]
		p, ok := plugins[id]
		if !ok {
			return fmt.Errorf("%w: workspace.toml sets a version or [plugins.options] for '%s', but it is not an "+
				"enabled plugin (add it to [plugins].enable, or remove the entry)",
				ErrInvalidVersionOverride, id)
		}
		if !p.Version.VersionCapable {
			return fmt.Errorf("%w: '%s' has version_capable = false and cannot be pinned"+
				" (drop the \"=<version>\" suffix from its [plugins].enable entry)",
				ErrInvalidVersionOverride, id)
		}
		if err := checkExtraOverrideKeys(id, override, p.Install.ExtraVersions); err != nil {
			return err
		}
		// The missing-checksum warning applies only to checksum-verified
		// plugins whose install method consumes a per-arch checksum. A pgp
		// plugin is verified in-script, and an installer / apt method ignores
		// $CHECKSUM_* entirely, so neither should be nagged.
		if override.Pin != "" && p.Version.VerifiesByChecksum() {
			method, err := plugin.ResolveMethod(p, id, methods)
			if err == nil && methodVerifiesByChecksum(method) {
				warnMissingChecksum(warnings, id, override, p.Version.Source)
			}
		}
	}
	return nil
}

// validateManualChecksums gates the [plugins.options] manual checksum escape
// hatch. It runs on the BASE (workspace-sourced) overrides — before the lock
// overlay — so a present checksum is necessarily one the user typed by hand. A
// manual checksum is only meaningful for a plugin whose selected install method
// consumes a per-arch checksum (binary / archive) AND whose upstream publishes
// none (so `cocoon lock` cannot record one). It is rejected when: the install
// method is installer / apt (which ignore $CHECKSUM_*), the plugin is pgp
// (verified in-script), or the checksum is auto-resolvable (`cocoon lock` would
// silently override the hand-typed value).
func validateManualChecksums(
	plugins map[string]*plugin.Plugin,
	base map[string]config.PluginVersionOverride,
	methods map[string]string,
) error {
	for _, id := range slices.Sorted(maps.Keys(base)) {
		ov := base[id]
		if ov.ChecksumAmd64 == nil && ov.ChecksumArm64 == nil {
			continue
		}
		p, ok := plugins[id]
		if !ok {
			continue // not-enabled is reported by validateVersionOverrides
		}
		if method, err := plugin.ResolveMethod(p, id, methods); err == nil && !methodVerifiesByChecksum(method) {
			return fmt.Errorf("%w: [plugins.options.%s] sets a checksum, but the %q install method does not"+
				" consume a per-arch checksum (only binary / archive methods do); remove it from [plugins.options]",
				ErrInvalidVersionOverride, id, method)
		}
		if !p.Version.VerifiesByChecksum() {
			return fmt.Errorf("%w: [plugins.options.%s] sets a checksum, but the plugin declares verify = %q and"+
				" verifies downloads in-script (not against a per-arch checksum); remove the checksum",
				ErrInvalidVersionOverride, id, p.Version.Verify)
		}
		if autoResolvesChecksum(p.Version.Source) {
			return fmt.Errorf("%w: [plugins.options.%s] sets a manual checksum, but `cocoon lock` resolves the"+
				" checksum automatically; remove it from [plugins.options] and run `cocoon lock`",
				ErrInvalidVersionOverride, id)
		}
	}
	return nil
}

// autoResolvesChecksum reports whether cocoon lock can fetch a per-arch
// checksum for the plugin (a sidecar / shasums-file source). A nil source or a
// "none" checksum kind means it cannot, so a manual checksum is permitted.
func autoResolvesChecksum(src *plugin.VersionSource) bool {
	return src != nil && src.Checksum.Type != "" && src.Checksum.Type != plugin.ChecksumNone
}

// methodVerifiesByChecksum reports whether the selected install method category
// consumes the $CHECKSUM_AMD64 / $CHECKSUM_ARM64 env pair. Only binary and
// archive download a discrete asset and run `sha256sum -c`; installer (pipes an
// upstream installer script) and apt (apt-get) verify by other means and ignore
// the variables. An empty method (legacy single-install.sh plugin) or any custom
// category is treated as checksum-capable to preserve existing behavior.
func methodVerifiesByChecksum(method string) bool {
	switch method {
	case "installer", "apt":
		return false
	default:
		return true
	}
}

// checkExtraOverrideKeys rejects [plugins.options].<id>.<key> entries that
// the plugin does not declare under [install.extra_versions]. Keys are sorted
// so the error message stays stable across runs.
func checkExtraOverrideKeys(
	id string,
	override config.PluginVersionOverride,
	declared map[string]plugin.ExtraVersionSpec,
) error {
	if len(override.Extra) == 0 {
		return nil
	}
	unknown := make([]string, 0, len(override.Extra))
	for k := range override.Extra {
		if _, ok := declared[k]; !ok {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)
	return fmt.Errorf("%w: [plugins.options.%s] sets %v; plugin '%s' does not declare these keys"+
		" under [install.extra_versions]; remove them or fix the typo",
		ErrUnknownExtraVersion, id, unknown, id)
}

// warnMissingChecksum emits the "pin without recorded checksum" advisory for a
// checksum-verified plugin whose effective override carries no checksum. No-op
// when warnings is nil or both checksums are present. The message depends on
// whether the source can auto-resolve a checksum: an auto-resolvable plugin's
// install step still verifies the download against the upstream-published
// checksum (so running `cocoon lock` records it), whereas a plugin whose
// upstream publishes none downloads WITHOUT verification until the user records
// a manual checksum in [plugins.options].
func warnMissingChecksum(
	warnings *warn.Sink, id string, override config.PluginVersionOverride, src *plugin.VersionSource,
) {
	if override.ChecksumAmd64 != nil && override.ChecksumArm64 != nil {
		return
	}
	if autoResolvesChecksum(src) {
		warnings.Warn(warn.PinNoChecksum, id, override.Pin)
		return
	}
	warnings.Warn(warn.PinNoVerify, id, override.Pin, id)
}

// userDirsBlockTmpl omits the trailing `USER ${USERNAME}`: the caller
// emits it (or skips it when the next phase needs root) so the boundary
// produces no redundant USER toggle.
var userDirsBlockTmpl = tmplx.MustParse(
	"dockerfile-user-dirs",
	`# Prepare volume mount directories with correct ownership
USER root
RUN mkdir -p {{ .Dirs }} && \
    chown ${USERNAME}:${USERNAME} {{ .Dirs }}`,
	nil,
)

func generateUserDirsBlock(userDirs []string) (string, error) {
	if len(userDirs) == 0 {
		return "", nil
	}
	dirs := expandUserDirs(userDirs)
	out, err := tmplx.Render(userDirsBlockTmpl, struct{ Dirs string }{
		Dirs: strings.Join(dirs, " "),
	})
	if err != nil {
		return "", fmt.Errorf("dockerfile: render user dirs block: %w", err)
	}
	return out, nil
}

// expandUserDirs emits every intermediate dir under /home/${USERNAME}/ so
// `chown` covers the full chain. Returns sorted, deduplicated.
func expandUserDirs(userDirs []string) []string {
	const home = "/home/${USERNAME}"
	homePrefix := home + "/"
	all := map[string]struct{}{}
	for _, d := range userDirs {
		if strings.HasPrefix(d, homePrefix) {
			rel := d[len(homePrefix):]
			parts := strings.Split(rel, "/")
			for i := range parts {
				all[home+"/"+strings.Join(parts[:i+1], "/")] = struct{}{}
			}
		} else {
			all[d] = struct{}{}
		}
	}
	return slices.Sorted(maps.Keys(all))
}
