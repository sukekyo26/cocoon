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
	Workspace    *WorkspaceSpec            `toml:"workspace,omitempty"`
	Container    ContainerSpec             `toml:"container"`
	Plugins      PluginsSpec               `toml:"plugins"`
	Ports        *PortsSpec                `toml:"ports,omitempty"`
	Apt          *AptSpec                  `toml:"apt,omitempty"`
	Volumes      map[string]string         `toml:"volumes,omitempty"`
	Env          map[string]string         `toml:"env,omitempty"`
	Mounts       []Mount                   `toml:"mounts,omitempty"`
	HomeFiles    *HomeFilesSpec            `toml:"home_files,omitempty"`
	Locale       *LocaleSpec               `toml:"locale,omitempty"`
	Certificates *CertificatesSpec         `toml:"certificates,omitempty"`
	Git          *GitIdentitySpec          `toml:"git,omitempty"`
	Dockerfile   *DockerfileSpec           `toml:"dockerfile,omitempty"`
	Services     map[string]SidecarService `toml:"services,omitempty"`
	Repositories *RepositoriesSpec         `toml:"repositories,omitempty"`
	Devcontainer Devcontainer              `toml:"devcontainer,omitempty"`
}

// HasDevcontainer reports whether a [devcontainer] table was present.
func (w *Workspace) HasDevcontainer() bool { return len(w.Devcontainer) > 0 }

// WorkspaceSpec configures cocoon-level knobs that affect how artifacts
// are generated and where they live. The whole section is optional; when
// it is missing or fields are zero, the defaults below apply.
type WorkspaceSpec struct {
	// MountRoot selects which host directory is bind-mounted into the
	// container at /workspace.
	//
	//   "."  — mount the project directory itself (default).
	//   ".." — mount the parent directory so sibling repos are visible.
	//
	// The picker in `cocoon init` writes one of these two values and
	// nothing else; loaders enforce the same constraint.
	MountRoot string `toml:"mount_root,omitempty"`

	// DevContainer toggles emission of .devcontainer/devcontainer.json.
	// nil ⇒ default (true). Pointer so the loader can distinguish
	// "field omitted" from an explicit `devcontainer = false`.
	DevContainer *bool `toml:"devcontainer,omitempty"`
}

// MountRootOrDefault returns the configured mount range, falling back to
// "." when [workspace] is omitted or mount_root is empty.
func (w *WorkspaceSpec) MountRootOrDefault() string {
	if w == nil || w.MountRoot == "" {
		return "."
	}
	return w.MountRoot
}

// DevContainerOrDefault returns true unless the user explicitly set
// `devcontainer = false`.
func (w *WorkspaceSpec) DevContainerOrDefault() bool {
	if w == nil || w.DevContainer == nil {
		return true
	}
	return *w.DevContainer
}

