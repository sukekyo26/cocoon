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
	// rxHostname matches an RFC 1123-style hostname: dot-joined labels,
	// each label 1+ alnum chars (hyphen allowed inside but not at the
	// start/end). Underscores and consecutive dots (a..b) are rejected.
	rxHostnameLabel = `[a-zA-Z0-9](?:[a-zA-Z0-9-]*[a-zA-Z0-9])?`
	rxHostname      = regexp.MustCompile(
		`^(?:` + rxHostnameLabel + `)(?:\.(?:` + rxHostnameLabel + `))*$`)
	rxSysctlKey    = regexp.MustCompile(`^[a-z][a-z0-9._-]*$`)
	rxCapability   = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
	rxAptName      = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	rxAptSuite     = regexp.MustCompile(`^[a-z][a-z0-9._-]*$`)
	rxAptComponent = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	rxAptArch      = regexp.MustCompile(`^(amd64|arm64|i386|armhf|ppc64el|s390x)$`)
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

// errAccumulator collects FieldError rows scoped under a base path.
type errAccumulator struct {
	base []string
	errs *[]FieldError
}

func newAccumulator() *errAccumulator {
	errs := make([]FieldError, 0)
	return &errAccumulator{base: nil, errs: &errs}
}

func (a *errAccumulator) at(seg ...string) *errAccumulator {
	out := make([]string, 0, len(a.base)+len(seg))
	out = append(out, a.base...)
	out = append(out, seg...)
	return &errAccumulator{base: out, errs: a.errs}
}

func (a *errAccumulator) add(msg string, seg ...string) {
	loc := make([]string, 0, len(a.base)+len(seg))
	loc = append(loc, a.base...)
	loc = append(loc, seg...)
	*a.errs = append(*a.errs, FieldError{Loc: loc, Message: msg})
}

// Validate runs every cross-field check on the workspace. On failure the
// returned error is a *ValidationError with Path = path.
func (w *Workspace) Validate(path string) error {
	a := newAccumulator()
	w.runValidate(a)
	if len(*a.errs) == 0 {
		return nil
	}
	return &ValidationError{Path: path, Errors: *a.errs}
}

func (w *Workspace) runValidate(a *errAccumulator) {
	if w.Workspace != nil {
		w.Workspace.validate(a.at("workspace"))
	}
	w.Container.validate(a.at("container"))
	w.Plugins.validate(a.at("plugins"))
	w.validateImagePluginConflict(a.at("container"))
	if w.Ports != nil {
		w.Ports.validate(a.at("ports"))
	}
	checkMapKeys(a.at("env"), w.Env, rxEnvKey, "env")
	if w.Apt != nil {
		w.Apt.validate(a.at("apt"))
	}
	if w.Locale != nil {
		w.Locale.validate(a.at("locale"))
	}
	if w.Certificates != nil {
		w.Certificates.validate(a.at("certificates"))
	}
	if w.Git != nil {
		w.Git.validate(a.at("git"))
	}
	for i, m := range w.Mounts {
		m.validate(a.at("mounts", fmt.Sprintf("%d", i)))
	}
	if w.HomeFiles != nil {
		w.HomeFiles.validate(a.at("home_files"))
	}
	w.validateServices(a)
	w.validateRepositories(a)
}

func (w *WorkspaceSpec) validate(a *errAccumulator) {
	if w.MountRoot != "" && w.MountRoot != "." && w.MountRoot != ".." {
		a.add(`mount_root must be "." or ".."`, "mount_root")
	}
}

