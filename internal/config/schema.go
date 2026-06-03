// Package config loads, validates and re-emits workspace.toml and plugin.toml.
//
// Optional sections are modelled as pointers so a missing section can be
// distinguished from an empty one.
package config

// Workspace mirrors workspace.toml.
type Workspace struct {
	Workspace     *WorkspaceSpec            `toml:"workspace,omitempty"`
	Container     ContainerSpec             `toml:"container"`
	Plugins       PluginsSpec               `toml:"plugins"`
	Ports         *PortsSpec                `toml:"ports,omitempty"`
	Apt           *AptSpec                  `toml:"apt,omitempty"`
	Volumes       map[string]string         `toml:"volumes,omitempty"`
	Env           map[string]string         `toml:"env,omitempty"`
	Mounts        []Mount                   `toml:"mounts,omitempty"`
	HomeFiles     *HomeFilesSpec            `toml:"home_files,omitempty"`
	Locale        *LocaleSpec               `toml:"locale,omitempty"`
	Certificates  *CertificatesSpec         `toml:"certificates,omitempty"`
	Dockerfile    *DockerfileSpec           `toml:"dockerfile,omitempty"`
	Services      map[string]SidecarService `toml:"services,omitempty"`
	Devcontainer  Devcontainer              `toml:"devcontainer,omitempty"`
	CodeWorkspace *CodeWorkspaceSpec        `toml:"code_workspace,omitempty"`
}

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

	// Dir overrides the in-container workdir parent directory under
	// /home/<user>/. Empty falls back to "workspace". Multi-segment paths
	// (e.g. "work/myproject") are accepted so callers can match a host
	// path layout that tools like AWS SAM expect. Validation lives in
	// rxWorkspaceDir plus a per-segment check; together they reject
	// absolute paths, "." and ".." segments, and any non-portable filename
	// character.
	Dir string `toml:"dir,omitempty"`

	// DevContainer toggles emission of .devcontainer/devcontainer.json.
	// Pointer so the loader can distinguish "field omitted" (defaults true)
	// from an explicit `devcontainer = false`.
	DevContainer *bool `toml:"devcontainer,omitempty"`
}

func (w *WorkspaceSpec) MountRootOrDefault() string {
	if w == nil || w.MountRoot == "" {
		return "."
	}
	return w.MountRoot
}

