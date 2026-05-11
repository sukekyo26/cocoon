// Package devcontainerjson generates the .devcontainer/devcontainer.json
// file. The output is tab-indented, raw UTF-8 (no HTML escaping), and
// preserves key insertion order via the in-package orderedMap.
package devcontainerjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/sukekyo26/cocoon/internal/generate"
)

const header = "// Auto-generated from workspace.toml — do not edit directly.\n"

// Generate returns the devcontainer.json body for ctx. HTML escaping is
// disabled so shell snippets in initializeCommand (e.g. `&&` chains)
// survive as the literal `&&` rather than json.Marshal's default
// `\u0026\u0026` (the encoder's escapeHTML pass rewrites `&`,
// `<`, `>` to their `\u00XX` JSON escapes for HTML-embedding
// safety, which we don't need here), matching the package-level
// "raw UTF-8" promise.
func Generate(ctx *generate.WorkspaceContext) (string, error) {
	cfg := buildConfig(ctx)
	var raw bytes.Buffer
	enc := json.NewEncoder(&raw)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(cfg); err != nil {
		return "", fmt.Errorf("devcontainerjson: marshal: %w", err)
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, bytes.TrimRight(raw.Bytes(), "\n"), "", "\t"); err != nil {
		return "", fmt.Errorf("devcontainerjson: indent: %w", err)
	}
	return header + indented.String() + "\n", nil
}

func buildConfig(ctx *generate.WorkspaceContext) *orderedMap {
	forwardPorts := append([]int{}, ctx.DevcontainerForwardPorts()...)

	base := newOrderedMap()
	base.set("name", ctx.ServiceName())
	base.set("dockerComposeFile", []string{"docker-compose.yml"})
	base.set("service", ctx.ServiceName())
	if cmd := initializeCommand(ctx); cmd != "" {
		base.set("initializeCommand", cmd)
	}
	workspaceFolder := "/home/" + ctx.Username() + "/workspace"
	if ctx.WS.Workspace.MountRootOrDefault() == "." {
		// Match the compose working_dir so VS Code opens the same
		// directory `cocoon exec` lands in.
		workspaceFolder += "/" + ctx.ServiceName()
	}
	base.set("workspaceFolder", workspaceFolder)
	base.set("forwardPorts", forwardPorts)
	base.set("shutdownAction", "stopCompose")

	customizations := newOrderedMap()
	vscode := newOrderedMap()
	vscode.set("extensions", []string{})
	customizations.set("vscode", vscode)
	base.set("customizations", customizations)

	overrides := ctx.DevcontainerOverrides()
	if len(overrides) == 0 {
		return base
	}

	if extra, ok := overrides["forwardPorts"]; ok {
		delete(overrides, "forwardPorts")
		merged := mergeForwardPorts(forwardPorts, extra)
		base.set("forwardPorts", merged)
	}
	deepMerge(base, overrides)
	return base
}

// initializeCommand assembles the host-side preparation that VS Code's
// Dev Containers extension runs before `docker compose up`. It mkdirs the
// certs build context (so BuildKit can resolve additional_contexts) and
// touches each [home_files] entry with mode 0o600 (so Docker does not
// auto-create them as directories when the file is missing on the host).
// Returns "" when neither feature is configured, in which case the caller
// omits the initializeCommand key entirely.
//
// Every path that embeds ${HOME:?…} is wrapped in double quotes so a
// host $HOME containing spaces (e.g. "/Users/Jane Doe" on macOS) does
// not word-split into two arguments. `dirname --` defends against the
// theoretical case of a home directory starting with `-`. The per-segment
// whitelist on home_files entries already rejects shell metacharacters,
// so quoting plus `--` is the remaining belt-and-suspenders.
func initializeCommand(ctx *generate.WorkspaceContext) string {
	var cmds []string
	if ctx.CertificatesEnabled() {
		cmds = append(cmds, `mkdir -p "`+generate.CertsHostPath+`"`)
	}
	for _, rel := range ctx.HomeFilesEntries() {
		p := `"` + generate.HomeFilesHostPathPrefix + "/" + rel + `"`
		// umask 077 must wrap both mkdir and touch so the parent dir
		// (e.g. ~/.gemini for .gemini/oauth_creds.json) inherits 0700,
		// matching ensureHomeFiles in cli/gen. Otherwise the parent
		// would be 0755 (default umask) and a 0600 file would sit in
		// a world-readable dir.
		cmds = append(cmds, `(umask 077 && mkdir -p "$(dirname -- `+p+`)" && touch `+p+`)`)
	}
	return strings.Join(cmds, " && ")
}

// mergeForwardPorts unions the base list with override entries, preserving
// base order and deduplicating. Override entries may be ints or any other
// JSON-compatible scalar.
func mergeForwardPorts(base []int, extra any) []any {
	out := make([]any, 0, len(base))
	seen := make(map[any]struct{}, len(base))
	for _, p := range base {
		out = append(out, p)
		seen[p] = struct{}{}
	}
	list, ok := extra.([]any)
	if !ok {
		return out
	}
	for _, p := range list {
		key := normalizeKey(p)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, p)
	}
	return out
}

// normalizeKey collapses int64/int variants so dedup catches `3000` vs
// `int64(3000)` mismatches that come from go-toml's numeric decoding.
func normalizeKey(v any) any {
	switch x := v.(type) {
	case int64:
		return int(x)
	case float64:
		if float64(int(x)) == x {
			return int(x)
		}
	}
	return v
}

// deepMerge folds override into base. Existing keys preserve their position;
// new keys are appended in sorted order so the output is deterministic across
// Go map iteration.
func deepMerge(base *orderedMap, override map[string]any) {
	keys := make([]string, 0, len(override))
	for k := range override {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := override[k]
		existing, has := base.get(k)
		if has {
			if subBase, ok := existing.(*orderedMap); ok {
				if subOver, ok := v.(map[string]any); ok {
					deepMerge(subBase, subOver)
					continue
				}
			}
		}
		base.set(k, v)
	}
}
