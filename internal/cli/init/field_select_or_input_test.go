//nolint:testpackage // white-box test for the unexported selectOrInputField model.
package initcli

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// errTestReject is the sentinel that validation-failure cases assert on via
// errors.Is (testing.md §2 — no inline errors.New comparison).
var errTestReject = errors.New("test-reject")

// newTestField wires the field the way runSingleFieldForm does at runtime:
// huh's default Select keymap plus a render width. The constructor leaves
// keymap empty, so navigation stays inert until WithKeyMap runs.
func newTestField(suggestions []string, target *string) *selectOrInputField {
	f := newSelectOrInputField("test_key", target, suggestions, "OTHER-LABEL")
	f.WithKeyMap(huh.NewDefaultKeyMap())
	f.WithWidth(80)
	return f
}

// runeKey builds a printable-key message ("k", "j", "g", "G", free text).
func runeKey(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// cmdKind classifies a tea.Cmd from the submit path: "nil", "next"
// (huh.NextField), "prev" (huh.PrevField), or the message's %T otherwise.
// huh.NextField/PrevField are funcs over unexported message structs, so
// identity comparison is impossible; invoke and match the type name.
func cmdKind(cmd tea.Cmd) string {
	if cmd == nil {
		return "nil"
	}
	msg := cmd()
	switch fmt.Sprintf("%T", msg) {
	case fmt.Sprintf("%T", huh.NextField()):
		return "next"
	case fmt.Sprintf("%T", huh.PrevField()):
		return "prev"
	default:
		return fmt.Sprintf("%T", msg)
	}
}

// sendKey feeds one key through Update and type-asserts the model back.
func sendKey(t *testing.T, f *selectOrInputField, msg tea.KeyMsg) (*selectOrInputField, tea.Cmd) {
	t.Helper()
	m, cmd := f.Update(msg)
	got, ok := m.(*selectOrInputField)
	if !ok {
		t.Fatalf("Update returned %T, want *selectOrInputField", m)
	}
	return got, cmd
}

// TestSelectOrInputField_AccessibleHeaderIncludesURLLine pins down the new
// URLLine setter: when set, RunAccessible's printed header carries the URL
// on its own line under the description. The accessible path is the
// rendering surface that doesn't depend on a live tty / lipgloss theme.
func TestSelectOrInputField_AccessibleHeaderIncludesURLLine(t *testing.T) {
	t.Parallel()
	var target string
	f := newSelectOrInputField("k", &target, []string{"LATEST"}, "Other").
		Title("Pin version for go").
		Description("desc here").
		URLLine("https://github.com/golang/go")

	var buf bytes.Buffer
	f.printAccessibleHeader(&buf)
	got := buf.String()
	for _, want := range []string{
		"Pin version for go",
		"desc here",
		"https://github.com/golang/go",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("accessible header missing %q\n--- got ---\n%s", want, got)
		}
	}
	// URL line should be on its own line, between description and the
	// suggestion list.
	descIdx := strings.Index(got, "desc here")
	urlIdx := strings.Index(got, "https://github.com/golang/go")
	latestIdx := strings.Index(got, "LATEST")
	if descIdx >= urlIdx || urlIdx >= latestIdx {
		t.Errorf("expected order: description < url < suggestions; got positions desc=%d url=%d latest=%d\n--- got ---\n%s",
			descIdx, urlIdx, latestIdx, got)
	}
}

// TestSelectOrInputField_URLLineEmptyOmitsRow pins down that callers that
// do not set a URL (or pass "") get no extra blank row in the accessible
// header.
func TestSelectOrInputField_URLLineEmptyOmitsRow(t *testing.T) {
	t.Parallel()
	var target string
	f := newSelectOrInputField("k", &target, []string{"LATEST"}, "Other").
		Title("title").
		Description("desc")

	var buf bytes.Buffer
	f.printAccessibleHeader(&buf)
	got := buf.String()
	if strings.Contains(got, "https://") {
		t.Errorf("URL row leaked into header when URLLine was unset:\n%s", got)
	}
	// Exactly three lines before the suggestion list: title, description,
	// suggestions (no URL row). Tolerate trailing whitespace.
	prefix := strings.SplitN(got, "1. LATEST", 2)[0]
	lines := strings.Split(strings.TrimRight(prefix, "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 header lines (title, description); got %d:\n%q", len(lines), lines)
	}
}

