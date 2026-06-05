package config

import (
	"sort"
	"strings"

	"github.com/sukekyo26/cocoon/internal/i18n"
)

// FieldError represents a single field-level validation failure: a dotted
// location path (e.g. "container.username"). The message is carried either as
// a localizable i18n Code (+ Args), set via Accumulator.AddCode, or as a
// pre-formatted English Message, set via the legacy Accumulator.Add. The empty
// Loc is rendered as "(root)".
type FieldError struct {
	Loc     []string
	Code    string // i18n catalog key; when set, takes precedence over Message
	Args    []any  // render-time args for Code
	Message string // pre-formatted English fallback (legacy Add path)
}

// Localize renders the field message in cat's language: via the catalog when a
// Code is set, otherwise the pre-formatted English Message. Pass
// i18n.English() for the English form.
func (e FieldError) Localize(cat *i18n.Catalog) string {
	if e.Code != "" {
		return cat.Msg(e.Code, e.Args...)
	}
	return e.Message
}

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

// Error summarises the first failure in English; the full list is available
// via Errors. Localize renders the same summary in a chosen language.
func (e *ValidationError) Error() string {
	return e.Localize(i18n.English())
}

// Localize renders the first-failure summary in cat's language, satisfying
// i18n.Localizer so the CLI boundary localizes workspace.toml validation errors.
func (e *ValidationError) Localize(cat *i18n.Catalog) string {
	if len(e.Errors) == 0 {
		return cat.Msg("err_validation_failed", e.Path)
	}
	first := e.Errors[0]
	base := e.Path + ": " + first.LocString() + ": " + first.Localize(cat)
	if len(e.Errors) == 1 {
		return base
	}
	return base + " " + cat.Msg("err_validation_more", len(e.Errors)-1)
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
