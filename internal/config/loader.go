package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// LoadWorkspace parses and validates a workspace.toml file. Unknown top-level
// or nested keys are rejected as *ValidationError, except inside
// [plugins.options] inline tables where extra keys are carried into
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
	if err := materializePlugins(path, &ws); err != nil {
		return nil, err
	}
	if err := ws.Validate(path); err != nil {
		return nil, err
	}
	return &ws, nil
}

// materializePlugins post-processes the decode-only [plugins].enable array
// and [plugins.options] table into the typed PluginsSpec.Enable (the ordered
// id list) and PluginsSpec.Versions map. The enable array seeds each id's
// Spec/Pin; the options table folds in a plugin's [install.extra_versions]
// knobs (and, once enabled, manual checksums). Plugin ids are iterated in
// array / sorted order so the FieldError sequence (and hence the "first
// error" summary) is deterministic across runs.
func materializePlugins(path string, ws *Workspace) error {
	a := NewAccumulator()
	enable, versions := materializeEnable(a, ws.Plugins.EnableRaw)
	materializeOptions(a, ws.Plugins.OptionsRaw, versions)
	if errs := a.Errors(); len(errs) > 0 {
		return &ValidationError{Path: path, Errors: errs}
	}
	ws.Plugins.Enable = enable
	ws.Plugins.Versions = versions
	return nil
}

// materializeEnable splits each [plugins].enable entry into an id and an
// optional version constraint. Order is preserved (it is the deterministic
// install order). A bare "<id>" enables the plugin unpinned (no Versions
// entry → "latest" at lock/build time); "<id>=<version>" / "<id>=latest"
// seed Versions[id].Spec/Pin. Bad ids and constraints accumulate on a.
func materializeEnable(a *Accumulator, raw []string) ([]string, map[string]PluginVersionOverride) {
	ids := make([]string, 0, len(raw))
	versions := make(map[string]PluginVersionOverride, len(raw))
	for i, entry := range raw {
		id, spec, hasSpec := splitEnableEntry(entry)
		if !rxPluginID.MatchString(id) {
			a.Add(enableIDMessage(entry, id, spec), "plugins", "enable", strconv.Itoa(i))
			continue
		}
		ids = append(ids, id)
		if !hasSpec {
			continue
		}
		ov, err := parseEnableSpec(spec)
		if err != nil {
			a.Add(enableSpecMessage(err), "plugins", "enable", strconv.Itoa(i))
			continue
		}
		versions[id] = ov
	}
	return ids, versions
}

// splitEnableEntry parses one enable-array element. The id is everything
// before the first "="; the remainder (if any) is the version constraint.
func splitEnableEntry(entry string) (id, spec string, hasSpec bool) {
	e := strings.TrimSpace(entry)
	if i := strings.IndexByte(e, '='); i >= 0 {
		return strings.TrimSpace(e[:i]), strings.TrimSpace(e[i+1:]), true
	}
	return e, "", false
}

// parseEnableSpec validates the constraint suffix of an enable entry. The
// array spells the version bare ("node=24.16.0"); "latest"/"*" stay floating
// and an explicit "=" prefix is tolerated. Validation is delegated to
// ParseVersionSpec (which classifies ranges / bad charset), so a bare
// version is normalised to the "=<version>" exact-pin form first.
func parseEnableSpec(raw string) (PluginVersionOverride, error) {
	t := strings.TrimSpace(raw)
	switch {
	case t == "":
		return PluginVersionOverride{}, ErrVersionSpecEmpty //nolint:exhaustruct // error path
	case t == VersionSpecLatest || t == "*" || hasRangeOperator(t) || strings.HasPrefix(t, "="):
		return ParseVersionSpec(t)
	default:
		return ParseVersionSpec("=" + t)
	}
}

// enableSpecMessage augments a parseEnableSpec error with the supported
// forms so the workspace.toml author sees the fix inline.
func enableSpecMessage(err error) string {
	return err.Error() + ` — write the version bare ("<id>=1.23.4"), "<id>=latest", or "<id>" to enable unpinned`
}

// enableIDMessage explains why an enable entry's id was rejected. An empty id
// (e.g. "=1.2.3" or a stray "=") names no plugin, so it points at the
// "<id>=..." form; a non-empty id simply violates the id charset.
func enableIDMessage(entry, id, spec string) string {
	if id != "" {
		return "plugin id does not match " + rxPluginID.String()
	}
	if spec != "" {
		return fmt.Sprintf("enable entry %q has no plugin id — write %q", entry, "<id>="+spec)
	}
	return fmt.Sprintf("enable entry %q has no plugin id — write %q or %q", entry, "<id>=<version>", "<id>")
}

