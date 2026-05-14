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

// selectOrInputField is a custom huh.Field with curated suggestions plus
// a trailing free-text input row on a single screen. huh's stock Select
// rows are immutable Option labels so this shape isn't expressible by
// composing Select + Input. Used for image-tag picking and plugin-version
// pinning (both fit "small curated set OR free text").
type selectOrInputField struct {
	key         string
	title       string
	description string
	urlLine     string
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

// newSelectOrInputField builds the field. otherLabel is the placeholder
// shown on the input row when the cursor is parked elsewhere.
func newSelectOrInputField(
	key string, target *string, suggestions []string, otherLabel string,
) *selectOrInputField {
	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 128

	f := &selectOrInputField{
		key:         key,
		title:       "",
		description: "",
		urlLine:     "",
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

	// Off-whitelist pre-fill lands on the input row with the value pre-typed.
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
func (f *selectOrInputField) Title(s string) *selectOrInputField { f.title = s; return f }

// Description sets the secondary line shown under the title.
func (f *selectOrInputField) Description(s string) *selectOrInputField {
	f.description = s
	return f
}

// URLLine sets an optional URL displayed under the description. The
// plugin-version prompt uses it to surface the upstream release page so
// the user can confirm a pin value before typing one.
func (f *selectOrInputField) URLLine(s string) *selectOrInputField {
	f.urlLine = s
	return f
}

// Validate registers the validator run against the chosen value on Submit.
func (f *selectOrInputField) Validate(fn func(string) error) *selectOrInputField {
	f.validate = fn
	return f
}

// Init keeps the caret animating from frame one when the cursor starts on
// the input row.
func (*selectOrInputField) Init() tea.Cmd { return textinput.Blink }

// Update routes keypresses and forwards stray messages to textinput.
func (f *selectOrInputField) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keymsg, ok := msg.(tea.KeyMsg)
	if !ok {
		// Forward non-key messages (e.g. cursor blink) to textinput only
		// while the input row is active so its caret keeps blinking.
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
	if f.cursor == len(f.suggestions) {
		var cmd tea.Cmd
		f.input, cmd = f.input.Update(msg)
		return f, cmd
	}
	return f, nil
}

func (f *selectOrInputField) handleNav(msg tea.KeyMsg) (tea.Cmd, bool) {
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

func (f *selectOrInputField) handleSubmit(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch {
	case key.Matches(msg, f.keymap.Prev):
		return huh.PrevField, true
	case key.Matches(msg, f.keymap.Next, f.keymap.Submit):
		return f.commit(), true
	}
	return nil, false
}

func (f *selectOrInputField) commit() tea.Cmd {
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

func (f *selectOrInputField) activeStyles() *huh.FieldStyles {
	theme := f.theme
	if theme == nil {
		theme = huh.ThemeCharm()
	}
	if f.focused {
		return &theme.Focused
	}
	return &theme.Blurred
}

// View shows the input row as a placeholder line when the cursor is
// elsewhere so the "type any tag" affordance stays visible while browsing.
func (f *selectOrInputField) View() string {
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
	if f.urlLine != "" {
		urlStyle := lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#0066CC", Dark: "#7FDBFF"}).
			Underline(true)
		sb.WriteString(urlStyle.Render(f.urlLine))
		sb.WriteString("\n\n")
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

// Blur removes focus.
func (f *selectOrInputField) Blur() tea.Cmd {
	f.focused = false
	f.input.Blur()
	return nil
}

// Focus also focuses the embedded textinput when the cursor is on the
// input row so the caret renders.
func (f *selectOrInputField) Focus() tea.Cmd {
	f.focused = true
	if f.cursor == len(f.suggestions) {
		return f.input.Focus()
	}
	return nil
}

// Error returns the most recent validation error.
func (f *selectOrInputField) Error() error { return f.err }

// Skip always returns false.
func (*selectOrInputField) Skip() bool { return false }

// Zoom always returns false.
func (*selectOrInputField) Zoom() bool { return false }

// KeyBinds omits Prev intentionally: each prompt is its own single-field
// form, so advertising "back" would be a lie. Prev is still wired through
// Update so the binding stays consistent with the rest of huh.
func (f *selectOrInputField) KeyBinds() []key.Binding {
	return []key.Binding{f.keymap.Up, f.keymap.Down, f.keymap.Submit}
}

// WithTheme injects huh's theme.
func (f *selectOrInputField) WithTheme(t *huh.Theme) huh.Field { f.theme = t; return f }

// WithAccessible toggles screen-reader mode.
func (f *selectOrInputField) WithAccessible(b bool) huh.Field { f.access = b; return f }

// WithKeyMap pulls navigation from huh's Select profile.
func (f *selectOrInputField) WithKeyMap(k *huh.KeyMap) huh.Field { f.keymap = k.Select; return f }

// WithWidth sets the rendered width.
func (f *selectOrInputField) WithWidth(w int) huh.Field { f.width = w; return f }

// WithHeight sets the rendered height.
func (f *selectOrInputField) WithHeight(h int) huh.Field { f.height = h; return f }

// WithPosition ignores huh's position info; this field's layout is
// independent of group position.
func (f *selectOrInputField) WithPosition(_ huh.FieldPosition) huh.Field {
	return f
}

// GetKey returns the field key.
func (f *selectOrInputField) GetKey() string { return f.key }

// GetValue returns the committed value.
func (f *selectOrInputField) GetValue() any {
	if f.target == nil {
		return ""
	}
	return *f.target
}

// Run keeps the type compatible with `(huh.Field).Run()` even though
// promptForMissing always wraps it in runSingleFieldForm.
func (f *selectOrInputField) Run() error {
	if f.access {
		return f.RunAccessible(os.Stdout, os.Stdin)
	}
	if err := huh.NewForm(huh.NewGroup(f)).Run(); err != nil {
		return fmt.Errorf("selectOrInputField form: %w", err)
	}
	return nil
}

// RunAccessible is the screen-reader fallback; users answer with the
// number or type a verbatim tag.
func (f *selectOrInputField) RunAccessible(w io.Writer, r io.Reader) error {
	f.printAccessibleHeader(w)
	for {
		fmt.Fprint(w, "Choose by number or type a tag: ")
		var choice string
		_, err := fmt.Fscanln(r, &choice)
		// EOF would spin the empty-input retry forever. "unexpected newline"
		// (blank line on a live tty) returns n=0 with a non-EOF error and
		// correctly falls through to the retry.
		if err != nil && errors.Is(err, io.EOF) {
			return fmt.Errorf("selectOrInputField: stdin closed before answer: %w", err)
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

func (f *selectOrInputField) printAccessibleHeader(w io.Writer) {
	if f.title != "" {
		fmt.Fprintln(w, f.title)
	}
	if f.description != "" {
		fmt.Fprintln(w, f.description)
	}
	if f.urlLine != "" {
		fmt.Fprintln(w, f.urlLine)
		fmt.Fprintln(w)
	}
	for i, s := range f.suggestions {
		fmt.Fprintf(w, "%d. %s\n", i+1, s)
	}
	fmt.Fprintln(w, "Or type any tag directly.")
}

// tryIndex maps a 1- or 2-digit numeric answer to the matching suggestion.
func (f *selectOrInputField) tryIndex(choice string) (string, bool) {
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

func (f *selectOrInputField) assignTarget(v string) {
	if f.target != nil {
		*f.target = v
	}
}

var _ huh.Field = (*selectOrInputField)(nil)
