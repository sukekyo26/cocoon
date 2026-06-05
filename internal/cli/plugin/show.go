package plugincli

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/logx"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

func newShowCmd(stdout, stderr io.Writer) *cobra.Command {
	cat := i18n.New(i18n.Detect())
	cmd := &cobra.Command{
		Use:           "show <id>",
		Short:         cat.Msg("cmd_plugin_show_short"),
		Long:          cat.Msg("cmd_plugin_show_long"),
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, args []string) error {
			return runShow(stdout, stderr, args[0])
		},
	}
	return cmd
}

func runShow(stdout, stderr io.Writer, id string) error {
	layered, err := resolveLayered()
	if err != nil {
		return err
	}
	src := layered.Source(id)
	if src == "" {
		return clihelpers.UsageErr("err_pluginshow_not_found", id)
	}
	p, err := loadPluginFromLayer(layered, id)
	if err != nil {
		return clihelpers.FailureWrap(err, "")
	}

	log := logx.New(stdout, stderr)
	// Label widths are pre-padded so colored bold sits flush against the
	// value with no extra separator; matches the legacy %s%s layout.
	row := func(label, value string) { log.Infof("%s%s", log.Bold(label), value) }
	row("id:           ", id)
	row("source:       ", src)
	row("name:         ", p.Metadata.Name)
	row("description:  ", p.Metadata.Description)
	row("url:          ", p.Metadata.URL)
	row("default:      ", fmt.Sprintf("%t", p.Metadata.Default))
	if len(p.Metadata.Conflicts) > 0 {
		row("conflicts:    ", strings.Join(p.Metadata.Conflicts, ", "))
	}
	row("requires_root: ", fmt.Sprintf("%t", p.Install.RequiresRoot))
	row("version_capable: ", fmt.Sprintf("%t", p.Version.VersionCapable))
	if p.Version.VersionCapable {
		verify := p.Version.Verify
		if verify == "" {
			verify = plugin.VerifyChecksum
		}
		row("verify:       ", verify)
	}
	if p.Apt != nil && len(p.Apt.Packages) > 0 {
		pkgs := append([]string(nil), p.Apt.Packages...)
		sort.Strings(pkgs)
		row("apt_packages: ", strings.Join(pkgs, ", "))
	}
	if len(p.Install.BuildArgs) > 0 {
		row("build_args:   ", strings.Join(p.Install.BuildArgs, ", "))
	}
	if len(p.Install.Volumes) > 0 {
		row("volumes:      ", strings.Join(p.Install.Volumes, ", "))
	}

	if len(p.Install.Env) > 0 {
		keys := slices.Sorted(maps.Keys(p.Install.Env))
		log.Info(log.Bold("env:"))
		for _, k := range keys {
			log.Infof("  %s=%s", k, p.Install.Env[k])
		}
	}
	return nil
}