// TestSelectOrInputField_NewPrefill pins the constructor doc claim that an
// off-whitelist pre-fill lands on the input row with the value pre-typed,
// and that a matching value lands on its suggestion.
func TestSelectOrInputField_NewPrefill(t *testing.T) {
	t.Parallel()
	// name, suggestions, target value, nilTarget, wantCursor, wantInput
	cases := []struct {
		name        string
		suggestions []string
		target      string
		nilTarget   bool
		wantCursor  int
		wantInput   string
	}{
		{"empty_target", []string{"A", "B"}, "", false, 0, ""},
		{"nil_target", []string{"A", "B"}, "", true, 0, ""},
		{"matches_first", []string{"A", "B", "C"}, "A", false, 0, ""},
		{"matches_middle", []string{"A", "B", "C"}, "B", false, 1, ""},
		{"matches_last", []string{"A", "B", "C"}, "C", false, 2, ""},
		{"off_list", []string{"A", "B"}, "custom-tag", false, 2, "custom-tag"},
		{"off_list_empty_suggestions", []string{}, "x", false, 0, "x"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var target *string
			if !tc.nilTarget {
				v := tc.target
				target = &v
			}
			f := newSelectOrInputField("k", target, tc.suggestions, "Other")
			if f.cursor != tc.wantCursor {
				t.Errorf("cursor = %d, want %d", f.cursor, tc.wantCursor)
			}
			if got := f.input.Value(); got != tc.wantInput {
				t.Errorf("input value = %q, want %q", got, tc.wantInput)
			}
		})
	}
}

// TestSelectOrInputField_ConstructorDefaults pins the textinput setup and
// that the keymap is left empty for huh.Form's WithKeyMap to populate.
func TestSelectOrInputField_ConstructorDefaults(t *testing.T) {
	t.Parallel()
	var target string
	f := newSelectOrInputField("mykey", &target, []string{"A"}, "Other")
	if f.input.Prompt != "" {
		t.Errorf("input.Prompt = %q, want empty", f.input.Prompt)
	}
	if f.input.CharLimit != 128 {
		t.Errorf("input.CharLimit = %d, want 128", f.input.CharLimit)
	}
	// The constructor leaves keymap empty; its bindings carry no keys until
	// WithKeyMap installs huh's real Select profile.
	if n := len(f.keymap.Submit.Keys()); n != 0 {
		t.Errorf("constructor keymap.Submit binds %d keys, want 0", n)
	}
	if f.err != nil {
		t.Errorf("err = %v, want nil", f.err)
	}
	if f.validate == nil {
		t.Fatal("validate must be non-nil")
	}
	if err := f.validate("anything"); err != nil {
		t.Errorf("default validate returned %v, want nil", err)
	}
}

// TestSelectOrInputField_SettersChain pins that the fluent setters apply
// their value and return the receiver for chaining.
func TestSelectOrInputField_SettersChain(t *testing.T) {
	t.Parallel()
	var target string
	f := newSelectOrInputField("k", &target, []string{"A"}, "Other")
	if f.Title("TITLE-X") != f {
		t.Error("Title should return the receiver")
	}
	if f.Description("DESC-X") != f {
		t.Error("Description should return the receiver")
	}
	if f.URLLine("https://x.example") != f {
		t.Error("URLLine should return the receiver")
	}
	if f.Validate(func(string) error { return nil }) != f {
		t.Error("Validate should return the receiver")
	}
	if f.title != "TITLE-X" || f.description != "DESC-X" || f.urlLine != "https://x.example" {
		t.Errorf("setters did not apply: title=%q desc=%q url=%q", f.title, f.description, f.urlLine)
	}
}

// TestSelectOrInputField_Init checks Init returns a non-nil blink command.
func TestSelectOrInputField_Init(t *testing.T) {
	t.Parallel()
	var target string
	f := newSelectOrInputField("k", &target, []string{"A"}, "Other")
	if f.Init() == nil {
		t.Error("Init should return a non-nil blink command")
	}
}

