package initcli

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

type containerSpec struct {
	ServiceName    string
	Username       string
	Image          string
	ImageVersion   string
	Shell          string
	Aliases        map[string]string
	MountRoot      string
	Dir            string
	Devcontainer   bool
	Certificates   bool
	Secure         bool
	ImagePathFix   bool
	Packages       []string
	Plugins        []string
	PluginVersions map[string]string
	PluginMethods  map[string]string
	Ports          []string
}

// renderWorkspaceToml emits workspace.toml. Inline comments come from
// the i18n catalog so the locale matches the original runner's $LANG
// (re-run with --force under a different LANG to switch). The section
// writers run in the file's top-to-bottom order.
func renderWorkspaceToml(s containerSpec, cat *i18n.Catalog) string {
	var sb strings.Builder
	sb.WriteString(cat.Msg("init_toml_header"))
	sb.WriteByte('\n')
	writeWorkspaceSection(&sb, cat, s)
	writeContainerSection(&sb, cat, s)
	writeShellSection(&sb, cat, s)
	writePluginsSection(&sb, cat, s)
	writeAptSection(&sb, cat, s)
	writeCertificatesSection(&sb, cat, s)
	writePortsSection(&sb, cat, s)
	writeTrailingTemplates(&sb, cat, s)
	return strings.TrimRight(sb.String(), "\n") + "\n"
}

// writeWorkspaceSection emits the [workspace] block.
func writeWorkspaceSection(sb *strings.Builder, cat *i18n.Catalog, s containerSpec) {
	sb.WriteString(cat.Msg("init_toml_section_workspace"))
	sb.WriteByte('\n')
	sb.WriteString("[workspace]\n")
	fmt.Fprintf(sb, "mount_root = %q\n", s.MountRoot)
	fmt.Fprintf(sb, "dir = %q\n", s.Dir)
	fmt.Fprintf(sb, "devcontainer = %t\n\n", s.Devcontainer)
}

// writeContainerSection emits the [container] block followed by the
// commented-out templates for its opt-in extras.
func writeContainerSection(sb *strings.Builder, cat *i18n.Catalog, s containerSpec) {
	sb.WriteString(cat.Msg("init_toml_section_container"))
	sb.WriteByte('\n')
	sb.WriteString("[container]\n")
	fmt.Fprintf(sb, "service_name = %q\n", s.ServiceName)
	fmt.Fprintf(sb, "username = %q\n", s.Username)
	fmt.Fprintf(sb, "image = %q\n", s.Image)
	fmt.Fprintf(sb, "image_version = %q\n\n", s.ImageVersion)

	// Commented-out templates for [container] opt-in extras. The flat-field
	// templates come first so an uncommented line still belongs to
	// [container]; the [container.*] subtables follow.
	for _, key := range []string{
		"init_toml_template_container_docker_socket",
		"init_toml_template_container_group_add",
		"init_toml_template_container_devices",
		"init_toml_template_container_ipc",
		"init_toml_template_container_gpus",
	} {
		emitTemplate(sb, cat, key)
	}
	for _, key := range []string{
		"init_toml_template_container_resources",
		"init_toml_template_container_hosts",
		"init_toml_template_container_dns",
		"init_toml_template_container_sysctls",
		"init_toml_template_container_capabilities",
	} {
		emitTemplate(sb, cat, key)
	}
	writeContainerSecurityOpt(sb, cat, s)
	emitTemplate(sb, cat, "init_toml_template_container_skel")
}

// writeContainerSecurityOpt emits the [container.security_opt] block. With
// --secure it writes an active block with no_new_privileges = true (the
// header comment spells out the sudo trade-off); otherwise it falls back to
// the commented-out template so the section stays discoverable. Mirrors the
// active⇄commented swap writeCertificatesSection uses for [certificates].
func writeContainerSecurityOpt(sb *strings.Builder, cat *i18n.Catalog, s containerSpec) {
	if !s.Secure {
		emitTemplate(sb, cat, "init_toml_template_container_security_opt")
		return
	}
	sb.WriteString(cat.Msg("init_toml_section_container_security_opt"))
	sb.WriteByte('\n')
	sb.WriteString("[container.security_opt]\n")
	sb.WriteString("no_new_privileges = true\n\n")
}

