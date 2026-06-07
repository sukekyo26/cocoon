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
	cat := i18n.New(i18n.Detect())
	if id == "" || ref == "" {
		return clihelpers.UsageErr("err_pin_both_required")
	}
	// Normalise the version constraint up front so the CLI rejects ranges
	// (">=1.0") with the same message the workspace loader uses and writes the
	// canonical "=<version>" / "latest" form. A bare version is accepted and
	// gets the "=" prepended for the user.
	override, specErr := normalizePinSpec(ref)
	if specErr != nil {
		return clihelpers.UsageErr("err_pin_invalid_version", id, specErr)
	}
	spec := override.Spec
	layered, err := resolveLayered()
	if err != nil {
		return err
	}
	if layered.Source(id) == "" {
		return clihelpers.UsageErr("err_pin_not_in_layer", id)
	}
	p, lErr := loadPluginFromLayer(layered, id)
	if lErr != nil {
		return clihelpers.FailureWrap(lErr, "")
	}
	// A pin only means something for a version_capable plugin: cocoon gen
	// hard-rejects a pinned enable entry for any other plugin. Fail fast here
	// so `plugin pin` never emits a config that cannot generate.
	if !p.Version.VersionCapable {
		return clihelpers.UsageErr("err_pin_not_version_capable", id, id)
	}
	if method != "" {
		if mErr := validateMethodForPin(p, id, method); mErr != nil {
			return mErr
		}
	}
	if write {
		return runPinWrite(stdout, stderr, cat, id, spec, method)
	}
	fmt.Fprint(stdout, renderPinSnippet(cat, id, spec, method))
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

// runPinWrite discovers the config file and upserts both the constraint line
// (always) and the method line (when method != "") in a single
// read-modify-write cycle via plugin.UpsertPinAndMethod. The single-write
// path means a transient I/O failure cannot persist the pin without the
// matching method or vice-versa — either both land or neither does.
func runPinWrite(stdout, stderr io.Writer, cat *i18n.Catalog, id, spec, method string) error {
	cwd, cwdErr := os.Getwd()
	if cwdErr != nil {
		return clihelpers.FailureWrap(cwdErr, "err_pin_getwd")
	}
	wsPath, dErr := config.Discover(cwd)
	if dErr != nil {
		return clihelpers.FailureWrap(dErr, "err_pin_discover")
	}
	if wsPath == "" {
		return clihelpers.UsageErr("err_pin_write_needs_workspace")
	}
	if uErr := plugin.UpsertPinAndMethod(wsPath, id, spec, method); uErr != nil {
		if errors.Is(uErr, plugin.ErrLegacyPluginVersions) {
			return clihelpers.UsageErr("err_pin_legacy_versions", uErr, wsPath)
		}
		return clihelpers.FailureWrap(uErr, "")
	}
	log := logx.New(stdout, stderr)
	log.Success(cat.Msg("plugin_pin_updated_enable", wsPath, plugin.FormatEnableEntry(id, spec)))
	if method != "" {
		log.Success(cat.Msg("plugin_pin_updated_method", wsPath, id, method))
	}
	return nil
}

// renderPinSnippet returns the stdout block the user pastes into
// the config file when --write is absent. Empty method emits the enable-array
// entry alone; a non-empty method appends a [plugins.methods] snippet so the
// user sees both halves of the pick.
func renderPinSnippet(cat *i18n.Catalog, id, spec, method string) string {
	var b strings.Builder
	entry := plugin.FormatEnableEntry(id, spec)
	if method == "" {
		fmt.Fprintln(&b, cat.Msg("plugin_pin_snippet_enable_header"))
		fmt.Fprintln(&b)
		fmt.Fprintf(&b, "%q\n", entry)
		return b.String()
	}
	fmt.Fprintln(&b, cat.Msg("plugin_pin_snippet_header"))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, cat.Msg("plugin_pin_snippet_enable_section"))
	fmt.Fprintf(&b, "%q\n", entry)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, cat.Msg("plugin_pin_snippet_methods_section"))
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
		return clihelpers.UsageErr("err_pin_no_methods", id)
	}
	if _, ok := p.Install.Methods[method]; !ok {
		declared := slices.Sorted(maps.Keys(p.Install.Methods))
		return clihelpers.UsageErr("err_pin_method_not_declared", id, method, strings.Join(declared, ", "))
	}
	return nil
}
