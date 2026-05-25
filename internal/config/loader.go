package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
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
// PluginsSpec.Versions map. Reserved keys (pin / checksum_amd64 /
// checksum_arm64) populate the dedicated fields; any remaining keys go
// into Extra so a plugin's [install.extra_versions] can pick them up at
// generation time. Non-string values and extra-key values containing
// any rune in UnsafeExtraVersionRune's reject set
// ('"', '\\', '\n', '\r', '$', '`') produce a *ValidationError — see
// UnsafeExtraVersionRune's doc for the rationale (Dockerfile RUN-prefix
// shell-quoting / parameter / command-substitution hazards). Plugin
// ids and per-entry keys are iterated in sorted order so the
// FieldError sequence (and hence the "first error" summary) is
// deterministic across runs.
func materializePluginVersions(path string, ws *Workspace) error {
	if len(ws.Plugins.VersionsRaw) == 0 {
		return nil
	}
	out := make(map[string]PluginVersionOverride, len(ws.Plugins.VersionsRaw))
	a := NewAccumulator()
	ids := make([]string, 0, len(ws.Plugins.VersionsRaw))
	for id := range ws.Plugins.VersionsRaw {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		out[id] = materializeOneOverride(a, id, ws.Plugins.VersionsRaw[id])
	}
	if errs := a.Errors(); len(errs) > 0 {
		return &ValidationError{Path: path, Errors: errs}
	}
	ws.Plugins.Versions = out
	return nil
}

// materializeOneOverride pulls the per-plugin loop body out of
// materializePluginVersions so the outer fn stays under the gocognit
// threshold. Errors accumulate on a; the returned override is always
// safe to store even on partial failure (later validation surfaces the
// accumulated errors).
func materializeOneOverride(a *Accumulator, id string, raw map[string]any) PluginVersionOverride {
	entry := PluginVersionOverride{} //nolint:exhaustruct // filled below
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		v := raw[k]
		s, ok := v.(string)
		if !ok {
			a.Add(fmt.Sprintf("value for %q must be a string, got %T", k, v),
				"plugins", "versions", id, k)
			continue
		}
		switch k {
		case "pin":
			entry.Pin = s
		case "checksum_amd64":
			cs := s
			entry.ChecksumAmd64 = &cs
		case "checksum_arm64":
			cs := s
			entry.ChecksumArm64 = &cs
		default:
			if bad, r := UnsafeExtraVersionRune(s); bad {
				a.Add(fmt.Sprintf("value contains unsafe character %q "+
					`(the value flows into the Dockerfile RUN prefix's KEY="..." env pair; `+
					"a bare \", \\, \\n, \\r, $, or backtick would break the shell quoting "+
					"or trigger parameter/command substitution)", r),
					"plugins", "versions", id, k)
				continue
			}
			if entry.Extra == nil {
				entry.Extra = make(map[string]string)
			}
			entry.Extra[k] = s
		}
	}
	return entry
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