// writeShellSection emits the [container.shell] block followed by an
// auto-injected [container.shell.env] subsection when the image-path-fix
// is on. The subsection is preceded by a three-line self-documenting
// comment (added / removal / coexist) so a future reader knows why the
// entries are there, what breaks if they delete the block, and that
// the inline `env = { ... }` form under [container.shell] cannot
// coexist (TOML rejects two definitions of the same key).
func writeShellSection(sb *strings.Builder, cat *i18n.Catalog, s containerSpec) {
	sb.WriteString(cat.Msg("init_toml_section_container_shell"))
	sb.WriteByte('\n')
	sb.WriteString("[container.shell]\n")
	fmt.Fprintf(sb, "default = %q\n", s.Shell)
	if len(s.Aliases) > 0 {
		sb.WriteString("aliases = ")
		writeInlineTable(sb, s.Aliases)
		sb.WriteByte('\n')
	}
	sb.WriteByte('\n')
	writeImagePathFixEnv(sb, cat, s)
}

// writeImagePathFixEnv emits the [container.shell.env] subsection that
// pairs with the prompt's auto-injection. No-op when the toggle is off
// or when the image has no fix entry, so subsequent runs of `cocoon init
// --force` against a different image cleanly drop the previous block.
//
// The three comment lines above the block carry the why (added), the
// blast radius of removal (removal), and the mutual-exclusion warning
// (coexist) with the [container.shell] inline `env = { ... }` form —
// TOML rejects mixing them since both define the same `env` key.
func writeImagePathFixEnv(sb *strings.Builder, cat *i18n.Catalog, s containerSpec) {
	if !s.ImagePathFix || !imagePathFixApplies(s.Image) {
		return
	}
	fix := imagePathFixFor(s.Image)
	sb.WriteString(cat.Msg("init_toml_comment_image_path_fix_added", s.Image))
	sb.WriteByte('\n')
	sb.WriteString(cat.Msg("init_toml_comment_image_path_fix_removal", fix.Command))
	sb.WriteByte('\n')
	sb.WriteString(cat.Msg("init_toml_comment_image_path_fix_coexist"))
	sb.WriteByte('\n')
	sb.WriteString("[container.shell.env]\n")
	for _, e := range fix.Entries {
		fmt.Fprintf(sb, "%s = %q\n", e.Key, e.Value)
	}
	sb.WriteByte('\n')
}

// imagePathFixVolumesActive reports whether the trailing [volumes] section
// should be emitted as an active block (true) or as the commented-out
// template (false). True only when the user opted in AND the image has
// volume entries — python carries no Volumes because its install target
// is already covered by the reserved `local:` named volume.
func imagePathFixVolumesActive(s containerSpec) bool {
	return s.ImagePathFix && imagePathFixApplies(s.Image) && len(imagePathFixFor(s.Image).Volumes) > 0
}