// TestSelectOrInputField_UpdateNonKeyMsg checks a non-KeyMsg leaves the
// model intact whether the cursor sits on a suggestion or the input row.
func TestSelectOrInputField_UpdateNonKeyMsg(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		onInput bool
	}{
		{"on_input_row", true},
		{"on_suggestion_row", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var target string
			f := newTestField([]string{"A", "B"}, &target)
			if tc.onInput {
				f.cursor = len(f.suggestions)
			}
			m, _ := f.Update(struct{}{})
			if got, ok := m.(*selectOrInputField); !ok || got != f {
				t.Errorf("Update(non-key) returned %T, want the same field", m)
			}
		})
	}
}

// TestSelectOrInputField_UpdateClearsError pins that every KeyMsg clears a
// stale validation error before dispatch.
func TestSelectOrInputField_UpdateClearsError(t *testing.T) {
	t.Parallel()
	var target string
	f := newTestField([]string{"A", "B"}, &target)
	f.err = errTestReject
	// "z" is unbound in huh's Select keymap, so it exercises Update's
	// pre-dispatch f.err = nil without triggering nav or submit.
	f, _ = sendKey(t, f, runeKey("z"))
	if f.Error() != nil {
		t.Errorf("Update should clear err on a KeyMsg; got %v", f.Error())
	}
}

// TestSelectOrInputField_HandleNav covers the navigation keys: cursor
// movement, clamping at both ends, and the textinput focus/blur transitions
// as the cursor crosses onto or off the input row.
func TestSelectOrInputField_HandleNav(t *testing.T) {
	t.Parallel()
	// name, startCursor, focusInput, key, wantCursor, wantInputFocus, wantCmdNil
	cases := []struct {
		name           string
		startCursor    int
		focusInput     bool
		key            tea.KeyMsg
		wantCursor     int
		wantInputFocus bool
		wantCmdNil     bool
	}{
		{"up_from_input_row", 2, true, tea.KeyMsg{Type: tea.KeyUp}, 1, false, true},
		{"up_clamps_at_top", 0, false, tea.KeyMsg{Type: tea.KeyUp}, 0, false, true},
		{"up_k_rune", 1, false, runeKey("k"), 0, false, true},
		{"down_into_input_row", 1, false, tea.KeyMsg{Type: tea.KeyDown}, 2, true, false},
		{"down_clamps_on_input_row", 2, true, tea.KeyMsg{Type: tea.KeyDown}, 2, true, true},
		{"down_j_rune", 0, false, runeKey("j"), 1, false, true},
		{"gototop_from_input_row", 2, true, tea.KeyMsg{Type: tea.KeyHome}, 0, false, true},
		{"gototop_g_rune", 1, false, runeKey("g"), 0, false, true},
		{"gotobottom_to_input_row", 0, false, tea.KeyMsg{Type: tea.KeyEnd}, 2, true, false},
		{"gotobottom_G_rune", 1, false, runeKey("G"), 2, true, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var target string
			f := newTestField([]string{"A", "B"}, &target)
			f.cursor = tc.startCursor
			if tc.focusInput {
				f.input.Focus()
			}
			f, cmd := sendKey(t, f, tc.key)
			if f.cursor != tc.wantCursor {
				t.Errorf("cursor = %d, want %d", f.cursor, tc.wantCursor)
			}
			if f.input.Focused() != tc.wantInputFocus {
				t.Errorf("input.Focused() = %v, want %v", f.input.Focused(), tc.wantInputFocus)
			}
			if (cmd == nil) != tc.wantCmdNil {
				t.Errorf("cmd == nil is %v, want %v", cmd == nil, tc.wantCmdNil)
			}
		})
	}
}