// ContainerSpec mirrors src/wsd/config.ContainerSpec. Image is the
// canonical DockerHub image name pulled verbatim into the FROM line:
// "ubuntu" / "debian" / "node" / "python" / "golang" / "rust" /
// "denoland/deno". ImageVersion is the image-specific tag ("26.04" for
// Ubuntu, "13" for Debian, "26-bookworm-slim" for Node,
// "3.14-slim-bookworm" for Python, "1.26.3-bookworm" for Golang,
// "1.95-bookworm" for Rust, "debian-2.7.14" for denoland/deno). The
// closed image set lives in SupportedImages; per-image suggestions in
// SupportedImageVersions. Using canonical names means a reader can
// recreate the FROM line from workspace.toml alone — no alias
// resolution required.
type ContainerSpec struct {
	ServiceName  string `toml:"service_name"`
	Username     string `toml:"username"`
	Image        string `toml:"image"`
	ImageVersion string `toml:"image_version"`

	// DeprecatedOs / DeprecatedOsVersion exist solely so a legacy
	// workspace.toml that still uses `os = "..."` / `os_version = "..."` (the
	// v0.2.x shape) can be detected and rejected with a migration message
	// pointing at the new `image` / `image_version` keys. They must not be
	// read or written outside of validation; do not feed these values into
	// generators. The `jsonschema:"-"` tag keeps the legacy fields out of
	// schemas/workspace.schema.json so editor autocomplete does not advertise
	// keys that the validator rejects.
	DeprecatedOs        string `toml:"os,omitempty" jsonschema:"-"`
	DeprecatedOsVersion string `toml:"os_version,omitempty" jsonschema:"-"`

	Resources    *Resources          `toml:"resources,omitempty"`
	Shell        *ContainerShellSpec `toml:"shell,omitempty"`
	Hosts        map[string]string   `toml:"hosts,omitempty"`
	DNS          *DNSSpec            `toml:"dns,omitempty"`
	Sysctls      map[string]any      `toml:"sysctls,omitempty"`
	Capabilities *CapabilitiesSpec   `toml:"capabilities,omitempty"`
	SecurityOpt  *SecurityOptSpec    `toml:"security_opt,omitempty"`
	Skel         []SkelEntry         `toml:"skel,omitempty"`

	// DockerSocket opts in to bind-mounting /var/run/docker.sock into the
	// container so docker-in-docker workflows can reach the host daemon.
	// nil ⇒ default (false). Pointer so that the loader can distinguish
	// "field omitted" from an explicit `docker_socket = false`.
	//
	// When true the generators add the bind mount and a `group_add:
	// ${DOCKER_GID}` entry so the container's user can talk to the socket.
	DockerSocket *bool `toml:"docker_socket,omitempty"`
}

// DockerSocketEnabled returns true when the user opted in to mounting
// /var/run/docker.sock. False is the secure default.
func (c *ContainerSpec) DockerSocketEnabled() bool {
	return c.DockerSocket != nil && *c.DockerSocket
}

// SupportedImages is the closed set of base container images the generator
// can build a Dockerfile for. Validation, the interactive picker and the
// Dockerfile template all key off this list.
//
// Each id is the **canonical DockerHub image name** — exactly what appears
// in the FROM line — so users can read a workspace.toml and know which
// image is pulled without any cocoon-side aliasing:
//
//   - Linux distributions: "ubuntu" / "debian" — library/ namespace.
//   - Language-runtime official images: "node" / "python" / "golang" / "rust"
//     — library/ namespace. The runtime is pre-installed; cocoon plugins
//     add tools around it.
//   - Vendor-published runtime: "denoland/deno" — vendor namespace,
//     spelled out so the FROM line is unambiguous.
//
// Every image is apt-based so the cocoon plugin catalog works the same
// way across all of them. ubuntu pulls its own archive (archive.ubuntu.com);
// the other six are Debian (bookworm) variants and pull from
// deb.debian.org. apt-mirror rewriting keys off this distinction —
// see aptMirrorOriginHosts in internal/generate/dockerfile/dockerfile.go.
//
//nolint:gochecknoglobals // tabular configuration data, file-scoped by design.
var SupportedImages = []string{"ubuntu", "debian", "node", "python", "golang", "rust", "denoland/deno"}

// SupportedShells is the closed set of login shells `cocoon init` can pick
// and that [container.shell].default validates against. The Dockerfile and
// shellrc generators already branch on these three.
//
//nolint:gochecknoglobals // tabular configuration data, file-scoped by design.
var SupportedShells = []string{"bash", "zsh", "fish"}

