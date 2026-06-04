package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	gencli "github.com/sukekyo26/cocoon/internal/cli/gen"
	initcli "github.com/sukekyo26/cocoon/internal/cli/init"
	lockcli "github.com/sukekyo26/cocoon/internal/cli/lock"
	plugincli "github.com/sukekyo26/cocoon/internal/cli/plugin"
	selfupdatecli "github.com/sukekyo26/cocoon/internal/cli/selfupdate"
	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/logx"
	"github.com/sukekyo26/cocoon/internal/updatecheck"
)

// updateCheckTimeout caps the network call in PersistentPreRun so an
// unreachable GitHub does not stall every invocation for release.DefaultTimeout (30s).
const updateCheckTimeout = 2 * time.Second

// newRootCommand builds a fresh tree per call so concurrent uses (parallel
// tests) are safe.
func newRootCommand(version string, stdout, stderr io.Writer) *cobra.Command {
	cat := i18n.New(i18n.Detect())
	root := &cobra.Command{
		Use:           "cocoon",
		Short:         cat.Msg("cmd_root_short"),
		Long:          cat.Msg("cmd_root_long"),
		Version:       version,
		Args:          cobra.ArbitraryArgs, // RunE handles unknown args explicitly.
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			maybeNotifyUpdate(cmd, version, stderr)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help() //nolint:wrapcheck // top-level help write error is descriptive
			}
			return fmt.Errorf("%w: %q (try `cocoon help`)", ErrUnknownCommand, args[0])
		},
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.AddCommand(
		initcli.NewCommand(stdout, stderr),
		gencli.NewCommand(stdout, stderr),
		lockcli.NewCommand(stdout, stderr),
		selfupdatecli.NewCommand(stdout, stderr),
		plugincli.NewCommand(stdout, stderr),
		newVersionSubcommand(version, stdout, cat),
	)
	addLeafHelpAlias(root)
	// Eagerly materialize cobra's auto-registered `completion` and `help`
	// subtrees so ApplyI18nHelp can recurse through them. Cobra normally
	// registers both lazily during Execute(); without these calls,
	// localization would race the lazy add and leave them in cobra's
	// English defaults.
	root.InitDefaultCompletionCmd()
	root.InitDefaultHelpCmd()
	clihelpers.ApplyI18nHelp(root, cat)
	clihelpers.LocalizeAutoCompletion(root, cat)
	clihelpers.LocalizeAutoHelpCmd(root, cat)
	return root
}

// addLeafHelpAlias is a catch-all pass that decorates every leaf command
// with a hidden `help` subcommand, in case a future addition forgets to
// call AttachHelpAlias itself.
func addLeafHelpAlias(c *cobra.Command) {
	for _, child := range c.Commands() {
		if child.Name() == "help" || child.Hidden {
			continue
		}
		if child.HasSubCommands() {
			addLeafHelpAlias(child)
			continue
		}
		clihelpers.AttachHelpAlias(child)
	}
}

// maybeNotifyUpdate is silent on failure so the notifier never interferes
// with the user's invocation. currentVersion is the same string passed to
// newRootCommand so the notice matches the running binary in embedded /
// test contexts where a custom version is injected.
func maybeNotifyUpdate(cmd *cobra.Command, currentVersion string, stderr io.Writer) {
	if shouldSkipUpdateCheck(cmd, stderr) {
		return
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), updateCheckTimeout)
	defer cancel()
	notice := updatecheck.Check(ctx, currentVersion, updatecheck.Options{
		Now:        nil,
		CacheDir:   "",
		HTTPClient: nil,
	})
	if notice == nil {
		return
	}
	logx.New(io.Discard, stderr).Notice(notice.Format())
}

func shouldSkipUpdateCheck(cmd *cobra.Command, stderr io.Writer) bool {
	if os.Getenv("COCOON_NO_UPDATE_CHECK") == "1" {
		return true
	}
	switch cmd.Name() {
	case "version", "self-update", "help":
		return true
	}
	// `cocoon --version` runs the root command so the switch above misses
	// it; cobra auto-registers `version` as a flag.
	if vf := cmd.Flags().Lookup("version"); vf != nil && vf.Changed {
		return true
	}
	// Skip when stderr is not a tty (pipe / file / CI log / Buffer) so the
	// notice does not become noise.
	f, ok := stderr.(*os.File)
	if !ok {
		return true
	}
	return !isatty.IsTerminal(f.Fd())
}

// newVersionSubcommand handles the positional `cocoon version` form;
// `--version` / `-v` flag forms are wired via SetVersionTemplate.
func newVersionSubcommand(version string, stdout io.Writer, cat *i18n.Catalog) *cobra.Command {
	return &cobra.Command{
		Use:           "version",
		Short:         cat.Msg("cmd_version_short"),
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			_, err := io.WriteString(stdout, version+"\n")
			if err != nil {
				return err //nolint:wrapcheck // top-level write error is already descriptive
			}
			return nil
		},
	}
}
