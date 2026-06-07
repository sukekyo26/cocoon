package config

import (
	"errors"
	"fmt"
	"maps"
	"net"
	"regexp"
	"slices"
	"strings"
)

var (
	rxServiceName  = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	rxUsername     = regexp.MustCompile(`^[a-z_][a-z0-9_-]*$`)
	rxPluginID     = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	rxPluginMethod = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	rxEnvKey       = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	rxShellEnvKey  = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
	rxAliasKey     = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_-]*$`)
	// rxHomeFilesSegment: each path segment of a [home_files].files entry
	// must consist only of POSIX portable filename chars (letters,
	// digits, dot, hyphen, underscore). home_files paths flow into the
	// generated initializeCommand as raw shell snippets (cocoon gen / VS
	// Code run them with /bin/sh), so anything with shell-special meaning
	// — $, backticks, ; & | < > ( ) * ? ! [ ] { } ~, quotes, backslashes,
	// whitespace, newlines — would let a repo-provided config file
	// inject commands into the host shell. Strict whitelist > best-effort
	// blacklist.
	rxHomeFilesSegment = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	rxLang             = regexp.MustCompile(`^[a-z]{2,3}_[A-Z]{2}\.UTF-8$`)
	rxAbsolutePath     = regexp.MustCompile(`^/`)
	// rxMountTarget: a [[mounts]].target is interpolated unquoted into the
	// generated Dockerfile `ENV COCOON_BIND_PATHS="..."` line and into the
	// docker-compose `source:target` volume spec. A quote, backtick, `$`,
	// `:`, whitespace, or newline would break out of either context, so the
	// container target is restricted to a path of POSIX portable filename
	// chars plus the literal `${USERNAME}` placeholder. Whitelist > blacklist.
	rxMountTarget = regexp.MustCompile(`^(?:/(?:\$\{USERNAME\}|[A-Za-z0-9._-])+)+/?$`)
	// rxHostname matches an RFC 1123-style hostname: dot-joined labels,
	// each label 1+ alnum chars (hyphen allowed inside but not at the
	// start/end). Underscores and consecutive dots (a..b) are rejected.
	rxHostnameLabel = `[a-zA-Z0-9](?:[a-zA-Z0-9-]*[a-zA-Z0-9])?`
	rxHostname      = regexp.MustCompile(
		`^(?:` + rxHostnameLabel + `)(?:\.(?:` + rxHostnameLabel + `))*$`)
	rxSysctlKey  = regexp.MustCompile(`^[a-z][a-z0-9._-]*$`)
	rxCapability = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
	// rxGroupName matches a Linux group name (lowercase, optional trailing $
	// for the useradd convention); rxGID matches a bare numeric GID. A
	// group_add entry must satisfy one of the two.
	rxGroupName   = regexp.MustCompile(`^[a-z_][a-z0-9_-]*\$?$`)
	rxGID         = regexp.MustCompile(`^[0-9]+$`)
	rxDevicePerms = regexp.MustCompile(`^[rwm]+$`)
	// rxContainerName matches the syntactic shape of a Docker container
	// name or ID (Docker's restricted-name pattern). Used to reject a
	// malformed `ipc = "container:<name>"` target at validation time
	// rather than letting whitespace/newlines reach the generated Compose
	// file and fail at `docker compose` time.
	rxContainerName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]+$`)
	rxAptName       = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	rxAptSuite      = regexp.MustCompile(`^[a-z][a-z0-9._-]*$`)
	rxAptComponent  = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	rxAptArch       = regexp.MustCompile(`^(amd64|arm64|i386|armhf|ppc64el|s390x)$`)
	// rxImageVersion bounds image_version to the Docker tag character set
	// minus the colon and the registry-path slash, both of which would let
	// a user smuggle a second `<image>:<tag>` segment past the FROM
	// template and break the generated Dockerfile. Per the Docker
	// reference spec a tag is alnum / underscore / period / hyphen with
	// the first character restricted to alnum or underscore (a leading
	// period or hyphen is forbidden because it collides with the digest
	// separator and POSIX option syntax respectively). The regex mirrors
	// that contract so tags like `_internal` or `2.7.14` are accepted and
	// `.hidden` / `-foo` / `library/node` / `node:24` are rejected.
	rxImageVersion = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]*$`)
	// rxSha256 bounds a manual [plugins.options] checksum to exactly 64
	// lowercase hex characters (a raw sha256 digest), matching the form
	// cocoon.lock records and sha256sum -c expects.
	rxSha256 = regexp.MustCompile(`^[a-f0-9]{64}$`)
	// rxWorkspaceDir validates [workspace].dir: one or more portable filename
	// segments joined by `/`. Each segment uses the same POSIX portable
	// charset rxHomeFilesSegment uses so the value can flow into Dockerfile
	// WORKDIR / docker-compose mount targets / docker-entrypoint.sh chown
	// loops without shell-special characters reaching those contexts. The
	// regex rejects absolute paths (leading `/`), trailing slashes, and
	// empty segments, but `.` and `..` slip through it because both consist
	// of charset chars — IsValidWorkspaceDir / WorkspaceSpec.validate strip
	// those with an explicit per-segment check so the field stays relative
	// to /home/<user>/ and cannot encode container-escape paths.
	rxWorkspaceDir = regexp.MustCompile(`^[A-Za-z0-9._-]+(?:/[A-Za-z0-9._-]+)*$`)
	// rxCodeWorkspaceName validates [code_workspace].name: a single portable
	// filename segment used verbatim as the output file basename
	// (<name>.code-workspace). Slash, backslash, colon, and whitespace are
	// rejected so the name cannot escape the project directory or break
	// filesystem semantics. "." and ".." pass the charset regex (both
	// consist of allowed chars) but would produce a broken output path, so
	// IsValidCodeWorkspaceName / CodeWorkspaceSpec.validate strip them with
	// an explicit per-string check.
	rxCodeWorkspaceName = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

// portMin and portMax bound ports forwarded into the dev container.
const (
	portMin = 1
	portMax = 65535
)

// IsValidPluginID lets scaffolders reject bad ids before files are written,
// mirroring the [plugins].enable regex.
func IsValidPluginID(id string) bool {
	return rxPluginID.MatchString(id)
}

// IsValidWorkspaceDir lets `cocoon init` reject bad [workspace].dir input
// before the toml is written. Mirrors WorkspaceSpec.validate's two-step
// check (charset + "no `.`/`..` segment").
func IsValidWorkspaceDir(s string) bool {
	if !rxWorkspaceDir.MatchString(s) {
		return false
	}
	for _, seg := range strings.Split(s, "/") {
		if seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

// IsValidCodeWorkspaceName lets `cocoon gen workspace --name` reject bad
// input before the output path is computed. Mirrors
// CodeWorkspaceSpec.validate's two-step check (charset + "not `.` or `..`").
func IsValidCodeWorkspaceName(s string) bool {
	if !rxCodeWorkspaceName.MatchString(s) {
		return false
	}
	return s != "." && s != ".."
}

// Accumulator collects FieldError rows scoped under a base path. The zero
// value is usable; NewAccumulator is just a convenience constructor.
type Accumulator struct {
	base []string
	errs *[]FieldError
}

func NewAccumulator() *Accumulator {
	errs := make([]FieldError, 0)
	return &Accumulator{base: nil, errs: &errs}
}

// ensure lazily allocates the shared error slice so a zero-value
// Accumulator (one not built via NewAccumulator) does not nil-panic on
// first use. At allocates before handing the pointer to a child, so the
// parent and every child created via At share one slice.
func (a *Accumulator) ensure() {
	if a.errs == nil {
		errs := make([]FieldError, 0)
		a.errs = &errs
	}
}

// At returns a child Accumulator whose collected errors are prefixed with
// seg; the shared error slice is carried over so child writes are visible
// to the parent.
func (a *Accumulator) At(seg ...string) *Accumulator {
	a.ensure()
	out := make([]string, 0, len(a.base)+len(seg))
	out = append(out, a.base...)
	out = append(out, seg...)
	return &Accumulator{base: out, errs: a.errs}
}

// AddCode records a localizable validation failure: an i18n catalog key plus
// render-time args, located at base+seg. The message is rendered in the active
// language at the CLI boundary (config.ValidationError.Localize).
func (a *Accumulator) AddCode(code string, args []any, seg ...string) {
	a.ensure()
	*a.errs = append(*a.errs, FieldError{Loc: a.loc(seg), Code: code, Args: args, Message: ""})
}

// loc builds the absolute location path base+seg.
func (a *Accumulator) loc(seg []string) []string {
	loc := make([]string, 0, len(a.base)+len(seg))
	loc = append(loc, a.base...)
	loc = append(loc, seg...)
	return loc
}

// Errors returns the FieldError rows collected so far across this
// Accumulator and every child created via At.
func (a *Accumulator) Errors() []FieldError {
	if a.errs == nil {
		return nil
	}
	return *a.errs
}

// Validate runs every cross-field check on the workspace. On failure the
// returned error is a *ValidationError with Path = path.
func (w *Workspace) Validate(path string) error {
	a := NewAccumulator()
	w.runValidate(a)
	errs := a.Errors()
	if len(errs) == 0 {
		return nil
	}
	return &ValidationError{Path: path, Errors: errs}
}

func (w *Workspace) runValidate(a *Accumulator) {
	if w.Workspace != nil {
		w.Workspace.validate(a.At("workspace"))
	}
	w.Container.validate(a.At("container"))
	w.Plugins.validate(a.At("plugins"))
	w.validateImagePluginConflict(a.At("container"))
	w.validatePasswordSudoVsNoNewPrivileges(a.At("container", "sudo"))
	if w.Container.IPC != nil {
		w.validateContainerIPC(a.At("container", "ipc"))
	}
	if w.Ports != nil {
		w.Ports.validate(a.At("ports"))
	}
	CheckMapKeys(a.At("env"), w.Env, rxEnvKey, "env")
	if w.Apt != nil {
		w.Apt.validate(a.At("apt"))
	}
	if w.Locale != nil {
		w.Locale.validate(a.At("locale"))
	}
	if w.Certificates != nil {
		w.Certificates.validate(a.At("certificates"))
	}
	for i, m := range w.Mounts {
		m.validate(a.At("mounts", fmt.Sprintf("%d", i)))
	}
	if w.HomeFiles != nil {
		w.HomeFiles.validate(a.At("home_files"))
	}
	w.validateServices(a)
	if w.CodeWorkspace != nil {
		w.CodeWorkspace.validate(a.At("code_workspace"))
	}
	if w.Lockfile != nil {
		w.Lockfile.validate(a.At("lockfile"))
	}
}

// validate checks [lockfile].name is a single safe basename. An unset or empty
// name defaults to DefaultLockFileName (matching [workspace].dir's "empty =
// default" policy), so only a non-empty value is checked. The charset reuses
// rxCodeWorkspaceName (no slash, so the lock always lands beside the config
// file and cannot traverse out); "." / ".." and either config filename
// (cocoon.toml / workspace.toml) are rejected explicitly — the latter would
// overwrite the user's own config.
func (l *LockFileSpec) validate(a *Accumulator) {
	if l.Name == nil || *l.Name == "" {
		return
	}
	name := *l.Name
	switch {
	case !rxCodeWorkspaceName.MatchString(name):
		a.AddCode("err_field_lockfile_name_charset", nil, "name")
	case name == "." || name == "..":
		a.AddCode("err_field_name_dot_dotdot", nil, "name")
	case name == DefaultConfigFileName || name == LegacyConfigFileName:
		a.AddCode("err_field_lockfile_name_config", nil, "name")
	}
}

// validate checks [code_workspace] structurally. Path-level semantics — "~"
// expansion against $HOME, "~user" rejection, relativization against the
// project directory — are enforced by the generator in
// internal/generate/codeworkspace. The generator does NOT stat each path
// (cocoon is a pure generator and a path that does not exist yet on the
// current host is still a legal entry), so validation here only ensures the
// config file can be safely consumed.
func (c *CodeWorkspaceSpec) validate(a *Accumulator) {
	if c.Name != "" {
		switch {
		case !rxCodeWorkspaceName.MatchString(c.Name):
			a.AddCode("err_field_code_workspace_name_charset", nil, "name")
		case c.Name == "." || c.Name == "..":
			a.AddCode("err_field_name_dot_dotdot", nil, "name")
		}
	}
	for i, f := range c.Folders {
		idx := fmt.Sprintf("%d", i)
		if f.Path == "" {
			a.At("folders", idx).AddCode("err_field_path_empty", nil, "path")
		}
	}
}

func (w *WorkspaceSpec) validate(a *Accumulator) {
	if w.MountRoot != "" && w.MountRoot != "." && w.MountRoot != ".." {
		a.AddCode("err_field_mount_root_dot_dotdot", nil, "mount_root")
	}
	if w.Dir == "" {
		return
	}
	if !rxWorkspaceDir.MatchString(w.Dir) {
		a.AddCode("err_field_dir_charset", nil, "dir")
		return
	}
	// Each segment matches [A-Za-z0-9._-]+ so "." and ".." sneak through the
	// regex — reject them explicitly to keep dir relative and inside the
	// container home.
	for _, seg := range strings.Split(w.Dir, "/") {
		if seg == "." || seg == ".." {
			a.AddCode("err_field_dir_dot_dotdot_segments", nil, "dir")
			return
		}
	}
}

func (c *ContainerSpec) validate(a *Accumulator) {
	if !rxServiceName.MatchString(c.ServiceName) {
		a.AddCode("err_field_service_name_pattern", []any{rxServiceName.String()}, "service_name")
	}
	if !rxUsername.MatchString(c.Username) {
		a.AddCode("err_field_username_pattern", []any{rxUsername.String()}, "username")
	}
	if c.DeprecatedOs != "" || c.DeprecatedOsVersion != "" {
		// The migration error already tells the user the exact rewrite, and
		// running validateImage on top would stack an "image is required" /
		// "image_version is required" pair on the same field — noise that
		// buries the actionable snippet. Skip the image check until the legacy
		// fields are gone; on the next run validateImage will fire normally.
		//
		// Echo only what was actually present in the legacy fields. Filling
		// in a guessed default (e.g. "ubuntu" for missing `os`, "26.04" for
		// missing `os_version`) would produce a copy/pasteable snippet
		// that's wrong half the time — `image = "ubuntu"` paired with a
		// Debian-shaped version, or vice versa. A placeholder makes the
		// user fill in the missing half themselves.
		legacyOs := c.DeprecatedOs
		if legacyOs == "" {
			legacyOs = "<fill in: ubuntu | debian | node | python | golang | rust | denoland/deno>"
		}
		legacyVersion := c.DeprecatedOsVersion
		if legacyVersion == "" {
			legacyVersion = "<fill in: see docs/configuration.md for the suggestion list>"
		}
		a.AddCode(
			"err_field_os_version_migration",
			[]any{legacyOs, legacyVersion},
			"os",
		)
	} else {
		validateImage(a, c.Image, c.ImageVersion)
	}
	if c.Shell != nil {
		c.Shell.validate(a.At("shell"))
	}
	validateExtraHosts(a.At("hosts"), c.Hosts)
	if c.DNS != nil {
		c.DNS.validate(a.At("dns"))
	}
	validateSysctls(a.At("sysctls"), c.Sysctls)
	if c.Capabilities != nil {
		c.Capabilities.validate(a.At("capabilities"))
	}
	if c.SecurityOpt != nil {
		c.SecurityOpt.validate(a.At("security_opt"))
	}
	if c.Sudo != nil {
		c.Sudo.validate(a.At("sudo"))
	}
	validateSkel(a.At("skel"), c.Skel)
	validateGroupAdd(a.At("group_add"), c.GroupAdd)
	validateDevices(a.At("devices"), c.Devices)
	if c.Gpus != nil {
		validateGpus(a.At("gpus"), *c.Gpus)
	}
}

// validateGroupAdd accepts a group name (rxGroupName) or a numeric GID. A
// name must resolve in the image's /etc/group at runtime; a numeric GID is
// passed straight through as a supplemental GID and needs no matching entry.
// Only the syntactic shape is checked here.
func validateGroupAdd(a *Accumulator, groups []string) {
	if HasDuplicates(groups) {
		a.AddCode("err_field_duplicate_entries", nil)
	}
	for i, g := range groups {
		idx := fmt.Sprintf("%d", i)
		switch {
		case g == "":
			a.AddCode("err_field_must_not_be_empty", nil, idx)
		case !rxGroupName.MatchString(g) && !rxGID.MatchString(g):
			a.AddCode("err_field_group_add_name_or_gid", []any{rxGroupName.String()}, idx)
		}
	}
}

// validateDevices checks Compose `devices:` entries of the form
// HOST:CONTAINER[:rwm]. Both paths must be absolute; CDI device syntax is
// not supported.
func validateDevices(a *Accumulator, devices []string) {
	if HasDuplicates(devices) {
		a.AddCode("err_field_duplicate_entries", nil)
	}
	for i, d := range devices {
		idx := fmt.Sprintf("%d", i)
		parts := strings.Split(d, ":")
		if len(parts) < 2 || len(parts) > 3 {
			a.AddCode("err_field_device_format", nil, idx)
			continue
		}
		if !rxAbsolutePath.MatchString(parts[0]) {
			a.AddCode("err_field_device_host_absolute", nil, idx)
		}
		if !rxAbsolutePath.MatchString(parts[1]) {
			a.AddCode("err_field_device_container_absolute", nil, idx)
		}
		if len(parts) == 3 && !rxDevicePerms.MatchString(parts[2]) {
			a.AddCode("err_field_device_cgroup_perms", nil, idx)
		}
	}
}

// validateContainerIPC checks [container].ipc. Bare Compose modes are taken
// as-is. container:<name> names an external container left to the runtime, so
// only its syntactic shape is checked (rxContainerName). service:<name> must
// resolve to the main service or a defined sidecar (mirroring the depends_on
// undefined-sidecar check) so a typo fails here rather than at
// `docker compose` time.
func (w *Workspace) validateContainerIPC(a *Accumulator) {
	ipc := *w.Container.IPC
	modes := []string{"none", "host", "private", "shareable"}
	if slices.Contains(modes, ipc) {
		return
	}
	if name, ok := strings.CutPrefix(ipc, "container:"); ok {
		switch {
		case name == "":
			a.AddCode("err_field_ipc_container_requires_name", nil)
		case !rxContainerName.MatchString(name):
			a.AddCode("err_field_ipc_container_invalid_name", []any{name, rxContainerName.String()})
		}
		return
	}
	if name, ok := strings.CutPrefix(ipc, "service:"); ok {
		if name == "" {
			a.AddCode("err_field_ipc_service_requires_name", nil)
			return
		}
		if _, defined := w.Services[name]; !defined && name != w.Container.ServiceName {
			a.AddCode("err_field_ipc_service_undefined", []any{name, name})
		}
		return
	}
	a.AddCode("err_field_ipc_oneof", []any{strings.Join(modes, ", ")})
}

// validateGpus currently accepts only the literal "all"; the per-device
// list form (driver/count) is not yet exposed.
func validateGpus(a *Accumulator, gpus string) {
	if gpus != "all" {
		a.AddCode("err_field_gpus_all_only", nil)
	}
}

// validateSkel leaves existence of Source to docker build (COPY surfaces a
// clean "file not found" error).
func validateSkel(a *Accumulator, entries []SkelEntry) {
	seenSrc := map[string]int{}
	seenTgt := map[string]int{}
	for i, e := range entries {
		idx := fmt.Sprintf("%d", i)
		checkSkelPath(a.At(idx, "source"), e.Source, "source")
		if strings.HasSuffix(e.Target, "/") {
			a.AddCode("err_field_skel_target_trailing_slash", nil, idx, "target")
		} else {
			checkSkelPath(a.At(idx, "target"), e.Target, "target")
		}
		if prev, dup := seenSrc[e.Source]; dup && e.Source != "" {
			a.AddCode("err_field_skel_source_duplicates", []any{prev}, idx, "source")
		} else if e.Source != "" {
			seenSrc[e.Source] = i
		}
		if prev, dup := seenTgt[e.Target]; dup && e.Target != "" {
			a.AddCode("err_field_skel_target_duplicates", []any{prev}, idx, "target")
		} else if e.Target != "" {
			seenTgt[e.Target] = i
		}
	}
}

// validateImage rejects [container].image outside SupportedImages and
// [container].image_version that fails rxImageVersion. image_version is
// intentionally NOT restricted to SupportedImageVersions — that map is a
// curated suggestion list (the picker defaults) but users may pin any
// patch / new-minor tag upstream publishes without waiting for a cocoon
// release. The regex excludes colon and slash that would let a malformed
// tag smuggle a second `<image>:<tag>` segment past the FROM template.
func validateImage(a *Accumulator, image, imageVersion string) {
	if image == "" {
		a.AddCode("err_field_image_required_oneof", []any{strings.Join(SupportedImages, ", ")}, "image")
		return
	}
	suggestions, known := SupportedImageVersions[image]
	if !known {
		a.AddCode("err_field_image_oneof_got", []any{strings.Join(SupportedImages, ", "), image}, "image")
		return
	}
	if imageVersion == "" {
		a.AddCode(
			"err_field_image_version_required",
			[]any{image, strings.Join(suggestions, ", "), rxImageVersion.String()},
			"image_version",
		)
		return
	}
	if !rxImageVersion.MatchString(imageVersion) {
		a.AddCode(
			"err_field_image_version_pattern",
			[]any{imageVersion, rxImageVersion.String(), suggestions[0]},
			"image_version",
		)
	}
}

// validateImagePluginConflict rejects pairs from ImageProvidesPlugin where
// the base toolchain would be overwritten or shadowed by the plugin (wasting
// docker-build time for no benefit). Other combinations coexist cleanly.
func (w *Workspace) validateImagePluginConflict(a *Accumulator) {
	pluginID, conflicts := ImageProvidesPlugin[w.Container.Image]
	if !conflicts {
		return
	}
	if !slices.Contains(w.Plugins.Enable, pluginID) {
		return
	}
	a.AddCode(
		"err_field_image_plugin_conflict",
		[]any{w.Container.Image, pluginID, pluginID, pluginID},
		"image",
	)
}

// validatePasswordSudoVsNoNewPrivileges rejects combining password sudo with
// no_new_privileges: the latter blocks setuid escalation kernel-side, so the
// sudo password could never be used. They are mutually exclusive.
func (w *Workspace) validatePasswordSudoVsNoNewPrivileges(a *Accumulator) {
	if w.Container.Sudo.SudoModeOrDefault() != SudoModePassword {
		return
	}
	so := w.Container.SecurityOpt
	if so != nil && so.NoNewPrivileges != nil && *so.NoNewPrivileges {
		a.AddCode(
			"err_field_password_sudo_vs_nnp",
			nil,
			"mode",
		)
	}
}

func checkSkelPath(a *Accumulator, p, label string) {
	switch {
	case p == "":
		a.AddCode("err_field_skel_path_empty", []any{label})
	case strings.HasPrefix(p, "/"):
		a.AddCode("err_field_skel_path_relative", []any{label})
	case strings.HasPrefix(p, "~"):
		a.AddCode("err_field_skel_path_tilde", []any{label})
	case strings.HasPrefix(p, "-"):
		a.AddCode("err_field_skel_path_dash", []any{label})
	case strings.Contains(p, ":"):
		a.AddCode("err_field_skel_path_colon", []any{label})
	case containsWhitespaceOrCtrl(p):
		a.AddCode("err_field_skel_path_whitespace", []any{label})
	default:
		for _, seg := range strings.Split(p, "/") {
			if seg == ".." {
				a.AddCode("err_field_skel_path_dotdot_segments", []any{label})
				return
			}
			if seg == "" {
				a.AddCode("err_field_skel_path_empty_segments", []any{label})
				return
			}
		}
	}
}

// containsWhitespaceOrCtrl defends the generated Dockerfile COPY line
// against arguments that would split tokens, inject newlines, or otherwise
// change the parser's view of the directive.
func containsWhitespaceOrCtrl(s string) bool {
	for _, r := range s {
		if r <= 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

// UnsafeExtraVersionRune reports the first rune in s that would break or
// alter the generated Dockerfile RUN-prefix `KEY="..."` env pair if the
// value were interpolated verbatim:
//
//   - `"` would close the env value early and turn the rest into
//     garbage tokens.
//   - `\` is the shell's escape character inside double quotes.
//   - `\n` / `\r` terminate the RUN line.
//   - `$` triggers parameter / command substitution
//     (`$VAR`, `${VAR}`, `$(...)`), which would silently change the
//     value seen by the install script (or execute commands at build
//     time) instead of passing the literal version string through.
//   - backtick triggers legacy command substitution with the same
//     concern.
//
// Returns (false, 0) when the value is safe to embed.
//
// Both the plugin-author-side default (plugin.toml's
// [install.extra_versions].<key>.default) and the user-side override
// (the config file's [plugins.options].<id>.<key>) are checked through
// this helper so the failure surfaces at decode/validate time, not at
// docker build.
func UnsafeExtraVersionRune(s string) (bool, rune) {
	for _, r := range s {
		switch r {
		case '"', '\\', '\n', '\r', '$', '`':
			return true, r
		}
	}
	return false, 0
}

