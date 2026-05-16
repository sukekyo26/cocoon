// Package config loads, validates and re-emits workspace.toml and plugin.toml.
//
// Optional sections are modelled as pointers so a missing section can be
// distinguished from an empty one.
package config

// Workspace mirrors workspace.toml.
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

// WorkspaceSpec models the optional [workspace] section. Defaults apply when
// the section is missing or fields are zero.
type WorkspaceSpec struct {
	// MountRoot selects which host directory is bind-mounted at /workspace.
	//
	//   "."  — mount the project directory itself (default).
	//   ".." — mount the parent directory so sibling repos are visible.
	//
	// The picker in `cocoon init` writes one of these two values and
	// nothing else; loaders enforce the same constraint.
	MountRoot string `toml:"mount_root,omitempty"`

	// DevContainer toggles emission of .devcontainer/devcontainer.json.
	// Pointer so the loader can distinguish "field omitted" (defaults true)
	// from an explicit `devcontainer = false`.
	DevContainer *bool `toml:"devcontainer,omitempty"`
}

// MountRootOrDefault falls back to "." when [workspace] is omitted or empty.
func (w *WorkspaceSpec) MountRootOrDefault() string {
	if w == nil || w.MountRoot == "" {
		return "."
	}
	return w.MountRoot
}

// DevContainerOrDefault is true unless explicitly set false.
func (w *WorkspaceSpec) DevContainerOrDefault() bool {
	if w == nil || w.DevContainer == nil {
		return true
	}
	return *w.DevContainer
}

// ContainerSpec models the [container] section. Image is the canonical
// DockerHub image name pulled verbatim into the FROM line; ImageVersion is
// the image-specific tag. The closed image set lives in SupportedImages;
// per-image suggestions in SupportedImageVersions. Using canonical names
// means a reader can recreate the FROM line from workspace.toml alone — no
// alias resolution required.
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

	// GroupAdd lists supplementary groups (name or numeric GID) the container
	// user joins, emitted as Compose `group_add:`. Needed because the user runs
	// as a numeric `${UID}:${GID}`, so groups baked into the image's /etc/group
	// are not applied at runtime.
	GroupAdd []string `toml:"group_add,omitempty"`
	// Devices maps host devices into the container as Compose `devices:`
	// entries (`HOST:CONTAINER[:rwm]`).
	Devices []string `toml:"devices,omitempty"`
	// IPC sets the container's IPC namespace mode (Compose `ipc:`), e.g.
	// "host" for ML workloads that need a large shared-memory segment.
	IPC *string `toml:"ipc,omitempty"`
	// Gpus requests GPU access (Compose `gpus:`). Only the literal "all" is
	// supported; the per-device list form is not yet exposed.
	Gpus *string `toml:"gpus,omitempty"`

	// DockerSocket opts in to bind-mounting /var/run/docker.sock so DinD
	// workflows can reach the host daemon. When true the generators add the
	// bind mount; docker-entrypoint.sh reconciles the container user's group
	// membership to the socket's GID at container start. Pointer so the
	// loader can distinguish "field omitted" (default false) from an explicit
	// `docker_socket = false`.
	DockerSocket *bool `toml:"docker_socket,omitempty"`
}

// DockerSocketEnabled defaults to the secure value (false).
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
//   - "node" → "node": the node plugin extracts the official tarball into
//     /usr/local/node and prepends /usr/local/node/bin to PATH, so the
//     base image's /usr/local/bin/node is shadowed and never used.
//   - "denoland/deno" → "deno": the deno plugin unzips the binary directly
//     over /usr/local/bin/deno, overwriting the one the base image ships.
//
// Other supported images (ubuntu, debian, python) have no matching plugin
// in the catalog and therefore no conflict to declare here.
//
//nolint:gochecknoglobals // tabular configuration data, file-scoped by design.
var ImageProvidesPlugin = map[string]string{
	"golang":        "go",
	"rust":          "rust",
	"node":          "node",
	"denoland/deno": "deno",
}

