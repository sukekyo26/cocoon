// In-package test so that the unexported shouldSkipUpdateCheck helper
// can be exercised without exporting it.
package cli //nolint:testpackage // unexported skip-policy helper

import (
	"bytes"
	"os"
	"testing"

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