func (s *SecurityOptSpec) validate(a *Accumulator) {
	if s.Seccomp != nil && *s.Seccomp == "" {
		a.AddCode("err_field_seccomp_empty", nil, "seccomp")
	}
	if s.AppArmor != nil && *s.AppArmor == "" {
		a.AddCode("err_field_apparmor_empty", nil, "apparmor")
	}
}

func (s *SudoSpec) validate(a *Accumulator) {
	if s.Mode == nil {
		return
	}
	// An explicit `mode = ""` is rejected, not silently defaulted: it is almost
	// always a typo, and the omitted-vs-explicit distinction the pointer buys is
	// only meaningful if explicit values are validated. Mirrors SecurityOptSpec's
	// "must not be empty (omit the key …)" handling.
	switch *s.Mode {
	case SudoModeNoPasswd, SudoModePassword:
	default:
		a.AddCode(
			"err_field_sudo_mode_oneof",
			[]any{SudoModeNoPasswd, SudoModePassword},
			"mode",
		)
	}
}

// entrypointRequiredCaps lists the capabilities docker-entrypoint.sh needs
// at container start: CHOWN to re-own the home subtree, SETUID/SETGID to
// drop privileges via setpriv. Dropping any of them (or ALL) strands the
// container as root, so reject the drop at generation time. Keys are bare
// names; a leading CAP_ is stripped before lookup since Docker accepts both
// "CHOWN" and "CAP_CHOWN".
var entrypointRequiredCaps = map[string]struct{}{
	"ALL": {}, "CHOWN": {}, "SETUID": {}, "SETGID": {},
}

