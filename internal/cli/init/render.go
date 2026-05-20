package initcli

import (
	"fmt"
	"sort"
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
		"init_toml_template_container_security_opt",
		"init_toml_template_container_skel",
	} {
		emitTemplate(sb, cat, key)
	}
}

// writeShellSection emits the [container.shell] block.
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
func writeTrailingTemplates(sb *strings.Builder, cat *i18n.Catalog, s containerSpec) {
	templateKeys := []string{
		"init_toml_template_volumes",
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
	ids := make([]string, 0, len(picks))
	for id := range picks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
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
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
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
