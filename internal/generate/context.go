package generate

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

// WorkspaceContext is a normalized read-only view over a workspace.toml that
// generator subpackages consume. It wraps a typed *config.Workspace (already
// validated upstream) and the per-workspace plugin file source so
// generators can read install scripts and plugin assets directly without
// staging them on the host filesystem first.
type WorkspaceContext struct {
	WS *config.Workspace
	// PluginsFS is the layered file source rooted at the plugin catalog
	// (each id is a top-level entry). dockerfile/plugins.go reads
	// install.sh / install_user.sh through this so the catalog can be
	// embedded, on-disk, or LayeredFS-merged transparently.
	PluginsFS fs.FS
	// ProjectDir is the directory holding workspace.toml. envfile uses its
	// basename as COMPOSE_PROJECT_NAME so the docker-compose namespace
	// matches the project root the user invoked `cocoon gen` against,
	// rather than aliasing it to container.service_name. May be empty for
	// older callers; envfile falls back to ServiceName in that case.
	ProjectDir string
	// Plugins maps plugin id -> loaded plugin definition. Iteration order is
	// fixed to WS.Plugins.Enable so generators emit deterministic output.
	Plugins map[string]*plugin.Plugin
	// Warnings receives non-fatal messages (e.g. TZ override) at construction
	// time. When nil, warnings are dropped. Generator wrappers that want
	// stderr behaviour identical to the Python implementation should pass
	// os.Stderr.
	Warnings io.Writer
}

// EnabledPlugins returns the configured enable list (never nil).
func (c *WorkspaceContext) EnabledPlugins() []string {
	if c.WS == nil || c.WS.Plugins.Enable == nil {
		return []string{}
	}
	return c.WS.Plugins.Enable
}

// ServiceName returns container.service_name with the Python default ("dev").
func (c *WorkspaceContext) ServiceName() string {
	if c.WS == nil || c.WS.Container.ServiceName == "" {
		return "dev"
	}
	return c.WS.Container.ServiceName
}

// Username returns container.username with the Python default ("developer").
func (c *WorkspaceContext) Username() string {
	if c.WS == nil || c.WS.Container.Username == "" {
		return "developer"
	}
	return c.WS.Container.Username
}

// ComposeForwardPorts returns the [ports].forward entries normalized for
// docker-compose YAML emission. Empty when [ports] is absent or empty.
func (c *WorkspaceContext) ComposeForwardPorts() []config.ComposePort {
	if c.WS == nil || c.WS.Ports == nil {
		return nil
	}
	return config.ComposePortEntries(c.WS.Ports.Forward)
}

// DevcontainerForwardPorts returns published-port integers usable by
// devcontainer.json's `forwardPorts`. When [ports] is absent the default
// [3000] is returned so the IDE still forwards the canonical port. Entries
// that cannot be reduced to a single port (port ranges, mode=host) are
// skipped, with one warning per skip emitted to c.Warnings.
func (c *WorkspaceContext) DevcontainerForwardPorts() []int {
	if c.WS == nil || c.WS.Ports == nil {
		return []int{3000}
	}
	return config.DevcontainerPortEntries(c.WS.Ports.Forward, c.Warnings)
}

// Resources returns [container.resources] (nil when absent).
func (c *WorkspaceContext) Resources() *config.Resources {
	if c.WS == nil {
		return nil
	}
	return c.WS.Container.Resources
}

// LoginShell returns the configured login shell ("bash" | "zsh" | "fish"),
// defaulting to "bash" when [container.shell].default is unset or empty.
func (c *WorkspaceContext) LoginShell() string {
	if c.WS == nil || c.WS.Container.Shell == nil || c.WS.Container.Shell.Default == nil {
		return "bash"
	}
	if v := *c.WS.Container.Shell.Default; v != "" {
		return v
	}
	return "bash"
}

// LoginShellPath returns the absolute binary path used by `useradd -s`.
func (c *WorkspaceContext) LoginShellPath() string {
	switch c.LoginShell() {
	case "fish":
		return "/usr/bin/fish"
	case "zsh":
		return "/usr/bin/zsh"
	default:
		return "/bin/bash"
	}
}

// LoginShellAptPackages returns apt packages required for the chosen login
// shell. bash gets bash-completion; zsh gets the shell binary plus
// zsh-autosuggestions; fish gets the shell binary (fish ships native
// completions).
func (c *WorkspaceContext) LoginShellAptPackages() []string {
	switch c.LoginShell() {
	case "zsh":
		return []string{"zsh", "zsh-autosuggestions"}
	case "fish":
		return []string{"fish"}
	default:
		return []string{"bash-completion"}
	}
}

