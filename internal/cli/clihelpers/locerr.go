package clihelpers

import (
	"errors"

	"github.com/sukekyo26/cocoon/internal/i18n"
)

// LocError is a cocoon-authored error whose user-facing text is an i18n catalog
// key plus render-time args, carrying a classification sentinel (ErrUsage /
// ErrFailure) so errors.Is keeps mapping exit codes and an optional wrapped
// cause so errors.Is/As still reach inner errors.
//
// Error() renders English (the catalog fallback language) so logs, %v, and any
// non-boundary caller still read. The binary boundary (cmd/cocoon/main.go)
// re-renders key+args in the active language via Localize; an inner
// stdlib/3rd-party cause is appended verbatim (English), since cocoon cannot
// translate text it did not author.
//
// Build LocErrors via the constructors (UsageErr / FailureErr / UsageWrap /
// FailureWrap); they keep err113 happy (no literal fmt.Errorf at the call site)
// and set every field for exhaustruct.
type LocError struct {
	sentinel error  // ErrUsage / ErrFailure — classification anchor for errors.Is
	key      string // i18n catalog key; "" means "render the cause only"
	args     []any  // render-time args (data, not pre-formatted)
	cause    error  // optional wrapped cause; may be nil
}

// Error renders English so non-boundary consumers always read. It deliberately
// ignores the active locale.
func (e *LocError) Error() string {
	return e.render(i18n.English())
}

// Localize renders the message in cat's language. Boundary use only.
func (e *LocError) Localize(cat *i18n.Catalog) string {
	return e.render(cat)
}

func (e *LocError) render(cat *i18n.Catalog) string {
	if e.key == "" {
		if e.cause != nil {
			return localizeCause(cat, e.cause)
		}
		if e.sentinel != nil {
			return e.sentinel.Error()
		}
		return ""
	}
	msg := cat.Msg(e.key, localizeArgs(cat, e.args)...)
	if e.cause != nil {
		return msg + ": " + localizeCause(cat, e.cause)
	}
	return msg
}

// Unwrap returns the classification sentinel and the cause (when present) so
// errors.Is reaches both — the exit-code switch finds ErrUsage/ErrFailure and
// callers can still match an inner sentinel (fs.ErrNotExist, plugin.ErrConflict, …).
func (e *LocError) Unwrap() []error {
	switch {
	case e.sentinel != nil && e.cause != nil:
		return []error{e.sentinel, e.cause}
	case e.sentinel != nil:
		return []error{e.sentinel}
	case e.cause != nil:
		return []error{e.cause}
	default:
		return nil
	}
}

// localizeCause renders an inner error: a Localizer (LocError, ValidationError)
// in cat's language, anything else (stdlib / 3rd-party) verbatim in English.
func localizeCause(cat *i18n.Catalog, cause error) string {
	var loc i18n.Localizer
	if errors.As(cause, &loc) {
		return loc.Localize(cat)
	}
	return cause.Error()
}

// locArg is a localizable message argument: a nested catalog key + args that
// the render site expands in cat's language before interpolating into the
// parent message. Built via L.
type locArg struct {
	key  string
	args []any
}

// L wraps a localizable sub-message (e.g. a reason fragment) as an arg so it is
// translated at render time rather than frozen to English at build time.
func L(key string, args ...any) any { return locArg{key: key, args: args} }

// localizeArgs expands locArg / LocError / Localizer args in cat's language;
// plain args (strings, ints, %q ids) pass through unchanged.
func localizeArgs(cat *i18n.Catalog, args []any) []any {
	if len(args) == 0 {
		return args
	}
	out := make([]any, len(args))
	for i, a := range args {
		switch v := a.(type) {
		case locArg:
			out[i] = cat.Msg(v.key, localizeArgs(cat, v.args)...)
		case i18n.Localizer:
			out[i] = v.Localize(cat)
		default:
			out[i] = a
		}
	}
	return out
}

// UsageErr builds an ErrUsage-classified LocError (exit code 2) with no cause.
func UsageErr(key string, args ...any) error {
	return &LocError{sentinel: ErrUsage, key: key, args: args, cause: nil}
}

// FailureErr builds an ErrFailure-classified LocError (exit code 1) with no cause.
func FailureErr(key string, args ...any) error {
	return &LocError{sentinel: ErrFailure, key: key, args: args, cause: nil}
}

// UsageWrap wraps cause as an ErrUsage-classified LocError. key may be "" for a
// pure passthrough that renders only the cause.
func UsageWrap(cause error, key string, args ...any) error {
	return &LocError{sentinel: ErrUsage, key: key, args: args, cause: cause}
}

// FailureWrap wraps cause as an ErrFailure-classified LocError. key may be ""
// for a pure passthrough that renders only the cause.
func FailureWrap(cause error, key string, args ...any) error {
	return &LocError{sentinel: ErrFailure, key: key, args: args, cause: cause}
}
