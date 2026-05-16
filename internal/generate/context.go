package generate

import (
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

// WorkspaceContext is a normalized read-only view over a workspace.toml that
// generator subpackages consume.
type WorkspaceContext struct {
	WS *config.Workspace
	// PluginsFS lets generators read install scripts directly so the catalog
	// can be embedded, on-disk, or LayeredFS-merged transparently.
	PluginsFS fs.FS
	// ProjectDir holds the directory containing workspace.toml. envfile uses
	// its basename as COMPOSE_PROJECT_NAME so the namespace matches the root
	// the user invoked `cocoon gen` against. May be empty for older callers;
	// envfile falls back to ServiceName in that case.
	ProjectDir string
	// Plugins is iterated in WS.Plugins.Enable order so generators emit
	// deterministic output.
	Plugins map[string]*plugin.Plugin
	// Warnings receives non-fatal messages (e.g. TZ override). Nil drops them.
	Warnings io.Writer
}

// EnabledPlugins returns the enable list, never nil.
func (c *WorkspaceContext) EnabledPlugins() []string {
	if c.WS == nil || c.WS.Plugins.Enable == nil {
		return []string{}
	}
	return c.WS.Plugins.Enable
}

// ServiceName defaults to "dev" when unset.
func (c *WorkspaceContext) ServiceName() string {
	if c.WS == nil || c.WS.Container.ServiceName == "" {
		return "dev"
	}
	return c.WS.Container.ServiceName
}

// Username defaults to "developer" when unset.
func (c *WorkspaceContext) Username() string {
	if c.WS == nil || c.WS.Container.Username == "" {
		return "developer"
	}
	return c.WS.Container.Username
}

// ComposeForwardPorts returns nil when [ports] is absent or empty.
func (c *WorkspaceContext) ComposeForwardPorts() []config.ComposePort {
	if c.WS == nil || c.WS.Ports == nil {
		return nil
	}
	return config.ComposePortEntries(c.WS.Ports.Forward)
}

// DevcontainerForwardPorts returns nil when [ports] is absent so the
// generator can omit the key entirely rather than baking in a default the
// user never asked for. Entries that cannot reduce to a single TCP integer
// (ranges, mode=host, protocol=udp) are skipped with one warning per skip
// to c.Warnings.
func (c *WorkspaceContext) DevcontainerForwardPorts() []int {
	if c.WS == nil || c.WS.Ports == nil {
		return nil
	}
	return config.DevcontainerPortEntries(c.WS.Ports.Forward, c.Warnings)
}

// Resources returns nil when [container.resources] is absent.
func (c *WorkspaceContext) Resources() *config.Resources {
	if c.WS == nil {
		return nil
	}
	return c.WS.Container.Resources
}

// LoginShell defaults to "bash" when [container.shell].default is unset.
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

// LoginShellAptPackages returns the apt packages required for the chosen
// login shell. zsh adds zsh-autosuggestions; fish ships native completions
// so only the shell binary is needed.
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

// RCFilePath returns the rc-file path relative to $HOME (no leading "~").
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

// RCFileAbs returns the rc-file path as a $HOME-rooted absolute string.
func (c *WorkspaceContext) RCFileAbs() string {
	return "$HOME/" + c.RCFilePath()
}

// RCSyntax is "posix" for bash + zsh, "fish" otherwise.
func (c *WorkspaceContext) RCSyntax() string {
	if c.LoginShell() == "fish" {
		return "fish"
	}
	return "posix"
}

// ShellAliases never returns nil.
func (c *WorkspaceContext) ShellAliases() map[string]string {
	if c.WS == nil || c.WS.Container.Shell == nil {
		return map[string]string{}
	}
	return c.WS.Container.Shell.Aliases
}

// ShellEnv never returns nil.
func (c *WorkspaceContext) ShellEnv() map[string]string {
	if c.WS == nil || c.WS.Container.Shell == nil {
		return map[string]string{}
	}
	return c.WS.Container.Shell.Env
}

// ExtraHosts never returns nil.
func (c *WorkspaceContext) ExtraHosts() map[string]string {
	if c.WS == nil || c.WS.Container.Hosts == nil {
		return map[string]string{}
	}
	return c.WS.Container.Hosts
}

// DNSServers never returns nil.
func (c *WorkspaceContext) DNSServers() []string {
	if c.WS == nil || c.WS.Container.DNS == nil || c.WS.Container.DNS.Servers == nil {
		return []string{}
	}
	return c.WS.Container.DNS.Servers
}

// DNSSearch never returns nil.
func (c *WorkspaceContext) DNSSearch() []string {
	if c.WS == nil || c.WS.Container.DNS == nil || c.WS.Container.DNS.Search == nil {
		return []string{}
	}
	return c.WS.Container.DNS.Search
}

// Sysctls never returns nil.
func (c *WorkspaceContext) Sysctls() map[string]any {
	if c.WS == nil || c.WS.Container.Sysctls == nil {
		return map[string]any{}
	}
	return c.WS.Container.Sysctls
}

// CapAdd never returns nil.
func (c *WorkspaceContext) CapAdd() []string {
	if c.WS == nil || c.WS.Container.Capabilities == nil || c.WS.Container.Capabilities.Add == nil {
		return []string{}
	}
	return c.WS.Container.Capabilities.Add
}

// CapDrop never returns nil.
func (c *WorkspaceContext) CapDrop() []string {
	if c.WS == nil || c.WS.Container.Capabilities == nil || c.WS.Container.Capabilities.Drop == nil {
		return []string{}
	}
	return c.WS.Container.Capabilities.Drop
}

// SkelEntries never returns nil.
func (c *WorkspaceContext) SkelEntries() []config.SkelEntry {
	if c.WS == nil || c.WS.Container.Skel == nil {
		return []config.SkelEntry{}
	}
	return c.WS.Container.Skel
}

// SecurityOptions emits Compose `security_opt:` entries in a fixed order
// (seccomp, apparmor, no-new-privileges) so generated YAML is deterministic.
// Returns nil when the section is absent or empty.
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

// GroupAdd returns the user-configured Compose `group_add:` list. Never
// returns nil.
func (c *WorkspaceContext) GroupAdd() []string {
	if c.WS == nil || c.WS.Container.GroupAdd == nil {
		return []string{}
	}
	return c.WS.Container.GroupAdd
}

// Devices never returns nil.
func (c *WorkspaceContext) Devices() []string {
	if c.WS == nil || c.WS.Container.Devices == nil {
		return []string{}
	}
	return c.WS.Container.Devices
}

// IPC returns the configured IPC namespace mode or "" when unset.
func (c *WorkspaceContext) IPC() string {
	if c.WS == nil || c.WS.Container.IPC == nil {
		return ""
	}
	return *c.WS.Container.IPC
}

// Gpus returns the configured GPU request or "" when unset.
func (c *WorkspaceContext) Gpus() string {
	if c.WS == nil || c.WS.Container.Gpus == nil {
		return ""
	}
	return *c.WS.Container.Gpus
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
// generator.
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

// UserEnv never returns nil.
func (c *WorkspaceContext) UserEnv() map[string]string {
	if c.WS == nil || c.WS.Env == nil {
		return map[string]string{}
	}
	return c.WS.Env
}

// BuildEnvironment emits CONTAINER_SERVICE_NAME first, then TZ from
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

// pelletier/go-toml decodes tables into Go maps which lose insertion order,
// so sorting alphabetically is the stable choice that yields deterministic
// output.
func (c *WorkspaceContext) envOrder() []string {
	m := c.UserEnv()
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}

// Mounts never returns nil.
func (c *WorkspaceContext) Mounts() []config.Mount {
	if c.WS == nil || c.WS.Mounts == nil {
		return []config.Mount{}
	}
	return c.WS.Mounts
}

// HasHomeFiles gates the host-side touch + initializeCommand emission.
func (c *WorkspaceContext) HasHomeFiles() bool {
	return c.WS != nil && c.WS.HomeFiles != nil && len(c.WS.HomeFiles.Files) > 0
}

// HomeFilesEntries never returns nil.
func (c *WorkspaceContext) HomeFilesEntries() []string {
	if !c.HasHomeFiles() {
		return []string{}
	}
	return c.WS.HomeFiles.Files
}

// HomeFileMounts keeps Source paths as Compose interpolation
// (${HOME:?…}/<rel>) so the generated docker-compose.yml works when
// `cocoon gen` ran in one environment (e.g. inside a container) and
// `docker compose up` runs in another (the Docker host).
func (c *WorkspaceContext) HomeFileMounts() []config.Mount {
	if !c.HasHomeFiles() {
		return nil
	}
	out := make([]config.Mount, 0, len(c.WS.HomeFiles.Files))
	for _, rel := range c.WS.HomeFiles.Files {
		out = append(out, config.Mount{
			Source:   HomeFilesHostPathPrefix + "/" + rel,
			Target:   "/home/${USERNAME}/" + rel,
			Readonly: false,
		})
	}
	return out
}

// CustomVolumes never returns nil.
func (c *WorkspaceContext) CustomVolumes() map[string]string {
	if c.WS == nil || c.WS.Volumes == nil {
		return map[string]string{}
	}
	return c.WS.Volumes
}

// AptExtraPackages never returns nil.
func (c *WorkspaceContext) AptExtraPackages() []string {
	if c.WS == nil || c.WS.Apt == nil || c.WS.Apt.Packages == nil {
		return []string{}
	}
	return c.WS.Apt.Packages
}

// AptMirrorURL returns "" when unset.
func (c *WorkspaceContext) AptMirrorURL() string {
	if c.WS == nil || c.WS.Apt == nil || c.WS.Apt.Mirror == nil {
		return ""
	}
	return c.WS.Apt.Mirror.URL
}

// AptProxy returns nil when unset.
func (c *WorkspaceContext) AptProxy() *config.AptProxy {
	if c.WS == nil || c.WS.Apt == nil {
		return nil
	}
	return c.WS.Apt.Proxy
}

// AptSources never returns nil.
func (c *WorkspaceContext) AptSources() []config.AptSource {
	if c.WS == nil || c.WS.Apt == nil || c.WS.Apt.Sources == nil {
		return []config.AptSource{}
	}
	return c.WS.Apt.Sources
}

// CertificatesEnabled gates all cert-related generator output.
func (c *WorkspaceContext) CertificatesEnabled() bool {
	if c == nil || c.WS == nil {
		return false
	}
	return c.WS.Certificates.EnableOrDefault()
}

// GitUserName returns "" when unset.
func (c *WorkspaceContext) GitUserName() string {
	if c.WS == nil || c.WS.Git == nil || c.WS.Git.UserName == nil {
		return ""
	}
	return *c.WS.Git.UserName
}

// GitUserEmail returns "" when unset.
func (c *WorkspaceContext) GitUserEmail() string {
	if c.WS == nil || c.WS.Git == nil || c.WS.Git.UserEmail == nil {
		return ""
	}
	return *c.WS.Git.UserEmail
}

// DockerfilePreUserSetup returns "" when unset.
func (c *WorkspaceContext) DockerfilePreUserSetup() string {
	if c.WS == nil || c.WS.Dockerfile == nil || c.WS.Dockerfile.PreUserSetup == nil {
		return ""
	}
	return *c.WS.Dockerfile.PreUserSetup
}

// DockerfilePostPlugins returns "" when unset.
func (c *WorkspaceContext) DockerfilePostPlugins() string {
	if c.WS == nil || c.WS.Dockerfile == nil || c.WS.Dockerfile.PostPlugins == nil {
		return ""
	}
	return *c.WS.Dockerfile.PostPlugins
}

// PluginVersionOverrides never returns nil.
func (c *WorkspaceContext) PluginVersionOverrides() map[string]config.PluginVersionOverride {
	if c.WS == nil {
		return map[string]config.PluginVersionOverride{}
	}
	return c.WS.Plugins.Versions
}

// PluginMethods returns the user's [plugins.methods] map. Never nil
// (returns an empty map when the workspace has no [plugins.methods]
// table). Keys are plugin ids; values name a method declared in that
// plugin's [install.methods] section. Absent entries fall back to the
// plugin's default_method.
func (c *WorkspaceContext) PluginMethods() map[string]string {
	if c.WS == nil || c.WS.Plugins.Methods == nil {
		return map[string]string{}
	}
	return c.WS.Plugins.Methods
}

// Sidecars never returns nil. Consumers that need deterministic ordering
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

// sortStrings is split out so tests can assert lexicographic ordering
// without importing sort across every generator subpackage.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
