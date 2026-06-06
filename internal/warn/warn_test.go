package warn_test

import (
	"testing"

	"github.com/sukekyo26/cocoon/internal/warn"
)

// TestNilSinkDropsSilently pins the documented contract that a nil *Sink
// silently drops diagnostics so callers need no nil guards.
func TestNilSinkDropsSilently(t *testing.T) {
	t.Parallel()
	var s *warn.Sink
	s.Warn("warn_code", 1)
	s.Info("info_code")
	if got := s.All(); got != nil {
		t.Fatalf("nil Sink.All() = %v, want nil", got)
	}
}

// TestSinkPreservesEmissionOrderAndLevel asserts All() returns diagnostics in
// the order recorded, tagged with the level of the recording method.
func TestSinkPreservesEmissionOrderAndLevel(t *testing.T) {
	t.Parallel()
	s := warn.New()
	s.Warn("a", 1, "x")
	s.Info("b")
	s.Warn("c")

	got := s.All()
	if len(got) != 3 {
		t.Fatalf("len(All()) = %d, want 3", len(got))
	}
	want := []struct {
		level warn.Level
		code  string
		nargs int
	}{
		{warn.LevelWarn, "a", 2},
		{warn.LevelInfo, "b", 0},
		{warn.LevelWarn, "c", 0},
	}
	for i, w := range want {
		if got[i].Level != w.level || got[i].Code != w.code || len(got[i].Args) != w.nargs {
			t.Errorf("All()[%d] = %+v, want level=%d code=%q nargs=%d",
				i, got[i], w.level, w.code, w.nargs)
		}
	}
}

// TestReasonBuildsRef checks Reason carries the code and args verbatim for the
// drain site to expand.
func TestReasonBuildsRef(t *testing.T) {
	t.Parallel()
	r := warn.Reason("warn_port_reason_range", "3000-3005:3000-3005")
	if r.Code != "warn_port_reason_range" || len(r.Args) != 1 {
		t.Fatalf("Reason() = %+v", r)
	}
}