// materializeOptions folds each [plugins.options].<id> inline table into the
// matching Versions[id] override. It carries a plugin's
// [install.extra_versions] knobs into Extra (applying the shell-safety guard
// that each value must pass before reaching the Dockerfile RUN-prefix
// `<ENV>="..."` pair) and an optional manual checksum_amd64 / checksum_arm64
// into the override's checksum fields (validated as 64 lowercase hex chars;
// the generator gates whether the plugin may carry a manual checksum). The
// main version belongs in the enable array, so a "version" key here is a
// migration error and "pin" is rejected. Ids and keys are iterated in sorted
// order for deterministic error sequencing.
func materializeOptions(a *Accumulator, raw map[string]any, versions map[string]PluginVersionOverride) {
	for _, id := range slices.Sorted(maps.Keys(raw)) {
		tbl, ok := raw[id].(map[string]any)
		if !ok {
			a.Add(fmt.Sprintf("[plugins.options].%s must be an inline table "+
				`(e.g. { api_level = "35" })`, id), "plugins", "options", id)
			continue
		}
		ov := versions[id]
		materializeOptionEntry(a, id, tbl, &ov)
		versions[id] = ov
	}
}

// materializeOptionEntry routes one [plugins.options].<id> table's keys: the
// version / pin keys are migration errors, checksum_amd64 / checksum_arm64 feed
// the override's checksum fields, and every other key is an extra-version knob.
func materializeOptionEntry(a *Accumulator, id string, tbl map[string]any, ov *PluginVersionOverride) {
	for _, k := range slices.Sorted(maps.Keys(tbl)) {
		switch k {
		case "version":
			a.Add(optionsVersionKeyMessage(id, tbl[k]), "plugins", "options", id, k)
		case "pin":
			a.Add(optionsPinKeyMessage(id), "plugins", "options", id, k)
		case "checksum_amd64":
			setOptionChecksum(a, id, k, tbl[k], &ov.ChecksumAmd64)
		case "checksum_arm64":
			setOptionChecksum(a, id, k, tbl[k], &ov.ChecksumArm64)
		default:
			s, ok := tbl[k].(string)
			if !ok {
				a.Add(fmt.Sprintf("value for %q must be a string, got %T", k, tbl[k]),
					"plugins", "options", id, k)
				continue
			}
			if bad, r := UnsafeExtraVersionRune(s); bad {
				a.Add(UnsafeExtraVersionMessage("value", r), "plugins", "options", id, k)
				continue
			}
			if ov.Extra == nil {
				ov.Extra = make(map[string]string)
			}
			ov.Extra[k] = s
		}
	}
}

// optionsVersionKeyMessage points the author at the enable array, echoing
// the actual version value (no guessed default).
func optionsVersionKeyMessage(id string, raw any) string {
	ver := "<version>"
	if v, ok := raw.(string); ok && v != "" {
		ver = strings.TrimPrefix(v, "=")
	}
	return fmt.Sprintf(
		`[plugins.options].%s must not set "version" — the version belongs in the enable array: `+
			`enable = [ %q ]`, id, id+"="+ver)
}

// optionsPinKeyMessage points the author at the enable array for the removed
// "pin" key.
func optionsPinKeyMessage(id string) string {
	return fmt.Sprintf(
		`the "pin" key was removed — put the version in the enable array: enable = [ "%s=<version>" ]`, id)
}

// setOptionChecksum validates a manual [plugins.options] checksum value (64
// lowercase hex chars) and stores it on the override. Whether the plugin may
// carry a manual checksum at all is gated later by the generator (only plugins
// whose upstream publishes none qualify; everything else is auto-resolved by
// `cocoon lock`).
func setOptionChecksum(a *Accumulator, id, key string, raw any, dst **string) {
	s, ok := raw.(string)
	if !ok {
		a.Add(fmt.Sprintf("value for %q must be a string, got %T", key, raw), "plugins", "options", id, key)
		return
	}
	if !rxSha256.MatchString(s) {
		a.Add(fmt.Sprintf("%q must be 64 lowercase hex characters", key), "plugins", "options", id, key)
		return
	}
	v := s
	*dst = &v
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
		// [plugins.versions] was folded into the enable array + [plugins.options].
		if mentionsLegacyPluginVersions(strict) {
			msg = `[plugins.versions] was removed; pin versions in the enable array ` +
				`(enable = [ "go=1.23.4", "node=latest" ]) and move any extra knobs ` +
				`(e.g. android-sdk's api_level) to [plugins.options]. ` +
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

// mentionsLegacyPluginVersions reports whether strict references the removed
// [plugins.versions] table (now split into the enable array + [plugins.options]).
func mentionsLegacyPluginVersions(strict *toml.StrictMissingError) bool {
	for _, e := range strict.Errors {
		key := e.Key()
		if len(key) >= 2 && key[0] == "plugins" && key[1] == "versions" {
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
