// Package config loads, validates and re-emits workspace.toml and plugin.toml
// files for the wsd binary.
//
// The structures defined here mirror the Pydantic models in src/wsd/config so
// the Go implementation can be swapped in for the Python CLI without changing
// the shell consumers in lib/*.sh.
//
// All struct field names are kept aligned with the legacy implementation so
// JSON/TOML round-trips remain byte-compatible. Optional sections are modelled
// as pointers so a missing section can be distinguished from an empty one
// (Pydantic's None vs. {}).
package config

// Workspace mirrors src/wsd/config.Workspace.
type Workspace struct {
	Container    ContainerSpec             `toml:"container"`
	Plugins      PluginsSpec               `toml:"plugins"`
	Ports        *PortsSpec                `toml:"ports,omitempty"`
	Apt          *AptSpec                  `toml:"apt,omitempty"`
	Volumes      map[string]string         `toml:"volumes,omitempty"`
	Env          map[string]string         `toml:"env,omitempty"`
	Mounts       []Mount                   `toml:"mounts,omitempty"`
	HomeFiles    *HomeFilesSpec            `toml:"home_files,omitempty"`
	Locale       *LocaleSpec               `toml:"locale,omitempty"`
	Git          *GitIdentitySpec          `toml:"git,omitempty"`
	Dockerfile   *DockerfileSpec           `toml:"dockerfile,omitempty"`
	Services     map[string]SidecarService `toml:"services,omitempty"`
	Repositories *RepositoriesSpec         `toml:"repositories,omitempty"`
	Devcontainer Devcontainer              `toml:"devcontainer,omitempty"`
}

// HasDevcontainer reports whether a [devcontainer] table was present.
func (w *Workspace) HasDevcontainer() bool { return len(w.Devcontainer) > 0 }

// ContainerSpec mirrors src/wsd/config.ContainerSpec. Os selects the base
// distribution ("ubuntu" or "debian"); OsVersion is the distribution-specific
// version string ("24.04" / "26.04" for Ubuntu; "12" / "13" for Debian).
type ContainerSpec struct {
	ServiceName string `toml:"service_name"`
	Username    string `toml:"username"`
	Os          string `toml:"os"`
	OsVersion   string `toml:"os_version"`

	// DeprecatedUbuntuVersion exists solely so a legacy workspace.toml that
	// still uses `ubuntu_version = "..."` can be detected and rejected with a
	// migration message. It must not be read or written outside of
	// validation. Do not feed this value into generators. The `jsonschema:"-"`
	// tag keeps the legacy field out of schemas/workspace.schema.json so editor
	// autocomplete does not advertise a key that the validator rejects.
	DeprecatedUbuntuVersion string `toml:"ubuntu_version,omitempty" jsonschema:"-"`

	Resources    *Resources          `toml:"resources,omitempty"`
	Shell        *ContainerShellSpec `toml:"shell,omitempty"`
	Hosts        map[string]string   `toml:"hosts,omitempty"`
	DNS          *DNSSpec            `toml:"dns,omitempty"`
	Sysctls      map[string]any      `toml:"sysctls,omitempty"`
	Capabilities *CapabilitiesSpec   `toml:"capabilities,omitempty"`
	SecurityOpt  *SecurityOptSpec    `toml:"security_opt,omitempty"`
	Skel         []SkelEntry         `toml:"skel,omitempty"`
}

// SupportedOSes is the closed set of base distributions the generator can
// build a Dockerfile for. Validation, the interactive picker and the
// Dockerfile template all key off this list.
//
//nolint:gochecknoglobals // tabular configuration data, file-scoped by design.
var SupportedOSes = []string{"ubuntu", "debian"}

// SupportedOsVersions maps an OS id to the closed set of version values that
// validation accepts. Keys must match SupportedOSes; the version strings are
// what users put in `os_version = "..."` and what gets baked into the
// Dockerfile FROM line (e.g. `FROM debian:12`, `FROM ubuntu:24.04`).
//
//nolint:gochecknoglobals // tabular configuration data, file-scoped by design.
var SupportedOsVersions = map[string][]string{
	"ubuntu": {"26.04", "24.04", "22.04"},
	"debian": {"13", "12"},
}

