//nolint:testpackage // white-box: drives the unexported selectOrInputField through teatest.
package initcli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/exp/teatest"
)

// teatestTimeout bounds both WaitFor polling and final-model joins. -race CI
// runners are slow, so 3s is generous; a genuine hang (a future bubbles bump
// breaking key routing or the draw loop — the regression these tests guard)
// still fails fast rather than wedging the suite.
const teatestTimeout = 3 * time.Second

// teatestUnset is seeded into *target AFTER construction so only a real commit
// overwrites it. newSelectOrInputField reads *target for off-whitelist prefill,
// so the constructor must see "" to keep the cursor on row 0 — hence poisoning
// post-construction. Without it a want=="" case (the empty input row) would
// pass even if key routing broke and no commit ran, since *target is already "".
const teatestUnset = "\x00unset"

// keyMsg builds a special-key message (arrows, Home/End, Enter, Tab). Printable
// runes go through tm.Type instead; "g"/"G"/"j"/"k" are typed verbatim on the
// input row but still drive nav on a suggestion row, so type them only once the
// cursor has parked on the input row.
func keyMsg(t tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: t} }

// newTeatestField wires the field the way runSingleFieldForm does (huh's
// default Select keymap + width), plus a pinned theme and Focus so the live
// View() renders deterministically when driven standalone. A real huh.Group
// would focus the field on Init; standalone we must focus it ourselves, else
// the input-row caret and the Focused style branch never engage.
func newTeatestField(suggestions []string, target *string) *selectOrInputField {
	f := newSelectOrInputField("test_key", target, suggestions, "OTHER-LABEL")
	f.WithKeyMap(huh.NewDefaultKeyMap())
	f.WithTheme(huh.ThemeCharm())
	f.WithWidth(80)
	f.Focus()
	return f
}

// waitForText blocks until the program's cumulative output contains want.
// teatest exposes a raw VT stream; lipgloss wraps SGR codes around ASCII
// literals (not inside them), so bytes.Contains on the plain text matches.
func waitForText(t *testing.T, tm *teatest.TestModel, want string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte(want))
	}, teatest.WithDuration(teatestTimeout))
}

// quitAndWait sends Quit and joins the program goroutine. Without the join
// -race flags a leaked goroutine still touching the model, and the target
// pointer would be read while Update may still be writing it.
func quitAndWait(t *testing.T, tm *teatest.TestModel) {
	t.Helper()
	if err := tm.Quit(); err != nil {
		t.Fatalf("quit: %v", err)
	}
	tm.WaitFinished(t, teatest.WithFinalTimeout(teatestTimeout))
}

// TestSelectOrInputField_Teatest drives the field through a real tea.Program
// — Init -> textinput.Blink -> Update -> View on every frame — which the
// Update/View unit tests skip by calling methods directly. This is the
// regression net for bubbletea/bubbles dependency bumps: a bump that breaks
// key routing, focus transitions, or the draw loop fails here while the unit
// tests stay green.
//
//nolint:paralleltest // teatest runs a tea.Program goroutine with timers.
func TestSelectOrInputField_Teatest(t *testing.T) {
	suggestions := []string{"alpha", "beta"}
	// teatest delivers messages FIFO, so the typed runes in the input-row
	// cases land before the trailing Enter without an explicit sync point.
	cases := []struct {
		name  string
		drive func(tm *teatest.TestModel)
		want  string
	}{
		{
			name:  "select_first_with_enter",
			drive: func(tm *teatest.TestModel) { tm.Send(keyMsg(tea.KeyEnter)) },
			want:  "alpha",
		},
		{
			name: "navigate_down_then_enter",
			drive: func(tm *teatest.TestModel) {
				tm.Send(keyMsg(tea.KeyDown))
				tm.Send(keyMsg(tea.KeyDown)) // onto the empty input row
				tm.Send(keyMsg(tea.KeyEnter))
			},
			want: "",
		},
		{
			name: "tab_commits_current",
			drive: func(tm *teatest.TestModel) {
				tm.Send(keyMsg(tea.KeyDown))
				tm.Send(keyMsg(tea.KeyTab))
			},
			want: "beta",
		},
		{
			name: "goto_bottom_then_top",
			drive: func(tm *teatest.TestModel) {
				tm.Send(keyMsg(tea.KeyEnd))  // jump to input row
				tm.Send(keyMsg(tea.KeyHome)) // back to row 0
				tm.Send(keyMsg(tea.KeyEnter))
			},
			want: "alpha",
		},
		{
			name: "down_to_input_then_type",
			drive: func(tm *teatest.TestModel) {
				tm.Send(keyMsg(tea.KeyDown))
				tm.Send(keyMsg(tea.KeyDown)) // input row
				tm.Type("1.2.3")
				tm.Send(keyMsg(tea.KeyEnter))
			},
			want: "1.2.3",
		},
		{
			name: "freetext_trimmed",
			drive: func(tm *teatest.TestModel) {
				tm.Send(keyMsg(tea.KeyEnd)) // input row
				tm.Type("  v9  ")
				tm.Send(keyMsg(tea.KeyEnter))
			},
			want: "v9", // commit trims surrounding space
		},
		{
			// Regression: j/k/g/G are typed verbatim on the input row instead
			// of being stolen by the vim Select nav keymap.
			name: "input_row_types_nav_runes",
			drive: func(tm *teatest.TestModel) {
				tm.Send(keyMsg(tea.KeyEnd)) // input row
				tm.Type("jdk-G21")
				tm.Send(keyMsg(tea.KeyEnter))
			},
			want: "jdk-G21",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			target := ""
			f := newTeatestField(suggestions, &target)
			target = teatestUnset // only a real commit clears the sentinel
			tm := teatest.NewTestModel(t, f, teatest.WithInitialTermSize(80, 24))
			waitForText(t, tm, "alpha") // program is up and has drawn the list
			tc.drive(tm)
			quitAndWait(t, tm)
			if target != tc.want {
				t.Errorf("target = %q, want %q", target, tc.want)
			}
		})
	}
}

