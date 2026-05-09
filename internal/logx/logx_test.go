package logx_test

import (
	"bytes"
	"testing"

	"github.com/sukekyo26/cocoon/internal/logx"
)

func newLogger(t *testing.T) (*logx.Logger, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	return logx.New(&stdout, &stderr), &stdout, &stderr
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
