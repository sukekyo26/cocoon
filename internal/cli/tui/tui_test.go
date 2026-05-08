package tuicli_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	tuicli "github.com/sukekyo26/cocoon/internal/cli/tui"
	"github.com/sukekyo26/cocoon/internal/tui"
)

type fakeSelector struct {
	wantSingleTitle string
	wantSingleOpts  []string
	singleResult    int
	singleErr       error

	wantMultiTitle  string
	wantMultiOpts   []string
	wantPreselected []int
	multiResult     []int
	multiErr        error
}

func (f *fakeSelector) SelectSingle(title string, options []string, _ int) (int, error) {
	f.wantSingleTitle = title
	f.wantSingleOpts = options
	return f.singleResult, f.singleErr
}

func (f *fakeSelector) SelectMulti(title string, options []string, preselected []int) ([]int, error) {
	f.wantMultiTitle = title
	f.wantMultiOpts = options
	f.wantPreselected = preselected
	return f.multiResult, f.multiErr
}

//nolint:unparam // test helper returns both buffers; not all callers use both
func runCmd(sel tui.Selector, args ...string) (stdout, stderr bytes.Buffer, err error) {
	cmd := tuicli.NewCommandWithSelector(sel, &stdout, &stderr)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return stdout, stderr, err
}

func TestSelectSingle_Success(t *testing.T) {
	t.Parallel()
	sel := &fakeSelector{singleResult: 1}
	stdout, _, err := runCmd(sel, "select-single", "--title", "Pick one", "--", "alpha", "beta", "gamma")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := stdout.String(), "1\n"; got != want {
		t.Fatalf("stdout = %q want %q", got, want)
	}
	if sel.wantSingleTitle != "Pick one" {
		t.Errorf("title = %q", sel.wantSingleTitle)
	}
	if strings.Join(sel.wantSingleOpts, ",") != "alpha,beta,gamma" {
		t.Errorf("opts = %v", sel.wantSingleOpts)
	}
}

func TestSelectSingle_Canceled(t *testing.T) {
	t.Parallel()
	sel := &fakeSelector{singleErr: tui.ErrCanceled}
	stdout, _, err := runCmd(sel, "select-single", "--title", "x", "--", "a")
	if !errors.Is(err, tuicli.ErrCanceled) {
		t.Fatalf("err = %v, want tuicli.ErrCanceled", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty on cancel, got %q", stdout.String())
	}
}

func TestSelectMulti_Success(t *testing.T) {
	t.Parallel()
	sel := &fakeSelector{multiResult: []int{0, 2}}
	stdout, _, err := runCmd(sel, "select-multi", "--title=Pick", "--preselected", "1,2", "--", "a", "b", "c")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got, want := stdout.String(), "0,2\n"; got != want {
		t.Fatalf("stdout = %q want %q", got, want)
	}
	if got, want := strings.Join(intsToStrings(sel.wantPreselected), ","), "1,2"; got != want {
		t.Errorf("preselected = %v", sel.wantPreselected)
	}
}

func TestSelectMulti_EmptyResult(t *testing.T) {
	t.Parallel()
	sel := &fakeSelector{multiResult: []int{}}
	stdout, _, err := runCmd(sel, "select-multi", "--title", "x", "--", "a", "b")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got, want := stdout.String(), "\n"; got != want {
		t.Fatalf("stdout = %q want %q", got, want)
	}
}

func TestUsageErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		args []string
	}{
		{"no_args", []string{"select-single"}},
		{"missing_separator", []string{"select-single", "--title", "x"}},
		{"no_options", []string{"select-single", "--title", "x", "--"}},
		{"missing_title_value", []string{"select-single", "--title"}},
		{"unknown_flag", []string{"select-single", "--bogus", "--", "a"}},
		{"bad_preselected", []string{"select-multi", "--title", "x", "--preselected", "abc", "--", "a"}},
		{"unknown_subcmd", []string{"foo"}},
		{"preselected_on_single", []string{"select-single", "--preselected", "0", "--", "a"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sel := &fakeSelector{}
			_, _, err := runCmd(sel, tc.args...)
			if !errors.Is(err, tuicli.ErrUsage) {
				t.Fatalf("err = %v, want tuicli.ErrUsage", err)
			}
		})
	}
}

func TestHelpToStdout(t *testing.T) {
	t.Parallel()
	sel := &fakeSelector{}
	stdout, _, err := runCmd(sel, "help")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(stdout.String(), "select-single") {
		t.Errorf("expected help to mention select-single, got %q", stdout.String())
	}
}

func TestSelectSingleLeafHelp(t *testing.T) {
	t.Parallel()
	sel := &fakeSelector{}
	for _, flag := range []string{"--help", "-h"} {
		stdout, _, err := runCmd(sel, "select-single", flag)
		if err != nil {
			t.Fatalf("flag=%q err = %v", flag, err)
		}
		if !strings.Contains(stdout.String(), "select-single") {
			t.Errorf("flag=%q expected help banner: %q", flag, stdout.String())
		}
	}
}

func TestSelectMultiLeafHelp(t *testing.T) {
	t.Parallel()
	sel := &fakeSelector{}
	for _, flag := range []string{"--help", "-h"} {
		stdout, _, err := runCmd(sel, "select-multi", flag)
		if err != nil {
			t.Fatalf("flag=%q err = %v", flag, err)
		}
		if !strings.Contains(stdout.String(), "select-multi") {
			t.Errorf("flag=%q expected help banner: %q", flag, stdout.String())
		}
	}
}

func TestRejectUnknownSubcommand(t *testing.T) {
	t.Parallel()
	sel := &fakeSelector{}
	_, _, err := runCmd(sel, "totally-unknown")
	if !errors.Is(err, tuicli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestSelectSingle_EmptyOptions(t *testing.T) {
	t.Parallel()
	sel := &fakeSelector{}
	_, _, err := runCmd(sel, "select-single", "--title", "x", "--")
	if !errors.Is(err, tuicli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestSelectMulti_EmptyOptions(t *testing.T) {
	t.Parallel()
	sel := &fakeSelector{}
	_, _, err := runCmd(sel, "select-multi", "--title", "x", "--")
	if !errors.Is(err, tuicli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestSelectSingle_SelectorFailure(t *testing.T) {
	t.Parallel()
	sel := &fakeSelector{singleErr: errors.New("boom")}
	_, _, err := runCmd(sel, "select-single", "--title", "x", "--", "alpha", "beta")
	if !errors.Is(err, tuicli.ErrFailure) {
		t.Fatalf("err = %v, want ErrFailure", err)
	}
}

func TestSelectMulti_Canceled(t *testing.T) {
	t.Parallel()
	sel := &fakeSelector{multiErr: tui.ErrCanceled}
	_, _, err := runCmd(sel, "select-multi", "--title", "x", "--", "alpha", "beta")
	if !errors.Is(err, tuicli.ErrCanceled) {
		t.Fatalf("err = %v, want ErrCanceled", err)
	}
}

func TestSelectMulti_SelectorFailure(t *testing.T) {
	t.Parallel()
	sel := &fakeSelector{multiErr: errors.New("boom")}
	_, _, err := runCmd(sel, "select-multi", "--title", "x", "--", "alpha", "beta")
	if !errors.Is(err, tuicli.ErrFailure) {
		t.Fatalf("err = %v, want ErrFailure", err)
	}
}

func intsToStrings(xs []int) []string {
	out := make([]string, len(xs))
	for i, n := range xs {
		out[i] = stringsItoa(n)
	}
	return out
}

func stringsItoa(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
