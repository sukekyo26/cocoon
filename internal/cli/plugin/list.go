package plugincli

import (
	"fmt"
	"io"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/plugin"
)

const listLong = `cocoon plugin list — show every available plugin

The list combines the embedded catalog with optional user (` + "`~/.cocoon/plugins`" + `)
and project (` + "`<project>/.cocoon/plugins`" + `) overlays. Same-id directories are
not merged; the highest-priority layer wins (project > user > embedded). The
SOURCE column shows which layer each id is currently served from.`

func newListCmd(stdout, stderr io.Writer) *cobra.Command {
	var sourceFilter string
	cmd := &cobra.Command{
		Use:           "list",
		Short:         "List available plugins with their source (embedded / user / project)",
		Long:          listLong,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runList(stdout, stderr, sourceFilter)
		},
	}
	cmd.Flags().StringVar(
		&sourceFilter,
		"source",
		"",
		`only show this layer (`+plugin.SourceEmbedded+`, `+plugin.SourceUser+`, `+plugin.SourceProject+`)`,
	)
	return cmd
}

func runList(stdout, _ io.Writer, sourceFilter string) error {
	if sourceFilter != "" &&
		sourceFilter != plugin.SourceEmbedded &&
		sourceFilter != plugin.SourceUser &&
		sourceFilter != plugin.SourceProject {
		return fmt.Errorf("%w: --source must be one of %q / %q / %q",
			ErrUsage, plugin.SourceEmbedded, plugin.SourceUser, plugin.SourceProject)
	}

	layered, err := resolveLayered()
	if err != nil {
		return err
	}
	sources := layered.Sources()
	ids := make([]string, 0, len(sources))
	for id := range sources {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSOURCE\tDEFAULT\tDESCRIPTION")
	for _, id := range ids {
		src := sources[id]
		if sourceFilter != "" && src != sourceFilter {
			continue
		}
		p, lerr := loadPluginFromLayer(layered, id)
		if lerr != nil {
			fmt.Fprintf(tw, "%s\t%s\t?\t<load failed: %v>\n", id, src, lerr)
			continue
		}
		def := "no"
		if p.Metadata.Default {
			def = "yes"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", id, src, def, p.Metadata.Description)
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("flush list: %w", err)
	}
	return nil
}
