package logx

import (
	"io"
	"os"

	"github.com/mattn/go-isatty"
)

// ColorMode controls whether Logger emits ANSI color escapes.
type ColorMode int

const (
	// ColorAuto selects color when the sink is a TTY and NO_COLOR is unset,
	// or when FORCE_COLOR is set.
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

// shouldColor honours no-color.org / npm conventions: NO_COLOR wins over
// FORCE_COLOR (any non-empty NO_COLOR disables).
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

// colorWriter brackets each Write with prefix/suffix. Callers must emit one
// logical line per Write; the wrapper does not scan for newlines mid-payload.
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

// YellowWriter returns w when ColorAuto does not select color.
func YellowWriter(w io.Writer) io.Writer {
	return wrapColor(w, ansiYellow)
}

func wrapColor(w io.Writer, color string) io.Writer {
	if !shouldColor(w, ColorAuto, os.Getenv) {
		return w
	}
	return &colorWriter{w: w, prefix: color, suffix: ansiReset}
}
