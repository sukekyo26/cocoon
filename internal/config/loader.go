package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"slices"

	"github.com/pelletier/go-toml/v2"
)

// LoadWorkspace parses and validates a workspace.toml file. Unknown top-level
// or nested keys are rejected as *ValidationError, except inside
// [plugins.versions] inline tables where extra keys are carried into
// PluginVersionOverride.Extra (cross-checked later against the plugin's
// declared [install.extra_versions]).
func LoadWorkspace(path string) (*Workspace, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path provided by trusted caller
	if err != nil {
		return nil, WrapIO(path, err)
	}
	var ws Workspace
	if err := StrictUnmarshal(path, data, &ws); err != nil {
		return nil, err
	}
	if err := materializePluginVersions(path, &ws); err != nil {
		return nil, err
	}
	if err := ws.Validate(path); err != nil {
		return nil, err
	}
	return &ws, nil
}

// materializePluginVersions converts PluginsSpec.VersionsRaw (the
// map-of-any shape that survived strict unmarshal) into the typed
// PluginsSpec.Versions map. Each value is a constraint string
// ("=1.23.4" / "latest") or an inline table whose "version" key carries
// the constraint and whose remaining keys feed a plugin's
// [install.extra_versions]. Extra-key values containing any rune in
// UnsafeExtraVersionRune's reject set ('"', '\\', '\n', '\r', '$', '`')
// produce a *ValidationError — see UnsafeExtraVersionRune's doc for the
// rationale (Dockerfile RUN-prefix shell-quoting / parameter /
// command-substitution hazards). Plugin ids are iterated in sorted order
// so the FieldError sequence (and hence the "first error" summary) is
// deterministic across runs.
func materializePluginVersions(path string, ws *Workspace) error {
	if len(ws.Plugins.VersionsRaw) == 0 {
		return nil
	}
	out := make(map[string]PluginVersionOverride, len(ws.Plugins.VersionsRaw))
	a := NewAccumulator()
	ids := slices.Sorted(maps.Keys(ws.Plugins.VersionsRaw))
	for _, id := range ids {
		out[id] = materializeOneOverride(a, id, ws.Plugins.VersionsRaw[id])
	}
	if errs := a.Errors(); len(errs) > 0 {
		return &ValidationError{Path: path, Errors: errs}
	}
	ws.Plugins.Versions = out
	return nil
}

// materializeOneOverride converts one [plugins.versions].<id> value — a
// constraint string or an inline table — into a PluginVersionOverride.
// Errors accumulate on a; the returned override is always safe to store
// even on partial failure (later validation surfaces the accumulated
// errors). Pulled out of materializePluginVersions so the outer fn stays
// under the gocognit threshold.
func materializeOneOverride(a *Accumulator, id string, raw any) PluginVersionOverride {
	switch v := raw.(type) {
	case string:
		ov, err := ParseVersionSpec(v)
		if err != nil {
			a.Add(versionSpecMessage(err), "plugins", "versions", id)
			return PluginVersionOverride{} //nolint:exhaustruct // error accumulated
		}
		return ov
	case map[string]any:
		return materializeTableOverride(a, id, v)
	default:
		a.Add(fmt.Sprintf(
			"value for [plugins.versions].%s must be a constraint string "+
				`(e.g. "=1.23.4" or "latest") or an inline table { version = "…", … }`, id),
			"plugins", "versions", id)
		return PluginVersionOverride{} //nolint:exhaustruct // error accumulated
	}
}

// materializeTableOverride handles the inline-table form
// ({ version = "=…", <extra-key> = "…" }) used by plugins that declare
// [install.extra_versions]. The removed legacy pin / checksum keys are
// detected and turned into a precise migration hint.
func materializeTableOverride(a *Accumulator, id string, raw map[string]any) PluginVersionOverride {
	entry := PluginVersionOverride{} //nolint:exhaustruct // filled below
	for _, legacy := range []string{"pin", "checksum_amd64", "checksum_arm64"} {
		if _, ok := raw[legacy]; ok {
			a.Add(legacyPinTableMessage(id, raw), "plugins", "versions", id)
			return entry
		}
	}
	verRaw, ok := raw["version"]
	if !ok {
		a.Add("inline table for [plugins.versions]."+id+` must set "version" `+
			`(e.g. { version = "=1.23.4", … })`, "plugins", "versions", id, "version")
		return entry
	}
	verStr, ok := verRaw.(string)
	if !ok {
		a.Add(fmt.Sprintf("value for %q must be a string, got %T", "version", verRaw),
			"plugins", "versions", id, "version")
		return entry
	}
	parsed, err := ParseVersionSpec(verStr)
	if err != nil {
		a.Add(versionSpecMessage(err), "plugins", "versions", id, "version")
		return entry
	}
	entry.Spec = parsed.Spec
	entry.Pin = parsed.Pin
	materializeExtras(a, id, raw, &entry)
	return entry
}