// writeImagePathFixVolumes emits the top-level [volumes] section that
// pairs with writeImagePathFixEnv. No-op unless imagePathFixVolumesActive
// returns true. Volume names are lockstepped with the equivalent catalog
// plugin's [install].volumes so swapping image⇄plugin yields the same
// compose volume keys.
//
// The three comment lines mirror writeImagePathFixEnv's pattern: why
// (added), what removal costs (removal), and how to extend safely (extra
// — append under the same [volumes], do not open a second one).
func writeImagePathFixVolumes(sb *strings.Builder, cat *i18n.Catalog, s containerSpec) {
	if !imagePathFixVolumesActive(s) {
		return
	}
	fix := imagePathFixFor(s.Image)
	sb.WriteString(cat.Msg("init_toml_section_volumes"))
	sb.WriteByte('\n')
	sb.WriteString(cat.Msg("init_toml_comment_image_path_fix_volumes_added", s.Image, fix.Command))
	sb.WriteByte('\n')
	sb.WriteString(cat.Msg("init_toml_comment_image_path_fix_volumes_removal", fix.Command))
	sb.WriteByte('\n')
	sb.WriteString(cat.Msg("init_toml_comment_image_path_fix_volumes_extra"))
	sb.WriteByte('\n')
	sb.WriteString("[volumes]\n")
	for _, v := range fix.Volumes {
		fmt.Fprintf(sb, "%s = %q\n", v.Name, v.Path)
	}
	sb.WriteByte('\n')
}

// writePluginsSection emits the [plugins] block plus the [plugins.methods]
// and [plugins.versions] sub-blocks.
func writePluginsSection(sb *strings.Builder, cat *i18n.Catalog, s containerSpec) {
	sb.WriteString(cat.Msg("init_toml_section_plugins"))
	sb.WriteByte('\n')
	sb.WriteString("[plugins]\n")
	if len(s.Plugins) == 0 {
		sb.WriteString("enable = []\n\n")
	} else {
		sb.WriteString("enable = [\n")
		for _, id := range s.Plugins {
			fmt.Fprintf(sb, "    %q,\n", id)
		}
		sb.WriteString("]\n\n")
	}

	writePluginMethods(sb, cat, s.PluginMethods)
	writePluginVersions(sb, cat, s.PluginVersions)
}

// writeAptSection emits the [apt] block followed by its commented-out
// templates.
func writeAptSection(sb *strings.Builder, cat *i18n.Catalog, s containerSpec) {
	sb.WriteString(cat.Msg("init_toml_section_apt"))
	sb.WriteByte('\n')
	sb.WriteString("[apt]\n")
	if len(s.Packages) == 0 {
		sb.WriteString("packages = []\n\n")
	} else {
		sb.WriteString("packages = [\n")
		for _, pkg := range s.Packages {
			fmt.Fprintf(sb, "    %q,\n", pkg)
		}
		sb.WriteString("]\n\n")
	}

	for _, key := range []string{
		"init_toml_template_apt_mirror",
		"init_toml_template_apt_proxy",
		"init_toml_template_apt_sources",
	} {
		emitTemplate(sb, cat, key)
	}
}

// writeCertificatesSection emits an active [certificates] block only when
// the user opted in; the commented-out template is emitted by
// writeTrailingTemplates in the opt-out case.
func writeCertificatesSection(sb *strings.Builder, cat *i18n.Catalog, s containerSpec) {
	if !s.Certificates {
		return
	}
	sb.WriteString(cat.Msg("init_toml_section_certificates"))
	sb.WriteByte('\n')
	sb.WriteString("[certificates]\n")
	sb.WriteString("enable = true\n\n")
}

// writePortsSection emits an active [ports] block when the user supplied
// ports via --ports or the interactive prompt. With no ports the
// commented-out template still emits so the file remains self-documenting
// (matches [volumes] / [env] / [mounts] behavior).
func writePortsSection(sb *strings.Builder, cat *i18n.Catalog, s containerSpec) {
	if len(s.Ports) == 0 {
		emitTemplate(sb, cat, "init_toml_template_ports")
		return
	}
	sb.WriteString(cat.Msg("init_toml_section_ports"))
	sb.WriteByte('\n')
	sb.WriteString("[ports]\nforward = [\n")
	for _, p := range s.Ports {
		fmt.Fprintf(sb, "    %q,\n", p)
	}
	sb.WriteString("]\n\n")
}