func (cs *CapabilitiesSpec) validate(a *Accumulator) {
	checkCapList(a.At("add"), cs.Add)
	checkCapList(a.At("drop"), cs.Drop)
	addSet := make(map[string]struct{}, len(cs.Add))
	for _, c := range cs.Add {
		addSet[c] = struct{}{}
	}
	for _, c := range cs.Drop {
		if _, conflict := addSet[c]; conflict {
			a.AddCode("err_field_cap_add_drop_conflict", []any{c})
			break
		}
	}
	for i, c := range cs.Drop {
		if _, required := entrypointRequiredCaps[strings.TrimPrefix(c, "CAP_")]; required {
			a.AddCode("err_field_cap_required_drop", []any{c}, "drop", fmt.Sprintf("%d", i))
		}
	}
}

func checkCapList(a *Accumulator, caps []string) {
	if HasDuplicates(caps) {
		a.AddCode("err_field_duplicate_entries", nil)
	}
	for i, c := range caps {
		if !rxCapability.MatchString(c) {
			a.AddCode("err_field_capability_pattern", []any{rxCapability.String()}, fmt.Sprintf("%d", i))
		}
	}
}

// validateSysctls accepts numeric or string values (Compose v3 forwards both
// transparently). Anything else is rejected so users notice typos before
// docker compose up complains.
func validateSysctls(a *Accumulator, sysctls map[string]any) {
	for key, val := range sysctls {
		if !rxSysctlKey.MatchString(key) {
			a.AddCode("err_field_sysctl_key_pattern", []any{rxSysctlKey.String()}, key)
			continue
		}
		switch val.(type) {
		case int64, int, string:
			// ok
		default:
			a.AddCode("err_field_sysctl_value_type", []any{val}, key)
		}
	}
}

