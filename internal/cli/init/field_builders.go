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

func mountRootSelect(cat *i18n.Catalog, target *string) *huh.Select[string] {
	return huh.NewSelect[string]().
		Title(cat.Msg("init_prompt_mount_root")).
		Description(cat.Msg("init_desc_mount_root")).
		Options(
			huh.NewOption(cat.Msg("init_option_mount_cwd"), "."),
			huh.NewOption(cat.Msg("init_option_mount_parent"), ".."),
		).
		Value(target)
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

// imagePathFixConfirm builds the prompt that asks whether to auto-inject
// the user-local install prefix / PATH for a language base image. The
// description spells out the exact entries that will be added and the
// example command they unlock so a no answer is informed, not accidental.
func imagePathFixConfirm(cat *i18n.Catalog, image string, target *bool) *huh.Confirm {
	fix := imagePathFixFor(image)
	return huh.NewConfirm().
		Title(cat.Msg("init_prompt_image_path_fix_title", image)).
		Description(cat.Msg("init_desc_image_path_fix",
			formatPathFixEntries(fix.Entries), fix.Command)).
		Affirmative(cat.Msg("init_confirm_yes")).
		Negative(cat.Msg("init_confirm_no")).
		Value(target)
}

// formatPathFixEntries renders the entries as TOML-looking lines so the
// prompt's preview matches what lands in workspace.toml byte-for-byte.
// Width is padded against the longest key for legibility on multi-entry
// images (node, rust); single-entry images (python, golang, deno) get a
// trivial alignment that still reads cleanly.
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

// portsInputValidator localizes the rejection message via the catalog.
// Separate from parsePorts so the `--ports` flag path keeps its English
// usage error consistent with the other init flag validators.
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
