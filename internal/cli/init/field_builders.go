package initcli

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/sukekyo26/cocoon/internal/aliasbundles"
	"github.com/sukekyo26/cocoon/internal/aptcategories"
	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

func identInput(cat *i18n.Catalog, titleKey, descKey, charsKey string,
	pattern *regexp.Regexp, target *string,
) *huh.Input {
	return huh.NewInput().
		Title(cat.Msg(titleKey)).
		Description(cat.Msg(descKey)).
		Validate(makeStrictValidator(pattern, charsKey, cat)).
		Value(target)
}

// Select/MultiSelect helpers omit Height() intentionally: explicit Height(len+2)
// breaks when title+description wraps to two lines (cursor stuck under a
// scrolling viewport); huh's default already covers our options count.

func imageSelect(cat *i18n.Catalog, target *string) *huh.Select[string] {
	options := make([]huh.Option[string], len(config.SupportedImages))
	for i, id := range config.SupportedImages {
		options[i] = huh.NewOption(id, id)
	}
	return huh.NewSelect[string]().
		Title(cat.Msg("init_prompt_image")).
		Description(cat.Msg("init_desc_image")).
		Options(options...).
		Value(target)
}

func shellSelect(cat *i18n.Catalog, target *string) *huh.Select[string] {
	options := make([]huh.Option[string], len(config.SupportedShells))
	for i, id := range config.SupportedShells {
		options[i] = huh.NewOption(id, id)
	}
	return huh.NewSelect[string]().
		Title(cat.Msg("init_prompt_shell")).
		Description(cat.Msg("init_desc_shell")).
		Options(options...).
		Value(target)
}

// mountRootCustom is the sentinel selected when the user wants a deeper
// ancestor mount than the two curated options. promptMountAndDir detects it
// and runs mountRootCustomInput to collect the actual ".." chain. The value
// is never a valid mount_root (IsValidMountRoot rejects it), so it cannot leak
// into a generated cocoon.toml.
const mountRootCustom = "custom"

func mountRootSelect(cat *i18n.Catalog, target *string) *huh.Select[string] {
	return huh.NewSelect[string]().
		Title(cat.Msg("init_prompt_mount_root")).
		Description(cat.Msg("init_desc_mount_root")).
		Options(
			huh.NewOption(cat.Msg("init_option_mount_cwd"), "."),
			huh.NewOption(cat.Msg("init_option_mount_parent"), ".."),
			huh.NewOption(cat.Msg("init_option_mount_custom"), mountRootCustom),
		).
		Value(target)
}

// mountRootCustomInput collects a deeper ".." chain after the picker's custom
// option is chosen. A blank entry is rejected — the custom path exists to type
// something deeper than ".", so falling back to "." silently would surprise
// the user who deliberately picked "custom".
func mountRootCustomInput(cat *i18n.Catalog, target *string) *huh.Input {
	return huh.NewInput().
		Title(cat.Msg("init_prompt_mount_root_custom")).
		Description(cat.Msg("init_desc_mount_root_custom")).
		Validate(mountRootInputValidator(cat)).
		Value(target)
}

// mountRootInputValidator localizes the rejection for inline TUI display.
// Separate from config.IsValidMountRoot so the prompt can also reject blank
// input (IsValidMountRoot treats "" as the "." default, which the custom
// branch must not accept).
func mountRootInputValidator(cat *i18n.Catalog) func(string) error {
	return func(s string) error {
		if s == "" || !config.IsValidMountRoot(s) {
			return errors.New(cat.Msg("init_err_mount_root_fmt")) //nolint:err113 // user-facing prompt
		}
		return nil
	}
}

// dirInput accepts blank input (caller treats blank as "keep default
// workspace") and validates non-blank entries through the shared
// IsValidWorkspaceDir so init never accepts a value `cocoon gen` would
// later reject.
func dirInput(cat *i18n.Catalog, target *string) *huh.Input {
	return huh.NewInput().
		Title(cat.Msg("init_prompt_dir")).
		Description(cat.Msg("init_desc_dir")).
		Validate(func(s string) error {
			if s == "" {
				return nil
			}
			if !config.IsValidWorkspaceDir(s) {
				return errors.New(cat.Msg("init_err_dir_fmt")) //nolint:err113 // user-facing prompt
			}
			return nil
		}).
		Value(target)
}

func devcontainerConfirm(cat *i18n.Catalog, target *bool) *huh.Confirm {
	return huh.NewConfirm().
		Title(cat.Msg("init_prompt_devcontainer")).
		Description(cat.Msg("init_desc_devcontainer")).
		Affirmative(cat.Msg("init_confirm_yes")).
		Negative(cat.Msg("init_confirm_no")).
		Value(target)
}

