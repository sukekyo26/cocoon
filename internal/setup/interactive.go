package setup

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os/user"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/logx"
	"github.com/sukekyo26/cocoon/internal/plugin"
	"github.com/sukekyo26/cocoon/internal/tui"
)

var (
	rxServiceName = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	rxUsername    = regexp.MustCompile(`^[a-z_][a-z0-9_-]*$`)
)

// defaultOs is the value written when the user takes the --yes fast path
// with no existing workspace.toml. Ubuntu remains the default to avoid
// surprising existing users; Debian is opt-in via the interactive picker or
// a hand-edited workspace.toml.
const defaultOs = "ubuntu"

// defaultOsVersion maps each supported OS to the version chosen on the --yes
// fast path. Pinned to the most-tested release of each distribution rather
// than SupportedOsVersions[os][0]: list ordering reflects display preference
// (newest first), but the safe default for unattended setups is the release
// the snapshots and CI exercise.
//
//nolint:gochecknoglobals // tabular configuration data, file-scoped by design.
var defaultOsVersion = map[string]string{
	"ubuntu": "24.04",
	"debian": "12",
}

// osVersionLabel returns the user-facing label for an os/os_version pair.
// Debian versions get a "12 (bookworm)" annotation so the picker shows both
// the numeric tag (which is what gets baked into the Dockerfile FROM line)
// and the codename (which is what most users recognize). Ubuntu values are
// returned verbatim — the numeric LTS version is already familiar.
func osVersionLabel(osID, version string) string {
	if osID == "debian" {
		switch version {
		case "12":
			return "12 (bookworm)"
		case "13":
			return "13 (trixie)"
		}
	}
	return version
}

