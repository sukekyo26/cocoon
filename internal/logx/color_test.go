package logx_test

import (
	"bytes"
	"testing"

	"github.com/sukekyo26/cocoon/internal/logx"
)

func TestYellowWriter_ColorAlways(t *testing.T) { //nolint:paralleltest // t.Setenv
	// Force the writer wrapper to colorize even on a bytes.Buffer (which
	// is not a TTY) so we can pin the exact wrapping contract.
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "1")

	var buf bytes.Buffer
	w := logx.YellowWriter(&buf)
	if _, err := w.Write([]byte("WARNING: thing\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if got, want := buf.String(), "\x1b[33mWARNING: thing\n\x1b[0m"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRedWriter_ColorAlways(t *testing.T) { //nolint:paralleltest // t.Setenv
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "1")

	var buf bytes.Buffer
	w := logx.RedWriter(&buf)
	if _, err := w.Write([]byte("error\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if got, want := buf.String(), "\x1b[31merror\n\x1b[0m"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestYellowWriter_NoColor_PassesThrough(t *testing.T) { //nolint:paralleltest // t.Setenv
	t.Setenv("NO_COLOR", "1")
	t.Setenv("FORCE_COLOR", "")

	var buf bytes.Buffer
	w := logx.YellowWriter(&buf)
	if _, err := w.Write([]byte("WARNING: thing\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if got, want := buf.String(), "WARNING: thing\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestYellowWriter_NonTTY_PassesThrough(t *testing.T) { //nolint:paralleltest // t.Setenv
	// bytes.Buffer is not *os.File; under ColorAuto without FORCE_COLOR
	// the wrapper must not colorize.
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "")

	var buf bytes.Buffer
	w := logx.YellowWriter(&buf)
	if _, err := w.Write([]byte("plain\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if got, want := buf.String(), "plain\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
