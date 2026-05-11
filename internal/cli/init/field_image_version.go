package initcli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// imageVersionField is a custom huh.Field that combines curated
// image-tag suggestions with a trailing free-text input row, all on a
// single screen. When the cursor sits on any suggestion row, Enter
// commits that exact tag; when it sits on the bottom input row, the
// row becomes editable and Enter commits whatever the user typed
// (after the same format validator validateImage uses server-side).
//
// huh's stock Select rows are immutable Option labels, so this shape
// isn't expressible by composing Select + Input — the field implements
// huh.Field directly. The bespoke code is worth the UX win: no second
// screen, no always-visible Input below the list, no "Other" sentinel
// to thread through promptForMissing.
type imageVersionField struct {
	title       string
	description string
	suggestions []string
	otherLabel  string

	target *string
	err    error

	cursor   int // 0..len(suggestions): suggestions; len(suggestions) = input row
	input    textinput.Model
	focused  bool
	validate func(string) error

	theme  *huh.Theme
	keymap huh.SelectKeyMap
	width  int
	height int
	access bool
}

// newImageVersionField builds a Bubble Tea / huh field that presents
// suggestions as a select list with an editable trailing input row.
// otherLabel is the placeholder text for the input row when the cursor
// is parked elsewhere.
func newImageVersionField(target *string, suggestions []string, otherLabel string) *imageVersionField {
	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 128

	f := &imageVersionField{
		title:       "",
		description: "",
		suggestions: suggestions,
		otherLabel:  otherLabel,
		target:      target,
		err:         nil,
		cursor:      0,
		input:       ti,
		focused:     false,
		validate:    func(string) error { return nil },
		theme:       nil,
		keymap:      huh.SelectKeyMap{}, //nolint:exhaustruct // populated by WithKeyMap from huh.Form.
		width:       0,
		height:      0,
		access:      false,
	}

	// Preset cursor + input value from any pre-filled *target. Matching
	// a suggestion lands the cursor on that row; an off-whitelist tag
	// jumps to the input row with the value pre-typed.
	initial := ""
	if target != nil {
		initial = *target
	}
	if initial == "" {
		return f
	}
	for i, v := range suggestions {
		if v == initial {
			f.cursor = i
			return f
		}
	}
	f.cursor = len(suggestions)
	f.input.SetValue(initial)
	return f
}

// Title sets the title rendered above the option list.
func (f *imageVersionField) Title(s string) *imageVersionField { f.title = s; return f }

// Description sets the secondary line shown under the title.
func (f *imageVersionField) Description(s string) *imageVersionField {
	f.description = s
	return f
}

// Validate registers the validator run against the chosen value on
// Submit. The same regex validateImage uses server-side is the typical
// argument.
func (f *imageVersionField) Validate(fn func(string) error) *imageVersionField {
	f.validate = fn
	return f
}

// ---- Bubble Tea / huh.Field implementation ----

// Init satisfies tea.Model. Returning textinput.Blink keeps the caret
// animating from frame one when the cursor starts on the input row.
func (*imageVersionField) Init() tea.Cmd { return textinput.Blink }

// Update routes keypresses: navigation keys move the cursor between
// suggestion rows and the input row; typing on the input row falls
// through to textinput; Submit commits via commit().
func (f *imageVersionField) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keymsg, ok := msg.(tea.KeyMsg)
	if !ok {
		// Forward non-key messages (e.g. cursor blink) to textinput only
		// while the input row is active, so its caret keeps blinking.
		if f.cursor == len(f.suggestions) {
			var cmd tea.Cmd
			f.input, cmd = f.input.Update(msg)
			return f, cmd
		}
		return f, nil
	}
	f.err = nil
	if cmd, handled := f.handleNav(keymsg); handled {
		return f, cmd
	}
	if cmd, handled := f.handleSubmit(keymsg); handled {
		return f, cmd
	}
	// Fall through: on input row, forward unhandled keys to textinput.
	if f.cursor == len(f.suggestions) {
		var cmd tea.Cmd
		f.input, cmd = f.input.Update(msg)
		return f, cmd
	}
	return f, nil
}

func (f *imageVersionField) handleNav(msg tea.KeyMsg) (tea.Cmd, bool) {
	onInput := f.cursor == len(f.suggestions)
	switch {
	case key.Matches(msg, f.keymap.Up):
		if f.cursor > 0 {
			f.cursor--
		}
		if onInput && f.cursor != len(f.suggestions) {
			f.input.Blur()
		}
		return nil, true
	case key.Matches(msg, f.keymap.Down):
		if f.cursor < len(f.suggestions) {
			f.cursor++
		}
		if !onInput && f.cursor == len(f.suggestions) {
			return f.input.Focus(), true
		}
		return nil, true
	case key.Matches(msg, f.keymap.GotoTop):
		f.cursor = 0
		if onInput {
			f.input.Blur()
		}
		return nil, true
	case key.Matches(msg, f.keymap.GotoBottom):
		f.cursor = len(f.suggestions)
		return f.input.Focus(), true
	}
	return nil, false
}