// SkelEntry seeds a file from the build context into /etc/skel so useradd -m
// copies it into the new user's home (e.g. .editorconfig / .gitignore_global
// / .tmux.conf).
type SkelEntry struct {
	Source string `toml:"source"`
	Target string `toml:"target"`
}

// SecurityOptSpec mirrors [container.security_opt]. Each field becomes one
// Compose `security_opt:` list entry.
type SecurityOptSpec struct {
	Seccomp         *string `toml:"seccomp,omitempty"`
	AppArmor        *string `toml:"apparmor,omitempty"`
	NoNewPrivileges *bool   `toml:"no_new_privileges,omitempty"`
}

// CapabilitiesSpec mirrors [container.capabilities]. Values are bare names
// without the `CAP_` prefix (e.g. "SYS_PTRACE").
type CapabilitiesSpec struct {
	Add  []string `toml:"add,omitempty"`
	Drop []string `toml:"drop,omitempty"`
}

// DNSSpec mirrors [container.dns].
type DNSSpec struct {
	Servers []string `toml:"servers,omitempty"`
	Search  []string `toml:"search,omitempty"`
}

// ContainerShellSpec mirrors [container.shell]. Aliases and Env are written
// into the rc-file at image build time using shell-appropriate syntax
// (bash/zsh: `export K=V` / `alias k='v'`; fish: `set -gx K V` /
// `alias k 'v'`). No host-side companion file is written.
type ContainerShellSpec struct {
	Default *string           `toml:"default,omitempty"`
	Aliases map[string]string `toml:"aliases,omitempty"`
	Env     map[string]string `toml:"env,omitempty"`
}

// Resources models [container.resources].
type Resources struct {
	ShmSize         *string  `toml:"shm_size,omitempty"`
	PidsLimit       *int     `toml:"pids_limit,omitempty"`
	StopGracePeriod *string  `toml:"stop_grace_period,omitempty"`
	CPUs            *float64 `toml:"cpus,omitempty"`
	Memory          *string  `toml:"memory,omitempty"`
	NofileSoft      *int     `toml:"nofile_soft,omitempty"`
	NofileHard      *int     `toml:"nofile_hard,omitempty"`
}

// PluginsSpec models [plugins]. Methods maps a plugin id to the
// user-selected install method name (a key in that plugin's
// [install.methods] section); absent entries fall back to the plugin's
// default_method.
type PluginsSpec struct {
	Enable   []string                         `toml:"enable"`
	Versions map[string]PluginVersionOverride `toml:"versions,omitempty"`
	Methods  map[string]string                `toml:"methods,omitempty"`
}

// PluginVersionOverride models one entry under [plugins.versions].
type PluginVersionOverride struct {
	Pin           string  `toml:"pin"`
	ChecksumAmd64 *string `toml:"checksum_amd64,omitempty"`
	ChecksumArm64 *string `toml:"checksum_arm64,omitempty"`
}

// PortsSpec models [ports]. Each Forward entry is either a docker-compose
// short-form string ("3000:3000", "127.0.0.1:5432:5432/tcp",
// "3000-3005:3000-3005") or a long-form table with the keys target,
// published, host_ip, protocol, mode. See ComposePortEntries for the
// normalized representation consumed by generators.
type PortsSpec struct {
	Forward []any `toml:"forward"`
}

// AptSpec models [apt].
type AptSpec struct {
	Packages []string    `toml:"packages,omitempty"`
	Mirror   *AptMirror  `toml:"mirror,omitempty"`
	Proxy    *AptProxy   `toml:"proxy,omitempty"`
	Sources  []AptSource `toml:"sources,omitempty"`
}

// AptMirror rewrites archive.ubuntu.com, security.ubuntu.com, and
// ports.ubuntu.com to URL.
type AptMirror struct {
	URL string `toml:"url"`
}