// TestSelectOrInputField_HandleSubmit covers the submit keys: Next and
// Submit commit the value, Prev hands control back to huh.
func TestSelectOrInputField_HandleSubmit(t *testing.T) {
	t.Parallel()
	// name, startCursor, key, wantKind, wantTarget
	cases := []struct {
		name        string
		startCursor int
		key         tea.KeyMsg
		wantKind    string
		wantTarget  string
	}{
		{"tab_commits_suggestion", 1, tea.KeyMsg{Type: tea.KeyTab}, "next", "B"},
		{"enter_commits_suggestion", 0, tea.KeyMsg{Type: tea.KeyEnter}, "next", "A"},
		{"shift_tab_goes_prev", 0, tea.KeyMsg{Type: tea.KeyShiftTab}, "prev", ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var target string
			f := newTestField([]string{"A", "B"}, &target)
			f.cursor = tc.startCursor
			_, cmd := sendKey(t, f, tc.key)
			if got := cmdKind(cmd); got != tc.wantKind {
				t.Errorf("cmd kind = %q, want %q", got, tc.wantKind)
			}
			if target != tc.wantTarget {
				t.Errorf("target = %q, want %q", target, tc.wantTarget)
			}
		})
	}
}

// TestSelectOrInputField_Commit covers commit's value sourcing (suggestion
// vs trimmed free text), validation success and failure, and the nil-target
// guard.
func TestSelectOrInputField_Commit(t *testing.T) {
	t.Parallel()
	reject := func(string) error { return errTestReject }
	// name, cursor, inputValue, validator, nilTarget, wantKind, wantTarget, wantErr
	cases := []struct {
		name       string
		cursor     int
		inputValue string
		validator  func(string) error
		nilTarget  bool
		wantKind   string
		wantTarget string
		wantErr    bool
	}{
		{"suggestion", 1, "", nil, false, "next", "B", false},
		{"freetext_trimmed", 2, "  v1.2.3  ", nil, false, "next", "v1.2.3", false},
		{"freetext_empty", 2, "", nil, false, "next", "", false},
		{"validate_fail_suggestion", 0, "", reject, false, "nil", "", true},
		{"validate_fail_freetext", 2, "bad", reject, false, "nil", "", true},
		{"nil_target", 0, "", nil, true, "next", "", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var target *string
			if !tc.nilTarget {
				v := ""
				target = &v
			}
			f := newSelectOrInputField("k", target, []string{"A", "B"}, "Other")
			f.cursor = tc.cursor
			if tc.inputValue != "" {
				f.input.SetValue(tc.inputValue)
			}
			if tc.validator != nil {
				f.Validate(tc.validator)
			}
			if got := cmdKind(f.commit()); got != tc.wantKind {
				t.Errorf("commit cmd kind = %q, want %q", got, tc.wantKind)
			}
			if tc.wantErr {
				if !errors.Is(f.Error(), errTestReject) {
					t.Errorf("f.Error() = %v, want errTestReject", f.Error())
				}
			} else if f.Error() != nil {
				t.Errorf("f.Error() = %v, want nil", f.Error())
			}
			if !tc.nilTarget && *target != tc.wantTarget {
				t.Errorf("target = %q, want %q", *target, tc.wantTarget)
			}
		})
	}
}

// TestSelectOrInputField_View checks the rendered surface: header lines,
// the suggestion list, the input row vs its placeholder, and the error row.
func TestSelectOrInputField_View(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		setup   func(*selectOrInputField)
		want    []string
		notWant []string
	}{
		{
			name:  "title_and_description",
			setup: func(f *selectOrInputField) { f.Title("PICK-IMG").Description("DESC-LINE") },
			want:  []string{"PICK-IMG", "DESC-LINE"},
		},
		{
			name:    "no_title",
			setup:   func(*selectOrInputField) {},
			notWant: []string{"PICK-IMG"},
		},
		{
			name:  "urlline_present",
			setup: func(f *selectOrInputField) { f.URLLine("https://x.example/rel") },
			want:  []string{"https://x.example/rel"},
		},
		{
			name:    "urlline_absent",
			setup:   func(*selectOrInputField) {},
			notWant: []string{"https://"},
		},
		{
			name:  "lists_suggestions",
			setup: func(*selectOrInputField) {},
			want:  []string{"alpha", "beta"},
		},
		{
			name:  "placeholder_when_cursor_off_input",
			setup: func(f *selectOrInputField) { f.cursor = 0 },
			want:  []string{"OTHER-LABEL"},
		},
		{
			name: "input_row_active",
			setup: func(f *selectOrInputField) {
				f.cursor = len(f.suggestions)
				f.input.SetValue("typed-val")
			},
			want: []string{"typed-val"},
		},
		{
			name:  "error_line",
			setup: func(f *selectOrInputField) { f.err = errTestReject },
			want:  []string{"✕", "test-reject"},
		},
		{
			name:    "no_error_line",
			setup:   func(*selectOrInputField) {},
			notWant: []string{"✕"},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var target string
			f := newTestField([]string{"alpha", "beta"}, &target)
			tc.setup(f)
			view := f.View()
			for _, w := range tc.want {
				if !strings.Contains(view, w) {
					t.Errorf("View() missing %q\n--- view ---\n%s", w, view)
				}
			}
			for _, nw := range tc.notWant {
				if strings.Contains(view, nw) {
					t.Errorf("View() unexpectedly contains %q\n--- view ---\n%s", nw, view)
				}
			}
		})
	}
}