// RCFilePath returns the rc-file path for the chosen login shell, relative to
// $HOME (no leading "~").
func (c *WorkspaceContext) RCFilePath() string {
	switch c.LoginShell() {
	case "fish":
		return ".config/fish/config.fish"
	case "zsh":
		return ".zshrc"
	default:
		return ".bashrc"
	}
}

// RCFileAbs returns the rc-file path as a Dockerfile-friendly $HOME-rooted
// absolute string ("$HOME/.bashrc" etc.).
func (c *WorkspaceContext) RCFileAbs() string {
	return "$HOME/" + c.RCFilePath()
}

// RCSyntax indicates which alias/export dialect the rc-file expects.
// "posix" covers bash + zsh; "fish" is its own dialect.
func (c *WorkspaceContext) RCSyntax() string {
	if c.LoginShell() == "fish" {
		return "fish"
	}
	return "posix"
}

// ShellAliases returns [container.shell].aliases (never nil).
func (c *WorkspaceContext) ShellAliases() map[string]string {
	if c.WS == nil || c.WS.Container.Shell == nil {
		return map[string]string{}
	}
	return c.WS.Container.Shell.Aliases
}

// ShellEnv returns [container.shell].env (never nil).
func (c *WorkspaceContext) ShellEnv() map[string]string {
	if c.WS == nil || c.WS.Container.Shell == nil {
		return map[string]string{}
	}
	return c.WS.Container.Shell.Env
}

// ExtraHosts returns [container.hosts] (never nil).
func (c *WorkspaceContext) ExtraHosts() map[string]string {
	if c.WS == nil || c.WS.Container.Hosts == nil {
		return map[string]string{}
	}
	return c.WS.Container.Hosts
}

// DNSServers returns [container.dns].servers (never nil).
func (c *WorkspaceContext) DNSServers() []string {
	if c.WS == nil || c.WS.Container.DNS == nil || c.WS.Container.DNS.Servers == nil {
		return []string{}
	}
	return c.WS.Container.DNS.Servers
}

// DNSSearch returns [container.dns].search (never nil).
func (c *WorkspaceContext) DNSSearch() []string {
	if c.WS == nil || c.WS.Container.DNS == nil || c.WS.Container.DNS.Search == nil {
		return []string{}
	}
	return c.WS.Container.DNS.Search
}

// Sysctls returns [container.sysctls] (never nil).
func (c *WorkspaceContext) Sysctls() map[string]any {
	if c.WS == nil || c.WS.Container.Sysctls == nil {
		return map[string]any{}
	}
	return c.WS.Container.Sysctls
}

// CapAdd returns [container.capabilities].add (never nil).
func (c *WorkspaceContext) CapAdd() []string {
	if c.WS == nil || c.WS.Container.Capabilities == nil || c.WS.Container.Capabilities.Add == nil {
		return []string{}
	}
	return c.WS.Container.Capabilities.Add
}

// CapDrop returns [container.capabilities].drop (never nil).
func (c *WorkspaceContext) CapDrop() []string {
	if c.WS == nil || c.WS.Container.Capabilities == nil || c.WS.Container.Capabilities.Drop == nil {
		return []string{}
	}
	return c.WS.Container.Capabilities.Drop
}

// SkelEntries returns [[container.skel]] (never nil).
func (c *WorkspaceContext) SkelEntries() []config.SkelEntry {
	if c.WS == nil || c.WS.Container.Skel == nil {
		return []config.SkelEntry{}
	}
	return c.WS.Container.Skel
}