func certificatesConfirm(cat *i18n.Catalog, target *bool) *huh.Confirm {
	return huh.NewConfirm().
		Title(cat.Msg("init_prompt_certificates")).
		Description(cat.Msg("init_desc_certificates")).
		Affirmative(cat.Msg("init_confirm_yes")).
		Negative(cat.Msg("init_confirm_no")).
		Value(target)
}

// sudoSelect picks the in-container sudo policy. "nopasswd" and "password" map
// to [container.sudo] mode; "none" maps to [container.security_opt]
// no_new_privileges = true. When "password" is chosen the password value is
// collected separately by sudoPasswordInput.
func sudoSelect(cat *i18n.Catalog, target *string) *huh.Select[string] {
	return huh.NewSelect[string]().
		Title(cat.Msg("init_prompt_sudo")).
		Description(cat.Msg("init_desc_sudo")).
		Options(
			huh.NewOption(cat.Msg("init_option_sudo_nopasswd"), config.SudoModeNoPasswd),
			huh.NewOption(cat.Msg("init_option_sudo_password"), config.SudoModePassword),
			huh.NewOption(cat.Msg("init_option_sudo_none"), sudoChoiceNone),
		).
		Value(target)
}

// sudoPasswordInput collects the sudo password when password mode is chosen
// interactively. EchoMode hides the typed characters; the value seeds
// .devcontainer/.env.local. A blank password is rejected — password mode
// requires one (an empty SUDO_PASSWORD would fail the build).
func sudoPasswordInput(cat *i18n.Catalog, target *string) *huh.Input {
	return huh.NewInput().
		Title(cat.Msg("init_prompt_sudo_password")).
		Description(cat.Msg("init_desc_sudo_password")).
		EchoMode(huh.EchoModePassword).
		Validate(func(s string) error { return validateSudoPassword(cat, s) }).
		Value(target)
}

// validateSudoPassword rejects a sudo password that is blank or spans more
// than one line. .devcontainer/.env.local must carry exactly one
// `SUDO_PASSWORD=<value>` line; a pasted value with an embedded newline or
// carriage return would be silently truncated at build time (the Dockerfile
// reads only the first SUDO_PASSWORD= line), so it is rejected at input rather
// than surfacing as a surprising password mismatch in the container.
func validateSudoPassword(cat *i18n.Catalog, s string) error {
	if strings.TrimSpace(s) == "" {
		return errors.New(cat.Msg("init_err_sudo_password_empty")) //nolint:err113 // user-facing prompt
	}
	if strings.ContainsAny(s, "\r\n") {
		return errors.New(cat.Msg("init_err_sudo_password_multiline")) //nolint:err113 // user-facing prompt
	}
	return nil
}

// imagePathFixConfirm builds the prompt that asks whether to auto-inject
// the user-local install prefix / PATH for a language base image. The
// description spells out the exact entries (both [container.shell.env]
// and, when applicable, the matching [volumes] block that persists install
// destinations across rebuilds) so a no answer is informed, not accidental.
func imagePathFixConfirm(cat *i18n.Catalog, image string, target *bool) *huh.Confirm {
	fix := imagePathFixFor(image)
	return huh.NewConfirm().
		Title(cat.Msg("init_prompt_image_path_fix_title", image)).
		Description(cat.Msg("init_desc_image_path_fix",
			formatPathFixPreview(fix), fix.Command)).
		Affirmative(cat.Msg("init_confirm_yes")).
		Negative(cat.Msg("init_confirm_no")).
		Value(target)
}

// formatPathFixPreview renders the full set of TOML changes the user is
// about to accept — section headers, env entries, and (when present)
// named-volume entries — so the prompt mirrors the structure that lands
// in cocoon.toml. Python carries no Volumes (its install target is
// already covered by the reserved `local:` named volume), so the
// [volumes] block is omitted for that image.
func formatPathFixPreview(fix imagePathFix) string {
	var sb strings.Builder
	sb.WriteString("  [container.shell.env]\n")
	sb.WriteString(formatPathFixEntries(fix.Entries))
	if len(fix.Volumes) > 0 {
		sb.WriteString("\n\n  [volumes]\n")
		sb.WriteString(formatPathFixVolumes(fix.Volumes))
	}
	return sb.String()
}

