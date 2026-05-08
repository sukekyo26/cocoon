package setup

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sukekyo26/cocoon/internal/certificates"
	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/logx"
)

const (
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorReset  = "\033[0m"
)

func green(s string) string  { return colorGreen + s + colorReset }
func yellow(s string) string { return colorYellow + s + colorReset }
func cyan(s string) string   { return colorCyan + s + colorReset }

func sectionHeader(title string) string {
	line := "========================================"
	return "\n" + line + "\n " + title + "\n" + line
}

func subsectionHeader(title string) string {
	return "--- " + title + " ---"
}

func formatStrSlice(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// formatPortsSlice renders a [ports].forward array (heterogeneous string /
// inline-table) as TOML for the setup template. Long-form maps emit only the
// docker-compose-allowed keys in fixed order so reconfigured workspace.toml
// stays diff-stable.
func formatPortsSlice(items []any) string {
	if len(items) == 0 {
		return "[]"
	}
	parts := make([]string, len(items))
	for i, v := range items {
		parts[i] = formatPortEntry(v)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func formatPortEntry(v any) string {
	switch x := v.(type) {
	case string:
		return strconv.Quote(x)
	case map[string]any:
		kvs := make([]string, 0, len(x))
		for _, k := range config.LongFormKeyOrder() {
			val, ok := x[k]
			if !ok {
				continue
			}
			kvs = append(kvs, k+" = "+formatPortValue(val))
		}
		return "{ " + strings.Join(kvs, ", ") + " }"
	case int:
		// Legacy `forward = [3000]` is no longer accepted, but
		// loadPartialWorkspace decodes it as int when reading existing
		// workspace.toml during `setup --init`. Auto-migrate to the
		// equivalent short-form "N:N" (matching how the old compose
		// generator rendered it) so partial-reconfig writes a valid file.
		return strconv.Quote(fmt.Sprintf("%d:%d", x, x))
	case int64:
		return strconv.Quote(fmt.Sprintf("%d:%d", x, x))
	default:
		return fmt.Sprintf("%v", v)
	}
}

func formatPortValue(v any) string {
	switch x := v.(type) {
	case string:
		return strconv.Quote(x)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func printResult(log *logx.Logger, t Translator, ws *config.Workspace, uid, gid, dockerGID int, workspaceDir string) {
	log.Info("")
	log.Info(green(t.Msg("setup_complete")))
	log.Info(t.Msg("setup_result_service", ws.Container.ServiceName))
	log.Info(t.Msg("setup_result_username", ws.Container.Username))
	log.Info(t.Msg("setup_result_os", ws.Container.Os))
	log.Info(t.Msg("setup_result_os_version", ws.Container.OsVersion))
	log.Info(t.Msg("setup_result_uid_gid", strconv.Itoa(uid), strconv.Itoa(gid)))
	log.Info(t.Msg("setup_result_docker_gid", strconv.Itoa(dockerGID)))
	log.Info("")
	log.Info(t.Msg("setup_result_plugins"))
	for _, p := range ws.Plugins.Enable {
		log.Info(t.Msg("setup_result_plugin_item", p))
	}
	if certificates.Has(workspaceDir) {
		log.Info(t.Msg("setup_result_certs"))
	}
	log.Info("")
	if ws.Ports != nil && len(ws.Ports.Forward) > 0 {
		strs := make([]string, len(ws.Ports.Forward))
		for i, p := range ws.Ports.Forward {
			strs[i] = formatPortEntry(p)
		}
		log.Info(t.Msg("setup_result_port", strings.Join(strs, ", ")))
	} else {
		log.Info(t.Msg("setup_result_port_none"))
	}
	log.Info("")
	log.Info(t.Msg("setup_result_files"))
	log.Info("  - workspace.toml (configuration — edit this file)")
	log.Info("  - Dockerfile")
	log.Info("  - docker-compose.yml")
	log.Info("  - .devcontainer/devcontainer.json")
	log.Info("  - .devcontainer/docker-compose.yml")
	log.Info("  - .env (auto-generated from workspace.toml)")
	log.Info("")
	log.Info(yellow(t.Msg("setup_build_hint")))
	log.Info(cyan("  docker compose build"))
	log.Info(cyan("  docker compose build --no-cache  # to rebuild without cache"))
	log.Info("")
	log.Info(yellow(t.Msg("setup_start_hint")))
	log.Info(cyan("  docker compose up -d"))
	log.Info("")
	log.Info(yellow(t.Msg("setup_access_hint")))
	log.Info(cyan(fmt.Sprintf("  docker compose exec %s bash", ws.Container.ServiceName)))
	log.Info("")
	log.Info(yellow(t.Msg("setup_stop_hint")))
	log.Info(cyan("  docker compose down"))
	log.Info("")
	log.Info(yellow(t.Msg("setup_reconfig_hint")))
	log.Info(cyan("  Edit workspace.toml and run ./setup-docker.sh"))
	log.Info(cyan("  Or run ./setup-docker.sh --init for interactive setup"))
	log.Info(cyan("  Or run ./setup-docker.sh --init --yes for non-interactive setup with defaults"))
}