func (d *DNSSpec) validate(a *Accumulator) {
	if HasDuplicates(d.Servers) {
		a.AddCode("err_field_servers_duplicate_entries", nil, "servers")
	}
	for i, ip := range d.Servers {
		if net.ParseIP(ip) == nil {
			a.AddCode("err_field_invalid_ip", []any{ip}, "servers", fmt.Sprintf("%d", i))
		}
	}
	if HasDuplicates(d.Search) {
		a.AddCode("err_field_search_duplicate_entries", nil, "search")
	}
	for i, dom := range d.Search {
		if !rxHostname.MatchString(dom) {
			a.AddCode("err_field_search_domain_pattern", []any{rxHostname.String()}, "search", fmt.Sprintf("%d", i))
		}
	}
}

// validateExtraHosts enforces hostname keys plus IP-or-"host-gateway" values.
// Empty maps are skipped so the generator can omit extra_hosts entirely.
func validateExtraHosts(a *Accumulator, hosts map[string]string) {
	for host, addr := range hosts {
		if !rxHostname.MatchString(host) {
			a.AddCode("err_field_hostname_pattern", []any{rxHostname.String()}, host)
			continue
		}
		if addr == "host-gateway" {
			continue
		}
		if net.ParseIP(addr) == nil {
			a.AddCode("err_field_host_addr_invalid", []any{addr}, host)
		}
	}
}