func (w *WorkspaceSpec) DirOrDefault() string {
	if w == nil || w.Dir == "" {
		return "workspace"
	}
	return w.Dir
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
	Sudo         *SudoSpec           `toml:"sudo,omitempty"`
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
// deb.debian.org. apt-mirror rewriting keys off this distinction via the
// ImageOSFamily classification — see aptMirrorOriginHosts in
// internal/generate/dockerfile/dockerfile.go.
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
	"debian":        {"12", "13"}, // 12 (bookworm) is the cocoon default base image; 13 (trixie) selectable.
	"ubuntu":        {"26.04", "24.04", "22.04"},
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

// ImageOSFamily classifies each SupportedImages entry by underlying distro
// family so apt-related generators (currently aptMirrorOriginHosts) can
// pick the right upstream archive hosts without hard-coding image ids.
// Replaces the previous literal `if image == "ubuntu"` check in
// aptMirrorOriginHosts so adding a future Ubuntu-derived image (e.g.
// eclipse-temurin) is a one-line map edit rather than a code change.
//
// Values are the literal "ubuntu" and "debian" — the same names used as
// image ids for the two plain-distro images. Sub-suite distinctions
// (bookworm vs trixie) deliberately are not encoded here because the
// archive host (`deb.debian.org/debian`) is the same across Debian suites.
//
// Every entry in SupportedImages MUST have a matching row here.
// TestImageOSFamilyLockstep enforces the invariant in both directions;
// at runtime, an image id with no row makes aptMirrorOriginHosts return
// nil and buildAptMirrorRewrite skips emission entirely (rather than
// silently falling through to the Debian host list).
//
//nolint:gochecknoglobals // tabular configuration data, file-scoped by design.
var ImageOSFamily = map[string]string{
	"ubuntu":        "ubuntu",
	"debian":        "debian",
	"node":          "debian",
	"python":        "debian",
	"golang":        "debian",
	"rust":          "debian",
	"denoland/deno": "debian",
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

// Sudo mode values for [container.sudo].mode. The set is intentionally just
// two: "nopasswd" (the default, passwordless sudo) and "password" (sudo
// requires a password set from the .env.local build secret). There is NO
// "none"/"disabled" value — disabling sudo is expressed by
// [container.security_opt] no_new_privileges = true, so encoding "no sudo"
// here too would be two sources of truth for one fact (DRY).
const (
	SudoModeNoPasswd = "nopasswd"
	SudoModePassword = "password"
)

// SudoSpec models [container.sudo]. mode selects the sudoers policy baked into
// the image. When mode = "password" the build reads the user's password from
// the .env.local build secret (key SUDO_PASSWORD) and sets it via chpasswd; a
// missing/empty password fails the build rather than degrading to passwordless.
type SudoSpec struct {
	// Pointer distinguishes "field omitted" (default "nopasswd") from an
	// explicit value.
	Mode *string `toml:"mode,omitempty"`
}

// SudoModeOrDefault is safe on a nil receiver and defaults to "nopasswd".
func (s *SudoSpec) SudoModeOrDefault() string {
	if s == nil || s.Mode == nil || *s.Mode == "" {
		return SudoModeNoPasswd
	}
	return *s.Mode
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

// PluginsSpec models [plugins]. A plugin is enabled (and optionally
// version-pinned) by one entry in the EnableRaw array; install method and
// extra knobs live in the two side tables.
//
// EnableRaw is the TOML-decoded [plugins].enable array. Each element is
// "<id>" (enable, unpinned), "<id>=<version>" (exact pin) or "<id>=latest"
// (floating, frozen by `cocoon lock`) — uv/pip-style so a plugin is named
// once, and array order is the deterministic install order. LoadWorkspace
// post-processes EnableRaw into Enable (the ordered id list) and seeds the
// matching Versions[id].Spec/Pin.
//
// OptionsRaw is the TOML-decoded [plugins.options] table: per-id inline
// tables carrying a plugin's [install.extra_versions] knobs (e.g. Android
// SDK's api_level / build_tools) and optional manual checksum_amd64 /
// checksum_arm64 (only for plugins whose upstream publishes none). It never
// carries the main version — that is in the enable array. LoadWorkspace
// folds it into Versions[id].Extra / Checksum*.
//
// Methods maps a plugin id to the user-selected install method name (a key
// in that plugin's [install.methods] section); absent entries fall back to
// the plugin's default_method.
//
// Test code that bypasses LoadWorkspace must populate Enable and Versions
// directly (EnableRaw / OptionsRaw are decode-only).
type PluginsSpec struct {
	EnableRaw  []string                         `toml:"enable"`
	OptionsRaw map[string]any                   `toml:"options,omitempty"`
	Methods    map[string]string                `toml:"methods,omitempty"`
	Enable     []string                         `toml:"-"`
	Versions   map[string]PluginVersionOverride `toml:"-"`
}

// PluginVersionOverride models the resolved version inputs for one enabled
// plugin. Spec is the user's version constraint, derived from the
// [plugins].enable entry: either "=<version>" (an exact pin) or "latest"
// (frozen to a concrete version by `cocoon lock`). Pin is the concrete
// version baked into the generated Dockerfile's PIN="..." env — the exact
// version for an "=<version>" Spec and "" for "latest" (build-time
// resolution) — and is overwritten by a cocoon.lock entry when one exists.
// ChecksumAmd64 / ChecksumArm64 normally come from cocoon.lock and are nil
// until `cocoon lock` records them; a plugin whose upstream publishes no
// checksum may instead carry a manual value from [plugins.options]. Extra
// carries any [plugins.options] knobs a plugin opts into via
// [install.extra_versions] (e.g. Android SDK's api_level / build_tools);
// it is nil when no extra keys are set.
type PluginVersionOverride struct {
	Spec          string
	Pin           string
	ChecksumAmd64 *string
	ChecksumArm64 *string
	Extra         map[string]string
}

// IsLatest reports whether the constraint is the floating "latest" form
// rather than an exact "=<version>" pin.
func (o PluginVersionOverride) IsLatest() bool {
	return o.Spec == VersionSpecLatest
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

// Devcontainer is a passthrough map: dump-devcontainer emits entries verbatim.
type Devcontainer map[string]any

// CodeWorkspaceSpec models the optional [code_workspace] section consumed by
// `cocoon gen workspace`. Output is a <name>.code-workspace file written at
// the project root (not under .devcontainer/), since VS Code's convention is
// to keep workspace files alongside the project they describe.
//
//   - Name: output file basename (without the .code-workspace extension).
//     Empty falls back to filepath.Base(projectDir).
//   - Folders: inline-table array of {path, name}. name is optional and
//     defaults to the basename of the resolved path. path supports "~"
//     home expansion and is relativized against the directory the
//     .code-workspace file is written to (the workspace.toml directory by
//     default, or `cocoon gen workspace --output <dir>` when set).
//   - Settings: VS Code workspace "settings" object, passed through verbatim
//     as JSON. Empty map is elided from the output.
//   - Extensions.Recommendations: VS Code recommended extension IDs, emitted
//     as `"extensions": { "recommendations": [...] }`. Empty list is elided.
type CodeWorkspaceSpec struct {
	Name       string                `toml:"name,omitempty"`
	Folders    []CodeWorkspaceFolder `toml:"folders,omitempty"`
	Settings   map[string]any        `toml:"settings,omitempty"`
	Extensions *CodeWorkspaceExtSpec `toml:"extensions,omitempty"`
}

// CodeWorkspaceFolder models one [[code_workspace.folders]] inline-table
// entry. Path is required; Name is optional and defaults to the basename of
// the resolved path at generation time.
type CodeWorkspaceFolder struct {
	Path string `toml:"path"`
	Name string `toml:"name,omitempty"`
}

// CodeWorkspaceExtSpec models [code_workspace.extensions]. Only the
// `recommendations` array is supported today; other VS Code extension keys
// (e.g. `unwantedRecommendations`) can be added later if requested.
type CodeWorkspaceExtSpec struct {
	Recommendations []string `toml:"recommendations,omitempty"`
}
