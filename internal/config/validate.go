package config

import (
	"fmt"
	"net"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var (
	rxServiceName  = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	rxUsername     = regexp.MustCompile(`^[a-z_][a-z0-9_-]*$`)
	rxPluginID     = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	rxPluginMethod = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	rxSha256       = regexp.MustCompile(`^[a-f0-9]{64}$`)
	rxEnvKey       = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	rxShellEnvKey  = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
	rxAliasKey     = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_-]*$`)
	rxRepoPath     = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9_./-]*$`)
	// rxHomeFilesSegment: each path segment of a [home_files].files entry
	// must consist only of POSIX portable filename chars (letters,
	// digits, dot, hyphen, underscore). home_files paths flow into the
	// generated initializeCommand as raw shell snippets (cocoon gen / VS
	// Code run them with /bin/sh), so anything with shell-special meaning
	// — $, backticks, ; & | < > ( ) * ? ! [ ] { } ~, quotes, backslashes,
	// whitespace, newlines — would let a repo-provided workspace.toml
	// inject commands into the host shell. Strict whitelist > best-effort
	// blacklist.
	rxHomeFilesSegment = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	rxLang             = regexp.MustCompile(`^[a-z]{2,3}_[A-Z]{2}\.UTF-8$`)
	rxEmail            = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
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
	// rxWorkspaceDir validates [workspace].dir: one or more portable filename
	// segments joined by `/`. Each segment is the same POSIX portable charset
	// rxHomeFilesSegment uses so the value can flow into Dockerfile WORKDIR /
	// docker-compose mount targets / docker-entrypoint.sh chown loops without
	// shell-special characters reaching those contexts. Absolute paths
	// (leading `/`), trailing slashes, `..` segments, and empty segments are
	// rejected by the regex shape; that lets the field stay relative to
	// /home/<user>/ and avoids container-escape paths.
	rxWorkspaceDir = regexp.MustCompile(`^[A-Za-z0-9._-]+(?:/[A-Za-z0-9._-]+)*$`)
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

// Accumulator collects FieldError rows scoped under a base path. The zero
// value is usable; NewAccumulator is just a convenience constructor.
type Accumulator struct {
	base []string
	errs *[]FieldError
}

// NewAccumulator returns an empty Accumulator rooted at no base path.
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

// Add records a FieldError at base+seg with the given message.
func (a *Accumulator) Add(msg string, seg ...string) {
	a.ensure()
	loc := make([]string, 0, len(a.base)+len(seg))
	loc = append(loc, a.base...)
	loc = append(loc, seg...)
	*a.errs = append(*a.errs, FieldError{Loc: loc, Message: msg})
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
	if w.Git != nil {
		w.Git.validate(a.At("git"))
	}
	for i, m := range w.Mounts {
		m.validate(a.At("mounts", fmt.Sprintf("%d", i)))
	}
	if w.HomeFiles != nil {
		w.HomeFiles.validate(a.At("home_files"))
	}
	w.validateServices(a)
	w.validateRepositories(a)
}

func (w *WorkspaceSpec) validate(a *Accumulator) {
	if w.MountRoot != "" && w.MountRoot != "." && w.MountRoot != ".." {
		a.Add(`mount_root must be "." or ".."`, "mount_root")
	}
	if w.Dir == "" {
		return
	}
	if !rxWorkspaceDir.MatchString(w.Dir) {
		a.Add(
			`dir must be one or more path segments of [A-Za-z0-9._-] joined by "/", `+
				`with no leading/trailing slash (e.g. "workspace" or "work/myproject")`,
			"dir",
		)
		return
	}
	// Each segment matches [A-Za-z0-9._-]+ so "." and ".." sneak through the
	// regex — reject them explicitly to keep dir relative and inside the
	// container home.
	for _, seg := range strings.Split(w.Dir, "/") {
		if seg == "." || seg == ".." {
			a.Add(`dir must not contain "." or ".." path segments`, "dir")
			return
		}
	}
}

