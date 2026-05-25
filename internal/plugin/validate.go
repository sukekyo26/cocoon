package plugin

import (
	"fmt"
	"regexp"
	"slices"

	"github.com/sukekyo26/cocoon/internal/config"
)

var (
	rxEnvKey          = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	rxBuildArg        = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
	rxPluginVolume    = regexp.MustCompile(`^/home/\$\{USERNAME\}/[^/]+$`)
	rxPluginURL       = regexp.MustCompile(`^https://[^\s]+$`)
	rxMethodName      = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	rxExtraVersionKey = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

// reservedExtraVersionKeys lists the [plugins.versions].<id> keys that
// the workspace loader consumes into dedicated PluginVersionOverride
// fields (Pin / ChecksumAmd64 / ChecksumArm64). A plugin declaring one
// of these names under [install.extra_versions] would be a silent no-op:
// the user can never override the value via [plugins.versions] because
// the loader routes the matching key into the reserved field, never
// into Extra. Reject the declaration up front so the plugin author sees
// the conflict instead of debugging a "default that never moves".
//
//nolint:gochecknoglobals // pin-down table for validation.
var reservedExtraVersionKeys = map[string]struct{}{
	"pin":            {},
	"checksum_amd64": {},
	"checksum_arm64": {},
}

// reservedExtraVersionEnvs lists env variable names cocoon supplies to
// every install script. A plugin's [install.extra_versions].<key>.env
// must not collide with these — otherwise the user-overridable value
// would silently shadow (or be shadowed by) the framework-provided
// value, with no fail-fast at decode time. Kept in sync with
// internal/generate/dockerfile/plugins.go's buildInstallEnvPairs.
//
//nolint:gochecknoglobals // pin-down table for validation.
var reservedExtraVersionEnvs = map[string]struct{}{
	"PIN":                   {},
	"CHECKSUM_AMD64":        {},
	"CHECKSUM_ARM64":        {},
	"RC_FILE":               {},
	"RC_SYNTAX":             {},
	"LOGIN_SHELL":           {},
	"COCOON_INSTALL_METHOD": {},
	"USERNAME":              {},
}

// Validate returns a *config.ValidationError on failure so the CLI's error
// renderer treats it identically to a workspace.toml failure.
func (p *Plugin) Validate(path string) error {
	a := config.NewAccumulator()
	p.runValidate(a)
	errs := a.Errors()
	if len(errs) == 0 {
		return nil
	}
	return &config.ValidationError{Path: path, Errors: errs}
}

func (p *Plugin) runValidate(a *config.Accumulator) {
	p.Metadata.validate(a.At("metadata"))
	p.Install.validate(a.At("install"))
	p.Version.validate(a.At("version"))
}

func (v *Version) validate(a *config.Accumulator) {
	switch v.Verify {
	case "", VerifyChecksum, VerifyPGP:
	default:
		a.Add(fmt.Sprintf("verify %q is not one of %q, %q", v.Verify, VerifyChecksum, VerifyPGP), "verify")
	}
	if v.Verify != "" && !v.VersionCapable {
		a.Add("verify requires version_capable = true", "verify")
	}
}

func (m *Metadata) validate(a *config.Accumulator) {
	if m.Name == "" {
		a.Add("name must not be empty", "name")
	}
	if m.Description == "" {
		a.Add("description must not be empty", "description")
	}
	if m.URL == "" {
		a.Add("url must not be empty", "url")
	} else if !rxPluginURL.MatchString(m.URL) {
		a.Add("url must start with https:// and contain no whitespace", "url")
	}
	if config.HasDuplicates(m.Conflicts) {
		a.Add("metadata.conflicts contains duplicate entries", "conflicts")
	}
}

func (i *Install) validate(a *config.Accumulator) {
	if i.Env != nil {
		config.CheckMapKeys(a.At("env"), i.Env, rxEnvKey, "install.env")
	}
	if config.HasDuplicates(i.BuildArgs) {
		a.Add("install.build_args contains duplicate entries", "build_args")
	}
	for idx, b := range i.BuildArgs {
		if !rxBuildArg.MatchString(b) {
			a.Add("build_arg does not match "+rxBuildArg.String(), "build_args", fmt.Sprintf("%d", idx))
		}
	}
	if config.HasDuplicates(i.Volumes) {
		a.Add("install.volumes contains duplicate entries", "volumes")
	}
	for idx, v := range i.Volumes {
		if !rxPluginVolume.MatchString(v) {
			a.Add("volume does not match "+rxPluginVolume.String(), "volumes", fmt.Sprintf("%d", idx))
		}
	}
	i.validateMethods(a)
	i.validateExtraVersions(a)
}

func (i *Install) validateExtraVersions(a *config.Accumulator) {
	if len(i.ExtraVersions) == 0 {
		return
	}
	buildArgs := make(map[string]struct{}, len(i.BuildArgs))
	for _, b := range i.BuildArgs {
		buildArgs[b] = struct{}{}
	}
	// Sort keys so the first-error summary is stable across runs.
	keys := make([]string, 0, len(i.ExtraVersions))
	for k := range i.ExtraVersions {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	seenEnv := make(map[string]string, len(keys))
	for _, k := range keys {
		validateOneExtraVersion(a, k, i.ExtraVersions[k], buildArgs, seenEnv)
	}
}

// validateOneExtraVersion runs the per-entry checks: reserved key,
// key shape, env shape, env-name collisions, then Default shape. The
// branches are layered, not fully short-circuiting:
//
//   - A reserved key (pin / checksum_*) returns immediately — no point
//     reporting downstream env/default issues for a name that can never
//     be used.
//   - The key-shape regex check is independent of the env-group checks:
//     a bad key shape records one error and validation continues into
//     the env checks, so a single entry can produce one key-shape error
//     plus one env-group error.
//   - Inside the env group (empty / shape / reserved env / build_args
//     collision / duplicate env) the checks short-circuit on first
//     failure so the user sees one actionable error per env field.
//   - Default checks run only when the env group passed; otherwise the
//     env failure is the actionable one to surface first.
func validateOneExtraVersion(
	a *config.Accumulator,
	k string,
	spec ExtraVersionSpec,
	buildArgs map[string]struct{},
	seenEnv map[string]string,
) {
	if _, reserved := reservedExtraVersionKeys[k]; reserved {
		a.Add(fmt.Sprintf("extra_versions key %q is reserved by [plugins.versions] "+
			"(consumed as the dedicated %s field, never as an extra) — pick a different key",
			k, k), "extra_versions", k)
		return
	}
	if !rxExtraVersionKey.MatchString(k) {
		a.Add("extra_versions key does not match "+rxExtraVersionKey.String(), "extra_versions", k)
	}
	if !checkExtraVersionEnv(a, k, spec.Env, buildArgs, seenEnv) {
		return
	}
	seenEnv[spec.Env] = k
	checkExtraVersionDefault(a, k, spec.Default)
}

// checkExtraVersionEnv returns true when env passed every shape /
// collision check, so the caller can record it in seenEnv. Returns
// false the moment any failure was recorded.
func checkExtraVersionEnv(
	a *config.Accumulator,
	k, env string,
	buildArgs map[string]struct{},
	seenEnv map[string]string,
) bool {
	switch {
	case env == "":
		a.Add("env must not be empty", "extra_versions", k, "env")
		return false
	case !rxBuildArg.MatchString(env):
		a.Add("env does not match "+rxBuildArg.String(), "extra_versions", k, "env")
		return false
	}
	if _, reserved := reservedExtraVersionEnvs[env]; reserved {
		a.Add(fmt.Sprintf("env %q collides with a cocoon-reserved variable", env),
			"extra_versions", k, "env")
		return false
	}
	if _, conflict := buildArgs[env]; conflict {
		a.Add(fmt.Sprintf("env %q collides with an install.build_args entry", env),
			"extra_versions", k, "env")
		return false
	}
	if prev, dup := seenEnv[env]; dup {
		a.Add(fmt.Sprintf("env %q is also used by extra_versions.%s", env, prev),
			"extra_versions", k, "env")
		return false
	}
	return true
}

// checkExtraVersionDefault rejects an empty default or one containing
// runes that would break the Dockerfile RUN-prefix env quoting.
func checkExtraVersionDefault(a *config.Accumulator, k, def string) {
	if def == "" {
		a.Add("default must not be empty (the install script reads the env as required; "+
			"an empty default would make the build unstable across invocations)",
			"extra_versions", k, "default")
		return
	}
	if bad, r := config.UnsafeExtraVersionRune(def); bad {
		a.Add(fmt.Sprintf("default contains unsafe character %q "+
			`(the value flows into the Dockerfile RUN prefix's KEY="..." env pair; `+
			"a bare \", \\, \\n, \\r, $, or backtick would break the shell quoting "+
			"or trigger parameter/command substitution)", r),
			"extra_versions", k, "default")
	}
}

func (i *Install) validateMethods(a *config.Accumulator) {
	if len(i.Methods) == 0 {
		if i.DefaultMethod != "" {
			a.Add("default_method requires at least one [install.methods.<name>] entry", "default_method")
		}
		return
	}
	if i.DefaultMethod == "" {
		a.Add("default_method must be set when [install.methods] is declared", "default_method")
	} else if _, ok := i.Methods[i.DefaultMethod]; !ok {
		a.Add(fmt.Sprintf("default_method %q is not declared in [install.methods]", i.DefaultMethod), "default_method")
	}
	// Sort method names so ValidationError.Error()'s "first error"
	// summary stays stable across runs (map iteration is randomised).
	names := make([]string, 0, len(i.Methods))
	for name := range i.Methods {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		m := i.Methods[name]
		if !rxMethodName.MatchString(name) {
			a.Add("method name does not match "+rxMethodName.String(), "methods", name)
		}
		if m.Description == "" {
			a.Add("description must not be empty", "methods", name, "description")
		}
	}
}