// SecurityOptions assembles the Compose `security_opt:` list from
// [container.security_opt]. Returns nil when the section is absent or empty.
// Order is fixed (seccomp, apparmor, no-new-privileges) so generated YAML is
// deterministic.
func (c *WorkspaceContext) SecurityOptions() []string {
	if c.WS == nil || c.WS.Container.SecurityOpt == nil {
		return nil
	}
	s := c.WS.Container.SecurityOpt
	out := make([]string, 0, 3)
	if s.Seccomp != nil {
		out = append(out, "seccomp="+*s.Seccomp)
	}
	if s.AppArmor != nil {
		out = append(out, "apparmor="+*s.AppArmor)
	}
	if s.NoNewPrivileges != nil && *s.NoNewPrivileges {
		out = append(out, "no-new-privileges:true")
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// LocaleLang returns locale.lang or "" if unset.
func (c *WorkspaceContext) LocaleLang() string {
	if c.WS == nil || c.WS.Locale == nil || c.WS.Locale.Lang == nil {
		return ""
	}
	return *c.WS.Locale.Lang
}

// LocaleTimezone returns locale.timezone or "" if unset.
func (c *WorkspaceContext) LocaleTimezone() string {
	if c.WS == nil || c.WS.Locale == nil || c.WS.Locale.Timezone == nil {
		return ""
	}
	return *c.WS.Locale.Timezone
}

// ResolveLocale returns (gen_list, lang, language) used by the Dockerfile
// generator. Mirrors WorkspaceContext.resolve_locale in Python.
func (c *WorkspaceContext) ResolveLocale() (genList, lang, language string) {
	l := c.LocaleLang()
	if l == "" {
		return "en_US.UTF-8", "en_US.UTF-8", "en_US:en"
	}
	base := l
	if i := strings.Index(l, "."); i >= 0 {
		base = l[:i]
	}
	language = base + ":en"
	if l == "en_US.UTF-8" {
		genList = "en_US.UTF-8"
	} else {
		genList = "en_US.UTF-8 " + l
	}
	return genList, l, language
}

// UserEnv returns the [env] map (never nil).
func (c *WorkspaceContext) UserEnv() map[string]string {
	if c.WS == nil || c.WS.Env == nil {
		return map[string]string{}
	}
	return c.WS.Env
}

// BuildEnvironment builds the container environment list. Mirrors the Python
// build_environment(): CONTAINER_SERVICE_NAME first, then TZ from
// [locale].timezone (which overrides any [env].TZ with a stderr warning when
// they differ), followed by remaining [env] entries in insertion order.
func (c *WorkspaceContext) BuildEnvironment() []string {
	out := []string{"CONTAINER_SERVICE_NAME=${CONTAINER_SERVICE_NAME}"}
	tz := c.LocaleTimezone()
	envMap := c.UserEnv()
	envOrder := c.envOrder()

	emittedTZ := false
	for _, k := range envOrder {
		v := envMap[k]
		if k == "TZ" {
			if tz != "" {
				if v != tz && c.Warnings != nil {
					fmt.Fprintf(c.Warnings,
						"WARNING: [env].TZ='%s' is overridden by [locale].timezone='%s'.\n",
						v, tz)
				}
				out = append(out, "TZ="+tz)
			} else {
				out = append(out, "TZ="+v)
			}
			emittedTZ = true
			continue
		}
		out = append(out, k+"="+v)
	}
	if !emittedTZ && tz != "" {
		out = append(out, "TZ="+tz)
	}
	return out
}

// envOrder returns the [env] keys in alphabetical order. pelletier/go-toml
// decodes tables into Go maps which lose insertion order, so sorting is the
// stable choice that yields deterministic output.
func (c *WorkspaceContext) envOrder() []string {
	m := c.UserEnv()
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}

// Mounts returns the [[mounts]] entries (never nil).
func (c *WorkspaceContext) Mounts() []config.Mount {
	if c.WS == nil || c.WS.Mounts == nil {
		return []config.Mount{}
	}
	return c.WS.Mounts
}

// HomeFileMounts synthesizes one config.Mount per [home_files].files entry,
// each pointing at <host_home>/<rel> -> /home/${USERNAME}/<rel>. Source paths
// are expanded to the absolute host home at generation time so the resulting
// docker-compose.yml is unambiguous (no ${HOME} indirection). Container-side
// USERNAME is left as a Compose variable for parity with [[mounts]] output.
//
// The host home is resolved via os.UserHomeDir(); the homeDir argument lets
// tests inject a fake $HOME without touching the process environment. Pass
// an empty string in production to use os.UserHomeDir.
func (c *WorkspaceContext) HomeFileMounts(homeDir string) ([]config.Mount, error) {
	if c.WS == nil || c.WS.HomeFiles == nil || len(c.WS.HomeFiles.Files) == 0 {
		return nil, nil
	}
	if homeDir == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("home_files: resolve home: %w", err)
		}
		homeDir = h
	}
	out := make([]config.Mount, 0, len(c.WS.HomeFiles.Files))
	for _, rel := range c.WS.HomeFiles.Files {
		out = append(out, config.Mount{
			Source:   filepath.Join(homeDir, rel),
			Target:   "/home/${USERNAME}/" + rel,
			Readonly: false,
		})
	}
	return out, nil
}