// AptProxy writes /etc/apt/apt.conf.d/95proxy at build time. Either field
// may be omitted.
type AptProxy struct {
	HTTP  *string `toml:"http,omitempty"`
	HTTPS *string `toml:"https,omitempty"`
}

// AptSource declares one third-party apt repository. The generator places
// the GPG key under /etc/apt/keyrings/<Name>.gpg and writes
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

// LocaleSpec models [locale].
type LocaleSpec struct {
	Timezone *string `toml:"timezone,omitempty"`
	Lang     *string `toml:"lang,omitempty"`
}

// GitIdentitySpec models [git].
type GitIdentitySpec struct {
	UserName  *string `toml:"user_name,omitempty"`
	UserEmail *string `toml:"user_email,omitempty"`
}

// CertificatesSpec gates TLS certificate auto-bake from ~/.cocoon/certs/.
// See docs/configuration.md `[certificates]`.
type CertificatesSpec struct {
	// Pointer distinguishes "field omitted" (default false) from explicit
	// `enable = false`.
	Enable *bool `toml:"enable,omitempty"`
}

// EnableOrDefault is safe on a nil receiver and defaults to false.
func (c *CertificatesSpec) EnableOrDefault() bool {
	if c == nil || c.Enable == nil {
		return false
	}
	return *c.Enable
}

// DockerfileSpec models [dockerfile].
type DockerfileSpec struct {
	PreUserSetup *string `toml:"pre_user_setup,omitempty"`
	PostPlugins  *string `toml:"post_plugins,omitempty"`
}

// Mount models one [[mounts]] entry.
type Mount struct {
	Source   string `toml:"source"`
	Target   string `toml:"target"`
	Readonly bool   `toml:"readonly"`
}

// HomeFilesSpec declares single files under $HOME to persist across container
// rebuilds. Per-segment characters are restricted to [A-Za-z0-9._-] because
// the path flows verbatim into the generated initializeCommand shell snippet
// (run under /bin/sh), so anything shell-special — $, backticks, ; & | < > * ? !,
// quotes, backslashes, whitespace — would let a repo-provided workspace.toml
// inject commands into the host shell. `cocoon gen` and the generated
// devcontainer.json's initializeCommand both touch missing host files
// (0o600, idempotent) so Docker does not auto-create the bind source as a
// directory at `docker compose up` time. Use [volumes] for directories and
// [[mounts]] for arbitrary host paths; [home_files] is for single files
// in $HOME that must outlive the container's writable layer.
type HomeFilesSpec struct {
	Files []string `toml:"files"`
}

// SidecarMount models one [[services.<name>.mounts]] entry.
type SidecarMount struct {
	Source   string `toml:"source"`
	Target   string `toml:"target"`
	Readonly bool   `toml:"readonly"`
}

// SidecarRestart enumerates the allowed values for SidecarService.Restart.
type SidecarRestart string

// Sidecar restart policy values (compose v3 subset).
const (
	RestartNo            SidecarRestart = "no"
	RestartAlways        SidecarRestart = "always"
	RestartOnFailure     SidecarRestart = "on-failure"
	RestartUnlessStopped SidecarRestart = "unless-stopped"
)

// HealthcheckSpec models [services.<name>.healthcheck]. Extra keys are
// preserved verbatim.
type HealthcheckSpec map[string]any

// SidecarService models one [services.<name>] table.
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

// RepositoryClone models one [[repositories.clone]] entry.
type RepositoryClone struct {
	URL               string  `toml:"url"`
	Path              *string `toml:"path,omitempty"`
	Branch            *string `toml:"branch,omitempty"`
	Depth             *int    `toml:"depth,omitempty"`
	RecurseSubmodules *bool   `toml:"recurse_submodules,omitempty"`
}

// RepositoriesSpec models [repositories].
type RepositoriesSpec struct {
	Clone []RepositoryClone `toml:"clone"`
}

// Devcontainer is a passthrough map: dump-devcontainer emits entries verbatim.
type Devcontainer map[string]any