// TestSelectOrInputField_FocusBlur pins that Focus drives the embedded
// textinput only when the cursor is on the input row, and Blur clears both.
func TestSelectOrInputField_FocusBlur(t *testing.T) {
	t.Parallel()
	t.Run("focus_on_input_row", func(t *testing.T) {
		t.Parallel()
		var target string
		f := newTestField([]string{"A", "B"}, &target)
		f.cursor = len(f.suggestions)
		if f.Focus() == nil {
			t.Error("Focus on the input row should return a non-nil cmd")
		}
		if !f.focused {
			t.Error("Focus should set focused = true")
		}
		if !f.input.Focused() {
			t.Error("Focus on the input row should focus the textinput")
		}
	})
	t.Run("focus_on_suggestion_row", func(t *testing.T) {
		t.Parallel()
		var target string
		f := newTestField([]string{"A", "B"}, &target)
		f.cursor = 0
		if f.Focus() != nil {
			t.Error("Focus on a suggestion row should return a nil cmd")
		}
		if !f.focused {
			t.Error("Focus should set focused = true")
		}
		if f.input.Focused() {
			t.Error("Focus on a suggestion row should not focus the textinput")
		}
	})
	t.Run("blur_clears", func(t *testing.T) {
		t.Parallel()
		var target string
		f := newTestField([]string{"A", "B"}, &target)
		f.cursor = len(f.suggestions)
		f.Focus()
		if f.Blur() != nil {
			t.Error("Blur should return a nil cmd")
		}
		if f.focused {
			t.Error("Blur should set focused = false")
		}
		if f.input.Focused() {
			t.Error("Blur should blur the textinput")
		}
	})
}

// TestSelectOrInputField_Accessors covers the small huh.Field accessors,
// including the KeyBinds contract that Prev is intentionally omitted.
func TestSelectOrInputField_Accessors(t *testing.T) {
	t.Parallel()
	t.Run("error", func(t *testing.T) {
		t.Parallel()
		var target string
		f := newSelectOrInputField("k", &target, []string{"A"}, "Other")
		if f.Error() != nil {
			t.Errorf("fresh field Error() = %v, want nil", f.Error())
		}
		f.err = errTestReject
		if !errors.Is(f.Error(), errTestReject) {
			t.Errorf("Error() = %v, want errTestReject", f.Error())
		}
	})
	t.Run("skip_and_zoom_false", func(t *testing.T) {
		t.Parallel()
		var target string
		f := newSelectOrInputField("k", &target, []string{"A"}, "Other")
		if f.Skip() {
			t.Error("Skip() = true, want false")
		}
		if f.Zoom() {
			t.Error("Zoom() = true, want false")
		}
	})
	t.Run("getkey", func(t *testing.T) {
		t.Parallel()
		var target string
		f := newSelectOrInputField("mykey", &target, []string{"A"}, "Other")
		if f.GetKey() != "mykey" {
			t.Errorf("GetKey() = %q, want %q", f.GetKey(), "mykey")
		}
	})
	t.Run("getvalue", func(t *testing.T) {
		t.Parallel()
		nilTargetField := newSelectOrInputField("k", nil, []string{"A"}, "Other")
		if got := nilTargetField.GetValue(); got != "" {
			t.Errorf("GetValue() with nil target = %v, want empty string", got)
		}
		v := "committed"
		f := newSelectOrInputField("k", &v, []string{"A"}, "Other")
		if got := f.GetValue(); got != "committed" {
			t.Errorf("GetValue() = %v, want %q", got, "committed")
		}
	})
	t.Run("keybinds_omit_prev", func(t *testing.T) {
		t.Parallel()
		var target string
		f := newTestField([]string{"A"}, &target)
		binds := f.KeyBinds()
		if len(binds) != 3 {
			t.Fatalf("KeyBinds() returned %d bindings, want 3", len(binds))
		}
		for _, b := range binds {
			for _, k := range b.Keys() {
				if k == "shift+tab" {
					t.Errorf("KeyBinds() must omit Prev; found shift+tab in %v", b.Keys())
				}
			}
		}
	})
}