// CustomVolumes returns the [volumes] map (never nil).
func (c *WorkspaceContext) CustomVolumes() map[string]string {
	if c.WS == nil || c.WS.Volumes == nil {
		return map[string]string{}
	}
	return c.WS.Volumes
}

// AptExtraPackages returns [apt].packages (never nil).
func (c *WorkspaceContext) AptExtraPackages() []string {
	if c.WS == nil || c.WS.Apt == nil || c.WS.Apt.Packages == nil {
		return []string{}
	}
	return c.WS.Apt.Packages
}

// AptMirrorURL returns [apt.mirror].url or "" when unset.
func (c *WorkspaceContext) AptMirrorURL() string {
	if c.WS == nil || c.WS.Apt == nil || c.WS.Apt.Mirror == nil {
		return ""
	}
	return c.WS.Apt.Mirror.URL
}

// AptProxy returns [apt.proxy] (nil when unset).
func (c *WorkspaceContext) AptProxy() *config.AptProxy {
	if c.WS == nil || c.WS.Apt == nil {
		return nil
	}
	return c.WS.Apt.Proxy
}

// AptSources returns [[apt.sources]] (never nil).
func (c *WorkspaceContext) AptSources() []config.AptSource {
	if c.WS == nil || c.WS.Apt == nil || c.WS.Apt.Sources == nil {
		return []config.AptSource{}
	}
	return c.WS.Apt.Sources
}

// CertificatesEnabled returns true iff `[certificates] enable = true`.
// Generators branch on this to gate all cert-related output.
func (c *WorkspaceContext) CertificatesEnabled() bool {
	if c == nil || c.WS == nil {
		return false
	}
	return c.WS.Certificates.EnableOrDefault()
}

// GitUserName returns [git].user_name or "".
func (c *WorkspaceContext) GitUserName() string {
	if c.WS == nil || c.WS.Git == nil || c.WS.Git.UserName == nil {
		return ""
	}
	return *c.WS.Git.UserName
}

// GitUserEmail returns [git].user_email or "".
func (c *WorkspaceContext) GitUserEmail() string {
	if c.WS == nil || c.WS.Git == nil || c.WS.Git.UserEmail == nil {
		return ""
	}
	return *c.WS.Git.UserEmail
}

// DockerfilePreUserSetup returns [dockerfile].pre_user_setup or "".
func (c *WorkspaceContext) DockerfilePreUserSetup() string {
	if c.WS == nil || c.WS.Dockerfile == nil || c.WS.Dockerfile.PreUserSetup == nil {
		return ""
	}
	return *c.WS.Dockerfile.PreUserSetup
}

// DockerfilePostPlugins returns [dockerfile].post_plugins or "".
func (c *WorkspaceContext) DockerfilePostPlugins() string {
	if c.WS == nil || c.WS.Dockerfile == nil || c.WS.Dockerfile.PostPlugins == nil {
		return ""
	}
	return *c.WS.Dockerfile.PostPlugins
}

// PluginVersionOverrides returns [plugins.versions] (never nil).
func (c *WorkspaceContext) PluginVersionOverrides() map[string]config.PluginVersionOverride {
	if c.WS == nil {
		return map[string]config.PluginVersionOverride{}
	}
	return c.WS.Plugins.Versions
}

// Sidecars returns the [services] map (never nil). The Python implementation
// preserves TOML insertion order; consumers that need deterministic ordering
// should iterate via SidecarNames().
func (c *WorkspaceContext) Sidecars() map[string]config.SidecarService {
	if c.WS == nil || c.WS.Services == nil {
		return map[string]config.SidecarService{}
	}
	return c.WS.Services
}

// SidecarNames returns sidecar service names in alphabetical order.
func (c *WorkspaceContext) SidecarNames() []string {
	m := c.Sidecars()
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}

// DevcontainerOverrides returns a shallow copy of the [devcontainer] table.
func (c *WorkspaceContext) DevcontainerOverrides() map[string]any {
	if c.WS == nil || len(c.WS.Devcontainer) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(c.WS.Devcontainer))
	for k, v := range c.WS.Devcontainer {
		out[k] = v
	}
	return out
}

// sortStrings is split into its own helper so tests can assert lexicographic
// ordering without importing sort across every generator subpackage.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