func (c *ContainerSpec) validate(a *Accumulator) {
	if !rxServiceName.MatchString(c.ServiceName) {
		a.Add("service_name does not match "+rxServiceName.String(), "service_name")
	}
	if !rxUsername.MatchString(c.Username) {
		a.Add("username does not match "+rxUsername.String(), "username")
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
		a.Add(
			`os / os_version are no longer supported. Replace them with two fields under [container]:`+"\n"+
				`        image = "`+legacyOs+`"`+"\n"+
				`        image_version = "`+legacyVersion+`"`+"\n"+
				`    See docs/configuration.md for the full image/version suggestion list and CHANGELOG.md for migration notes.`,
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
		a.Add("contains duplicate entries")
	}
	for i, g := range groups {
		idx := fmt.Sprintf("%d", i)
		switch {
		case g == "":
			a.Add("must not be empty", idx)
		case !rxGroupName.MatchString(g) && !rxGID.MatchString(g):
			a.Add("must be a group name ("+rxGroupName.String()+") or a numeric GID", idx)
		}
	}
}

// validateDevices checks Compose `devices:` entries of the form
// HOST:CONTAINER[:rwm]. Both paths must be absolute; CDI device syntax is
// not supported.
func validateDevices(a *Accumulator, devices []string) {
	if HasDuplicates(devices) {
		a.Add("contains duplicate entries")
	}
	for i, d := range devices {
		idx := fmt.Sprintf("%d", i)
		parts := strings.Split(d, ":")
		if len(parts) < 2 || len(parts) > 3 {
			a.Add("must be HOST:CONTAINER or HOST:CONTAINER:rwm", idx)
			continue
		}
		if !rxAbsolutePath.MatchString(parts[0]) {
			a.Add("host path must be absolute", idx)
		}
		if !rxAbsolutePath.MatchString(parts[1]) {
			a.Add("container path must be absolute", idx)
		}
		if len(parts) == 3 && !rxDevicePerms.MatchString(parts[2]) {
			a.Add("cgroup permissions must be a combination of r, w, m", idx)
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
			a.Add(`container: requires a target name (e.g. "container:db")`)
		case !rxContainerName.MatchString(name):
			a.Add(fmt.Sprintf(
				`container:%s is not a valid Docker container name or ID (%s)`,
				name, rxContainerName.String()))
		}
		return
	}
	if name, ok := strings.CutPrefix(ipc, "service:"); ok {
		if name == "" {
			a.Add(`service: requires a target name (e.g. "service:db")`)
			return
		}
		if _, defined := w.Services[name]; !defined && name != w.Container.ServiceName {
			a.Add(fmt.Sprintf(
				`service:%s references undefined service %q. `+
					`Name the main service or a defined [services.<name>] sidecar.`,
				name, name,
			))
		}
		return
	}
	a.Add("ipc must be one of " + strings.Join(modes, ", ") +
		`, "service:<name>" or "container:<name>"`)
}

// validateGpus currently accepts only the literal "all"; the per-device
// list form (driver/count) is not yet exposed.
func validateGpus(a *Accumulator, gpus string) {
	if gpus != "all" {
		a.Add(`gpus must be "all" (the only value currently supported)`)
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
			a.Add("target must not end with / (files only, not directories)", idx, "target")
		} else {
			checkSkelPath(a.At(idx, "target"), e.Target, "target")
		}
		if prev, dup := seenSrc[e.Source]; dup && e.Source != "" {
			a.Add(fmt.Sprintf("source duplicates entry [%d]", prev), idx, "source")
		} else if e.Source != "" {
			seenSrc[e.Source] = i
		}
		if prev, dup := seenTgt[e.Target]; dup && e.Target != "" {
			a.Add(fmt.Sprintf("target duplicates entry [%d]", prev), idx, "target")
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
		a.Add("image is required and must be one of "+strings.Join(SupportedImages, ", "), "image")
		return
	}
	suggestions, known := SupportedImageVersions[image]
	if !known {
		a.Add("image must be one of "+strings.Join(SupportedImages, ", ")+" (got "+image+")", "image")
		return
	}
	if imageVersion == "" {
		a.Add(
			"image_version is required for image="+image+
				" (suggestions: "+strings.Join(suggestions, ", ")+
				"; any tag matching "+rxImageVersion.String()+" is accepted)",
			"image_version",
		)
		return
	}
	if !rxImageVersion.MatchString(imageVersion) {
		a.Add(
			"image_version "+strconv.Quote(imageVersion)+" does not match "+rxImageVersion.String()+
				" (use a plain Docker tag, e.g. "+suggestions[0]+")",
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
	a.Add(
		`image = "`+w.Container.Image+`" already provides `+pluginID+
			`. Remove "`+pluginID+`" from [plugins].enable, or switch to `+
			`image = "ubuntu" / "debian" to pin a custom `+pluginID+
			` version via the plugin.`,
		"image",
	)
}

func checkSkelPath(a *Accumulator, p, label string) {
	switch {
	case p == "":
		a.Add(label + " must not be empty")
	case strings.HasPrefix(p, "/"):
		a.Add(label + " must be relative (no leading /)")
	case strings.HasPrefix(p, "~"):
		a.Add(label + " must not start with ~")
	case strings.HasPrefix(p, "-"):
		a.Add(label + " must not start with `-` (would look like a Dockerfile flag in the COPY line)")
	case strings.Contains(p, ":"):
		a.Add(label + " must not contain `:` (would corrupt the COPY directive)")
	case containsWhitespaceOrCtrl(p):
		a.Add(label + " must not contain whitespace or control characters " +
			"(unquoted in the generated COPY line)")
	default:
		for _, seg := range strings.Split(p, "/") {
			if seg == ".." {
				a.Add(label + " must not contain `..` segments")
				return
			}
			if seg == "" {
				a.Add(label + " must not contain empty segments (// or trailing /)")
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

// hasDotDotSegment treats ".." as a full path segment (split on "/"),
// distinct from a substring like "foo..bar".
func hasDotDotSegment(p string) bool {
	return slices.Contains(strings.Split(p, "/"), "..")
}

func (s *SecurityOptSpec) validate(a *Accumulator) {
	if s.Seccomp != nil && *s.Seccomp == "" {
		a.Add("seccomp must not be empty (omit the key to use Docker's default)", "seccomp")
	}
	if s.AppArmor != nil && *s.AppArmor == "" {
		a.Add("apparmor must not be empty (omit the key to use Docker's default)", "apparmor")
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
			a.Add(fmt.Sprintf("%q appears in both add and drop", c))
			break
		}
	}
	for i, c := range cs.Drop {
		if _, required := entrypointRequiredCaps[strings.TrimPrefix(c, "CAP_")]; required {
			a.Add(fmt.Sprintf(
				"%q cannot be dropped: docker-entrypoint.sh needs CHOWN, SETUID, "+
					"and SETGID at container start to remap the user and drop "+
					"privileges", c), "drop", fmt.Sprintf("%d", i))
		}
	}
}

func checkCapList(a *Accumulator, caps []string) {
	if HasDuplicates(caps) {
		a.Add("contains duplicate entries")
	}
	for i, c := range caps {
		if !rxCapability.MatchString(c) {
			a.Add("capability does not match "+rxCapability.String(), fmt.Sprintf("%d", i))
		}
	}
}

// validateSysctls accepts numeric or string values (Compose v3 forwards both
// transparently). Anything else is rejected so users notice typos before
// docker compose up complains.
func validateSysctls(a *Accumulator, sysctls map[string]any) {
	for key, val := range sysctls {
		if !rxSysctlKey.MatchString(key) {
			a.Add("sysctl key does not match "+rxSysctlKey.String(), key)
			continue
		}
		switch val.(type) {
		case int64, int, string:
			// ok
		default:
			a.Add(fmt.Sprintf("sysctl value must be int or string (got %T)", val), key)
		}
	}
}

func (d *DNSSpec) validate(a *Accumulator) {
	if HasDuplicates(d.Servers) {
		a.Add("servers contains duplicate entries", "servers")
	}
	for i, ip := range d.Servers {
		if net.ParseIP(ip) == nil {
			a.Add(fmt.Sprintf("%q is not a valid IPv4/IPv6 address", ip), "servers", fmt.Sprintf("%d", i))
		}
	}
	if HasDuplicates(d.Search) {
		a.Add("search contains duplicate entries", "search")
	}
	for i, dom := range d.Search {
		if !rxHostname.MatchString(dom) {
			a.Add("search domain does not match "+rxHostname.String(), "search", fmt.Sprintf("%d", i))
		}
	}
}

// validateExtraHosts enforces hostname keys plus IP-or-"host-gateway" values.
// Empty maps are skipped so the generator can omit extra_hosts entirely.
func validateExtraHosts(a *Accumulator, hosts map[string]string) {
	for host, addr := range hosts {
		if !rxHostname.MatchString(host) {
			a.Add("hostname does not match "+rxHostname.String(), host)
			continue
		}
		if addr == "host-gateway" {
			continue
		}
		if net.ParseIP(addr) == nil {
			a.Add(fmt.Sprintf(`%q must be an IPv4/IPv6 address or the literal "host-gateway"`, addr), host)
		}
	}
}

func (s *ContainerShellSpec) validate(a *Accumulator) {
	if s.Default != nil {
		v := *s.Default
		// "" is treated as bash by the generator; anything else must be in
		// the SupportedShells closed set.
		if v != "" && !slices.Contains(SupportedShells, v) {
			a.Add(`default must be one of "`+strings.Join(SupportedShells, `", "`)+`"`, "default")
		}
	}
	CheckMapKeys(a.At("aliases"), s.Aliases, rxAliasKey, "container.shell.aliases")
	CheckMapKeys(a.At("env"), s.Env, rxShellEnvKey, "container.shell.env")
}

func (p *PluginsSpec) validate(a *Accumulator) {
	for i, id := range p.Enable {
		if !rxPluginID.MatchString(id) {
			a.Add("plugin id does not match "+rxPluginID.String(), "enable", fmt.Sprintf("%d", i))
		}
	}
	if HasDuplicates(p.Enable) {
		a.Add("plugins.enable contains duplicate entries", "enable")
	}
	for name, ov := range p.Versions {
		if ov.Pin == "" {
			a.Add("pin must not be empty", "versions", name, "pin")
		}
		if ov.ChecksumAmd64 != nil && !rxSha256.MatchString(*ov.ChecksumAmd64) {
			a.Add("checksum_amd64 must be 64 lowercase hex chars", "versions", name, "checksum_amd64")
		}
		if ov.ChecksumArm64 != nil && !rxSha256.MatchString(*ov.ChecksumArm64) {
			a.Add("checksum_arm64 must be 64 lowercase hex chars", "versions", name, "checksum_arm64")
		}
	}
	// Sort plugin ids so ValidationError.Error()'s "first error"
	// summary stays stable across runs (map iteration is randomised).
	methodIDs := make([]string, 0, len(p.Methods))
	for id := range p.Methods {
		methodIDs = append(methodIDs, id)
	}
	slices.Sort(methodIDs)
	for _, id := range methodIDs {
		method := p.Methods[id]
		if !rxPluginID.MatchString(id) {
			a.Add("plugin id does not match "+rxPluginID.String(), "methods", id)
		}
		if !rxPluginMethod.MatchString(method) {
			a.Add("method name does not match "+rxPluginMethod.String(), "methods", id)
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
		a.Add("url must start with http:// or https://", "url")
	case containsUnsafeForSed(m.URL):
		a.Add("url must not contain whitespace, control characters, "+
			"or any of `'`, `|`, `&`, `\\` (would corrupt the generated "+
			"sed RUN block)", "url")
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
		a.Add("http must start with http:// or https://", "http")
	}
	if p.HTTPS != nil && !isHTTPURL(*p.HTTPS) {
		a.Add("https must start with http:// or https://", "https")
	}
}

func (src *AptSource) validate(a *Accumulator, i int, seen map[string]int) {
	switch {
	case !rxAptName.MatchString(src.Name):
		a.Add("name does not match "+rxAptName.String(), "name")
	default:
		if prev, dup := seen[src.Name]; dup {
			a.Add(fmt.Sprintf("name duplicates entry [%d]", prev), "name")
		} else {
			seen[src.Name] = i
		}
	}
	if !rxAptSuite.MatchString(src.Suite) {
		a.Add("suite does not match "+rxAptSuite.String(), "suite")
	}
	if len(src.Components) == 0 {
		a.Add("components must not be empty", "components")
	}
	for ci, comp := range src.Components {
		if !rxAptComponent.MatchString(comp) {
			a.Add("component does not match "+rxAptComponent.String(),
				"components", fmt.Sprintf("%d", ci))
		}
	}
	if !isHTTPURL(src.URL) {
		a.Add("url must start with http:// or https://", "url")
	}
	if !isHTTPURL(src.KeyURL) {
		a.Add("key_url must start with http:// or https://", "key_url")
	}
	if src.Arch != nil && !rxAptArch.MatchString(*src.Arch) {
		a.Add("arch must be one of amd64/arm64/i386/armhf/ppc64el/s390x", "arch")
	}
}

func isHTTPURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func (l *LocaleSpec) validate(a *Accumulator) {
	if l.Lang != nil && !rxLang.MatchString(*l.Lang) {
		a.Add("lang does not match "+rxLang.String(), "lang")
	}
}

func (g *GitIdentitySpec) validate(a *Accumulator) {
	if g.UserEmail != nil && !rxEmail.MatchString(*g.UserEmail) {
		a.Add("user_email does not match "+rxEmail.String(), "user_email")
	}
}

// No-op hook (only field is enable: bool); kept so future fields slot in
// without re-wiring runValidate.
func (*CertificatesSpec) validate(_ *Accumulator) {}

func (m *Mount) validate(a *Accumulator) {
	if m.Source == "" {
		a.Add("source must not be empty", "source")
	}
	if !rxAbsolutePath.MatchString(m.Target) {
		a.Add("target must be an absolute path", "target")
	} else if !rxMountTarget.MatchString(m.Target) {
		a.Add("target may contain only [A-Za-z0-9._/-] and the ${USERNAME} "+
			"placeholder (quotes, backticks, $, :, whitespace are rejected "+
			"because the target flows unquoted into the generated Dockerfile "+
			"ENV and the docker-compose volume spec)", "target")
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
		a.Add("files contains duplicate entries", "files")
	}
	for i, p := range h.Files {
		idx := fmt.Sprintf("%d", i)
		switch {
		case p == "":
			a.Add("entry must not be empty", "files", idx)
		case strings.HasPrefix(p, "/"):
			a.Add("entry must be relative to ~/ (no leading /)", "files", idx)
		case strings.HasPrefix(p, "~"):
			a.Add("entry must not start with ~ (~/ is implied)", "files", idx)
		case strings.HasPrefix(p, "./") || strings.HasPrefix(p, "../"):
			a.Add("entry must not start with ./ or ../", "files", idx)
		case strings.Contains(p, ":"):
			a.Add("entry must not contain `:` (would corrupt the docker-compose volume spec)", "files", idx)
		case strings.HasSuffix(p, "/"):
			a.Add("entry must not end with / (files only, not directories)", "files", idx)
		default:
			rejected := false
			for _, seg := range strings.Split(p, "/") {
				if seg == ".." {
					a.Add("entry must not contain `..` segments", "files", idx)
					rejected = true
					break
				}
				if seg == "" {
					a.Add("entry must not contain empty segments (// or trailing /)", "files", idx)
					rejected = true
					break
				}
			}
			if !rejected && !rxHomeFilesSegmentPath(p) {
				a.Add(
					"entry must match [A-Za-z0-9._/-]+ "+
						"(shell-special characters like $, `, ;, &, |, spaces, "+
						"quotes are rejected because home_files paths flow into "+
						"the generated initializeCommand shell snippet)",
					"files", idx,
				)
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
		a.Add(fmt.Sprintf(
			`[services.%s] collides with [container].service_name = %q. Rename one of them.`,
			main, main,
		), "services")
	}
	for name, spec := range w.Services {
		scope := a.At("services", name)
		if !rxServiceName.MatchString(name) {
			a.Add("service name does not match "+rxServiceName.String(), "services", name)
		}
		if spec.Image == "" {
			scope.Add("image must not be empty", "image")
		}
		if spec.Env != nil {
			CheckMapKeys(scope.At("env"), spec.Env, rxEnvKey, "services.env")
		}
		if HasDuplicates(spec.DependsOn) {
			scope.Add("services.depends_on contains duplicate entries", "depends_on")
		}
		for _, dep := range spec.DependsOn {
			switch dep {
			case main:
				scope.Add(fmt.Sprintf(
					`[services.%s].depends_on references the main service %q. `+
						`Sidecars cannot depend on the main dev container; the dependency `+
						`direction is the opposite.`,
					name, main,
				), "depends_on")
			case name:
				scope.Add(fmt.Sprintf(
					`[services.%s].depends_on references itself.`, name,
				), "depends_on")
			default:
				if _, ok := w.Services[dep]; !ok {
					scope.Add(fmt.Sprintf(
						`[services.%s].depends_on references undefined sidecar %q. `+
							`Define [services.%s] or remove the dependency.`,
						name, dep, dep,
					), "depends_on")
				}
			}
		}
		if _, hasLocal := spec.Volumes["local"]; hasLocal {
			scope.Add(fmt.Sprintf(
				`[services.%s].volumes uses reserved name "local". Pick another volume key.`, name,
			), "volumes")
		}
		for k, v := range spec.Volumes {
			if !rxAbsolutePath.MatchString(v) {
				scope.Add("volume target must be an absolute path", "volumes", k)
			}
		}
		for i, m := range spec.Mounts {
			m.validate(scope.At("mounts", fmt.Sprintf("%d", i)))
		}
		if spec.Restart != nil && !validRestart(*spec.Restart) {
			scope.Add(fmt.Sprintf(
				"restart must be one of no/always/on-failure/unless-stopped (got %q)",
				*spec.Restart,
			), "restart")
		}
	}
}

func (sm *SidecarMount) validate(a *Accumulator) {
	if sm.Source == "" {
		a.Add("source must not be empty", "source")
	}
	if !rxAbsolutePath.MatchString(sm.Target) {
		a.Add("target must be an absolute path", "target")
	}
}

//nolint:gocognit,gocyclo // straight-line per-entry validation; splitting fragments the rules.
func (w *Workspace) validateRepositories(a *Accumulator) {
	if w.Repositories == nil || len(w.Repositories.Clone) == 0 {
		return
	}
	scope := a.At("repositories", "clone")
	seenPaths := make(map[string]int, len(w.Repositories.Clone))
	seenURLs := make(map[string]int, len(w.Repositories.Clone))
	for i, entry := range w.Repositories.Clone {
		idx := fmt.Sprintf("%d", i)
		if entry.URL == "" {
			scope.Add("url must not be empty", idx, "url")
		}
		if entry.Path != nil {
			p := *entry.Path
			switch {
			case hasDotDotSegment(p):
				scope.Add("path must not contain `..` segments", idx, "path")
			case !rxRepoPath.MatchString(p):
				scope.Add(
					`path must contain only [A-Za-z0-9_./-], not start with "." or "/" `+
						`(e.g. "foo/bar")`, idx, "path")
			}
		}
		var pathField string
		if entry.Path != nil {
			pathField = *entry.Path
		}
		resolved := ResolveRepoPath(pathField, entry.URL)
		if resolved == "" {
			scope.Add(fmt.Sprintf(
				`[repositories].clone[%d]: cannot derive target path from url=%q; `+
					`specify `+"`path`"+` explicitly.`,
				i, entry.URL,
			), idx)
			continue
		}
		segments := strings.Split(resolved, "/")
		for _, s := range segments {
			if s == ".." {
				scope.Add(fmt.Sprintf(
					"[repositories].clone[%d].path=%q: must not contain `..` segments "+
						"(would escape the parent workspace).",
					i, resolved,
				), idx, "path")
				break
			}
		}
		if strings.HasPrefix(resolved, "..") {
			scope.Add(fmt.Sprintf(
				"[repositories].clone[%d].path=%q: must not contain `..` segments "+
					"(would escape the parent workspace).",
				i, resolved,
			), idx, "path")
		}
		if resolved == "workspace-docker" {
			scope.Add(fmt.Sprintf(
				"[repositories].clone[%d].path=%q: cannot overwrite workspace-docker itself.",
				i, resolved,
			), idx, "path")
		}
		if prev, ok := seenPaths[resolved]; ok {
			scope.Add(fmt.Sprintf(
				"[repositories].clone[%d].path=%q: collides with entry [%d] (same target dir).",
				i, resolved, prev,
			), idx, "path")
		} else {
			seenPaths[resolved] = i
		}
		if prev, ok := seenURLs[entry.URL]; ok {
			scope.Add(fmt.Sprintf(
				"[repositories].clone[%d].url=%q: duplicates entry [%d].",
				i, entry.URL, prev,
			), idx, "url")
		} else {
			seenURLs[entry.URL] = i
		}
		if entry.Depth != nil && *entry.Depth < 1 {
			scope.Add("depth must be >= 1", idx, "depth")
		}
	}
}

// CheckMapKeys records a FieldError for every key of m that fails rx.
func CheckMapKeys(a *Accumulator, m map[string]string, rx *regexp.Regexp, label string) {
	for k := range m {
		if !rx.MatchString(k) {
			a.Add(fmt.Sprintf("%s key %q does not match pattern %q", label, k, rx.String()), k)
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
