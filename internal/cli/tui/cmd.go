package tuicli

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/tui"
)

const tuiLong = `wsd tui — interactive select widgets

Used by lib/tui.sh in the bash entry points so that interactive widgets share
a single Go implementation. Widget rendering is performed against /dev/tty
by huh; only the final result (selected index or comma-separated indices)
is written to stdout.`

// NewCommand returns the cobra subtree for ` + "`wsd tui`" + `.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	return NewCommandWithSelector(tui.HuhSelector{}, stdout, stderr)
}

// NewCommandWithSelector lets tests inject a fake [tui.Selector].
func NewCommandWithSelector(sel tui.Selector, stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "tui",
		Short:         "Interactive select widgets used by the bash entry point scripts",
		Long:          tuiLong,
		Args:          rejectUnknownSubcommand,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help() //nolint:wrapcheck // help write error is descriptive
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return fmt.Errorf("%w: %w", ErrUsage, err)
	})
	cmd.AddCommand(newSelectSingleCmd(sel, stdout, stderr))
	cmd.AddCommand(newSelectMultiCmd(sel, stdout, stderr))
	return cmd
}

// rejectUnknownSubcommand returns an ErrUsage-wrapped error when a stray
// positional appears under a parent that only carries subcommands.
func rejectUnknownSubcommand(_ *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	return fmt.Errorf("%w: unknown subcommand %q", ErrUsage, args[0])
}

// newSelectSingleCmd uses DisableFlagParsing because the existing protocol
// requires an explicit `--` separator before the option list.
func newSelectSingleCmd(sel tui.Selector, stdout, stderr io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:                "select-single --title TEXT -- opt1 opt2 ...",
		Short:              "Select one option (prints its index)",
		DisableFlagParsing: true,
		SilenceUsage:       true,
		SilenceErrors:      true,
		RunE: func(_ *cobra.Command, args []string) error {
			return runSelectSingle(args, sel, stdout, stderr)
		},
	}
}

func newSelectMultiCmd(sel tui.Selector, stdout, stderr io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:                "select-multi --title TEXT [--preselected CSV] -- opt1 opt2 ...",
		Short:              "Select zero+ options (prints CSV of indices)",
		DisableFlagParsing: true,
		SilenceUsage:       true,
		SilenceErrors:      true,
		RunE: func(_ *cobra.Command, args []string) error {
			return runSelectMulti(args, sel, stdout, stderr)
		},
	}
}

func runSelectSingle(args []string, sel tui.Selector, stdout, _ io.Writer) error {
	// Intercept `--help`/`-h` ourselves because DisableFlagParsing also
	// disables cobra's automatic help handling for this command.
	if isHelpArg(args) {
		return printSelectSingleHelp(stdout)
	}
	p, err := parseSelectFlags(args, false)
	if err != nil {
		return err
	}
	if len(p.options) == 0 {
		return fmt.Errorf("%w: at least one option is required", ErrUsage)
	}
	idx, err := sel.SelectSingle(p.title, p.options, 0)
	if err != nil {
		if errors.Is(err, tui.ErrCanceled) {
			return ErrCanceled
		}
		return fmt.Errorf("%w: %v", ErrFailure, err) //nolint:errorlint
	}
	// Selector output is a scripting-protocol boundary (`idx=$(wsd tui
	// select-single ...)`); a silently-dropped write would let callers act
	// on an empty index, so write directly and propagate the error.
	if _, err := fmt.Fprintln(stdout, idx); err != nil {
		return fmt.Errorf("write result: %w", err)
	}
	return nil
}

func runSelectMulti(args []string, sel tui.Selector, stdout, _ io.Writer) error {
	if isHelpArg(args) {
		return printSelectMultiHelp(stdout)
	}
	p, err := parseSelectFlags(args, true)
	if err != nil {
		return err
	}
	if len(p.options) == 0 {
		return fmt.Errorf("%w: at least one option is required", ErrUsage)
	}
	picked, err := sel.SelectMulti(p.title, p.options, p.preselected)
	if err != nil {
		if errors.Is(err, tui.ErrCanceled) {
			return ErrCanceled
		}
		return fmt.Errorf("%w: %v", ErrFailure, err) //nolint:errorlint
	}
	parts := make([]string, len(picked))
	for i, n := range picked {
		parts[i] = strconv.Itoa(n)
	}
	if _, err := fmt.Fprintln(stdout, strings.Join(parts, ",")); err != nil {
		return fmt.Errorf("write result: %w", err)
	}
	return nil
}

func isHelpArg(args []string) bool {
	for _, a := range args {
		switch a {
		case "help", "--help", "-h":
			return true
		case "--":
			return false
		}
	}
	return false
}

func printSelectSingleHelp(w io.Writer) error {
	const usage = `wsd tui select-single — select one option

Usage:
  wsd tui select-single --title TEXT -- opt1 opt2 ...
`
	if _, err := io.WriteString(w, usage); err != nil {
		return fmt.Errorf("write usage: %w", err)
	}
	return nil
}

func printSelectMultiHelp(w io.Writer) error {
	const usage = `wsd tui select-multi — select zero+ options

Usage:
  wsd tui select-multi --title TEXT [--preselected CSV] -- opt1 opt2 ...
`
	if _, err := io.WriteString(w, usage); err != nil {
		return fmt.Errorf("write usage: %w", err)
	}
	return nil
}