// writeTrailingTemplates emits the top-level opt-in extras at the end of the
// file. Order roughly follows "compose runtime knobs first, then host-side
// persistence, then locale + Dockerfile hooks, then certificates, then
// sidecars + IDE config".
//
// [volumes] swaps between the commented-out template and an active block
// written by writeImagePathFixVolumes when image-path-fix is on and the
// chosen image has volume entries (writePortsSection follows the same
// active⇄commented pattern).
func writeTrailingTemplates(sb *strings.Builder, cat *i18n.Catalog, s containerSpec) {
	if imagePathFixVolumesActive(s) {
		writeImagePathFixVolumes(sb, cat, s)
	} else {
		emitTemplate(sb, cat, "init_toml_template_volumes")
	}
	templateKeys := []string{
		"init_toml_template_env",
		"init_toml_template_mounts",
		"init_toml_template_home_files",
		"init_toml_template_locale",
		"init_toml_template_dockerfile",
	}
	if !s.Certificates {
		templateKeys = append(templateKeys, "init_toml_template_certificates")
	}
	templateKeys = append(templateKeys,
		"init_toml_template_services",
		"init_toml_template_devcontainer",
		"init_toml_template_code_workspace",
	)
	for _, key := range templateKeys {
		emitTemplate(sb, cat, key)
	}
}

// writePluginMethods emits a single `[plugins.methods]` block with one
// `<id> = "<method>"` line per pick, alphabetically sorted by id. When picks
// is empty it falls back to the commented-out template so users discover the
// section without us having to render an empty `[plugins.methods]` table.
func writePluginMethods(sb *strings.Builder, cat *i18n.Catalog, picks map[string]string) {
	if len(picks) == 0 {
		emitTemplate(sb, cat, "init_toml_template_plugins_methods")
		return
	}
	ids := slices.Sorted(maps.Keys(picks))
	sb.WriteString(cat.Msg("init_toml_section_plugins_methods"))
	sb.WriteByte('\n')
	sb.WriteString("[plugins.methods]\n")
	for _, id := range ids {
		fmt.Fprintf(sb, "%s = %q\n", id, picks[id])
	}
	sb.WriteByte('\n')
}

// writePluginVersions emits a single `[plugins.versions]` section with one
// inline-table line per pin, alphabetically sorted by id. When pins is empty
// it falls back to the commented-out example template so the reader still
// discovers the section.
func writePluginVersions(sb *strings.Builder, cat *i18n.Catalog, pins map[string]string) {
	if len(pins) == 0 {
		emitTemplate(sb, cat, "init_toml_template_plugins_versions")
		return
	}
	lines := make([]plugin.PinLine, 0, len(pins))
	for id, ref := range pins {
		lines = append(lines, plugin.PinLine{ID: id, Ref: ref, ChecksumAmd64: "", ChecksumArm64: ""})
	}
	sb.WriteString(cat.Msg("init_toml_section_plugins_versions"))
	sb.WriteByte('\n')
	sb.WriteString(plugin.FormatPinSection(lines))
	sb.WriteByte('\n')
}

// emitTemplate writes a localized commented-out section template to sb,
// followed by exactly one blank line so adjacent templates stay visually
// separated. Each i18n value is the raw `# ...` block (no trailing
// newline) — the `\n\n` here adds the closing newline for that line plus
// the blank-line separator.
func emitTemplate(sb *strings.Builder, cat *i18n.Catalog, key string) {
	sb.WriteString(cat.Msg(key))
	sb.WriteString("\n\n")
}

// writeInlineTable emits a TOML inline-table value (`{ k = "v", ... }`)
// with keys sorted so the output is deterministic across runs. Used for
// `[container.shell] aliases = { ... }` so the generated workspace.toml
// stays diff-friendly when the user re-runs `cocoon init --force`.
func writeInlineTable(sb *strings.Builder, m map[string]string) {
	keys := slices.Sorted(maps.Keys(m))
	sb.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			sb.WriteString(", ")
		} else {
			sb.WriteByte(' ')
		}
		fmt.Fprintf(sb, "%s = %q", k, m[k])
	}
	sb.WriteString(" }")
}
