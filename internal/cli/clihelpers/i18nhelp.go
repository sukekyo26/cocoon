package clihelpers

import (
	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/i18n"
)

// ApplyI18nHelp installs localized help / usage / version templates on
// cmd and every descendant, and rewrites the cobra-managed `--help` and
// `--version` flag usage strings. Idempotent; safe to call on the root
// only since the templates inherit through cobra's parent chain — the
// recursion is defensive so future code that calls SetHelpTemplate on a
// subcommand cannot accidentally bypass localization.
func ApplyI18nHelp(cmd *cobra.Command, cat *i18n.Catalog) {
	cmd.SetHelpTemplate(cat.Msg("help_template_full"))
	cmd.SetUsageTemplate(cat.Msg("usage_template_full"))
	cmd.SetVersionTemplate(cat.Msg("version_template_full"))
	localizeBuiltinFlags(cmd, cat)
	for _, child := range cmd.Commands() {
		ApplyI18nHelp(child, cat)
	}
}

// localizeBuiltinFlags rewrites the cobra-managed `--help` / `--version`
// flag usage on cmd. cobra adds these lazily during Execute; the explicit
// Init* calls guarantee the flag exists at help-template time so Lookup
// does not return nil on a freshly built command. `--version` is created
// by cobra only when Version is set, so its rewrite is conditional.
func localizeBuiltinFlags(cmd *cobra.Command, cat *i18n.Catalog) {
	cmd.InitDefaultHelpFlag()
	cmd.InitDefaultVersionFlag()
	if f := cmd.Flags().Lookup("help"); f != nil {
		f.Usage = cat.Msg("flag_global_help_usage", cmd.Name())
	}
	if f := cmd.Flags().Lookup("version"); f != nil {
		f.Usage = cat.Msg("flag_global_version_usage", cmd.Name())
	}
}

// LocalizeAutoCompletion overrides the Short / Long doc on the
// cobra-managed `completion` subtree (auto-registered when
// DisableDefaultCmd is false). The subtree's behaviour and flag
// compatibility are preserved; only display text is touched.
func LocalizeAutoCompletion(root *cobra.Command, cat *i18n.Catalog) {
	completion, _, err := root.Find([]string{"completion"})
	if err != nil || completion == nil || completion.Name() != "completion" {
		return
	}
	completion.Short = cat.Msg("cmd_completion_short")
	completion.Long = cat.Msg("cmd_completion_long")
	for _, shell := range completion.Commands() {
		key := "cmd_completion_" + shell.Name()
		// Catalog.Msg falls back to the key itself when neither language
		// table has the entry; we treat that sentinel as "no translation
		// available" and leave cobra's English string in place.
		if short := cat.Msg(key + "_short"); short != key+"_short" {
			shell.Short = short
		}
		if long := cat.Msg(key + "_long"); long != key+"_long" {
			shell.Long = long
		}
	}
}

// LocalizeAutoHelpCmd overrides the Short / Long doc on the cobra-managed
// `help` subcommand (auto-registered by InitDefaultHelpCmd). Like
// LocalizeAutoCompletion it preserves cobra's behaviour and only touches
// the display text.
func LocalizeAutoHelpCmd(root *cobra.Command, cat *i18n.Catalog) {
	help, _, err := root.Find([]string{"help"})
	if err != nil || help == nil || help.Name() != "help" {
		return
	}
	help.Short = cat.Msg("cmd_help_subcommand_short")
	help.Long = cat.Msg("cmd_help_subcommand_long", root.Name())
}
