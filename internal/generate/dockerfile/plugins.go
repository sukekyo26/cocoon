package dockerfile

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/generate/tmplx"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

// errPluginsFSNil is returned when a Dockerfile renderer is asked to
// read a plugin asset but the WorkspaceContext was constructed without
// a PluginsFS. Wrapped (rather than emitted as a dynamic errors.New)
// so callers can identify the cause via errors.Is.
var errPluginsFSNil = errors.New("plugins fs is nil")

// fileExistsInFS reports whether name is a regular file inside fsys.
// Returns false on missing file, missing fs, or any other stat error
// (callers treat plugins without an install.sh as a no-op).
func fileExistsInFS(fsys fs.FS, name string) bool {
	if fsys == nil {
		return false
	}
	st, err := fs.Stat(fsys, name)
	if err != nil {
		return false
	}
	return !st.IsDir()
}

func readFileFromFS(fsys fs.FS, name string) ([]byte, error) {
	if fsys == nil {
		return nil, errPluginsFSNil
	}
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

func buildPluginSnippets(
	id string,
	p *plugin.Plugin,
	pluginsFS fs.FS,
	overrides map[string]config.PluginVersionOverride,
	sh shellEnv,
) (pluginSnippets, bool, error) {
	install := p.Install
	versionCapable := p.Version.VersionCapable
	var override config.PluginVersionOverride
	hasOverride := false
	if versionCapable {
		if o, ok := overrides[id]; ok {
			override = o
			hasOverride = true
		}
	}

	installPath := path.Join(id, "install.sh")
	userPath := path.Join(id, "install_user.sh")
	hasInstall := fileExistsInFS(pluginsFS, installPath)
	hasUserInstall := fileExistsInFS(pluginsFS, userPath)
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

	out := pluginSnippets{install: "", userInstall: "", requiresRoot: install.RequiresRoot}
	if hasInstall {
		body, err := readFileFromFS(pluginsFS, installPath)
		if err != nil {
			return pluginSnippets{}, false, err
		}
		snippet := renderInstallRun(id, "# Install "+comment, argLines, body,
			versionCapable, hasOverride, override, install.BuildArgs, sh)
		envBlock, err := renderEnvBlock(install.Env, pluginsFS, id)
		if err != nil {
			return pluginSnippets{}, false, err
		}
		if envBlock != "" {
			snippet = snippet + "\n" + envBlock
		}
		out.install = snippet
	}
	if hasUserInstall {
		body, err := readFileFromFS(pluginsFS, userPath)
		if err != nil {
			return pluginSnippets{}, false, err
		}
		out.userInstall = renderInstallRun(id, "# Configure "+comment+" (user)", nil, body,
			versionCapable, hasOverride, override, install.BuildArgs, sh)
	}
	return out, true, nil
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

// generatePluginInstalls renders the {{PLUGIN_INSTALLS}} block. enabled
// preserves user-declared order (matches Python dict insertion order). Missing
// plugin entries are silently skipped (callers warn elsewhere).
func collectAllUserDirs(plugins map[string]*plugin.Plugin, enabled, extra []string) []string {
	out := make([]string, 0)
	for _, id := range enabled {
		p, ok := plugins[id]
		if !ok {
			continue
		}
		out = append(out, p.Install.UserDirs...)
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
	sh shellEnv,
) (snippetGroups, error) {
	g := snippetGroups{root: nil, nonRoot: nil, userPost: nil}
	for _, id := range enabled {
		p, ok := plugins[id]
		if !ok {
			continue
		}
		snips, emit, err := buildPluginSnippets(id, p, pluginsFS, overrides, sh)
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
	warnings io.Writer,
	sh shellEnv,
) (string, error) {
	if len(overrides) > 0 {
		if err := validateVersionOverrides(plugins, overrides, warnings); err != nil {
			return "", err
		}
	}

	dirBlock := generateUserDirsBlock(collectAllUserDirs(plugins, enabled, extraUserDirs))
	g, err := collectPluginSnippets(plugins, enabled, pluginsFS, overrides, sh)
	if err != nil {
		return "", err
	}
	// Track which USER the previous emitted line ends in so we only emit a
	// `USER` directive when the next phase actually needs to switch. The block
	// is invoked from a `USER ${USERNAME}` baseline and we leave it in the same
	// state so the surrounding template stays unchanged.
	var parts []string
	currentUser := "${USERNAME}"
	if dirBlock != "" {
		// userDirsBlockTmpl ends in `USER root` (the trailing `USER ${USERNAME}`
		// is delegated here so we can fall straight into a root-requiring
		// plugin install without emitting a redundant toggle pair).
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

// installRunTmpl emits the plugin install script as a single RUN with
// the shell body inlined via a quoted heredoc. Single-quoted
// 'COCOON_PLUGIN_EOF' suppresses parameter and command substitution so
// the catalog install.sh contents land verbatim, and per-RUN env vars
// (PIN / RC_FILE / etc.) stay scoped to that step rather than leaking
// into subsequent layers.
var installRunTmpl = tmplx.MustParse("dockerfile-plugin-install", `{{ .Comment }}
{{- range .ArgLines }}
{{ . }}
{{- end }}
RUN {{ range .EnvPairs }}{{ . }} {{ end }}bash <<'COCOON_PLUGIN_EOF'
{{ .ScriptBody }}COCOON_PLUGIN_EOF`, nil)

// renderInstallRun returns the rendered RUN block. scriptBody is the
// raw install.sh contents and should end with a newline so the closing
// COCOON_PLUGIN_EOF starts on its own line; renderInstallRun normalises
// the trailing newline so callers can pass either flavour.
func renderInstallRun(
	pluginID, comment string,
	argLines []string,
	scriptBody []byte,
	versionCapable, hasOverride bool,
	override config.PluginVersionOverride,
	buildArgs []string,
	sh shellEnv,
) string {
	_ = pluginID // reserved for future log/comment hooks; kept to preserve callers.
	envPairs := buildInstallEnvPairs(versionCapable, hasOverride, override, buildArgs, sh)
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
		// Static template, well-formed input — unreachable in practice.
		return ""
	}
	return out
}

func buildInstallEnvPairs(
	versionCapable, hasOverride bool,
	override config.PluginVersionOverride,
	buildArgs []string,
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
	if versionCapable {
		pin := ""
		amd64 := ""
		arm64 := ""
		if hasOverride {
			pin = override.Pin
			if override.ChecksumAmd64 != nil {
				amd64 = *override.ChecksumAmd64
			}
			if override.ChecksumArm64 != nil {
				arm64 = *override.ChecksumArm64
			}
		}
		pairs = append(pairs,
			fmt.Sprintf(`PIN="%s"`, pin),
			fmt.Sprintf(`CHECKSUM_AMD64="%s"`, amd64),
			fmt.Sprintf(`CHECKSUM_ARM64="%s"`, arm64),
		)
	}
	for _, a := range buildArgs {
		pairs = append(pairs, fmt.Sprintf(`%s="${%s}"`, a, a))
	}
	return pairs
}

func validateVersionOverrides(
	plugins map[string]*plugin.Plugin,
	overrides map[string]config.PluginVersionOverride,
	warnings io.Writer,
) error {
	ids := make([]string, 0, len(overrides))
	for id := range overrides {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		override := overrides[id]
		p, ok := plugins[id]
		if !ok {
			return fmt.Errorf("%w: [plugins.versions] references '%s', but that plugin is not enabled in [plugins].enable",
				ErrInvalidVersionOverride, id)
		}
		if !p.Version.VersionCapable {
			return fmt.Errorf("%w: [plugins.versions.%s] is not allowed: plugin '%s' has"+
				" version_capable = false (version pinning is unsupported by this plugin's install method)",
				ErrInvalidVersionOverride, id, id)
		}
		if override.Pin != "" {
			var missing []string
			if override.ChecksumAmd64 == nil {
				missing = append(missing, "checksum_amd64")
			}
			if override.ChecksumArm64 == nil {
				missing = append(missing, "checksum_arm64")
			}
			if len(missing) > 0 && warnings != nil {
				fmt.Fprintf(warnings,
					"WARNING: [plugins.versions.%s] sets pin=\"%s\" without %s; "+
						"the install step will skip SHA256 verification. "+
						"Provide the checksum(s) in workspace.toml to enable integrity checking.\n",
					id, override.Pin, strings.Join(missing, ", "))
			}
		}
	}
	return nil
}

// userDirsBlockTmpl emits the volume-mount mkdir/chown step. The trailing
// `USER ${USERNAME}` switch is intentionally omitted: generatePluginInstalls
// is responsible for emitting it (or skipping it when the next phase also
// needs root) so the boundary between this block and root-requiring plugin
// installs does not produce a redundant USER toggle pair.
var userDirsBlockTmpl = tmplx.MustParse(
	"dockerfile-user-dirs",
	`# Prepare volume mount directories with correct ownership
USER root
RUN mkdir -p {{ .Dirs }} && \
    chown ${USERNAME}:${USERNAME} {{ .Dirs }}`,
	nil,
)

func generateUserDirsBlock(userDirs []string) string {
	if len(userDirs) == 0 {
		return ""
	}
	dirs := expandUserDirs(userDirs)
	out, err := tmplx.Render(userDirsBlockTmpl, struct{ Dirs string }{
		Dirs: strings.Join(dirs, " "),
	})
	if err != nil {
		return ""
	}
	return out
}

// expandUserDirs walks each input dir; for paths under /home/${USERNAME}/, every
// intermediate directory is emitted so `mkdir -p` and `chown` cover the full
// chain. Returns a sorted, deduplicated slice.
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
	sorted := make([]string, 0, len(all))
	for p := range all {
		sorted = append(sorted, p)
	}
	sort.Strings(sorted)
	return sorted
}
