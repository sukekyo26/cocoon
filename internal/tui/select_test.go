//nolint:testpackage // white-box test for unexported filterPreset and runForm seam.
package tui

import (
	"errors"
	"testing"

	"github.com/charmbracelet/huh"
)

func TestSelectSingle_NoOptions(t *testing.T) {
	t.Parallel()
	_, err := HuhSelector{}.SelectSingle("title", nil, 0)
	if !errors.Is(err, ErrNoOptions) {
		t.Fatalf("err = %v, want ErrNoOptions", err)
	}
}

func TestSelectMulti_NoOptions(t *testing.T) {
	t.Parallel()
	_, err := HuhSelector{}.SelectMulti("title", nil, []int{0})
	if !errors.Is(err, ErrNoOptions) {
		t.Fatalf("err = %v, want ErrNoOptions", err)
	}
}

func TestFilterPreset(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		n           int
		preselected []int
		want        []int
	}{
		{"empty", 5, nil, []int{}},
		{"in_range", 5, []int{0, 2, 4}, []int{0, 2, 4}},
		{"drops_negative", 3, []int{-1, 0, 1}, []int{0, 1}},
		{"drops_overflow", 3, []int{2, 3, 4}, []int{2}},
		{"keeps_duplicates", 5, []int{1, 1, 2}, []int{1, 1, 2}},
		{"zero_options", 0, []int{0, 1}, []int{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := filterPreset(tc.preselected, tc.n)
			if !equalInts(got, tc.want) {
				t.Errorf("filterPreset(%v, %d) = %v, want %v", tc.preselected, tc.n, got, tc.want)
			}
		})
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// fakeRunForm builds a runForm function that records the call and returns the
// supplied error. The form pointer is captured for assertions but its bound
// values are not mutated — callers rely on the fact that picked is initialised
// before form.Run is called (zero for SelectSingle, filtered preset for
// SelectMulti) so the post-run value reflects the pre-run setup.
func fakeRunForm(err error) (func(*huh.Form) error, *int) {
	calls := 0
	return func(_ *huh.Form) error {
		calls++
		return err
	}, &calls
}

func TestSelectSingle_HappyReturnsZero(t *testing.T) {
	t.Parallel()
	run, calls := fakeRunForm(nil)
	got, err := huhSelectSingleImpl("title", []string{"a", "b", "c"}, 0, run)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if got != 0 {
		t.Errorf("picked = %d, want 0 (the Value-bound zero value, since fake runForm leaves it untouched)", got)
	}
	if *calls != 1 {
		t.Errorf("runForm called %d times, want 1", *calls)
	}
}

// TestSelectSingle_DefaultIdxBindsInitialPick proves defaultIdx feeds the
// huh.NewSelect Value() initial value, not just a no-op argument.
func TestSelectSingle_DefaultIdxBindsInitialPick(t *testing.T) {
	t.Parallel()
	run, _ := fakeRunForm(nil)
	got, err := huhSelectSingleImpl("title", []string{"a", "b", "c"}, 2, run)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if got != 2 {
		t.Errorf("picked = %d, want 2 (defaultIdx bound to Value before runForm)", got)
	}
}

func TestSelectSingle_DefaultIdxOutOfRangeClampsToZero(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		def  int
	}{
		{"negative", -1},
		{"equal_len", 3},
		{"overflow", 99},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			run, _ := fakeRunForm(nil)
			got, err := huhSelectSingleImpl("title", []string{"a", "b", "c"}, tc.def, run)
			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if got != 0 {
				t.Errorf("picked = %d, want 0 (defaultIdx %d clamped)", got, tc.def)
			}
		})
	}
}

func TestSelectSingle_CanceledMapping(t *testing.T) {
	t.Parallel()
	run, _ := fakeRunForm(huh.ErrUserAborted)
	_, err := huhSelectSingleImpl("title", []string{"a"}, 0, run)
	if !errors.Is(err, ErrCanceled) {
		t.Errorf("err = %v, want errors.Is(err, ErrCanceled)", err)
	}
}

func TestSelectSingle_GenericErrorWrap(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	run, _ := fakeRunForm(boom)
	_, err := huhSelectSingleImpl("title", []string{"a"}, 0, run)
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want errors.Is(err, boom)", err)
	}
	if errors.Is(err, ErrCanceled) {
		t.Errorf("generic error must not be mapped to ErrCanceled")
	}
}

func TestSelectMulti_PresetFilteringEndToEnd(t *testing.T) {
	t.Parallel()
	run, _ := fakeRunForm(nil)
	// 3 options, preselected mixes valid/duplicate/out-of-range/negative.
	// Expected: filterPreset drops -1 and 99, keeps duplicate, then sort.
	got, err := huhSelectMultiImpl("title", []string{"a", "b", "c"}, []int{2, -1, 99, 0, 0}, run)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	want := []int{0, 0, 2}
	if !equalInts(got, want) {
		t.Errorf("picked = %v, want %v", got, want)
	}
}

func TestSelectMulti_EmptyPresetReturnsEmptySlice(t *testing.T) {
	t.Parallel()
	run, _ := fakeRunForm(nil)
	got, err := huhSelectMultiImpl("title", []string{"a", "b"}, nil, run)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Errorf("picked = %v, want empty slice", got)
	}
}

func TestSelectMulti_CanceledMapping(t *testing.T) {
	t.Parallel()
	run, _ := fakeRunForm(huh.ErrUserAborted)
	_, err := huhSelectMultiImpl("title", []string{"a"}, nil, run)
	if !errors.Is(err, ErrCanceled) {
		t.Errorf("err = %v, want errors.Is(err, ErrCanceled)", err)
	}
}

func TestSelectMulti_GenericErrorWrap(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	run, _ := fakeRunForm(boom)
	_, err := huhSelectMultiImpl("title", []string{"a"}, nil, run)
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want errors.Is(err, boom)", err)
	}
	if errors.Is(err, ErrCanceled) {
		t.Errorf("generic error must not be mapped to ErrCanceled")
	}
}

// TestHuhSelectorIsComparable guards against accidentally re-introducing a
// non-comparable field (function, slice, map) on HuhSelector. If the type ever
// picks up such a field, this test fails to compile — the desired signal.
// The map key usage is the strongest guard because a non-comparable type
// cannot be a map key.
func TestHuhSelectorIsComparable(t *testing.T) {
	t.Parallel()
	_ = map[HuhSelector]struct{}{{}: {}}
}
