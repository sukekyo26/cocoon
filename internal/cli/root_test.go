// In-package test so that the unexported shouldSkipUpdateCheck helper
// can be exercised without exporting it.
package cli //nolint:testpackage // unexported skip-policy helper

import (
	"bytes"
	"os"
	"testing"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

func TestShouldSkipUpdateCheck_EnvOptOut(t *testing.T) { //nolint:paralleltest // t.Setenv
	t.Setenv("COCOON_NO_UPDATE_CHECK", "1")
	cmd := &cobra.Command{Use: "gen"}
	if !shouldSkipUpdateCheck(cmd, os.Stderr) {
		t.Error("expected skip when COCOON_NO_UPDATE_CHECK=1")
	}
}

func TestShouldSkipUpdateCheck_BuiltinSubcommands(t *testing.T) { //nolint:paralleltest // t.Setenv
	t.Setenv("COCOON_NO_UPDATE_CHECK", "")
	for _, name := range []string{"version", "self-update", "help"} { //nolint:paralleltest // t.Setenv in parent
		t.Run(name, func(t *testing.T) { //nolint:paralleltest // shares parent env
			cmd := &cobra.Command{Use: name}
			if !shouldSkipUpdateCheck(cmd, os.Stderr) {
				t.Errorf("%s: expected skip", name)
			}
		})
	}
}

func TestShouldSkipUpdateCheck_NonTTYStderr(t *testing.T) { //nolint:paralleltest // t.Setenv
	t.Setenv("COCOON_NO_UPDATE_CHECK", "")
	cmd := &cobra.Command{Use: "gen"}
	var buf bytes.Buffer
	if !shouldSkipUpdateCheck(cmd, &buf) {
		t.Error("expected skip for non-*os.File stderr")
	}
}

// `cocoon --version` parses on the root command (cmd.Name() == "cocoon")
// so the subcommand-name switch alone does not skip it. The flag-changed
// branch must catch it instead.
// `cocoon --version` parses on the root command (cmd.Name() == "cocoon")
// so the subcommand-name switch alone does not skip it. The flag-changed
// branch must catch it instead. InitDefaultVersionFlag mirrors what
// cobra runs internally when Execute encounters a command with Version
// set; ParseFlags alone would not register it.
func TestShouldSkipUpdateCheck_VersionFlag(t *testing.T) { //nolint:paralleltest // t.Setenv
	t.Setenv("COCOON_NO_UPDATE_CHECK", "")
	cmd := &cobra.Command{Use: "cocoon", Version: "9.9.9"}
	cmd.InitDefaultVersionFlag()
	if err := cmd.ParseFlags([]string{"--version"}); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	if !shouldSkipUpdateCheck(cmd, os.Stderr) {
		t.Error("expected skip when --version is set on the root command")
	}
}

// Sanity check: a command with the version flag registered but not
// toggled (plain `cocoon` invocation) must not get caught by the
// flag-changed branch — otherwise the notifier never fires at all.
func TestShouldSkipUpdateCheck_RootNoVersionFlag(t *testing.T) { //nolint:paralleltest // t.Setenv
	t.Setenv("COCOON_NO_UPDATE_CHECK", "")
	cmd := &cobra.Command{Use: "cocoon", Version: "9.9.9"}
	cmd.InitDefaultVersionFlag()
	if err := cmd.ParseFlags(nil); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	// stderr is *os.File here. Under a non-TTY (CI runs) the non-TTY
	// guard would short-circuit before reaching the flag-changed
	// branch, so only assert the flag branch when stderr is a TTY.
	if isatty.IsTerminal(os.Stderr.Fd()) && shouldSkipUpdateCheck(cmd, os.Stderr) {
		t.Error("expected no skip for plain `cocoon` without --version under a TTY")
	}
}