// materializeExtras routes the non-"version" inline-table keys into
// entry.Extra, applying the shell-safety guard each value must pass before
// it can reach the Dockerfile RUN-prefix `<ENV>="..."` pair. Keys are
// iterated in sorted order for deterministic error sequencing.
func materializeExtras(a *Accumulator, id string, raw map[string]any, entry *PluginVersionOverride) {
	for _, k := range slices.Sorted(maps.Keys(raw)) {
		if k == "version" {
			continue
		}
		s, ok := raw[k].(string)
		if !ok {
			a.Add(fmt.Sprintf("value for %q must be a string, got %T", k, raw[k]),
				"plugins", "versions", id, k)
			continue
		}
		if bad, r := UnsafeExtraVersionRune(s); bad {
			a.Add(UnsafeExtraVersionMessage("value", r), "plugins", "versions", id, k)
			continue
		}
		if entry.Extra == nil {
			entry.Extra = make(map[string]string)
		}
		entry.Extra[k] = s
	}
}

// versionSpecMessage augments a ParseVersionSpec error with the two
// supported forms so the workspace.toml author sees the fix inline.
func versionSpecMessage(err error) string {
	return err.Error() + ` — write an exact pin as "=1.23.4" or use "latest"`
}

// legacyPinTableMessage builds the migration hint shown when a
// [plugins.versions] entry still uses the removed inline-table pin form.
// The echoed replacement carries the user's actual pin value (no guessed
// default); when only a checksum was present it falls back to a placeholder.
func legacyPinTableMessage(id string, raw map[string]any) string {
	ver := "=<version>"
	if p, ok := raw["pin"].(string); ok && p != "" {
		ver = "=" + p
	}
	return fmt.Sprintf(
		"the [plugins.versions] inline-table pin form was removed; write the "+
			"constraint as a string and run `cocoon lock` (checksums now live in "+
			"cocoon.lock):\n    %s = %q",
		id, ver)
}

// StrictUnmarshal decodes data into v with DisallowUnknownFields enabled. The
// returned error is wrapped in *ValidationError so callers can format it as
// "ERROR: <path>: <loc>: <msg>". Other packages (e.g. internal/plugin) use it
// for their own TOML schemas that share the same strict-decode contract.
func StrictUnmarshal(path string, data []byte, v any) error {
	dec := toml.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return toValidationError(path, err)
	}
	return nil
}

// toValidationError converts a go-toml error into a *ValidationError so the
// downstream CLI can render uniform "<path>: <loc>: <msg>" messages.
func toValidationError(path string, err error) error {
	var strict *toml.StrictMissingError
	if errors.As(err, &strict) {
		msg := "unknown fields: " + strict.String()
		// v8.0.0 removed top-level [shell] in favour of [container.shell].
		// Detect that specific case and emit a migration hint instead of the
		// generic "unknown fields" message so users know exactly what to do.
		if mentionsLegacyShell(strict) {
			msg = `top-level [shell] section was removed in v8.0.0; ` +
				`move "aliases" / "env" under [container.shell] and ` +
				`optionally set [container.shell].default = "bash"|"zsh"|"fish". ` +
				`See CHANGELOG.md for the migration diff.`
		}
		return &ValidationError{
			Path: path,
			Errors: []FieldError{{
				Loc:     nil,
				Message: msg,
			}},
		}
	}
	var decErr *toml.DecodeError
	if errors.As(err, &decErr) {
		row, col := decErr.Position()
		return &ValidationError{
			Path: path,
			Errors: []FieldError{{
				Loc:     nil,
				Message: fmt.Sprintf("toml decode error at %d:%d: %s", row, col, decErr.Error()),
			}},
		}
	}
	return &ValidationError{
		Path: path,
		Errors: []FieldError{{
			Loc:     nil,
			Message: "toml decode error: " + err.Error(),
		}},
	}
}

// mentionsLegacyShell reports whether strict references the removed top-level
// [shell] table. Pelletier's StrictMissingError exposes a per-entry Key()
// returning the dotted-key segments; the legacy section anchors on the first
// segment "shell" with no parent.
func mentionsLegacyShell(strict *toml.StrictMissingError) bool {
	for _, e := range strict.Errors {
		key := e.Key()
		if len(key) > 0 && key[0] == "shell" {
			return true
		}
	}
	return false
}

// WrapIO wraps an os/fs error with the file path so callers can present a
// stable "ERROR: <path>: <reason>" message. Used by both LoadWorkspace and
// internal/plugin's loader for a uniform on-disk error format.
func WrapIO(path string, err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s: file not found: %w", path, err)
	}
	return fmt.Errorf("%s: %w", path, err)
}
