package plugincli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
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

func runShow(stdout, _ io.Writer, id string) error {
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

	fmt.Fprintf(stdout, "id:           %s\n", id)
	fmt.Fprintf(stdout, "source:       %s\n", src)
	fmt.Fprintf(stdout, "name:         %s\n", p.Metadata.Name)
	fmt.Fprintf(stdout, "description:  %s\n", p.Metadata.Description)
	fmt.Fprintf(stdout, "default:      %t\n", p.Metadata.Default)
	if len(p.Metadata.Conflicts) > 0 {
		fmt.Fprintf(stdout, "conflicts:    %s\n", strings.Join(p.Metadata.Conflicts, ", "))
	}
	fmt.Fprintf(stdout, "requires_root: %t\n", p.Install.RequiresRoot)
	fmt.Fprintf(stdout, "version_capable: %t\n", p.Version.VersionCapable)
	if p.Apt != nil && len(p.Apt.Packages) > 0 {
		pkgs := append([]string(nil), p.Apt.Packages...)
		sort.Strings(pkgs)
		fmt.Fprintf(stdout, "apt_packages: %s\n", strings.Join(pkgs, ", "))
	}
	if len(p.Install.UserDirs) > 0 {
		fmt.Fprintf(stdout, "user_dirs:    %s\n", strings.Join(p.Install.UserDirs, ", "))
	}
	if len(p.Install.BuildArgs) > 0 {
		fmt.Fprintf(stdout, "build_args:   %s\n", strings.Join(p.Install.BuildArgs, ", "))
	}
	if len(p.Install.Volumes) > 0 {
		fmt.Fprintf(stdout, "volumes:      %s\n", strings.Join(p.Install.Volumes, ", "))
	}
	if len(p.Install.Env) > 0 {
		keys := make([]string, 0, len(p.Install.Env))
		for k := range p.Install.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Fprintln(stdout, "env:")
		for _, k := range keys {
			fmt.Fprintf(stdout, "  %s=%s\n", k, p.Install.Env[k])
		}
	}
	return nil
}
