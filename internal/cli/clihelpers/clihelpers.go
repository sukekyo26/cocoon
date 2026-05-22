// Package clihelpers exposes small cobra utilities shared across the
// internal/cli/<sub> packages.
package clihelpers

import (
	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/i18n"
)

// AttachHelpAlias adds a hidden ` + "`help`" + ` subcommand whose RunE prints the
// parent command's usage. This preserves the legacy behaviour where every
// subcommand accepted ` + "`<sub> help`" + ` (positional) as a synonym for
// ` + "`<sub> --help`" + `.
//
// Idempotent: calling it twice is a no-op.
func AttachHelpAlias(cmd *cobra.Command) {
	for _, c := range cmd.Commands() {
		if c.Name() == "help" {
			return
		}
	}
	cat := i18n.New(i18n.Detect())
	cmd.AddCommand(&cobra.Command{
		Use:           "help",
		Short:         cat.Msg("cmd_help_alias_short"),
		Hidden:        true,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(c *cobra.Command, _ []string) error {
			if parent := c.Parent(); parent != nil {
				return parent.Help() //nolint:wrapcheck // help write error is descriptive
			}
			return c.Help() //nolint:wrapcheck // help write error is descriptive
		},
	})
}
