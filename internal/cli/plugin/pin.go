package plugincli

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/logx"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

func newPinCmd(stdout, stderr io.Writer) *cobra.Command {
	cat := i18n.New(i18n.Detect())
	var (
		method string
		write  bool
	)
	cmd := &cobra.Command{
		Use:           "pin <id> <ref>",
		Short:         cat.Msg("cmd_plugin_pin_short"),
		Long:          cat.Msg("cmd_plugin_pin_long"),
		Args:          cobra.ExactArgs(2),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, args []string) error {
			return runPin(stdout, stderr, args[0], args[1], method, write)
		},
	}
	cmd.Flags().StringVar(&method, "method", "", cat.Msg("flag_plugin_pin_method_usage"))
	cmd.Flags().BoolVar(&write, "write", false, cat.Msg("flag_plugin_pin_write_usage"))
	return cmd
}

func runPin(stdout, stderr io.Writer, id, ref, method string, write bool) error {
	if id == "" || ref == "" {
		return fmt.Errorf("%w: both <id> and <ref> are required", clihelpers.ErrUsage)
	}
	// Normalise the version constraint up front so the CLI rejects ranges
	// (">=1.0") with the same message the workspace loader uses and writes the
	// canonical "=<version>" / "latest" form. A bare version is accepted and
	// gets the "=" prepended for the user.
	override, specErr := normalizePinSpec(ref)
	if specErr != nil {
		return fmt.Errorf(
			`%w: invalid version for %q: %w — write a bare version (1.23.4) or "latest"`,
			clihelpers.ErrUsage, id, specErr)
	}
	spec := override.Spec
	layered, err := resolveLayered()
	if err != nil {
		return err
	}
	if layered.Source(id) == "" {
		return fmt.Errorf("%w: plugin %q is not in any layer (cocoon plugin list)", clihelpers.ErrUsage, id)
	}
	p, lErr := loadPluginFromLayer(layered, id)
	if lErr != nil {
		return fmt.Errorf("%w: %w", clihelpers.ErrFailure, lErr)
	}
	// A pin only means something for a version_capable plugin: cocoon gen
	// hard-rejects a pinned enable entry for any other plugin. Fail fast here
	// so `plugin pin` never emits a config that cannot generate.
	if !p.Version.VersionCapable {
		return fmt.Errorf(
			"%w: plugin %q is not version_capable; it cannot be pinned "+
				"(a \"%s=<version>\" enable entry is rejected by cocoon gen)",
			clihelpers.ErrUsage, id, id)
	}
	if method != "" {
		if mErr := validateMethodForPin(p, id, method); mErr != nil {
			return mErr
		}
	}
	if write {
		return runPinWrite(stdout, stderr, id, spec, method)
	}
	fmt.Fprint(stdout, renderPinSnippet(id, spec, method))
	return nil
}

// normalizePinSpec turns the CLI's <ref> argument into a validated
// PluginVersionOverride. For ergonomics a bare version ("1.23.4") gets the
// "=" prepended; "latest"/"*" and any operator-led string (=, >, <, ^, ~, !)
// are passed to config.ParseVersionSpec verbatim so ranges are classified
// and rejected precisely rather than mangled into a charset error.
func normalizePinSpec(ref string) (config.PluginVersionOverride, error) {
	t := strings.TrimSpace(ref)
	candidate := t
	if t != "" && t != config.VersionSpecLatest && t != "*" && isVersionStart(t[0]) {
		candidate = "=" + t
	}
	//nolint:wrapcheck // runPin wraps the returned spec error into clihelpers.ErrUsage.
	return config.ParseVersionSpec(candidate)
}

// isVersionStart reports whether b can begin a bare version token (so the
// CLI knows to prepend "="). Operators and "latest" are handled separately.
func isVersionStart(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_'
}

// runPinWrite discovers workspace.toml and upserts both the constraint line
// (always) and the method line (when method != "") in a single
// read-modify-write cycle via plugin.UpsertPinAndMethod. The single-write
// path means a transient I/O failure cannot persist the pin without the
// matching method or vice-versa — either both land or neither does.
func runPinWrite(stdout, stderr io.Writer, id, spec, method string) error {
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
	if uErr := plugin.UpsertPinAndMethod(wsPath, id, spec, method); uErr != nil {
		if errors.Is(uErr, plugin.ErrLegacyPluginVersions) {
			return fmt.Errorf("%w: %w (in %s)", clihelpers.ErrUsage, uErr, wsPath)
		}
		return fmt.Errorf("%w: %w", clihelpers.ErrFailure, uErr)
	}
	log := logx.New(stdout, stderr)
	log.Successf("Updated %s: [plugins].enable %q", wsPath, plugin.FormatEnableEntry(id, spec))
	if method != "" {
		log.Successf("Updated %s: [plugins.methods] %s = %q", wsPath, id, method)
	}
	return nil
}

// renderPinSnippet returns the stdout block the user pastes into
// workspace.toml when --write is absent. Empty method emits the enable-array
// entry alone; a non-empty method appends a [plugins.methods] snippet so the
// user sees both halves of the pick.
func renderPinSnippet(id, spec, method string) string {
	var b strings.Builder
	entry := plugin.FormatEnableEntry(id, spec)
	if method == "" {
		fmt.Fprintln(&b, "# Add (or update) this entry in the [plugins].enable array in workspace.toml:")
		fmt.Fprintln(&b)
		fmt.Fprintf(&b, "%q\n", entry)
		return b.String()
	}
	fmt.Fprintln(&b, "# Add the following to workspace.toml:")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "# In the [plugins].enable array:")
	fmt.Fprintf(&b, "%q\n", entry)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "# Under [plugins.methods]:")
	b.WriteString(plugin.FormatMethodLine(id, method))
	return b.String()
}

// validateMethodForPin confirms the requested method name exists under the
// plugin's [install.methods]. Returns clihelpers.ErrUsage for the
// user-correctable failures (no methods declared / method name not declared).
//
// p comes from loadPluginFromLayer, which runs strict unmarshal only — it
// skips validateMethodScripts and plugin.Validate — so a user-overlay
// plugin without [install.methods] reaches this function with an empty
// Install.Methods map. Handle that case explicitly rather than falling
// through into the "method not declared (declared: )" branch, which
// would surface an empty declared list and obscure the real fix.
func validateMethodForPin(p *plugin.Plugin, id, method string) error {
	if len(p.Install.Methods) == 0 {
		return fmt.Errorf(
			"%w: plugin %q declares no [install.methods] in plugin.toml; "+
				"--method is only meaningful when the plugin offers two or more "+
				"install variants — drop --method to pin only the version",
			clihelpers.ErrUsage, id)
	}
	if _, ok := p.Install.Methods[method]; !ok {
		declared := slices.Sorted(maps.Keys(p.Install.Methods))
		return fmt.Errorf(
			"%w: plugin %q has no method %q in [install.methods] (declared: %s)",
			clihelpers.ErrUsage, id, method, strings.Join(declared, ", "))
	}
	return nil
}
