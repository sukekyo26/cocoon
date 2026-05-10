// Package devcontainerjson generates the .devcontainer/devcontainer.json
// file. The output is tab-indented, raw UTF-8 (no HTML escaping), and
// preserves key insertion order via the in-package orderedMap.
package devcontainerjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/sukekyo26/cocoon/internal/generate"
)

const header = "// Auto-generated from workspace.toml — do not edit directly.\n"

// Generate returns the devcontainer.json body for ctx.
func Generate(ctx *generate.WorkspaceContext) (string, error) {
	cfg := buildConfig(ctx)
	raw, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("devcontainerjson: marshal: %w", err)
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, raw, "", "\t"); err != nil {
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
	if ctx.CertificatesEnabled() {
		// initializeCommand runs on the host (via /bin/sh on Linux/macOS)
		// before container build/create. It idempotently creates the
		// user cert directory referenced by docker-compose.yml's
		// additional_contexts so VS Code Dev Containers users get a
		// working build with no extra setup. Only emitted when the
		// workspace opts into [certificates] enable=true; cert-free
		// teams get a devcontainer.json without this hook.
		//
		// The argument uses POSIX shell parameter expansion
		// (${HOME:?...}) so an unset HOME fails fast with a visible
		// message instead of silently mkdir'ing /.cocoon/certs.
		base.set("initializeCommand", "mkdir -p "+generate.CertsHostPath)
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
