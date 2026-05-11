package logx_test

import (
	"bytes"
	"testing"

	"github.com/sukekyo26/cocoon/internal/logx"
)

func newLogger(t *testing.T) (*logx.Logger, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	// bytes.Buffer is not *os.File so ColorAuto disables color naturally,
	// but pin it to ColorNever so the test does not depend on that fact.
	return logx.NewWithMode(&stdout, &stderr, logx.ColorNever), &stdout, &stderr
}

func TestInfoAndInfof(t *testing.T) {
	t.Parallel()
	l, stdout, stderr := newLogger(t)
	l.Info("hello")
	l.Infof("count=%d name=%s", 3, "wsd")

	if got, want := stdout.String(), "hello\ncount=3 name=wsd\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestErrorAndErrorf(t *testing.T) {
	t.Parallel()
	l, stdout, stderr := newLogger(t)
	l.Error("boom")
	l.Errorf("op %s: %v", "open", "denied")

	if got, want := stderr.String(), "boom\nop open: denied\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestWarnAndWarnf(t *testing.T) {
	t.Parallel()
	l, stdout, stderr := newLogger(t)
	l.Warn("ports collide")
	l.Warnf("plugin %s missing", "foo")

	if got, want := stderr.String(), "ports collide\nplugin foo missing\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestNoticeAndNoticef(t *testing.T) {
	t.Parallel()
	l, stdout, stderr := newLogger(t)
	l.Notice("update available")
	l.Noticef("v%s is out", "1.2.3")

	if got, want := stderr.String(), "update available\nv1.2.3 is out\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestSuccessAndSuccessf(t *testing.T) {
	t.Parallel()
	l, stdout, stderr := newLogger(t)
	l.Success("wrote file")
	l.Successf("updated %d items", 4)

	if got, want := stdout.String(), "wrote file\nupdated 4 items\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestBoldDim_NoColor(t *testing.T) {
	t.Parallel()
	l, _, _ := newLogger(t)
	if got := l.Bold("LBL"); got != "LBL" {
		t.Errorf("Bold w/o color = %q, want %q", got, "LBL")
	}
	if got := l.Dim("dim"); got != "dim" {
		t.Errorf("Dim w/o color = %q, want %q", got, "dim")
	}
}

func TestColorAlways(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	l := logx.NewWithMode(&stdout, &stderr, logx.ColorAlways)

	l.Error("e")
	l.Warn("w")
	l.Notice("n")
	l.Success("s")

	if got, want := stderr.String(),
		"\x1b[31me\x1b[0m\n\x1b[33mw\x1b[0m\n\x1b[36mn\x1b[0m\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
	if got, want := stdout.String(), "\x1b[32ms\x1b[0m\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	if got := l.Bold("hdr"); got != "\x1b[1mhdr\x1b[0m" {
		t.Errorf("Bold w/ color = %q", got)
	}
	if got := l.Dim("d"); got != "\x1b[2md\x1b[0m" {
		t.Errorf("Dim w/ color = %q", got)
	}
}