// TestSelectOrInputField_Teatest_ValidationRecovery drives a rejected
// free-text entry through the live loop, asserts the error glyph actually
// renders on screen (commit() called directly only sets f.err — it never
// proves View paints it), then recovers by returning to a valid suggestion.
//
//nolint:paralleltest // teatest runs a tea.Program goroutine with timers.
func TestSelectOrInputField_Teatest_ValidationRecovery(t *testing.T) {
	target := ""
	f := newTeatestField([]string{"alpha", "beta"}, &target)
	target = teatestUnset // only a real commit clears the sentinel
	f.Validate(func(s string) error {
		if strings.Contains(s, "/") {
			return errTestReject
		}
		return nil
	})
	tm := teatest.NewTestModel(t, f, teatest.WithInitialTermSize(80, 24))
	waitForText(t, tm, "alpha")

	tm.Send(keyMsg(tea.KeyEnd)) // input row
	tm.Type("x/y")              // '/' triggers reject; no g/G/j/k to be stolen by nav
	waitForText(t, tm, "x/y")
	tm.Send(keyMsg(tea.KeyEnter)) // commit -> validate fails
	waitForText(t, tm, "✕")       // error row painted by the live View

	tm.Send(keyMsg(tea.KeyHome))  // back to suggestion row 0 (Update clears f.err first)
	tm.Send(keyMsg(tea.KeyEnter)) // commit "alpha" -> passes
	quitAndWait(t, tm)

	if target != "alpha" {
		t.Errorf("target = %q, want %q after recovery", target, "alpha")
	}
}

// TestSelectOrInputField_Teatest_InForm wraps the field in a real huh.Form and
// drives it through teatest, exercising huh's group/keymap/submit plumbing
// around the custom field under the TTY draw loop — coverage the accessible
// E2E (TERM=dumb, line-based) cannot give. SubmitCmd/CancelCmd must be set
// explicitly: huh assigns them inside RunWithContext, which teatest never
// calls, so a completed form would return a nil cmd and hang until timeout.
//
//nolint:paralleltest // teatest runs a tea.Program goroutine with timers.
func TestSelectOrInputField_Teatest_InForm(t *testing.T) {
	target := ""
	f := newSelectOrInputField("test_key", &target, []string{"alpha", "beta"}, "OTHER-LABEL")
	target = teatestUnset // only a real commit clears the sentinel
	form := huh.NewForm(huh.NewGroup(f))
	form.SubmitCmd = tea.Quit
	form.CancelCmd = tea.Interrupt

	tm := teatest.NewTestModel(t, form, teatest.WithInitialTermSize(80, 24))
	waitForText(t, tm, "alpha")
	tm.Send(keyMsg(tea.KeyDown))  // select "beta"
	tm.Send(keyMsg(tea.KeyEnter)) // commit -> NextField -> form completes -> SubmitCmd (tea.Quit)
	tm.WaitFinished(t, teatest.WithFinalTimeout(teatestTimeout))

	if target != "beta" {
		t.Errorf("target = %q, want %q", target, "beta")
	}
}
