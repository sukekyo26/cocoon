package clihelpers_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
)

func newRoot() *cobra.Command {
	root := &cobra.Command{Use: "root", Short: "root cmd"}
	sub := &cobra.Command{Use: "sub", Short: "sub cmd"}
	root.AddCommand(sub)
	return root
}

func TestAttachHelpAliasAddsHiddenCommand(t *testing.T) {
	t.Parallel()
	root := newRoot()
	clihelpers.AttachHelpAlias(root.Commands()[0])

	helpCmd, _, err := root.Find([]string{"sub", "help"})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if helpCmd.Name() != "help" {
		t.Errorf("expected help cmd, got %q", helpCmd.Name())
	}
	if !helpCmd.Hidden {
		t.Errorf("help alias should be hidden")
	}
}

func TestAttachHelpAliasIsIdempotent(t *testing.T) {
	t.Parallel()
	root := newRoot()
	sub := root.Commands()[0]
	clihelpers.AttachHelpAlias(sub)
	clihelpers.AttachHelpAlias(sub)

	helpCount := 0
	for _, c := range sub.Commands() {
		if c.Name() == "help" {
			helpCount++
		}
	}
	if helpCount != 1 {
		t.Errorf("expected 1 help alias after double-attach, got %d", helpCount)
	}
}

func TestAttachHelpAliasRunPrintsParentUsage(t *testing.T) {
	t.Parallel()
	root := newRoot()
	sub := root.Commands()[0]
	clihelpers.AttachHelpAlias(sub)

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"sub", "help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(buf.String(), "sub cmd") {
		t.Errorf("expected parent short text in help output:\n%s", buf.String())
	}
}

func TestAttachHelpAliasRunOnRootlessCmd(t *testing.T) {
	t.Parallel()
	// Detached cobra command (no parent): the alias should fall back to its
	// own Help() rather than panic.
	cmd := &cobra.Command{Use: "lonely", Short: "lonely cmd"}
	clihelpers.AttachHelpAlias(cmd)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(buf.String(), "lonely") {
		t.Errorf("expected own usage when no parent:\n%s", buf.String())
	}
}