// formatPathFixEntries renders the entries as TOML-looking lines so the
// prompt's preview shows the same keys and values that land in
// cocoon.toml. The preview adds a 2-space indent and pads keys
// against the longest one for legibility on multi-entry images
// (node, rust); the file emit in writeImagePathFixEnv stays unpadded
// canonical TOML, so the two surfaces share the key=value pairs but
// not their layout.
func formatPathFixEntries(entries []pathFixEnvEntry) string {
	width := 0
	for _, e := range entries {
		if len(e.Key) > width {
			width = len(e.Key)
		}
	}
	var sb strings.Builder
	for i, e := range entries {
		if i > 0 {
			sb.WriteByte('\n')
		}
		fmt.Fprintf(&sb, "  %-*s = %q", width, e.Key, e.Value)
	}
	return sb.String()
}

// formatPathFixVolumes mirrors formatPathFixEntries for the [volumes]
// section so the two preview blocks line up visually. The file emit in
// writeImagePathFixVolumes stays unpadded canonical TOML.
func formatPathFixVolumes(volumes []pathFixVolume) string {
	width := 0
	for _, v := range volumes {
		if len(v.Name) > width {
			width = len(v.Name)
		}
	}
	var sb strings.Builder
	for i, v := range volumes {
		if i > 0 {
			sb.WriteByte('\n')
		}
		fmt.Fprintf(&sb, "  %-*s = %q", width, v.Name, v.Path)
	}
	return sb.String()
}

func aptMultiSelect(cat *i18n.Catalog, target *[]string) *huh.MultiSelect[string] {
	options := make([]huh.Option[string], len(aptcategories.AptCategories))
	for i, c := range aptcategories.AptCategories {
		options[i] = huh.NewOption(fmt.Sprintf("%s (%s)", c.Label, c.Description), c.ID)
	}
	return huh.NewMultiSelect[string]().
		Title(cat.Msg("init_prompt_apt")).
		Description(cat.Msg("init_desc_apt")).
		Options(options...).
		Value(target)
}

func aliasBundlesMultiSelect(cat *i18n.Catalog, target *[]string) *huh.MultiSelect[string] {
	options := make([]huh.Option[string], len(aliasbundles.AliasBundles))
	for i, b := range aliasbundles.AliasBundles {
		options[i] = huh.NewOption(fmt.Sprintf("%s (%s)", b.Label, b.Description), b.ID)
	}
	return huh.NewMultiSelect[string]().
		Title(cat.Msg("init_prompt_alias_bundles")).
		Description(cat.Msg("init_desc_alias_bundles")).
		Options(options...).
		Value(target)
}

// portsInput accepts blank input (renderer emits the commented template).
// Non-empty input is validated by portsInputValidator so init never
// accepts a string `cocoon gen` would later reject.
func portsInput(cat *i18n.Catalog, target *string) *huh.Input {
	return huh.NewInput().
		Title(cat.Msg("init_prompt_ports")).
		Description(cat.Msg("init_desc_ports")).
		Validate(portsInputValidator(cat)).
		Value(target)
}

// portsInputValidator localizes the rejection via the catalog for inline TUI
// display. Separate from parsePorts because the prompt shows one concise
// message (init_err_port_invalid_fmt) rather than ValidateShortForm's detailed
// reason; the `--ports` flag path surfaces that reason (also localized) at the
// CLI boundary.
func portsInputValidator(cat *i18n.Catalog) func(string) error {
	return func(s string) error {
		for _, part := range strings.Split(s, ",") {
			p := strings.TrimSpace(part)
			if p == "" {
				continue
			}
			if err := config.ValidateShortForm(p); err != nil {
				return errors.New(cat.Msg("init_err_port_invalid_fmt", p)) //nolint:err113 // user-facing prompt
			}
		}
		return nil
	}
}

// pluginsMultiSelect sorts options by id so the order is stable across runs
// (LoadDir returns a map). excludeID hides one plugin id (empty = none) and
// is used to keep assertNoImagePluginConflict-tripping picks out of the
// picker entirely.
func pluginsMultiSelect(cat *i18n.Catalog, plugins map[string]*plugin.Plugin,
	excludeID string, target *[]string,
) *huh.MultiSelect[string] {
	ids := slices.Sorted(maps.Keys(plugins))
	options := make([]huh.Option[string], 0, len(ids))
	for _, id := range ids {
		if id == excludeID {
			continue
		}
		options = append(options, huh.NewOption(formatPluginLabel(id, plugins[id]), id))
	}
	return huh.NewMultiSelect[string]().
		Title(cat.Msg("init_prompt_plugins")).
		Description(cat.Msg("init_desc_plugins")).
		Options(options...).
		Value(target)
}
