// Package logx is a thin io.Writer wrapper used by the CLI subcommands.
// forbidigo+depguard ban fmt.Print*/log elsewhere in the tree; routing
// through Logger keeps the print seam centralised.
//
// No buffering, no level filtering, no structured fields — output is
// localised i18n text, not structured logs. Color is decided once at
// construction (pass [ColorNever] in tests that compare exact output).
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
	stdoutBold, stdoutGreen, stdoutReset            string
	stderrRed, stderrYellow, stderrCyan, stderrBold string
	stderrDim, stderrReset                          string
}

// New writes Info to stdout and Error/Warn/Notice to stderr, auto-detecting
// color support per sink.
func New(stdout, stderr io.Writer) *Logger {
	return NewWithMode(stdout, stderr, ColorAuto)
}

// NewWithMode applies the given ColorMode to both sinks. Pass [ColorNever]
// in tests that compare exact byte output.
func NewWithMode(stdout, stderr io.Writer, mode ColorMode) *Logger {
	l := &Logger{ //nolint:exhaustruct // ANSI fields below are conditionally populated.
		stdout: stdout,
		stderr: stderr,
	}
	if shouldColor(stdout, mode, os.Getenv) {
		l.stdoutBold = ansiBold
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

// Info writes to stdout (no color).
func (l *Logger) Info(msg string) { _, _ = fmt.Fprintln(l.stdout, msg) }

func (l *Logger) Infof(format string, args ...any) {
	_, _ = fmt.Fprintf(l.stdout, format+"\n", args...)
}

// Success writes to stdout in green.
func (l *Logger) Success(msg string) {
	_, _ = fmt.Fprintf(l.stdout, "%s%s%s\n", l.stdoutGreen, msg, l.stdoutReset)
}

func (l *Logger) Successf(format string, args ...any) {
	l.Success(fmt.Sprintf(format, args...))
}

// Warn writes to stderr in yellow.
func (l *Logger) Warn(msg string) {
	_, _ = fmt.Fprintf(l.stderr, "%s%s%s\n", l.stderrYellow, msg, l.stderrReset)
}

// Error writes to stderr in red.
func (l *Logger) Error(msg string) {
	_, _ = fmt.Fprintf(l.stderr, "%s%s%s\n", l.stderrRed, msg, l.stderrReset)
}

func (l *Logger) Errorf(format string, args ...any) {
	l.Error(fmt.Sprintf(format, args...))
}

// Notice writes to stderr in cyan (informational, e.g. update available).
func (l *Logger) Notice(msg string) {
	_, _ = fmt.Fprintf(l.stderr, "%s%s%s\n", l.stderrCyan, msg, l.stderrReset)
}

// Progress writes to stderr in dim — transient lines that should not
// pollute stdout (download spinners, "doing X..." steps).
func (l *Logger) Progress(msg string) {
	_, _ = fmt.Fprintf(l.stderr, "%s%s%s\n", l.stderrDim, msg, l.stderrReset)
}

func (l *Logger) Progressf(format string, args ...any) {
	l.Progress(fmt.Sprintf(format, args...))
}

// Bold wraps s in ANSI bold for the stdout sink.
func (l *Logger) Bold(s string) string {
	if l.stdoutBold == "" {
		return s
	}
	return l.stdoutBold + s + l.stdoutReset
}