// SkelEntry mirrors one [[container.skel]] entry. Source is a path relative
// to the workspace root (the docker build context); Target is a path relative
// to /etc/skel (which useradd -m copies into the new user's home). Use this
// to seed dotfiles like .editorconfig / .gitignore_global / .tmux.conf into
// the dev container's home directory declaratively.
type SkelEntry struct {
	Source string `toml:"source"`
	Target string `toml:"target"`
}

// SecurityOptSpec mirrors the [container.security_opt] table. Each field maps
// to one Compose `security_opt:` list entry. Set Seccomp / AppArmor to
// "unconfined" (or a profile name) to relax sandboxing; set NoNewPrivileges
// to true to harden the container by blocking setuid privilege escalation.
type SecurityOptSpec struct {
	Seccomp         *string `toml:"seccomp,omitempty"`
	AppArmor        *string `toml:"apparmor,omitempty"`
	NoNewPrivileges *bool   `toml:"no_new_privileges,omitempty"`
}

// CapabilitiesSpec mirrors the [container.capabilities] table. Add and Drop
// each become a Compose `cap_add:` / `cap_drop:` list entry verbatim. Values
// are bare capability names without the `CAP_` prefix (e.g. "SYS_PTRACE").
type CapabilitiesSpec struct {
	Add  []string `toml:"add,omitempty"`
	Drop []string `toml:"drop,omitempty"`
}

// DNSSpec mirrors the [container.dns] table. Servers populate Compose's `dns:`
// key (one IP per entry); Search populates `dns_search:` (TLD-style suffixes
// applied to short names).
type DNSSpec struct {
	Servers []string `toml:"servers,omitempty"`
	Search  []string `toml:"search,omitempty"`
}

// ContainerShellSpec mirrors the [container.shell] table introduced in v8.0.0.
//
// Default selects the login shell ("bash" | "zsh" | "fish"); unset/empty falls
// back to "bash". Aliases and Env are written into the chosen shell's rc file
// (config/.bashrc_custom.generated, .zshrc_custom.generated, or
// config.fish_custom.generated) using shell-appropriate syntax.
type ContainerShellSpec struct {
	Default *string           `toml:"default,omitempty"`
	Aliases map[string]string `toml:"aliases,omitempty"`
	Env     map[string]string `toml:"env,omitempty"`
}

// Resources mirrors src/wsd/config.Resources.
type Resources struct {
	ShmSize         *string  `toml:"shm_size,omitempty"`
	PidsLimit       *int     `toml:"pids_limit,omitempty"`
	StopGracePeriod *string  `toml:"stop_grace_period,omitempty"`
	CPUs            *float64 `toml:"cpus,omitempty"`
	Memory          *string  `toml:"memory,omitempty"`
	NofileSoft      *int     `toml:"nofile_soft,omitempty"`
	NofileHard      *int     `toml:"nofile_hard,omitempty"`
}

// PluginsSpec mirrors src/wsd/config.PluginsSpec.
type PluginsSpec struct {
	Enable   []string                         `toml:"enable"`
	Versions map[string]PluginVersionOverride `toml:"versions,omitempty"`
}

// PluginVersionOverride mirrors src/wsd/config.PluginVersionOverride.
type PluginVersionOverride struct {
	Pin           string  `toml:"pin"`
	ChecksumAmd64 *string `toml:"checksum_amd64,omitempty"`
	ChecksumArm64 *string `toml:"checksum_arm64,omitempty"`
}

// PortsSpec mirrors src/wsd/config.PortsSpec. Each Forward entry is either a
// docker-compose short-form string ("3000:3000", "127.0.0.1:5432:5432/tcp",
// "3000-3005:3000-3005") or a long-form table with the keys target,
// published, host_ip, protocol, mode. See ComposePortEntries for the
// normalized representation consumed by generators.
type PortsSpec struct {
	Forward []any `toml:"forward"`
}

// AptSpec mirrors src/wsd/config.AptSpec.
type AptSpec struct {
	Packages []string    `toml:"packages,omitempty"`
	Mirror   *AptMirror  `toml:"mirror,omitempty"`
	Proxy    *AptProxy   `toml:"proxy,omitempty"`
	Sources  []AptSource `toml:"sources,omitempty"`
}

// AptMirror swaps the default Ubuntu archive URLs out for a regional or
// internal mirror. The generator rewrites archive.ubuntu.com,
// security.ubuntu.com, and ports.ubuntu.com to URL.
type AptMirror struct {
	URL string `toml:"url"`
}

