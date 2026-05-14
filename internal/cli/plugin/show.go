package plugincli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/logx"
)

const showLong = `cocoon plugin show — print the resolved plugin manifest for <id>

Resolves <id> through the same project > user > embedded layered view as
` + "`cocoon plugin list`" + `, prints the metadata, install hints, apt packages,
and the source layer it was read from.`

func newShowCmd(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "show <id>",
		Short:         "Print the resolved manifest for a single plugin",
		Long:          showLong,
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
		return fmt.Errorf("%w: plugin %q not found in any layer", ErrUsage, id)
	}
	p, err := loadPluginFromLayer(layered, id)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFailure, err)
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
		keys := make([]string, 0, len(p.Install.Env))
		for k := range p.Install.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		log.Info(log.Bold("env:"))
		for _, k := range keys {
			log.Infof("  %s=%s", k, p.Install.Env[k])
		}
	}
	return nil
}
