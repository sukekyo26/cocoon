package plugincli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/logx"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

const pinLong = `cocoon plugin pin — emit pin lines for [plugins.versions] (and optionally [plugins.methods])

By default a ` + "`<id> = { pin = \"<ref>\" }`" + ` line is printed to stdout for you
to paste under the [plugins.versions] section in workspace.toml. With
--write the line is upserted in place (inserted, or the existing
<id> = { ... } line is replaced); comments and blank lines outside the
target line are preserved verbatim.

Use the --amd64-checksum / --arm64-checksum flags when the upstream
release ships per-arch SHA256 sums you want the install script to
verify.

Pass --method <name> for plugins that declare two or more entries under
[install.methods] in their plugin.toml — the pin then writes (or prints)
both lines together: ` + "`<id> = \"<method>\"`" + ` for [plugins.methods] and
the inline-table line for [plugins.versions]. Checksums are workspace-
scoped (not per-method); when switching methods, refresh
--amd64-checksum / --arm64-checksum so the install script's SHA256
verification still matches the new artifact.`

func newPinCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		amd64Checksum string
		arm64Checksum string
		method        string
		write         bool
	)
	cmd := &cobra.Command{
		Use:           "pin <id> <ref>",
		Short:         "Emit inline-table pin lines for workspace.toml (stdout, or in-place with --write)",
		Long:          pinLong,
		Args:          cobra.ExactArgs(2),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, args []string) error {
			return runPin(stdout, stderr, args[0], args[1], amd64Checksum, arm64Checksum, method, write)
		},
	}
	cmd.Flags().StringVar(&amd64Checksum, "amd64-checksum", "", "sha256 of the amd64 artifact (optional)")
	cmd.Flags().StringVar(&arm64Checksum, "arm64-checksum", "", "sha256 of the arm64 artifact (optional)")
	cmd.Flags().StringVar(&method, "method", "",
		"install method name; only meaningful for plugins that declare [install.methods]. "+
			"When set, the command pins both [plugins.methods] and [plugins.versions] in a "+
			"single workspace.toml read-write. When omitted, only [plugins.versions] is updated "+
			"(the existing [plugins.methods] entry — or the plugin's default_method — stays in effect).")
	cmd.Flags().BoolVar(&write, "write", false,
		"upsert the inline-table line in workspace.toml (auto-discovered from cwd)")
	return cmd
}

func runPin(stdout, stderr io.Writer, id, ref, amd64sum, arm64sum, method string, write bool) error {
	if id == "" || ref == "" {
		return fmt.Errorf("%w: both <id> and <ref> are required", clihelpers.ErrUsage)
	}
	layered, err := resolveLayered()
	if err != nil {
		return err
	}
	if layered.Source(id) == "" {
		return fmt.Errorf("%w: plugin %q is not in any layer (cocoon plugin list)", clihelpers.ErrUsage, id)
	}
	if method != "" {
		if mErr := validateMethodForPin(layered, id, method); mErr != nil {
			return mErr
		}
	}
	if write {
		return runPinWrite(stdout, stderr, id, ref, amd64sum, arm64sum, method)
	}
	fmt.Fprint(stdout, renderPinSnippet(id, ref, amd64sum, arm64sum, method))
	return nil
}

// runPinWrite discovers workspace.toml and upserts both the pin line
// (always) and the method line (when method != "") in a single
// read-modify-write cycle via plugin.UpsertPinAndMethod. The single-write
// path means a transient I/O failure cannot persist the pin without the
// matching method or vice-versa — either both land or neither does.
func runPinWrite(stdout, stderr io.Writer, id, ref, amd64sum, arm64sum, method string) error {
	cwd, cwdErr := os.Getwd()
	if cwdErr != nil {
		return fmt.Errorf("%w: getwd: %w", clihelpers.ErrFailure, cwdErr)
	}
	wsPath, dErr := config.Discover(cwd)
	if dErr != nil {
		return fmt.Errorf("%w: discover workspace.toml: %w", clihelpers.ErrFailure, dErr)
	}
	if wsPath == "" {
		return fmt.Errorf(
			"%w: --write needs a discoverable workspace.toml (run inside a cocoon project)",
			clihelpers.ErrUsage)
	}
	if uErr := plugin.UpsertPinAndMethod(wsPath, id, ref, amd64sum, arm64sum, method); uErr != nil {
		if errors.Is(uErr, plugin.ErrLegacyPinSubsection) {
			return fmt.Errorf("%w: %w (in %s)", clihelpers.ErrUsage, uErr, wsPath)
		}
		return fmt.Errorf("%w: %w", clihelpers.ErrFailure, uErr)
	}
	log := logx.New(stdout, stderr)
	log.Successf("Updated %s: [plugins.versions] %s", wsPath, id)
	if method != "" {
		log.Successf("Updated %s: [plugins.methods] %s = %q", wsPath, id, method)
	}
	return nil
}

// renderPinSnippet returns the stdout block the user pastes into
// workspace.toml when --write is absent. Empty method emits the
// legacy single-section shape; a non-empty method prepends a
// [plugins.methods] snippet so the user sees both halves of the pick.
func renderPinSnippet(id, ref, amd64sum, arm64sum, method string) string {
	var b strings.Builder
	if method == "" {
		fmt.Fprintln(&b, "# Add the following line under [plugins.versions] in workspace.toml:")
		fmt.Fprintln(&b)
		b.WriteString(plugin.FormatPinLine(id, ref, amd64sum, arm64sum))
		return b.String()
	}
	fmt.Fprintln(&b, "# Add the following lines to workspace.toml:")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "# Under [plugins.methods]:")
	b.WriteString(plugin.FormatMethodLine(id, method))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "# Under [plugins.versions]:")
	b.WriteString(plugin.FormatPinLine(id, ref, amd64sum, arm64sum))
	return b.String()
}

// validateMethodForPin loads the resolved plugin and confirms the requested
// method name exists under [install.methods]. Returns clihelpers.ErrUsage for the
// user-correctable failures (no methods declared / method name not declared)
// and clihelpers.ErrFailure when the manifest itself cannot be read.
//
// loadPluginFromLayer only runs strict unmarshal — it skips
// validateMethodScripts and plugin.Validate — so a user-overlay plugin
// without [install.methods] reaches this function with an empty
// Install.Methods map. Handle that case explicitly rather than falling
// through into the "method not declared (declared: )" branch, which
// would surface an empty declared list and obscure the real fix.
func validateMethodForPin(layered *plugin.LayeredFS, id, method string) error {
	p, err := loadPluginFromLayer(layered, id)
	if err != nil {
		return fmt.Errorf("%w: %w", clihelpers.ErrFailure, err)
	}
	if len(p.Install.Methods) == 0 {
		return fmt.Errorf(
			"%w: plugin %q declares no [install.methods] in plugin.toml; "+
				"--method is only meaningful when the plugin offers two or more "+
				"install variants — drop --method to pin only the version",
			clihelpers.ErrUsage, id)
	}
	if _, ok := p.Install.Methods[method]; !ok {
		declared := make([]string, 0, len(p.Install.Methods))
		for name := range p.Install.Methods {
			declared = append(declared, name)
		}
		sort.Strings(declared)
		return fmt.Errorf(
			"%w: plugin %q has no method %q in [install.methods] (declared: %s)",
			clihelpers.ErrUsage, id, method, strings.Join(declared, ", "))
	}
	return nil
}