func (s *ContainerShellSpec) validate(a *Accumulator) {
	if s.Default != nil {
		v := *s.Default
		// "" is treated as bash by the generator; anything else must be in
		// the SupportedShells closed set.
		if v != "" && !slices.Contains(SupportedShells, v) {
			a.AddCode("err_field_shell_default_oneof", []any{strings.Join(SupportedShells, `", "`)}, "default")
		}
	}
	CheckMapKeys(a.At("aliases"), s.Aliases, rxAliasKey, "container.shell.aliases")
	CheckMapKeys(a.At("env"), s.Env, rxShellEnvKey, "container.shell.env")
	// Values flow verbatim into the Dockerfile heredoc that writes the shell
	// rc (shellrc.RenderDockerfileBlock). $ / backtick stay legal so $HOME and
	// $(cmd) expand, but an embedded newline would let the value forge a
	// heredoc terminator and inject a top-level RUN directive.
	const shellValueHazard = "a newline would let the value escape the Dockerfile heredoc that writes the shell rc"
	CheckMapValues(a.At("aliases"), s.Aliases, "\n\r", shellValueHazard)
	CheckMapValues(a.At("env"), s.Env, "\n\r", shellValueHazard)
}

// VersionSpecLatest is the canonical floating enable-array version constraint.
// "*" is accepted as a synonym on input and normalised to this value.
const VersionSpecLatest = "latest"