// AptProxy sets HTTP/HTTPS proxies for apt by writing
// /etc/apt/apt.conf.d/95proxy at build time. Either field may be omitted.
type AptProxy struct {
	HTTP  *string `toml:"http,omitempty"`
	HTTPS *string `toml:"https,omitempty"`
}

// AptSource declares one third-party apt repository, including its GPG key.
// The generator places the key under /etc/apt/keyrings/<Name>.gpg and writes
// /etc/apt/sources.list.d/<Name>.list referring to it via signed-by, before
// the main apt-get update so the repository is visible during install.
type AptSource struct {
	Name       string   `toml:"name"`
	Suite      string   `toml:"suite"`
	Components []string `toml:"components"`
	URL        string   `toml:"url"`
	KeyURL     string   `toml:"key_url"`
	Arch       *string  `toml:"arch,omitempty"`
}

// LocaleSpec mirrors src/wsd/config.LocaleSpec.
type LocaleSpec struct {
	Timezone *string `toml:"timezone,omitempty"`
	Lang     *string `toml:"lang,omitempty"`
}

// GitIdentitySpec mirrors src/wsd/config.GitIdentitySpec.
type GitIdentitySpec struct {
	UserName  *string `toml:"user_name,omitempty"`
	UserEmail *string `toml:"user_email,omitempty"`
}

// DockerfileSpec mirrors src/wsd/config.DockerfileSpec.
type DockerfileSpec struct {
	PreUserSetup *string `toml:"pre_user_setup,omitempty"`
	PostPlugins  *string `toml:"post_plugins,omitempty"`
}

// Mount mirrors src/wsd/config.Mount.
type Mount struct {
	Source   string `toml:"source"`
	Target   string `toml:"target"`
	Readonly bool   `toml:"readonly"`
}

// HomeFilesSpec declares single files under the host user's home directory
// to persist across container rebuilds. Each entry is a path relative to ~/
// on both host and container. wsd setup touches missing host files (0o600,
// idempotent) before they are bind-mounted, so Docker does not auto-create
// them as directories. Use [volumes] for whole directories and [[mounts]]
// for arbitrary host paths; [home_files] is the narrow case of single files
// in $HOME that must outlive the container's writable layer.
type HomeFilesSpec struct {
	Files []string `toml:"files"`
}

// SidecarMount mirrors src/wsd/config.SidecarMount.
type SidecarMount struct {
	Source   string `toml:"source"`
	Target   string `toml:"target"`
	Readonly bool   `toml:"readonly"`
}

// SidecarRestart enumerates the allowed values for SidecarService.Restart.
type SidecarRestart string

// Sidecar restart policy values (mirrors compose v3 spec subset).
const (
	RestartNo            SidecarRestart = "no"
	RestartAlways        SidecarRestart = "always"
	RestartOnFailure     SidecarRestart = "on-failure"
	RestartUnlessStopped SidecarRestart = "unless-stopped"
)

// HealthcheckSpec mirrors src/wsd/config.HealthcheckSpec which allows extras.
type HealthcheckSpec map[string]any

// SidecarService mirrors src/wsd/config.SidecarService.
type SidecarService struct {
	Image       string            `toml:"image"`
	Ports       []any             `toml:"ports,omitempty"`
	Env         map[string]string `toml:"env,omitempty"`
	Volumes     map[string]string `toml:"volumes,omitempty"`
	Mounts      []SidecarMount    `toml:"mounts,omitempty"`
	Command     any               `toml:"command,omitempty"`
	DependsOn   []string          `toml:"depends_on,omitempty"`
	Healthcheck HealthcheckSpec   `toml:"healthcheck,omitempty"`
	Restart     *SidecarRestart   `toml:"restart,omitempty"`
}

// RepositoryClone mirrors src/wsd/config.RepositoryClone.
type RepositoryClone struct {
	URL               string  `toml:"url"`
	Path              *string `toml:"path,omitempty"`
	Branch            *string `toml:"branch,omitempty"`
	Depth             *int    `toml:"depth,omitempty"`
	RecurseSubmodules *bool   `toml:"recurse_submodules,omitempty"`
}

// RepositoriesSpec mirrors src/wsd/config.RepositoriesSpec.
type RepositoriesSpec struct {
	Clone []RepositoryClone `toml:"clone"`
}

// Devcontainer mirrors src/wsd/config.Devcontainer (extra=allow). All known
// fields fall through into the map and the dump-devcontainer subcommand emits
// the entries verbatim.
type Devcontainer map[string]any
