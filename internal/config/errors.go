package config

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// FieldError represents a single field-level validation failure: a dotted
// location path (e.g. "container.username") paired with a human message. The
// empty Loc is rendered as "(root)".
type FieldError struct {
	Loc     []string
	Message string
}

// LocString renders Loc as a dotted path, falling back to "(root)" when empty.
func (e FieldError) LocString() string {
	parts := make([]string, 0, len(e.Loc))
	for _, p := range e.Loc {
		if p != "" && p != "__root__" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return "(root)"
	}
	return strings.Join(parts, ".")
}

// ValidationError aggregates one or more FieldError values for a single TOML
// file. The path is preserved so CLI subcommands can format errors as
// "ERROR: <path>: <loc>: <msg>".
type ValidationError struct {
	Path   string
	Errors []FieldError
}

// Error implements error. The first failure is summarised; the full list is
// available via Errors.
func (e *ValidationError) Error() string {
	if len(e.Errors) == 0 {
		return fmt.Sprintf("validation failed: %s", e.Path)
	}
	first := e.Errors[0]
	if len(e.Errors) == 1 {
		return fmt.Sprintf("%s: %s: %s", e.Path, first.LocString(), first.Message)
	}
	return fmt.Sprintf("%s: %s: %s (and %d more)", e.Path, first.LocString(), first.Message, len(e.Errors)-1)
}

// Sort returns a copy of e with errors sorted lexicographically by location
// for deterministic output.
func (e *ValidationError) Sort() *ValidationError {
	out := make([]FieldError, len(e.Errors))
	copy(out, e.Errors)
	sort.SliceStable(out, func(i, j int) bool {
		return strings.Join(out[i].Loc, ".") < strings.Join(out[j].Loc, ".")
	})
	return &ValidationError{Path: e.Path, Errors: out}
}

// AsValidationError returns the wrapped *ValidationError if err contains one.
func AsValidationError(err error) (*ValidationError, bool) {
	var v *ValidationError
	if errors.As(err, &v) {
		return v, true
	}
	return nil, false
}