//nolint:gocognit,gocyclo,funlen // sequential TUI prompt orchestration; splitting fragments the user-visible flow.
func runInteractive(opts Options, wsPath, pluginsDir string) (*config.Workspace, error) {
	t := opts.Catalog
	log := opts.Logger

	var existing *partialWS
	if fileExists(wsPath) {
		p, err := loadPartialWorkspace(wsPath)
		if err == nil {
			existing = p
		}
	}
	partialReconfig := existing != nil && !opts.ForceInit

	var serviceName, username string
	if opts.AutoYes {
		serviceName = "dev"
		if u, err := user.Current(); err == nil {
			username = u.Username
		} else {
			username = "user"
		}
		log.Info(t.Msg("setup_service_default", serviceName))
		log.Info(t.Msg("setup_username_current", username))
	} else {
		serviceName = promptValidated(opts.Stdin, log, t.Msg("setup_prompt_service_name"), func(s string) error {
			return validateServiceName(s, t)
		})
		username = promptValidated(opts.Stdin, log, t.Msg("setup_prompt_username"), func(s string) error {
			return validateUsername(s, t)
		})
	}

	osID, osErr := pickOs(opts, existing)
	if osErr != nil {
		return nil, osErr
	}
	osVersion, versionErr := pickOsVersion(opts, existing, osID)
	if versionErr != nil {
		return nil, versionErr
	}

	allPlugins, loadErr := plugin.LoadDir(pluginsDir)
	if loadErr != nil {
		allPlugins = map[string]*plugin.Plugin{}
	}

	pluginIDs := make([]string, 0, len(allPlugins))
	for id := range allPlugins {
		pluginIDs = append(pluginIDs, id)
	}
	sort.Strings(pluginIDs)

	log.Info(subsectionHeader(t.Msg("setup_header_software")))

	var selectedPlugins []string
	switch {
	case opts.AutoYes && partialReconfig && len(existing.Plugins.Enable) > 0:
		selectedPlugins = existing.Plugins.Enable
		for _, id := range selectedPlugins {
			log.Info(t.Msg("setup_plugin_enabled", id))
		}
	case opts.AutoYes:
		for _, id := range pluginIDs {
			if allPlugins[id].Metadata.Default {
				selectedPlugins = append(selectedPlugins, id)
				log.Info(t.Msg("setup_plugin_enabled", id))
			} else {
				log.Info(t.Msg("setup_plugin_skipped", id))
			}
		}
	default:
		labels := make([]string, len(pluginIDs))
		for i, id := range pluginIDs {
			p := allPlugins[id]
			if p.Metadata.Description != "" {
				labels[i] = p.Metadata.Name + " — " + p.Metadata.Description
			} else {
				labels[i] = p.Metadata.Name
			}
		}
		preselected := []int{}
		if partialReconfig && len(existing.Plugins.Enable) > 0 {
			existSet := make(map[string]struct{})
			for _, e := range existing.Plugins.Enable {
				existSet[e] = struct{}{}
			}
			for i, id := range pluginIDs {
				if _, ok := existSet[id]; ok {
					preselected = append(preselected, i)
				}
			}
		} else {
			for i, id := range pluginIDs {
				if allPlugins[id].Metadata.Default {
					preselected = append(preselected, i)
				}
			}
		}
		indices, err := opts.Selector.SelectMulti(t.Msg("setup_select_plugins"), labels, preselected)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrCanceled, err)
		}
		for _, idx := range indices {
			selectedPlugins = append(selectedPlugins, pluginIDs[idx])
		}
	}

	var forwardPorts []any
	switch {
	case partialReconfig && existing.Ports != nil && len(existing.Ports.Forward) > 0:
		forwardPorts = existing.Ports.Forward
		portStrs := make([]string, len(forwardPorts))
		for i, p := range forwardPorts {
			portStrs[i] = formatPortEntry(p)
		}
		log.Info(t.Msg("setup_port_default", strings.Join(portStrs, ", ")))
	case opts.AutoYes:
		forwardPorts = []any{}
		log.Info(t.Msg("setup_port_none"))
	default:
		log.Info(subsectionHeader(t.Msg("setup_header_port")))
		forwardPorts = promptPorts(opts.Stdin, log, t)
	}

	ws := buildWorkspace(serviceName, username, osID, osVersion, selectedPlugins, forwardPorts, existing)

	log.Info(t.Msg("setup_gen_workspace_toml"))
	if err := writeWorkspaceToml(wsPath, ws); err != nil {
		return nil, fmt.Errorf("write workspace.toml: %w", err)
	}

	loaded, err := config.LoadWorkspace(wsPath)
	if err != nil {
		return nil, fmt.Errorf("validate workspace.toml: %w", err)
	}
	return loaded, nil
}

// pickOs returns the base OS to write into workspace.toml. AutoYes mode uses
// the existing value when available (logged as "preserved") or defaultOs
// (logged as "default"). Interactive mode opens a single-select picker over
// config.SupportedOSes with the existing value (or defaultOs) as the initial
// cursor position.
func pickOs(opts Options, existing *partialWS) (string, error) {
	t := opts.Catalog
	log := opts.Logger

	var existingOs string
	if existing != nil && existing.Container != nil {
		existingOs = existing.Container.Os
	}

	if opts.AutoYes {
		if existingOs != "" {
			log.Info(t.Msg("setup_os_preserved", existingOs))
			return existingOs, nil
		}
		log.Info(t.Msg("setup_os_default", defaultOs))
		return defaultOs, nil
	}

	defaultIdx := indexOf(config.SupportedOSes, existingOs)
	if defaultIdx < 0 {
		defaultIdx = indexOf(config.SupportedOSes, defaultOs)
	}
	if defaultIdx < 0 {
		defaultIdx = 0
	}
	idx, err := opts.Selector.SelectSingle(
		t.Msg("setup_prompt_os"), config.SupportedOSes, defaultIdx,
	)
	if err != nil {
		if errors.Is(err, tui.ErrCanceled) {
			return "", fmt.Errorf("%w: %w", ErrCanceled, err)
		}
		return "", fmt.Errorf("select os: %w", err)
	}
	return config.SupportedOSes[idx], nil
}