// TestSelectOrInputField_WithSetters covers the huh.Field injection setters,
// including WithKeyMap pulling navigation from the Select profile.
func TestSelectOrInputField_WithSetters(t *testing.T) {
	t.Parallel()
	t.Run("with_theme", func(t *testing.T) {
		t.Parallel()
		var target string
		f := newSelectOrInputField("k", &target, []string{"A"}, "Other")
		f.WithTheme(huh.ThemeBase())
		if f.theme == nil {
			t.Error("WithTheme should set the theme")
		}
	})
	t.Run("with_accessible", func(t *testing.T) {
		t.Parallel()
		var target string
		f := newSelectOrInputField("k", &target, []string{"A"}, "Other")
		f.WithAccessible(true)
		if !f.access {
			t.Error("WithAccessible(true) should set access = true")
		}
	})
	t.Run("with_keymap_extracts_select", func(t *testing.T) {
		t.Parallel()
		var target string
		f := newSelectOrInputField("k", &target, []string{"A"}, "Other")
		f.WithKeyMap(huh.NewDefaultKeyMap())
		// With the Select profile installed, enter must reach commit; an
		// empty keymap would leave the keystroke inert.
		_, cmd := sendKey(t, f, tea.KeyMsg{Type: tea.KeyEnter})
		if got := cmdKind(cmd); got != "next" {
			t.Errorf("after WithKeyMap, enter cmd kind = %q, want next", got)
		}
	})
	t.Run("with_width_and_height", func(t *testing.T) {
		t.Parallel()
		var target string
		f := newSelectOrInputField("k", &target, []string{"A"}, "Other")
		f.WithWidth(72)
		f.WithHeight(9)
		if f.width != 72 || f.height != 9 {
			t.Errorf("width/height = %d/%d, want 72/9", f.width, f.height)
		}
	})
	t.Run("with_position_noop", func(t *testing.T) {
		t.Parallel()
		var target string
		f := newSelectOrInputField("k", &target, []string{"A"}, "Other")
		if f.WithPosition(huh.FieldPosition{}) != huh.Field(f) {
			t.Error("WithPosition should return the same field")
		}
	})
}

// TestSelectOrInputField_NoKeymapInert pins that a field built straight from
// the constructor (empty keymap) ignores every navigation and submit key —
// the regression guard for the WithKeyMap wiring contract.
func TestSelectOrInputField_NoKeymapInert(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		key  tea.KeyMsg
	}{
		{"up", tea.KeyMsg{Type: tea.KeyUp}},
		{"down", tea.KeyMsg{Type: tea.KeyDown}},
		{"home", tea.KeyMsg{Type: tea.KeyHome}},
		{"end", tea.KeyMsg{Type: tea.KeyEnd}},
		{"enter", tea.KeyMsg{Type: tea.KeyEnter}},
		{"shift_tab", tea.KeyMsg{Type: tea.KeyShiftTab}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var target string
			f := newSelectOrInputField("k", &target, []string{"A", "B"}, "Other")
			startCursor := f.cursor
			f, cmd := sendKey(t, f, tc.key)
			if f.cursor != startCursor {
				t.Errorf("cursor moved to %d with an empty keymap; want %d", f.cursor, startCursor)
			}
			if target != "" {
				t.Errorf("target = %q, want empty (no commit with an empty keymap)", target)
			}
			if cmd != nil {
				t.Errorf("cmd = %v, want nil with an empty keymap", cmd)
			}
		})
	}
}
