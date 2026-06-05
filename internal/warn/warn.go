// Package warn collects non-fatal diagnostics (warnings and informational
// notes) as structured codes + args instead of pre-formatted strings, so the
// CLI boundary can render them in the active language.
//
// It deliberately has no dependency on internal/i18n: a [Warning.Code] is a
// catalog key, resolved by whichever CLI command drains the [Sink]. This keeps
// the generator packages (generate / config / plugin) free of i18n wiring.
package warn

// Level distinguishes a warning (rendered yellow on stderr) from an
// informational note (cyan on stderr).
type Level int

const (
	// LevelWarn marks a warning-level diagnostic.
	LevelWarn Level = iota
	// LevelInfo marks an informational diagnostic.
	LevelInfo
)

// Warning is one collected diagnostic: a catalog key plus its render-time
// args. An arg may itself be a [Ref] when it is a localizable sub-message.
type Warning struct {
	Level Level
	Code  string
	Args  []any
}

// Ref is a localizable argument: a nested catalog key + args that the drain
// site expands at render time (e.g. the per-entry reason embedded in a
// port-skip warning).
type Ref struct {
	Code string
	Args []any
}

// Reason builds a [Ref].
func Reason(code string, args ...any) Ref { return Ref{Code: code, Args: args} }

// Sink accumulates diagnostics in emission order. A nil *Sink silently drops
// everything — mirroring the previous "nil writer drops warnings" contract —
// so callers need no nil guards around [Sink.Warn] / [Sink.Info].
type Sink struct {
	items []Warning
}

// New returns an empty Sink.
func New() *Sink { return &Sink{items: nil} }

// Warn records a warning-level diagnostic.
func (s *Sink) Warn(code string, args ...any) {
	if s == nil {
		return
	}
	s.items = append(s.items, Warning{Level: LevelWarn, Code: code, Args: args})
}

// Info records an informational diagnostic.
func (s *Sink) Info(code string, args ...any) {
	if s == nil {
		return
	}
	s.items = append(s.items, Warning{Level: LevelInfo, Code: code, Args: args})
}

// All returns the collected diagnostics in emission order.
func (s *Sink) All() []Warning {
	if s == nil {
		return nil
	}
	return s.items
}