// Version-spec sentinels. ParseVersionSpec wraps these with the offending
// input so callers (cocoon plugin pin, the workspace loader) can classify a
// bad version constraint via errors.Is.
var (
	// ErrVersionSpecEmpty is returned for an empty constraint string.
	ErrVersionSpecEmpty = errors.New("version constraint must not be empty")
	// ErrVersionSpecRange is returned for a range operator (>=, <=, >, <, ^,
	// ~, !=); cocoon supports only exact "=<version>" pins and "latest".
	ErrVersionSpecRange = errors.New("version constraint ranges are not supported")
	// ErrVersionSpecBare is returned for a bare version with no leading "=".
	ErrVersionSpecBare = errors.New(`exact version must be prefixed with "="`)
	// ErrVersionSpecCharset is returned when the version after "=" falls
	// outside the accepted tag charset (rxImageVersion).
	ErrVersionSpecCharset = errors.New("version contains unsupported characters")
)

// ParseVersionSpec parses one enable-array version constraint into a
// PluginVersionOverride (Spec plus the derived Pin; Extra and checksums are
// left zero for the caller to fill). Accepted forms: "=<version>" (an exact
// pin) and "latest"/"*" (floating, frozen by `cocoon lock`). Range
// operators and bare versions are rejected with a wrapped sentinel so
// callers can teach the two supported forms. On success Pin holds the exact
// version for an "=<version>" spec and "" for "latest".
func ParseVersionSpec(s string) (PluginVersionOverride, error) {
	t := strings.TrimSpace(s)
	switch {
	case t == "":
		return PluginVersionOverride{}, ErrVersionSpecEmpty //nolint:exhaustruct // error path
	case t == VersionSpecLatest || t == "*":
		return PluginVersionOverride{Spec: VersionSpecLatest}, nil //nolint:exhaustruct // latest leaves Pin/Extra zero
	case hasRangeOperator(t):
		return PluginVersionOverride{}, fmt.Errorf("%q: %w", s, ErrVersionSpecRange) //nolint:exhaustruct // error path
	case t[0] == '=':
		v := t[1:]
		if !rxImageVersion.MatchString(v) {
			return PluginVersionOverride{}, fmt.Errorf("%q: %w", s, ErrVersionSpecCharset) //nolint:exhaustruct // error path
		}
		return PluginVersionOverride{Spec: "=" + v, Pin: v}, nil //nolint:exhaustruct // checksums/extra filled by caller
	default:
		return PluginVersionOverride{}, fmt.Errorf("%q: %w", s, ErrVersionSpecBare) //nolint:exhaustruct // error path
	}
}

