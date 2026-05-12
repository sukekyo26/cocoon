package logx

import (
	"io"
	"os"

	"github.com/mattn/go-isatty"
)

// ColorMode controls whether Logger emits ANSI color escapes. ColorAuto
// inspects each sink and the environment at construction time; ColorAlways
// and ColorNever override that detection.
type ColorMode int

const (
	// ColorAuto selects color when the sink is a TTY and NO_COLOR is not
	// set, or when FORCE_COLOR is set.
	ColorAuto ColorMode = iota
	// ColorAlways forces color on regardless of TTY/env.
	ColorAlways
	// ColorNever forces color off regardless of TTY/env.
	ColorNever
)

const (
	ansiReset  = "\x1b[0m"
	ansiRed    = "\x1b[31m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiCyan   = "\x1b[36m"
	ansiBold   = "\x1b[1m"
	ansiDim    = "\x1b[2m"
)

// shouldColor reports whether ANSI sequences should be emitted to w under
// mode, consulting getenv for NO_COLOR / FORCE_COLOR (no-color.org / npm
// conventions). NO_COLOR wins over FORCE_COLOR when both are set, matching
// no-color.org's "any non-empty value disables" guarantee.
func shouldColor(w io.Writer, mode ColorMode, getenv func(string) string) bool {
	switch mode {
	case ColorAlways:
		return true
	case ColorNever:
		return false
	case ColorAuto:
	}
	if getenv("NO_COLOR") != "" {
		return false
	}
	if getenv("FORCE_COLOR") != "" {
		return true
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd())
}

// colorWriter wraps w so that each Write is bracketed by prefix and suffix.
// Callers must emit one logical line per Write call; the wrapper does not
// scan for newlines mid-payload. All existing WARNING emitters in the tree
// satisfy this (`fmt.Fprintf(w, "WARNING: ...\n", ...)` is one Write).
type colorWriter struct {
	w              io.Writer
	prefix, suffix string
}

func (c *colorWriter) Write(p []byte) (int, error) {
	if len(c.prefix) == 0 {
		return c.w.Write(p) //nolint:wrapcheck // pass-through writer
	}
	if _, err := c.w.Write([]byte(c.prefix)); err != nil {
		return 0, err //nolint:wrapcheck // surface the inner writer error verbatim
	}
	n, err := c.w.Write(p)
	if err != nil {
		return n, err //nolint:wrapcheck // surface the inner writer error verbatim
	}
	if _, err := c.w.Write([]byte(c.suffix)); err != nil {
		return n, err //nolint:wrapcheck // surface the inner writer error verbatim
	}
	return n, nil
}

// YellowWriter returns an io.Writer that wraps each Write to w with the
// ANSI yellow color sequence when w supports colored output under
// ColorAuto. Returns w unchanged when color is disabled.
func YellowWriter(w io.Writer) io.Writer {
	return wrapColor(w, ansiYellow)
}

// RedWriter returns an io.Writer that colors each Write red.
func RedWriter(w io.Writer) io.Writer {
	return wrapColor(w, ansiRed)
}

func wrapColor(w io.Writer, color string) io.Writer {
	if !shouldColor(w, ColorAuto, os.Getenv) {
		return w
	}
	return &colorWriter{w: w, prefix: color, suffix: ansiReset}
}
