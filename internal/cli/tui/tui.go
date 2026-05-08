// Package tuicli implements the `wsd tui` subcommand tree.
//
// The bash entry points (lib/tui.sh) shell out to these subcommands so that
// interactive widgets share a single Go implementation. Widget rendering is
// performed against /dev/tty by huh; only the final result (selected index
// or comma-separated indices) is written to stdout.
package tuicli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrUsage signals a usage error (missing argument, unknown flag) and maps
// to exit code 2 at the binary boundary.
var ErrUsage = errors.New("usage error")

// ErrCanceled signals the user aborted the prompt; maps to exit code 130
// (the conventional shell exit code for SIGINT).
var ErrCanceled = errors.New("canceled")

// ErrFailure signals a non-usage runtime failure (TUI library error) and
// maps to exit code 1.
var ErrFailure = errors.New("failure")

type parsedFlags struct {
	title       string
	preselected []int
	options     []string
	hasOptions  bool
}

func parseSelectFlags(args []string, allowPreselected bool) (parsedFlags, error) {
	var p parsedFlags
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--":
			p.options = append([]string{}, args[i+1:]...)
			p.hasOptions = true
			return p, nil
		case a == "--title":
			if i+1 >= len(args) {
				return p, fmt.Errorf("%w: --title requires a value", ErrUsage)
			}
			p.title = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--title="):
			p.title = strings.TrimPrefix(a, "--title=")
			i++
		case a == "--preselected" && allowPreselected:
			if i+1 >= len(args) {
				return p, fmt.Errorf("%w: --preselected requires a value", ErrUsage)
			}
			parsed, err := parseCSVIndices(args[i+1])
			if err != nil {
				return p, err
			}
			p.preselected = parsed
			i += 2
		case strings.HasPrefix(a, "--preselected=") && allowPreselected:
			parsed, err := parseCSVIndices(strings.TrimPrefix(a, "--preselected="))
			if err != nil {
				return p, err
			}
			p.preselected = parsed
			i++
		default:
			return p, fmt.Errorf("%w: unknown flag %q (use -- to terminate flags)", ErrUsage, a)
		}
	}
	return p, fmt.Errorf("%w: missing -- separator before option list", ErrUsage)
}

func parseCSVIndices(csv string) ([]int, error) {
	if csv == "" {
		return nil, nil
	}
	parts := strings.Split(csv, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		n, err := strconv.Atoi(trimmed)
		if err != nil {
			return nil, fmt.Errorf("%w: --preselected: %q is not an integer", ErrUsage, trimmed)
		}
		out = append(out, n)
	}
	return out, nil
}
