package clihelpers

import (
	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/logx"
	"github.com/sukekyo26/cocoon/internal/warn"
)

// DrainWarnings renders every diagnostic collected in sink through cat and
// writes it via log: warnings in yellow on stderr, informational notes in
// cyan. Nested warn.Ref args (e.g. a per-entry port-skip reason) are localized
// before interpolation. A nil sink is a no-op.
func DrainWarnings(log *logx.Logger, cat *i18n.Catalog, sink *warn.Sink) {
	for _, w := range sink.All() {
		msg := cat.Msg(w.Code, localizeWarnArgs(cat, w.Args)...)
		switch w.Level {
		case warn.LevelInfo:
			log.Notice(msg)
		default:
			log.Warn(msg)
		}
	}
}

// localizeWarnArgs expands warn.Ref args to their localized text so the parent
// message can interpolate them; non-Ref args pass through unchanged.
func localizeWarnArgs(cat *i18n.Catalog, args []any) []any {
	out := make([]any, len(args))
	for i, a := range args {
		if ref, ok := a.(warn.Ref); ok {
			out[i] = cat.Msg(ref.Code, localizeWarnArgs(cat, ref.Args)...)
		} else {
			out[i] = a
		}
	}
	return out
}