func (c *ContainerSpec) validate(a *errAccumulator) {
	if !rxServiceName.MatchString(c.ServiceName) {
		a.add("service_name does not match "+rxServiceName.String(), "service_name")
	}
	if !rxUsername.MatchString(c.Username) {
		a.add("username does not match "+rxUsername.String(), "username")
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
		a.add(
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
		c.Shell.validate(a.at("shell"))
	}
	validateExtraHosts(a.at("hosts"), c.Hosts)
	if c.DNS != nil {
		c.DNS.validate(a.at("dns"))
	}
	validateSysctls(a.at("sysctls"), c.Sysctls)
	if c.Capabilities != nil {
		c.Capabilities.validate(a.at("capabilities"))
	}
	if c.SecurityOpt != nil {
		c.SecurityOpt.validate(a.at("security_opt"))
	}
	validateSkel(a.at("skel"), c.Skel)
}

// validateSkel leaves existence of Source to docker build (COPY surfaces a
// clean "file not found" error).
func validateSkel(a *errAccumulator, entries []SkelEntry) {
	seenSrc := map[string]int{}
	seenTgt := map[string]int{}
	for i, e := range entries {
		idx := fmt.Sprintf("%d", i)
		checkSkelPath(a.at(idx, "source"), e.Source, "source")
		if strings.HasSuffix(e.Target, "/") {
			a.add("target must not end with / (files only, not directories)", idx, "target")
		} else {
			checkSkelPath(a.at(idx, "target"), e.Target, "target")
		}
		if prev, dup := seenSrc[e.Source]; dup && e.Source != "" {
			a.add(fmt.Sprintf("source duplicates entry [%d]", prev), idx, "source")
		} else if e.Source != "" {
			seenSrc[e.Source] = i
		}
		if prev, dup := seenTgt[e.Target]; dup && e.Target != "" {
			a.add(fmt.Sprintf("target duplicates entry [%d]", prev), idx, "target")
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
func validateImage(a *errAccumulator, image, imageVersion string) {
	if image == "" {
		a.add("image is required and must be one of "+strings.Join(SupportedImages, ", "), "image")
		return
	}
	suggestions, known := SupportedImageVersions[image]
	if !known {
		a.add("image must be one of "+strings.Join(SupportedImages, ", ")+" (got "+image+")", "image")
		return
	}
	if imageVersion == "" {
		a.add(
			"image_version is required for image="+image+
				" (suggestions: "+strings.Join(suggestions, ", ")+
				"; any tag matching "+rxImageVersion.String()+" is accepted)",
			"image_version",
		)
		return
	}
	if !rxImageVersion.MatchString(imageVersion) {
		a.add(
			"image_version "+strconv.Quote(imageVersion)+" does not match "+rxImageVersion.String()+
				" (use a plain Docker tag, e.g. "+suggestions[0]+")",
			"image_version",
		)
	}
}

// validateImagePluginConflict rejects pairs from ImageProvidesPlugin where
// the base toolchain would be overwritten or shadowed by the plugin (wasting
// docker-build time for no benefit). Other combinations coexist cleanly.
func (w *Workspace) validateImagePluginConflict(a *errAccumulator) {
	pluginID, conflicts := ImageProvidesPlugin[w.Container.Image]
	if !conflicts {
		return
	}
	if !slices.Contains(w.Plugins.Enable, pluginID) {
		return
	}
	a.add(
		`image = "`+w.Container.Image+`" already provides `+pluginID+
			`. Remove "`+pluginID+`" from [plugins].enable, or switch to `+
			`image = "ubuntu" / "debian" to pin a custom `+pluginID+
			` version via the plugin.`,
		"image",
	)
}

func checkSkelPath(a *errAccumulator, p, label string) {
	switch {
	case p == "":
		a.add(label + " must not be empty")
	case strings.HasPrefix(p, "/"):
		a.add(label + " must be relative (no leading /)")
	case strings.HasPrefix(p, "~"):
		a.add(label + " must not start with ~")
	case strings.HasPrefix(p, "-"):
		a.add(label + " must not start with `-` (would look like a Dockerfile flag in the COPY line)")
	case strings.Contains(p, ":"):
		a.add(label + " must not contain `:` (would corrupt the COPY directive)")
	case containsWhitespaceOrCtrl(p):
		a.add(label + " must not contain whitespace or control characters " +
			"(unquoted in the generated COPY line)")
	default:
		for _, seg := range strings.Split(p, "/") {
			if seg == ".." {
				a.add(label + " must not contain `..` segments")
				return
			}
			if seg == "" {
				a.add(label + " must not contain empty segments (// or trailing /)")
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

func (s *SecurityOptSpec) validate(a *errAccumulator) {
	if s.Seccomp != nil && *s.Seccomp == "" {
		a.add("seccomp must not be empty (omit the key to use Docker's default)", "seccomp")
	}
	if s.AppArmor != nil && *s.AppArmor == "" {
		a.add("apparmor must not be empty (omit the key to use Docker's default)", "apparmor")
	}
}

func (cs *CapabilitiesSpec) validate(a *errAccumulator) {
	checkCapList(a.at("add"), cs.Add)
	checkCapList(a.at("drop"), cs.Drop)
	addSet := make(map[string]struct{}, len(cs.Add))
	for _, c := range cs.Add {
		addSet[c] = struct{}{}
	}
	for _, c := range cs.Drop {
		if _, conflict := addSet[c]; conflict {
			a.add(fmt.Sprintf("%q appears in both add and drop", c))
			break
		}
	}
}

func checkCapList(a *errAccumulator, caps []string) {
	if hasDuplicates(caps) {
		a.add("contains duplicate entries")
	}
	for i, c := range caps {
		if !rxCapability.MatchString(c) {
			a.add("capability does not match "+rxCapability.String(), fmt.Sprintf("%d", i))
		}
	}
}

// validateSysctls accepts numeric or string values (Compose v3 forwards both
// transparently). Anything else is rejected so users notice typos before
// docker compose up complains.
func validateSysctls(a *errAccumulator, sysctls map[string]any) {
	for key, val := range sysctls {
		if !rxSysctlKey.MatchString(key) {
			a.add("sysctl key does not match "+rxSysctlKey.String(), key)
			continue
		}
		switch val.(type) {
		case int64, int, string:
			// ok
		default:
			a.add(fmt.Sprintf("sysctl value must be int or string (got %T)", val), key)
		}
	}
}

func (d *DNSSpec) validate(a *errAccumulator) {
	if hasDuplicates(d.Servers) {
		a.add("servers contains duplicate entries", "servers")
	}
	for i, ip := range d.Servers {
		if net.ParseIP(ip) == nil {
			a.add(fmt.Sprintf("%q is not a valid IPv4/IPv6 address", ip), "servers", fmt.Sprintf("%d", i))
		}
	}
	if hasDuplicates(d.Search) {
		a.add("search contains duplicate entries", "search")
	}
	for i, dom := range d.Search {
		if !rxHostname.MatchString(dom) {
			a.add("search domain does not match "+rxHostname.String(), "search", fmt.Sprintf("%d", i))
		}
	}
}

// validateExtraHosts enforces hostname keys plus IP-or-"host-gateway" values.
// Empty maps are skipped so the generator can omit extra_hosts entirely.
func validateExtraHosts(a *errAccumulator, hosts map[string]string) {
	for host, addr := range hosts {
		if !rxHostname.MatchString(host) {
			a.add("hostname does not match "+rxHostname.String(), host)
			continue
		}
		if addr == "host-gateway" {
			continue
		}
		if net.ParseIP(addr) == nil {
			a.add(fmt.Sprintf(`%q must be an IPv4/IPv6 address or the literal "host-gateway"`, addr), host)
		}
	}
}

func (s *ContainerShellSpec) validate(a *errAccumulator) {
	if s.Default != nil {
		v := *s.Default
		// "" is treated as bash by the generator; anything else must be in
		// the SupportedShells closed set.
		if v != "" && !slices.Contains(SupportedShells, v) {
			a.add(`default must be one of "`+strings.Join(SupportedShells, `", "`)+`"`, "default")
		}
	}
	checkMapKeys(a.at("aliases"), s.Aliases, rxAliasKey, "container.shell.aliases")
	checkMapKeys(a.at("env"), s.Env, rxShellEnvKey, "container.shell.env")
}

func (p *PluginsSpec) validate(a *errAccumulator) {
	for i, id := range p.Enable {
		if !rxPluginID.MatchString(id) {
			a.add("plugin id does not match "+rxPluginID.String(), "enable", fmt.Sprintf("%d", i))
		}
	}
	if hasDuplicates(p.Enable) {
		a.add("plugins.enable contains duplicate entries", "enable")
	}
	for name, ov := range p.Versions {
		if ov.Pin == "" {
			a.add("pin must not be empty", "versions", name, "pin")
		}
		if ov.ChecksumAmd64 != nil && !rxSha256.MatchString(*ov.ChecksumAmd64) {
			a.add("checksum_amd64 must be 64 lowercase hex chars", "versions", name, "checksum_amd64")
		}
		if ov.ChecksumArm64 != nil && !rxSha256.MatchString(*ov.ChecksumArm64) {
			a.add("checksum_arm64 must be 64 lowercase hex chars", "versions", name, "checksum_arm64")
		}
	}
	for id, method := range p.Methods {
		if !rxPluginID.MatchString(id) {
			a.add("plugin id does not match "+rxPluginID.String(), "methods", id)
		}
		if !rxPluginMethod.MatchString(method) {
			a.add("method name does not match "+rxPluginMethod.String(), "methods", id)
		}
	}
}

func (p *PortsSpec) validate(a *errAccumulator) {
	validatePortsForward(a, p.Forward)
}

func (s *AptSpec) validate(a *errAccumulator) {
	if s.Mirror != nil {
		s.Mirror.validate(a.at("mirror"))
	}
	if s.Proxy != nil {
		s.Proxy.validate(a.at("proxy"))
	}
	seenNames := map[string]int{}
	for i, src := range s.Sources {
		idx := fmt.Sprintf("%d", i)
		src.validate(a.at("sources", idx), i, seenNames)
	}
}

func (m *AptMirror) validate(a *errAccumulator) {
	switch {
	case !isHTTPURL(m.URL):
		a.add("url must start with http:// or https://", "url")
	case containsUnsafeForSed(m.URL):
		a.add("url must not contain whitespace, control characters, "+
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

func (p *AptProxy) validate(a *errAccumulator) {
	if p.HTTP != nil && !isHTTPURL(*p.HTTP) {
		a.add("http must start with http:// or https://", "http")
	}
	if p.HTTPS != nil && !isHTTPURL(*p.HTTPS) {
		a.add("https must start with http:// or https://", "https")
	}
}

func (src *AptSource) validate(a *errAccumulator, i int, seen map[string]int) {
	switch {
	case !rxAptName.MatchString(src.Name):
		a.add("name does not match "+rxAptName.String(), "name")
	default:
		if prev, dup := seen[src.Name]; dup {
			a.add(fmt.Sprintf("name duplicates entry [%d]", prev), "name")
		} else {
			seen[src.Name] = i
		}
	}
	if !rxAptSuite.MatchString(src.Suite) {
		a.add("suite does not match "+rxAptSuite.String(), "suite")
	}
	if len(src.Components) == 0 {
		a.add("components must not be empty", "components")
	}
	for ci, comp := range src.Components {
		if !rxAptComponent.MatchString(comp) {
			a.add("component does not match "+rxAptComponent.String(),
				"components", fmt.Sprintf("%d", ci))
		}
	}
	if !isHTTPURL(src.URL) {
		a.add("url must start with http:// or https://", "url")
	}
	if !isHTTPURL(src.KeyURL) {
		a.add("key_url must start with http:// or https://", "key_url")
	}
	if src.Arch != nil && !rxAptArch.MatchString(*src.Arch) {
		a.add("arch must be one of amd64/arm64/i386/armhf/ppc64el/s390x", "arch")
	}
}

func isHTTPURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func (l *LocaleSpec) validate(a *errAccumulator) {
	if l.Lang != nil && !rxLang.MatchString(*l.Lang) {
		a.add("lang does not match "+rxLang.String(), "lang")
	}
}

func (g *GitIdentitySpec) validate(a *errAccumulator) {
	if g.UserEmail != nil && !rxEmail.MatchString(*g.UserEmail) {
		a.add("user_email does not match "+rxEmail.String(), "user_email")
	}
}

// No-op hook (only field is enable: bool); kept so future fields slot in
// without re-wiring runValidate.
func (*CertificatesSpec) validate(_ *errAccumulator) {}

func (m *Mount) validate(a *errAccumulator) {
	if m.Source == "" {
		a.add("source must not be empty", "source")
	}
	if !rxAbsolutePath.MatchString(m.Target) {
		a.add("target must be an absolute path", "target")
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

func (h *HomeFilesSpec) validate(a *errAccumulator) {
	if hasDuplicates(h.Files) {
		a.add("files contains duplicate entries", "files")
	}
	for i, p := range h.Files {
		idx := fmt.Sprintf("%d", i)
		switch {
		case p == "":
			a.add("entry must not be empty", "files", idx)
		case strings.HasPrefix(p, "/"):
			a.add("entry must be relative to ~/ (no leading /)", "files", idx)
		case strings.HasPrefix(p, "~"):
			a.add("entry must not start with ~ (~/ is implied)", "files", idx)
		case strings.HasPrefix(p, "./") || strings.HasPrefix(p, "../"):
			a.add("entry must not start with ./ or ../", "files", idx)
		case strings.Contains(p, ":"):
			a.add("entry must not contain `:` (would corrupt the docker-compose volume spec)", "files", idx)
		case strings.HasSuffix(p, "/"):
			a.add("entry must not end with / (files only, not directories)", "files", idx)
		default:
			rejected := false
			for _, seg := range strings.Split(p, "/") {
				if seg == ".." {
					a.add("entry must not contain `..` segments", "files", idx)
					rejected = true
					break
				}
				if seg == "" {
					a.add("entry must not contain empty segments (// or trailing /)", "files", idx)
					rejected = true
					break
				}
			}
			if !rejected && !rxHomeFilesSegmentPath(p) {
				a.add(
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
func (w *Workspace) validateServices(a *errAccumulator) {
	if len(w.Services) == 0 {
		return
	}
	main := w.Container.ServiceName
	if _, collide := w.Services[main]; collide {
		a.add(fmt.Sprintf(
			`[services.%s] collides with [container].service_name = %q. Rename one of them.`,
			main, main,
		), "services")
	}
	for name, spec := range w.Services {
		scope := a.at("services", name)
		if !rxServiceName.MatchString(name) {
			a.add("service name does not match "+rxServiceName.String(), "services", name)
		}
		if spec.Image == "" {
			scope.add("image must not be empty", "image")
		}
		if spec.Env != nil {
			checkMapKeys(scope.at("env"), spec.Env, rxEnvKey, "services.env")
		}
		if hasDuplicates(spec.DependsOn) {
			scope.add("services.depends_on contains duplicate entries", "depends_on")
		}
		for _, dep := range spec.DependsOn {
			switch dep {
			case main:
				scope.add(fmt.Sprintf(
					`[services.%s].depends_on references the main service %q. `+
						`Sidecars cannot depend on the main dev container; the dependency `+
						`direction is the opposite.`,
					name, main,
				), "depends_on")
			case name:
				scope.add(fmt.Sprintf(
					`[services.%s].depends_on references itself.`, name,
				), "depends_on")
			default:
				if _, ok := w.Services[dep]; !ok {
					scope.add(fmt.Sprintf(
						`[services.%s].depends_on references undefined sidecar %q. `+
							`Define [services.%s] or remove the dependency.`,
						name, dep, dep,
					), "depends_on")
				}
			}
		}
		if _, hasLocal := spec.Volumes["local"]; hasLocal {
			scope.add(fmt.Sprintf(
				`[services.%s].volumes uses reserved name "local". Pick another volume key.`, name,
			), "volumes")
		}
		for k, v := range spec.Volumes {
			if !rxAbsolutePath.MatchString(v) {
				scope.add("volume target must be an absolute path", "volumes", k)
			}
		}
		for i, m := range spec.Mounts {
			m.validate(scope.at("mounts", fmt.Sprintf("%d", i)))
		}
		if spec.Restart != nil && !validRestart(*spec.Restart) {
			scope.add(fmt.Sprintf(
				"restart must be one of no/always/on-failure/unless-stopped (got %q)",
				*spec.Restart,
			), "restart")
		}
	}
}

func (sm *SidecarMount) validate(a *errAccumulator) {
	if sm.Source == "" {
		a.add("source must not be empty", "source")
	}
	if !rxAbsolutePath.MatchString(sm.Target) {
		a.add("target must be an absolute path", "target")
	}
}

//nolint:gocognit,gocyclo // straight-line per-entry validation; splitting fragments the rules.
func (w *Workspace) validateRepositories(a *errAccumulator) {
	if w.Repositories == nil || len(w.Repositories.Clone) == 0 {
		return
	}
	scope := a.at("repositories", "clone")
	seenPaths := make(map[string]int, len(w.Repositories.Clone))
	seenURLs := make(map[string]int, len(w.Repositories.Clone))
	for i, entry := range w.Repositories.Clone {
		idx := fmt.Sprintf("%d", i)
		if entry.URL == "" {
			scope.add("url must not be empty", idx, "url")
		}
		if entry.Path != nil {
			p := *entry.Path
			switch {
			case hasDotDotSegment(p):
				scope.add("path must not contain `..` segments", idx, "path")
			case !rxRepoPath.MatchString(p):
				scope.add(
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
			scope.add(fmt.Sprintf(
				`[repositories].clone[%d]: cannot derive target path from url=%q; `+
					`specify `+"`path`"+` explicitly.`,
				i, entry.URL,
			), idx)
			continue
		}
		segments := strings.Split(resolved, "/")
		for _, s := range segments {
			if s == ".." {
				scope.add(fmt.Sprintf(
					"[repositories].clone[%d].path=%q: must not contain `..` segments "+
						"(would escape the parent workspace).",
					i, resolved,
				), idx, "path")
				break
			}
		}
		if strings.HasPrefix(resolved, "..") {
			scope.add(fmt.Sprintf(
				"[repositories].clone[%d].path=%q: must not contain `..` segments "+
					"(would escape the parent workspace).",
				i, resolved,
			), idx, "path")
		}
		if resolved == "workspace-docker" {
			scope.add(fmt.Sprintf(
				"[repositories].clone[%d].path=%q: cannot overwrite workspace-docker itself.",
				i, resolved,
			), idx, "path")
		}
		if prev, ok := seenPaths[resolved]; ok {
			scope.add(fmt.Sprintf(
				"[repositories].clone[%d].path=%q: collides with entry [%d] (same target dir).",
				i, resolved, prev,
			), idx, "path")
		} else {
			seenPaths[resolved] = i
		}
		if prev, ok := seenURLs[entry.URL]; ok {
			scope.add(fmt.Sprintf(
				"[repositories].clone[%d].url=%q: duplicates entry [%d].",
				i, entry.URL, prev,
			), idx, "url")
		} else {
			seenURLs[entry.URL] = i
		}
		if entry.Depth != nil && *entry.Depth < 1 {
			scope.add("depth must be >= 1", idx, "depth")
		}
	}
}

func checkMapKeys(a *errAccumulator, m map[string]string, rx *regexp.Regexp, label string) {
	for k := range m {
		if !rx.MatchString(k) {
			a.add(fmt.Sprintf("%s key %q does not match pattern %q", label, k, rx.String()), k)
		}
	}
}

func hasDuplicates[T comparable](items []T) bool {
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