func (f *imageVersionField) handleSubmit(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch {
	case key.Matches(msg, f.keymap.Prev):
		return huh.PrevField, true
	case key.Matches(msg, f.keymap.Next, f.keymap.Submit):
		return f.commit(), true
	}
	return nil, false
}

func (f *imageVersionField) commit() tea.Cmd {
	var val string
	if f.cursor == len(f.suggestions) {
		val = strings.TrimSpace(f.input.Value())
	} else {
		val = f.suggestions[f.cursor]
	}
	if err := f.validate(val); err != nil {
		f.err = err
		return nil
	}
	if f.target != nil {
		*f.target = val
	}
	return huh.NextField
}

func (f *imageVersionField) activeStyles() *huh.FieldStyles {
	theme := f.theme
	if theme == nil {
		theme = huh.ThemeCharm()
	}
	if f.focused {
		return &theme.Focused
	}
	return &theme.Blurred
}

// View renders the title, description, suggestion list, and inline
// input row. The input row falls back to a placeholder line when the
// cursor is elsewhere so users can still see the "type any tag"
// affordance while browsing.
func (f *imageVersionField) View() string {
	styles := f.activeStyles()
	maxW := f.width - styles.Base.GetHorizontalFrameSize()

	var sb strings.Builder
	if f.title != "" {
		sb.WriteString(styles.Title.Render(f.title))
		sb.WriteString("\n")
	}
	if f.description != "" {
		sb.WriteString(styles.Description.Render(f.description))
		sb.WriteString("\n")
	}

	cursorIndicator := styles.SelectSelector.String()
	cw := lipgloss.Width(cursorIndicator)
	pad := strings.Repeat(" ", cw)

	for i, s := range f.suggestions {
		if f.cursor == i {
			sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Left, cursorIndicator, styles.SelectedOption.Render(s)))
		} else {
			sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Left, pad, styles.UnselectedOption.Render(s)))
		}
		sb.WriteString("\n")
	}

	if f.cursor == len(f.suggestions) {
		f.input.PromptStyle = styles.TextInput.Prompt
		f.input.Cursor.Style = styles.TextInput.Cursor
		f.input.Cursor.TextStyle = styles.TextInput.CursorText
		f.input.TextStyle = styles.TextInput.Text
		f.input.PlaceholderStyle = styles.TextInput.Placeholder
		f.input.Placeholder = f.otherLabel
		w := maxW - cw - 1
		if w < 8 {
			w = 8
		}
		f.input.Width = w
		sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Left, cursorIndicator, f.input.View()))
	} else {
		sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Left, pad, styles.UnselectedOption.Render(f.otherLabel)))
	}

	if f.err != nil {
		sb.WriteString("\n")
		sb.WriteString(styles.ErrorMessage.Render("✕ " + f.err.Error()))
	}

	return styles.Base.
		Width(f.width).
		Height(f.height).
		Render(sb.String())
}

// Blur removes focus from the field and the embedded textinput.
func (f *imageVersionField) Blur() tea.Cmd {
	f.focused = false
	f.input.Blur()
	return nil
}

// Focus marks the field active; when the cursor is on the input row
// the embedded textinput is also focused so the caret renders.
func (f *imageVersionField) Focus() tea.Cmd {
	f.focused = true
	if f.cursor == len(f.suggestions) {
		return f.input.Focus()
	}
	return nil
}

// Error returns the most recent validation error, if any.
func (f *imageVersionField) Error() error { return f.err }

// Skip reports whether huh should skip this field (always false).
func (*imageVersionField) Skip() bool { return false }

// Zoom reports whether huh should give the field exclusive screen
// height (always false).
func (*imageVersionField) Zoom() bool { return false }

// KeyBinds returns the bindings huh shows in the help bar. Prev is
// intentionally omitted: cocoon runs each prompt as its own
// single-field huh.Form (see initLong), so Shift+Tab has no previous
// field to land on and advertising "back" in the help row would be a
// lie. The Prev keymap is still wired through Update so the binding
// itself stays consistent with the rest of huh, just not listed.
func (f *imageVersionField) KeyBinds() []key.Binding {
	return []key.Binding{f.keymap.Up, f.keymap.Down, f.keymap.Submit}
}

