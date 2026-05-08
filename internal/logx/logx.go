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
package logx

import (
	"fmt"
	"io"
)

// Logger writes user-facing messages to the injected stdout/stderr sinks.
type Logger struct {
	stdout io.Writer
	stderr io.Writer
}

// New constructs a Logger that writes Info/Print output to stdout and
// Warn/Error output to stderr.
func New(stdout, stderr io.Writer) *Logger {
	return &Logger{stdout: stdout, stderr: stderr}
}

// Stdout returns the configured stdout sink so callers that still need a raw
// io.Writer (for example to pipe a sub-process's output) can reuse it.
func (l *Logger) Stdout() io.Writer { return l.stdout }

// Stderr returns the configured stderr sink.
func (l *Logger) Stderr() io.Writer { return l.stderr }

// Info writes msg followed by a newline to stdout.
func (l *Logger) Info(msg string) { _, _ = fmt.Fprintln(l.stdout, msg) }

// Infof writes the formatted message followed by a newline to stdout.
func (l *Logger) Infof(format string, args ...any) {
	_, _ = fmt.Fprintf(l.stdout, format+"\n", args...)
}

// Warn writes msg with a "warning: " prefix and a trailing newline to stderr.
func (l *Logger) Warn(msg string) { _, _ = fmt.Fprintln(l.stderr, "warning: "+msg) }

// Warnf writes the formatted message with a "warning: " prefix and a
// trailing newline to stderr.
func (l *Logger) Warnf(format string, args ...any) {
	_, _ = fmt.Fprintf(l.stderr, "warning: "+format+"\n", args...)
}

// Error writes msg followed by a newline to stderr.
func (l *Logger) Error(msg string) { _, _ = fmt.Fprintln(l.stderr, msg) }

// Errorf writes the formatted message followed by a newline to stderr.
func (l *Logger) Errorf(format string, args ...any) {
	_, _ = fmt.Fprintf(l.stderr, format+"\n", args...)
}

// Print writes msg to stdout without a trailing newline.
func (l *Logger) Print(msg string) { _, _ = fmt.Fprint(l.stdout, msg) }

// Printf writes the formatted message to stdout without a trailing newline.
func (l *Logger) Printf(format string, args ...any) {
	_, _ = fmt.Fprintf(l.stdout, format, args...)
}