// hasRangeOperator reports whether t begins with a version-range operator
// cocoon deliberately does not support (only "=<version>" / "latest" are).
func hasRangeOperator(t string) bool {
	if t == "" {
		return false // t[0] below would panic on ""; callers already exclude it
	}
	if strings.HasPrefix(t, "!=") {
		return true
	}
	switch t[0] {
	case '>', '<', '^', '~':
		return true
	default:
		return false
	}
}

func (p *PluginsSpec) validate(a *Accumulator) {
	if HasDuplicates(p.Enable) {
		a.AddCode("err_field_plugins_enable_duplicate_entries", nil, "enable")
	}
	// Enable entries (id + optional "=<version>"/"latest" constraint) and the
	// [plugins.options] table are parsed and validated in materializePlugins
	// before validate runs; by the time we get here Enable holds clean ids and
	// checksums live in cocoon.lock, not the config file.
	// Sort plugin ids so ValidationError.Error()'s "first error"
	// summary stays stable across runs (map iteration is randomised).
	methodIDs := slices.Sorted(maps.Keys(p.Methods))
	for _, id := range methodIDs {
		method := p.Methods[id]
		if !rxPluginID.MatchString(id) {
			a.AddCode("err_field_plugin_id_pattern", []any{rxPluginID.String()}, "methods", id)
		}
		if !rxPluginMethod.MatchString(method) {
			a.AddCode("err_field_plugin_method_pattern", []any{rxPluginMethod.String()}, "methods", id)
		}
	}
}

func (p *PortsSpec) validate(a *Accumulator) {
	validatePortsForward(a, p.Forward)
}

func (s *AptSpec) validate(a *Accumulator) {
	if s.Mirror != nil {
		s.Mirror.validate(a.At("mirror"))
	}
	if s.Proxy != nil {
		s.Proxy.validate(a.At("proxy"))
	}
	seenNames := map[string]int{}
	for i, src := range s.Sources {
		idx := fmt.Sprintf("%d", i)
		src.validate(a.At("sources", idx), i, seenNames)
	}
}

func (m *AptMirror) validate(a *Accumulator) {
	switch {
	case !isHTTPURL(m.URL):
		a.AddCode("err_field_url_http_scheme", nil, "url")
	case containsUnsafeForSed(m.URL):
		a.AddCode("err_field_apt_mirror_url_unsafe_sed", nil, "url")
	}
}

// containsUnsafeForSed rejects bytes that would break the generated
// `sed 's|FROM|TO|g'` pattern (single quote ends the arg; `|` ends the
// s-command; `&` is a backreference; `\` escapes) or Dockerfile parsing
// (whitespace splits tokens, control chars inject directives).
func containsUnsafeForSed(s string) bool {
	for _, r := range s {
		if r <= 0x20 || r == 0x7f {
			return true
		}
		switch r {
		case '\'', '|', '&', '\\':
			return true
		}
	}
	return false
}

func (p *AptProxy) validate(a *Accumulator) {
	if p.HTTP != nil && !isHTTPURL(*p.HTTP) {
		a.AddCode("err_field_apt_proxy_http_scheme", nil, "http")
	}
	if p.HTTPS != nil && !isHTTPURL(*p.HTTPS) {
		a.AddCode("err_field_apt_proxy_https_scheme", nil, "https")
	}
}

func (src *AptSource) validate(a *Accumulator, i int, seen map[string]int) {
	switch {
	case !rxAptName.MatchString(src.Name):
		a.AddCode("err_field_apt_name_pattern", []any{rxAptName.String()}, "name")
	default:
		if prev, dup := seen[src.Name]; dup {
			a.AddCode("err_field_apt_source_name_duplicates", []any{prev}, "name")
		} else {
			seen[src.Name] = i
		}
	}
	if !rxAptSuite.MatchString(src.Suite) {
		a.AddCode("err_field_apt_suite_pattern", []any{rxAptSuite.String()}, "suite")
	}
	if len(src.Components) == 0 {
		a.AddCode("err_field_components_empty", nil, "components")
	}
	for ci, comp := range src.Components {
		if !rxAptComponent.MatchString(comp) {
			a.AddCode("err_field_apt_component_pattern", []any{rxAptComponent.String()},
				"components", fmt.Sprintf("%d", ci))
		}
	}
	if !isHTTPURL(src.URL) {
		a.AddCode("err_field_url_http_scheme", nil, "url")
	}
	if !isHTTPURL(src.KeyURL) {
		a.AddCode("err_field_key_url_http_scheme", nil, "key_url")
	}
	if src.Arch != nil && !rxAptArch.MatchString(*src.Arch) {
		a.AddCode("err_field_apt_arch", nil, "arch")
	}
}

func isHTTPURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func (l *LocaleSpec) validate(a *Accumulator) {
	if l.Lang != nil && !rxLang.MatchString(*l.Lang) {
		a.AddCode("err_field_lang_pattern", []any{rxLang.String()}, "lang")
	}
}

// No-op hook (only field is enable: bool); kept so future fields slot in
// without re-wiring runValidate.
func (*CertificatesSpec) validate(_ *Accumulator) {}

func (m *Mount) validate(a *Accumulator) {
	if m.Source == "" {
		a.AddCode("err_field_source_empty", nil, "source")
	}
	if !rxAbsolutePath.MatchString(m.Target) {
		a.AddCode("err_field_target_absolute", nil, "target")
	} else if !rxMountTarget.MatchString(m.Target) {
		a.AddCode("err_field_mount_target_charset", nil, "target")
	}
}

// rxHomeFilesSegmentPath guards the shell-character whitelist. Empty / `..`
// segments are caught upstream by dedicated checks.
func rxHomeFilesSegmentPath(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if !rxHomeFilesSegment.MatchString(seg) {
			return false
		}
	}
	return true
}