// WithTheme injects huh's theme so the field renders styles in sync
// with the rest of the form.
func (f *imageVersionField) WithTheme(t *huh.Theme) huh.Field { f.theme = t; return f }

// WithAccessible toggles the accessibility (screen reader) mode.
func (f *imageVersionField) WithAccessible(b bool) huh.Field { f.access = b; return f }

// WithKeyMap pulls the navigation keymap from huh's Select profile so
// Up/Down/Submit/Prev match every other field in the form.
func (f *imageVersionField) WithKeyMap(k *huh.KeyMap) huh.Field { f.keymap = k.Select; return f }

// WithWidth sets the rendered width (forwarded from huh.Form layout).
func (f *imageVersionField) WithWidth(w int) huh.Field { f.width = w; return f }

// WithHeight sets the rendered height (forwarded from huh.Form layout).
func (f *imageVersionField) WithHeight(h int) huh.Field { f.height = h; return f }

// WithPosition accepts huh's position info; this field's layout is
// independent of group position, so the input is ignored. The receiver
// is kept named so the method can still return the same pointer for
// huh's fluent chaining contract.
func (f *imageVersionField) WithPosition(_ huh.FieldPosition) huh.Field {
	return f
}

// GetKey returns the catalog key used to look the field's value up
// after the form exits.
func (*imageVersionField) GetKey() string { return "image_version" }

// GetValue returns the field's committed value as an any.
func (f *imageVersionField) GetValue() any {
	if f.target == nil {
		return ""
	}
	return *f.target
}

// Run renders the field as a standalone single-field form. cocoon's
// promptForMissing always wraps it in runSingleFieldForm, but this
// keeps the type compatible with `(huh.Field).Run()` in case it's
// reused outside the package.
func (f *imageVersionField) Run() error {
	if f.access {
		return f.RunAccessible(os.Stdout, os.Stdin)
	}
	if err := huh.NewForm(huh.NewGroup(f)).Run(); err != nil {
		return fmt.Errorf("imageVersionField form: %w", err)
	}
	return nil
}

// RunAccessible is the screen-reader fallback. Suggestions are listed
// numbered; the user may answer with the number or type a verbatim
// tag. The same validator the Bubble Tea UI runs gates manual entries.
func (f *imageVersionField) RunAccessible(w io.Writer, r io.Reader) error {
	f.printAccessibleHeader(w)
	for {
		fmt.Fprint(w, "Choose by number or type a tag: ")
		var choice string
		_, err := fmt.Fscanln(r, &choice)
		// EOF (or any persistent read error) means the reader is closed —
		// keep looping in that state would spin forever on the empty-input
		// branch below, so surface it as a wrapped error and let the
		// caller decide. "unexpected newline" (blank line on a live tty)
		// returns n=0 with a non-EOF error and falls through to the
		// empty-input retry, which is the right behavior.
		if err != nil && errors.Is(err, io.EOF) {
			return fmt.Errorf("imageVersionField: stdin closed before answer: %w", err)
		}
		choice = strings.TrimSpace(choice)
		if choice == "" {
			fmt.Fprintln(w, "empty input not accepted")
			continue
		}
		if val, ok := f.tryIndex(choice); ok {
			if err := f.validate(val); err != nil {
				return err
			}
			f.assignTarget(val)
			return nil
		}
		if err := f.validate(choice); err != nil {
			fmt.Fprintln(w, err.Error())
			continue
		}
		f.assignTarget(choice)
		return nil
	}
}

func (f *imageVersionField) printAccessibleHeader(w io.Writer) {
	if f.title != "" {
		fmt.Fprintln(w, f.title)
	}
	if f.description != "" {
		fmt.Fprintln(w, f.description)
	}
	for i, s := range f.suggestions {
		fmt.Fprintf(w, "%d. %s\n", i+1, s)
	}
	fmt.Fprintln(w, "Or type any tag directly.")
}

// tryIndex returns the suggestion matched by a single-/two-digit
// numeric answer, or (_, false) when the answer isn't an in-range
// index.
func (f *imageVersionField) tryIndex(choice string) (string, bool) {
	if len(choice) == 0 || len(choice) > 2 {
		return "", false
	}
	var n int
	if _, err := fmt.Sscanf(choice, "%d", &n); err != nil {
		return "", false
	}
	if n < 1 || n > len(f.suggestions) {
		return "", false
	}
	return f.suggestions[n-1], true
}

func (f *imageVersionField) assignTarget(v string) {
	if f.target != nil {
		*f.target = v
	}
}

// Compile-time assertion that imageVersionField fully satisfies huh.Field.
var _ huh.Field = (*imageVersionField)(nil)
