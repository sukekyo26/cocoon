package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/pelletier/go-toml/v2"
)

// LoadWorkspace parses and validates a workspace.toml file. Unknown top-level
// or nested keys are rejected as *ValidationError.
func LoadWorkspace(path string) (*Workspace, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path provided by trusted caller
	if err != nil {
		return nil, WrapIO(path, err)
	}
	var ws Workspace
	if err := StrictUnmarshal(path, data, &ws); err != nil {
		return nil, err
	}
	if err := ws.Validate(path); err != nil {
		return nil, err
	}
	return &ws, nil
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
