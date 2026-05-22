package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/cli"
)

func TestExecuteVersion(t *testing.T) {
	t.Parallel()

	cases := []string{"version", "--version", "-v"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			app := cli.New("1.2.3", &stdout, &stderr)
			if err := app.Execute([]string{name}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := strings.TrimSpace(stdout.String()); got != "1.2.3" {
				t.Fatalf("stdout = %q, want %q", got, "1.2.3")
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

//nolint:paralleltest // t.Setenv pins the locale; the test must stay non-parallel.
func TestExecuteUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"help"}, {"--help"}, {"-h"}} {
		t.Run(strings.Join(append([]string{"args"}, args...), "_"), func(t *testing.T) {
			// Pin English so the banner check is stable on hosts where
			// LANG / LC_ALL select a non-English catalog.
			for _, k := range []string{"WORKSPACE_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
				t.Setenv(k, "")
			}
			t.Setenv("WORKSPACE_LANG", "en")
			var stdout, stderr bytes.Buffer
			app := cli.New("dev", &stdout, &stderr)
			if err := app.Execute(args); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(stdout.String(), "cocoon — project-aware container workspace generator") {
				t.Fatalf("usage banner missing in stdout: %q", stdout.String())
			}
		})
	}
}

func TestExecuteUnknownCommand(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	app := cli.New("dev", &stdout, &stderr)
	err := app.Execute([]string{"bogus"})
	if err == nil {
		t.Fatal("expected error for unknown command, got nil")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("unexpected error text: %v", err)
	}
}

// TestExecuteSubcommandDispatch routes `help` into every subcommand so the
// switch arms in (*App).Execute are all exercised. Subcommand-specific
// behaviour is owned by their own _test.go files; here we only verify that
// the dispatch wiring reaches each one.
func TestExecuteSubcommandDispatch(t *testing.T) {
	t.Parallel()
	subs := []string{
		// Generator commands.
		"init", "gen", "self-update",
		// Noun groups
		"plugin",
	}
	for _, sub := range subs {
		t.Run(sub, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			app := cli.New("dev", &stdout, &stderr)
			// `help` is accepted by every subcommand and returns nil.
			err := app.Execute([]string{sub, "help"})
			if err != nil {
				// Some subcommands (e.g. verify-image) require positional
				// args even for help-like behaviour; they may return a
				// usage error. We still consider the dispatch covered.
				if !strings.Contains(err.Error(), "usage") &&
					!strings.Contains(err.Error(), "Usage") {
					t.Errorf("unexpected error from `%s help`: %v", sub, err)
				}
			}
		})
	}
}
