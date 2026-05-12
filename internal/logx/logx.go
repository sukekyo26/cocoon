// Package logx provides thin io.Writer-based output helpers used by the
// CLI subcommands. Direct use of fmt.Print* / panic / the standard log
// package is forbidden by the forbidigo + depguard linters elsewhere in the
// tree; route user-facing output through Logger so the print seam stays
// centralised and CI can keep forbidigo enabled.
//
// Logger does not buffer, level-filter, or format structured fields. It is a
// deliberately small wrapper around io.Writer because the CLI's output is
// localised human-readable text (via internal/i18n), not structured logs —
// adding slog handlers, key/value attrs, or sloglint-friendly static
// messages would conflict with i18n's dynamic message strings.
//
// Logger paints messages with ANSI color (red/yellow/green/cyan, plus bold
// and dim decorators) when its sink is a TTY and the environment permits
// (see [ColorMode] and [shouldColor]). Color decisions are made once at
// construction; pass [ColorNever] in tests that compare exact byte output.
package logx

import (
	"fmt"
	"io"
	"os"
)

// Logger writes user-facing messages to the injected stdout/stderr sinks.
type Logger struct {
	stdout, stderr io.Writer
	// Per-sink ANSI sequences, empty when color is disabled for that sink.
	stdoutBold, stdoutDim, stdoutGreen, stdoutReset string
	stderrRed, stderrYellow, stderrCyan, stderrBold string
	stderrDim, stderrReset                          string
}

// New constructs a Logger that writes Info output to stdout and
// Error/Warn/Notice output to stderr, auto-detecting color support per
// sink (see [ColorAuto]).
func New(stdout, stderr io.Writer) *Logger {
	return NewWithMode(stdout, stderr, ColorAuto)
}

// NewWithMode constructs a Logger with the given ColorMode applied to both
// sinks. Tests that compare exact byte output should pass [ColorNever].
func NewWithMode(stdout, stderr io.Writer, mode ColorMode) *Logger {
	l := &Logger{ //nolint:exhaustruct // ANSI fields below are conditionally populated.
		stdout: stdout,
		stderr: stderr,
	}
	if shouldColor(stdout, mode, os.Getenv) {
		l.stdoutBold = ansiBold
		l.stdoutDim = ansiDim
		l.stdoutGreen = ansiGreen
		l.stdoutReset = ansiReset
	}
	if shouldColor(stderr, mode, os.Getenv) {
		l.stderrRed = ansiRed
		l.stderrYellow = ansiYellow
		l.stderrCyan = ansiCyan
		l.stderrBold = ansiBold
		l.stderrDim = ansiDim
		l.stderrReset = ansiReset
	}
	return l
}

// Info writes msg followed by a newline to stdout (no color).
func (l *Logger) Info(msg string) { _, _ = fmt.Fprintln(l.stdout, msg) }

// Infof writes the formatted message followed by a newline to stdout.
func (l *Logger) Infof(format string, args ...any) {
	_, _ = fmt.Fprintf(l.stdout, format+"\n", args...)
}

// Success writes msg to stdout in green — for confirmations of successful
// operations (file written, version upgraded, etc.).
func (l *Logger) Success(msg string) {
	_, _ = fmt.Fprintf(l.stdout, "%s%s%s\n", l.stdoutGreen, msg, l.stdoutReset)
}

// Successf writes the formatted message to stdout in green.
func (l *Logger) Successf(format string, args ...any) {
	l.Success(fmt.Sprintf(format, args...))
}

// Warn writes msg to stderr in yellow — for non-fatal warnings.
func (l *Logger) Warn(msg string) {
	_, _ = fmt.Fprintf(l.stderr, "%s%s%s\n", l.stderrYellow, msg, l.stderrReset)
}

// Warnf writes the formatted message to stderr in yellow.
func (l *Logger) Warnf(format string, args ...any) {
	l.Warn(fmt.Sprintf(format, args...))
}

// Error writes msg to stderr in red.
func (l *Logger) Error(msg string) {
	_, _ = fmt.Fprintf(l.stderr, "%s%s%s\n", l.stderrRed, msg, l.stderrReset)
}

// Errorf writes the formatted message to stderr in red.
func (l *Logger) Errorf(format string, args ...any) {
	l.Error(fmt.Sprintf(format, args...))
}

// Notice writes msg to stderr in cyan — for informational announcements
// that are neither errors nor warnings (e.g. an available update).
func (l *Logger) Notice(msg string) {
	_, _ = fmt.Fprintf(l.stderr, "%s%s%s\n", l.stderrCyan, msg, l.stderrReset)
}

// Noticef writes the formatted message to stderr in cyan.
func (l *Logger) Noticef(format string, args ...any) {
	l.Notice(fmt.Sprintf(format, args...))
}

// Progress writes msg to stderr in dim — for transient progress lines
// (download spinners, "doing X ..." steps) that should not pollute
// stdout so scripts parsing stdout see only stable info / success
// output.
func (l *Logger) Progress(msg string) {
	_, _ = fmt.Fprintf(l.stderr, "%s%s%s\n", l.stderrDim, msg, l.stderrReset)
}

// Progressf writes the formatted message to stderr in dim.
func (l *Logger) Progressf(format string, args ...any) {
	l.Progress(fmt.Sprintf(format, args...))
}

// Bold returns s wrapped in ANSI bold sequences when the stdout sink
// supports color. Used to emphasise labels and headers inline.
func (l *Logger) Bold(s string) string {
	if l.stdoutBold == "" {
		return s
	}
	return l.stdoutBold + s + l.stdoutReset
}

// Dim returns s wrapped in ANSI dim sequences when the stdout sink
// supports color. Used for progress / less-important lines.
func (l *Logger) Dim(s string) string {
	if l.stdoutDim == "" {
		return s
	}
	return l.stdoutDim + s + l.stdoutReset
}