// SupportedImageVersions maps an image id to the curated tag list cocoon
// suggests in `cocoon init`'s picker and validates auto-completion against.
// Unlike SupportedImages, this is **not a closed set** for validation —
// validateImage accepts any tag that matches rxImageVersion (alnum + dot +
// underscore + hyphen) so users can pin patch versions or new minors that
// cocoon has not yet baked into its whitelist (e.g. golang:1.26.4-bookworm
// the day it ships). Tags in this map appear as quick picks in the
// interactive flow; an "Other (manual input)" option lets users type any
// other tag at the prompt. The first entry per key is the default
// `cocoon init` picks when `--image-version` is omitted.
//
//nolint:gochecknoglobals // tabular configuration data, file-scoped by design.
var SupportedImageVersions = map[string][]string{
	"ubuntu":        {"26.04", "24.04", "22.04"},
	"debian":        {"13", "12"},
	"node":          {"26-bookworm-slim", "24-bookworm-slim", "22-bookworm-slim"},
	"python":        {"3.14-slim-bookworm", "3.13-slim-bookworm", "3.12-slim-bookworm"},
	"golang":        {"1.26.3-bookworm", "1.26-bookworm", "1.25-bookworm", "1.24-bookworm"},
	"rust":          {"1.95-bookworm", "1.94-bookworm", "1.93-bookworm"},
	"denoland/deno": {"debian-2.7.14", "debian-2.6.10", "debian-2.5.7"},
}

// ImageProvidesPlugin marks images that already pre-install a language
// and must not be combined with the cocoon plugin of the same name. The
// mapping is image id (= canonical DockerHub name) → conflicting plugin
// id (= cocoon catalog id); validation rejects workspace.toml files that
// set `image = <key>` AND enable the named plugin so users get a
// fail-fast error instead of silently wasting docker-build time.
//
// Why the listed pairs conflict:
//
//   - "golang" → "go": the go plugin runs `tar -C /usr/local -xzf go.tar.gz`
//     which directly overwrites the /usr/local/go directory the golang
//     base image ships, making the base layer dead weight.
//   - "rust" → "rust": the rust plugin's PATH = "$HOME/.cargo/bin:$PATH"
//     prepends the user-local cargo dir ahead of the base image's
//     /usr/local/cargo/bin, so the base toolchain is shadowed and never
//     used.
//
// Other supported images (ubuntu, debian, node, python, denoland/deno)
// have no matching plugin in the catalog and therefore no conflict to
// declare here.
//
//nolint:gochecknoglobals // tabular configuration data, file-scoped by design.
var ImageProvidesPlugin = map[string]string{
	"golang": "go",
	"rust":   "rust",
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

// ContainerShellSpec mirrors the [container.shell] table.
//
// Default selects the login shell ("bash" | "zsh" | "fish"); unset/empty falls
// back to "bash". Aliases and Env are appended directly into the chosen
// login shell's rc file inside the container at image build time using
// shell-appropriate syntax (bash/zsh: `export K=V` / `alias k='v'`;
// fish: `set -gx K V` / `alias k 'v'`). No host-side companion file is
// written — see internal/generate/shellrc.
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

// CertificatesSpec gates TLS certificate auto-bake from
// ~/.cocoon/certs/. See docs/configuration.md `[certificates]`.
type CertificatesSpec struct {
	// nil ⇒ default false (pointer distinguishes "field omitted" from
	// explicit `enable = false`).
	Enable *bool `toml:"enable,omitempty"`
}

// EnableOrDefault returns false unless explicitly set true. Safe on a
// nil receiver.
func (c *CertificatesSpec) EnableOrDefault() bool {
	if c == nil || c.Enable == nil {
		return false
	}
	return *c.Enable
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
// on both host and container. `cocoon gen` touches missing host files
// (0o600, idempotent) on first run, and the generated devcontainer.json's
// initializeCommand performs the same touch so VS Code Reopen-in-Container
// users do not need to invoke `cocoon gen`. Both safeguards prevent Docker
// from auto-creating the bind source as a directory when the file is
// absent at `docker compose up` time. Use [volumes] for whole directories
// and [[mounts]] for arbitrary host paths; [home_files] is the narrow case
// of single files in $HOME that must outlive the container's writable
// layer.
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
