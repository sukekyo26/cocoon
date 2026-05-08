// Package tui provides interactive select widgets used by the bash entry
// point scripts (setup-docker.sh, generate-workspace.sh, clean-docker.sh).
//
// Widgets are implemented on top of github.com/charmbracelet/huh and return
// the selected option index/indices so callers can treat the result as data
// (mirroring the legacy lib/tui.sh API).
package tui

import (
	"errors"
	"fmt"
	"sort"

	"github.com/charmbracelet/huh"
)

// ErrCanceled is returned when the user aborts the prompt (Ctrl-C / Esc).
var ErrCanceled = errors.New("selection canceled")

// ErrNoOptions is returned when SelectSingle/SelectMulti is called with an
// empty options slice.
var ErrNoOptions = errors.New("no options provided")

// Selector abstracts the underlying TUI implementation so that callers can
// inject a deterministic fake during tests.
//
// SelectSingle's defaultIdx sets the initial cursor / highlighted option. Pass
// 0 when there is no meaningful default. Out-of-range values are clamped to 0
// rather than rejected so callers can map "no previous value" to either 0 or
// -1 without an extra branch.
type Selector interface {
	SelectSingle(title string, options []string, defaultIdx int) (int, error)
	SelectMulti(title string, options []string, preselected []int) ([]int, error)
}

// HuhSelector is the production Selector backed by charmbracelet/huh.
// It is intentionally an empty struct so the zero value is comparable and
// usable as a map key. The test seam lives on the package-private
// huhSelect{Single,Multi}Impl helpers so this exported type stays stable.
type HuhSelector struct{}

// SelectSingle prompts the user to pick exactly one option and returns its
// zero-based index. defaultIdx sets the initial cursor position; values
// outside [0, len(options)) are clamped to 0. It returns ErrCanceled when the
// user aborts.
func (HuhSelector) SelectSingle(title string, options []string, defaultIdx int) (int, error) {
	return huhSelectSingleImpl(title, options, defaultIdx, nil)
}

// huhSelectSingleImpl is the package-private implementation of SelectSingle.
// runForm is an optional test seam: when nil it dispatches to (*huh.Form).Run
// (production), otherwise it dispatches to the supplied function.
func huhSelectSingleImpl(
	title string, options []string, defaultIdx int, runForm func(*huh.Form) error,
) (int, error) {
	if len(options) == 0 {
		return 0, fmt.Errorf("select-single: %w", ErrNoOptions)
	}
	opts := make([]huh.Option[int], len(options))
	for i, label := range options {
		opts[i] = huh.NewOption(label, i)
	}
	picked := defaultIdx
	if picked < 0 || picked >= len(options) {
		picked = 0
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[int]().
				Title(title).
				Options(opts...).
				Value(&picked),
		),
	)
	if runForm == nil {
		runForm = func(f *huh.Form) error { return f.Run() } //nolint:wrapcheck // wrapped by caller below
	}
	if err := runForm(form); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return 0, ErrCanceled
		}
		return 0, fmt.Errorf("select-single: %w", err)
	}
	return picked, nil
}

// SelectMulti prompts for any number of options (including zero) and returns
// the selected indices in ascending order. preselected indices are checked
// when the form opens.
//
// Preselected indices are passed to huh exclusively via Value() rather than
// huh.Option.Selected(true). With Selected(true) the huh MultiSelect snaps the
// cursor and viewport to the first preselected option (see huh v1.0.0
// field_multiselect.go selectOptions), which hides every option above it on
// initial render. Binding through Value() marks the same options as checked
// without moving the cursor, so the list always opens at the first row.
func (HuhSelector) SelectMulti(title string, options []string, preselected []int) ([]int, error) {
	return huhSelectMultiImpl(title, options, preselected, nil)
}

// huhSelectMultiImpl is the package-private implementation of SelectMulti.
// See huhSelectSingleImpl for the runForm seam contract.
func huhSelectMultiImpl(
	title string, options []string, preselected []int, runForm func(*huh.Form) error,
) ([]int, error) {
	if len(options) == 0 {
		return nil, fmt.Errorf("select-multi: %w", ErrNoOptions)
	}
	opts := make([]huh.Option[int], len(options))
	for i, label := range options {
		opts[i] = huh.NewOption(label, i)
	}
	picked := filterPreset(preselected, len(options))
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[int]().
				Title(title).
				Options(opts...).
				Value(&picked),
		),
	)
	if runForm == nil {
		runForm = func(f *huh.Form) error { return f.Run() } //nolint:wrapcheck // wrapped by caller below
	}
	if err := runForm(form); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil, ErrCanceled
		}
		return nil, fmt.Errorf("select-multi: %w", err)
	}
	sort.Ints(picked)
	return picked, nil
}

// filterPreset returns the indices in preselected that fall within [0, n).
// Out-of-range entries are silently dropped, matching the huh.MultiSelect
// contract that preselected values must reference an existing option.
func filterPreset(preselected []int, n int) []int {
	out := make([]int, 0, len(preselected))
	for _, idx := range preselected {
		if idx >= 0 && idx < n {
			out = append(out, idx)
		}
	}
	return out
}