func (h *HomeFilesSpec) validate(a *Accumulator) {
	if HasDuplicates(h.Files) {
		a.AddCode("err_field_files_duplicate_entries", nil, "files")
	}
	for i, p := range h.Files {
		idx := fmt.Sprintf("%d", i)
		switch {
		case p == "":
			a.AddCode("err_field_home_files_entry_empty", nil, "files", idx)
		case strings.HasPrefix(p, "/"):
			a.AddCode("err_field_home_files_entry_leading_slash", nil, "files", idx)
		case strings.HasPrefix(p, "~"):
			a.AddCode("err_field_home_files_entry_tilde", nil, "files", idx)
		case strings.HasPrefix(p, "./") || strings.HasPrefix(p, "../"):
			a.AddCode("err_field_home_files_entry_dot_prefix", nil, "files", idx)
		case strings.Contains(p, ":"):
			a.AddCode("err_field_home_files_entry_colon", nil, "files", idx)
		case strings.HasSuffix(p, "/"):
			a.AddCode("err_field_home_files_entry_trailing_slash", nil, "files", idx)
		default:
			rejected := false
			for _, seg := range strings.Split(p, "/") {
				if seg == ".." {
					a.AddCode("err_field_home_files_entry_dotdot_segments", nil, "files", idx)
					rejected = true
					break
				}
				if seg == "" {
					a.AddCode("err_field_home_files_entry_empty_segments", nil, "files", idx)
					rejected = true
					break
				}
			}
			if !rejected && !rxHomeFilesSegmentPath(p) {
				a.AddCode("err_field_home_files_entry_charset", nil, "files", idx)
			}
		}
	}
}

//nolint:gocognit,gocyclo // composed of small per-service checks; splitting hurts readability.
func (w *Workspace) validateServices(a *Accumulator) {
	if len(w.Services) == 0 {
		return
	}
	main := w.Container.ServiceName
	if _, collide := w.Services[main]; collide {
		a.AddCode("err_field_service_name_collides", []any{main, main}, "services")
	}
	for name, spec := range w.Services {
		scope := a.At("services", name)
		if !rxServiceName.MatchString(name) {
			a.AddCode("err_field_service_name_pattern_services", []any{rxServiceName.String()}, "services", name)
		}
		if spec.Image == "" {
			scope.AddCode("err_field_image_empty", nil, "image")
		}
		if spec.Env != nil {
			CheckMapKeys(scope.At("env"), spec.Env, rxEnvKey, "services.env")
		}
		if HasDuplicates(spec.DependsOn) {
			scope.AddCode("err_field_depends_on_duplicate_entries", nil, "depends_on")
		}
		for _, dep := range spec.DependsOn {
			switch dep {
			case main:
				scope.AddCode("err_field_depends_on_main_service", []any{name, main}, "depends_on")
			case name:
				scope.AddCode("err_field_depends_on_self", []any{name}, "depends_on")
			default:
				if _, ok := w.Services[dep]; !ok {
					scope.AddCode("err_field_depends_on_undefined_sidecar", []any{name, dep, dep}, "depends_on")
				}
			}
		}
		if _, hasLocal := spec.Volumes["local"]; hasLocal {
			scope.AddCode("err_field_volume_reserved_local", []any{name}, "volumes")
		}
		for k, v := range spec.Volumes {
			if !rxAbsolutePath.MatchString(v) {
				scope.AddCode("err_field_volume_target_absolute", nil, "volumes", k)
			}
		}
		for i, m := range spec.Mounts {
			m.validate(scope.At("mounts", fmt.Sprintf("%d", i)))
		}
		if spec.Restart != nil && !validRestart(*spec.Restart) {
			scope.AddCode("err_field_restart_invalid", []any{*spec.Restart}, "restart")
		}
	}
}

func (sm *SidecarMount) validate(a *Accumulator) {
	if sm.Source == "" {
		a.AddCode("err_field_source_empty", nil, "source")
	}
	if !rxAbsolutePath.MatchString(sm.Target) {
		a.AddCode("err_field_target_absolute", nil, "target")
	}
}

// CheckMapKeys records a FieldError for every key of m that fails rx.
func CheckMapKeys(a *Accumulator, m map[string]string, rx *regexp.Regexp, label string) {
	for k := range m {
		if !rx.MatchString(k) {
			a.AddCode("err_field_map_key_pattern", []any{label, k, rx.String()}, k)
		}
	}
}

// CheckMapValues reports each value in m that contains a rune from unsafe.
// Where CheckMapKeys guards key syntax, this guards the value against runes
// that would break out of the generated artifact the value is interpolated
// into (e.g. a newline forging a Dockerfile heredoc terminator or escaping an
// ENV line). reason explains the hazard in the rejection message. Keys are
// visited in sorted order so the first-error summary is stable across runs.
func CheckMapValues(a *Accumulator, m map[string]string, unsafe, reason string) {
	for _, k := range slices.Sorted(maps.Keys(m)) {
		for _, r := range m[k] {
			if strings.ContainsRune(unsafe, r) {
				a.AddCode("err_field_map_value_unsafe_char", []any{r, reason}, k)
				break
			}
		}
	}
}

// HasDuplicates reports whether items contains any value more than once.
func HasDuplicates[T comparable](items []T) bool {
	seen := make(map[T]struct{}, len(items))
	for _, v := range items {
		if _, dup := seen[v]; dup {
			return true
		}
		seen[v] = struct{}{}
	}
	return false
}

func validRestart(r SidecarRestart) bool {
	switch r {
	case RestartNo, RestartAlways, RestartOnFailure, RestartUnlessStopped:
		return true
	default:
		return false
	}
}