// pickOsVersion returns the OS version to write into workspace.toml. The
// version list is keyed off the previously-picked osID so users only see
// versions valid for the OS they chose. The picker labels are
// version-specific (e.g. Debian shows "12 (bookworm)") while the returned
// value is the canonical numeric/dotted tag baked into the Dockerfile FROM.
func pickOsVersion(opts Options, existing *partialWS, osID string) (string, error) {
	t := opts.Catalog
	log := opts.Logger

	versions := config.SupportedOsVersions[osID]
	if len(versions) == 0 {
		return "", fmt.Errorf("%w: %q", ErrConfig, osID)
	}

	var existingVersion string
	if existing != nil && existing.Container != nil && existing.Container.Os == osID {
		existingVersion = existing.Container.OsVersion
	}

	if opts.AutoYes {
		if existingVersion != "" && indexOf(versions, existingVersion) >= 0 {
			log.Info(t.Msg("setup_os_version_preserved", existingVersion))
			return existingVersion, nil
		}
		fallback := defaultOsVersion[osID]
		if fallback == "" {
			fallback = versions[0]
		}
		log.Info(t.Msg("setup_os_version_default", fallback))
		return fallback, nil
	}

	defaultIdx := indexOf(versions, existingVersion)
	if defaultIdx < 0 {
		defaultIdx = indexOf(versions, defaultOsVersion[osID])
	}
	if defaultIdx < 0 {
		defaultIdx = 0
	}

	labels := make([]string, len(versions))
	for i, v := range versions {
		labels[i] = osVersionLabel(osID, v)
	}
	idx, err := opts.Selector.SelectSingle(
		t.Msg("setup_prompt_os_version"), labels, defaultIdx,
	)
	if err != nil {
		if errors.Is(err, tui.ErrCanceled) {
			return "", fmt.Errorf("%w: %w", ErrCanceled, err)
		}
		return "", fmt.Errorf("select os version: %w", err)
	}
	return versions[idx], nil
}

func indexOf(haystack []string, needle string) int {
	for i, s := range haystack {
		if s == needle {
			return i
		}
	}
	return -1
}

func promptValidated(in io.Reader, log *logx.Logger, prompt string, validate func(string) error) string {
	sc := bufio.NewScanner(in)
	for {
		log.Print(prompt)
		if !sc.Scan() {
			return ""
		}
		val := strings.TrimSpace(sc.Text())
		if err := validate(val); err != nil {
			log.Info(err.Error())
			continue
		}
		return val
	}
}

func validateServiceName(s string, t Translator) error {
	if !rxServiceName.MatchString(s) {
		return fmt.Errorf("%w: %s", ErrInvalidInput, t.Msg("setup_invalid_service_name", s))
	}
	return nil
}

func validateUsername(s string, t Translator) error {
	if !rxUsername.MatchString(s) {
		return fmt.Errorf("%w: %s", ErrInvalidInput, t.Msg("setup_invalid_username", s))
	}
	return nil
}

// promptPorts asks the user for a comma-separated list of integer port
// numbers. Each entry becomes a docker-compose short-form string "N:N" so the
// resulting workspace.toml is well-formed under the new []any schema. Users
// who want long-form / IP-bound / range ports edit workspace.toml directly.
func promptPorts(in io.Reader, log *logx.Logger, t Translator) []any {
	sc := bufio.NewScanner(in)
	for {
		log.Print(t.Msg("setup_prompt_port"))
		if !sc.Scan() {
			return []any{}
		}
		input := strings.TrimSpace(sc.Text())
		if input == "" {
			return []any{}
		}
		ports, err := parsePorts(input, t)
		if err != nil {
			log.Info(err.Error())
			continue
		}
		return ports
	}
}

func parsePorts(input string, t Translator) ([]any, error) {
	var ports []any
	for _, tok := range strings.Split(input, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		n, err := strconv.Atoi(tok)
		if err != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("%w: %s", ErrInvalidInput, t.Msg("setup_invalid_port", tok))
		}
		ports = append(ports, fmt.Sprintf("%d:%d", n, n))
	}
	return ports, nil
}
