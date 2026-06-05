package plugincli

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

func newListCmd(stdout, stderr io.Writer) *cobra.Command {
	cat := i18n.New(i18n.Detect())
	var sourceFilter string
	cmd := &cobra.Command{
		Use:           "list",
		Short:         cat.Msg("cmd_plugin_list_short"),
		Long:          cat.Msg("cmd_plugin_list_long"),
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runList(stdout, stderr, sourceFilter)
		},
	}
	cmd.Flags().StringVar(&sourceFilter, "source", "",
		cat.Msg("flag_plugin_list_source_usage",
			plugin.SourceEmbedded, plugin.SourceUser, plugin.SourceProject))
	return cmd
}

func runList(stdout, _ io.Writer, sourceFilter string) error {
	cat := i18n.New(i18n.Detect())
	if sourceFilter != "" &&
		sourceFilter != plugin.SourceEmbedded &&
		sourceFilter != plugin.SourceUser &&
		sourceFilter != plugin.SourceProject {
		return clihelpers.UsageErr("err_pluginlist_invalid_source",
			plugin.SourceEmbedded, plugin.SourceUser, plugin.SourceProject)
	}

	layered, err := resolveLayered()
	if err != nil {
		return err
	}
	sources := layered.Sources()
	ids := slices.Sorted(maps.Keys(sources))

	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, cat.Msg("plugin_list_header"))
	for _, id := range ids {
		src := sources[id]
		if sourceFilter != "" && src != sourceFilter {
			continue
		}
		p, lerr := loadPluginFromLayer(layered, id)
		if lerr != nil {
			fmt.Fprintf(tw, "%s\t%s\t?\t%s\t\n", id, src, cat.Msg("plugin_list_load_failed", lerr))
			continue
		}
		def := cat.Msg("plugin_list_no")
		if p.Metadata.Default {
			def = cat.Msg("plugin_list_yes")
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", id, src, def, p.Metadata.Description, p.Metadata.URL)
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("flush list: %w", err)
	}
	return nil
}
