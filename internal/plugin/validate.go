package plugin

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/sukekyo26/cocoon/internal/config"
)

var (
	rxEnvKey              = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	rxBuildArg            = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
	rxPluginVolume        = regexp.MustCompile(`^/home/\$\{USERNAME\}/[^/]+$`)
	rxPluginURL           = regexp.MustCompile(`^https://[^\s]+$`)
	rxMethodName          = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	rxExtraVersionKey     = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	rxTemplatePlaceholder = regexp.MustCompile(`\$\{[^}]*\}`)
)

// reservedExtraVersionKeys lists the [plugins.options].<id> keys that the
// workspace loader treats specially (pin is removed; checksum_amd64 /
// checksum_arm64 feed the dedicated PluginVersionOverride checksum fields). A
// plugin declaring one of these names under [install.extra_versions] would be
// a silent no-op: the user could never override the value via
// [plugins.options] because the loader never routes a reserved key into Extra.
// Reject the declaration up front so the plugin author sees the conflict
// instead of debugging a "default that never moves".
//
//nolint:gochecknoglobals // pin-down table for validation.
var reservedExtraVersionKeys = map[string]struct{}{
	"pin":            {},
	"checksum_amd64": {},
	"checksum_arm64": {},
}

// reservedExtraVersionEnvs lists env variable names cocoon supplies to
// every install script. A plugin's [install.extra_versions].<key>.env
// — and any name in [install].build_args — must not collide with
// these: otherwise the user-overridable value would silently shadow
// (or be shadowed by) the framework-provided value, with no fail-fast
// at decode time.
//
// The set has two sources cocoon must keep in lockstep:
//
//   - Per-RUN env injected by buildInstallEnvPairs in
//     internal/generate/dockerfile/plugins.go: PIN, CHECKSUM_AMD64,
//     CHECKSUM_ARM64, RC_FILE, RC_SYNTAX, LOGIN_SHELL,
//     COCOON_INSTALL_METHOD.
//   - Image-wide ARG/ENV emitted by the Dockerfile generator outside
//     buildInstallEnvPairs: USERNAME. It is still off-limits because
//     a colliding extra_versions.env / build_args entry would create
//     `USERNAME="${USERNAME}"` in the RUN prefix and reduce to a
//     redundant no-op (or, worse, mislead readers about its source).
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
// renderer treats it identically to a config-file failure.
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
		a.AddCode("err_pval_verify_oneof", []any{v.Verify, VerifyChecksum, VerifyPGP}, "verify")
	}
	if v.Verify != "" && !v.VersionCapable {
		a.AddCode("err_pval_verify_needs_capable", nil, "verify")
	}
	if v.Source != nil {
		if !v.VersionCapable {
			a.AddCode("err_pval_source_needs_capable", nil, "source")
		}
		v.Source.validate(a.At("source"), v.Verify)
	}
}

func (s *VersionSource) validate(a *config.Accumulator, verify string) {
	s.Latest.validate(a.At("latest"))
	s.Checksum.validate(a.At("checksum"), verify)
	if s.usesArch() && len(s.Arch) == 0 {
		a.AddCode("err_pval_arch_required", nil, "arch")
	}
}

func (s *VersionSource) usesArch() bool {
	return strings.Contains(s.Latest.URL, "${arch}") ||
		strings.Contains(s.Checksum.AssetURL, "${arch}") ||
		strings.Contains(s.Checksum.ManifestURL, "${arch}") ||
		strings.Contains(s.Checksum.AssetName, "${arch}")
}

func (l *LatestSpec) validate(a *config.Accumulator) {
	switch l.Type {
	case LatestGitHubRelease:
		if l.Repo == "" {
			a.AddCode("err_pval_latest_repo_required", nil, "repo")
		}
	case LatestText, LatestTab:
		validateSourceURL(a, l.URL, "url")
	case LatestJSONField:
		validateSourceURL(a, l.URL, "url")
		if l.Field == "" {
			a.AddCode("err_pval_latest_field_required", nil, "field")
		}
	case "":
		a.AddCode("err_pval_latest_type_required", nil, "type")
	default:
		a.AddCode("err_pval_latest_type_oneof",
			[]any{l.Type, LatestGitHubRelease, LatestText, LatestJSONField, LatestTab}, "type")
	}
}

func (c *ChecksumSpec) validate(a *config.Accumulator, verify string) {
	switch c.Type {
	case ChecksumNone:
	case ChecksumSidecar:
		validateSourceURL(a, c.AssetURL, "asset_url")
	case ChecksumShasumsFile:
		validateSourceURL(a, c.ManifestURL, "manifest_url")
		if c.AssetName == "" {
			a.AddCode("err_pval_checksum_asset_required", nil, "asset_name")
		} else {
			// asset_name is template-expanded by the resolver (matched against
			// the SHASUMS manifest), so a typo'd placeholder must fail here too.
			validateTemplatePlaceholders(a, c.AssetName, "asset_name")
		}
	case "":
		a.AddCode("err_pval_checksum_type_required", nil, "type")
	default:
		a.AddCode("err_pval_checksum_type_oneof",
			[]any{c.Type, ChecksumNone, ChecksumSidecar, ChecksumShasumsFile}, "type")
	}
	if c.Type != "" && c.Type != ChecksumNone && verify == VerifyPGP {
		a.AddCode("err_pval_checksum_none_for_pgp", nil, "type")
	}
}

// validateTemplatePlaceholders rejects any ${...} the resolver does not expand
// (only ${version} / ${arch}) in a template-expanded field, so a typo fails at
// plugin.toml load instead of with a confusing lock-time fetch / not-found.
func validateTemplatePlaceholders(a *config.Accumulator, raw, field string) {
	for _, ph := range rxTemplatePlaceholder.FindAllString(raw, -1) {
		if ph != "${version}" && ph != "${arch}" {
			a.AddCode("err_pval_unknown_placeholder", []any{field, ph}, field)
		}
	}
}

// validateSourceURL checks a source URL: every ${...} placeholder must be one
// the resolver expands (${version} / ${arch}), and after stripping them the
// https-only / no-whitespace contract must hold for the literal parts a
// third-party plugin.toml supplies. Untrusted-input gate per defensive-coding §5.
func validateSourceURL(a *config.Accumulator, raw, field string) {
	if raw == "" {
		a.AddCode("err_pval_field_required", []any{field}, field)
		return
	}
	validateTemplatePlaceholders(a, raw, field)
	stripped := rxTemplatePlaceholder.ReplaceAllString(raw, "x")
	if !rxPluginURL.MatchString(stripped) {
		a.AddCode("err_pval_field_https", []any{field}, field)
	}
}

func (m *Metadata) validate(a *config.Accumulator) {
	if m.Name == "" {
		a.AddCode("err_pval_field_required", []any{"name"}, "name")
	}
	if m.Description == "" {
		a.AddCode("err_pval_field_required", []any{"description"}, "description")
	}
	if m.URL == "" {
		a.AddCode("err_pval_field_required", []any{"url"}, "url")
	} else if !rxPluginURL.MatchString(m.URL) {
		a.AddCode("err_pval_field_https", []any{"url"}, "url")
	}
	if config.HasDuplicates(m.Conflicts) {
		a.AddCode("err_pval_duplicate_entries", []any{"metadata.conflicts"}, "conflicts")
	}
}

func (i *Install) validate(a *config.Accumulator) {
	if i.Env != nil {
		config.CheckMapKeys(a.At("env"), i.Env, rxEnvKey, "install.env")
		// Values are interpolated unquoted into a Dockerfile `ENV K="v"` line
		// (dockerfile/plugins.go). $ stays legal so earlier ENV/ARG vars
		// expand, but a newline or double-quote would break out of the line
		// and inject a new directive.
		config.CheckMapValues(a.At("env"), i.Env, "\n\r\"",
			"a newline or double-quote would let the value escape the generated Dockerfile ENV line")
	}
	if config.HasDuplicates(i.BuildArgs) {
		a.AddCode("err_pval_duplicate_entries", []any{"install.build_args"}, "build_args")
	}
	for idx, b := range i.BuildArgs {
		if !rxBuildArg.MatchString(b) {
			a.AddCode("err_pval_build_arg_pattern", []any{rxBuildArg.String()}, "build_args", fmt.Sprintf("%d", idx))
			continue
		}
		if _, reserved := reservedExtraVersionEnvs[b]; reserved {
			// install.build_args entries are appended to the RUN env prefix
			// as `<NAME>="${<NAME>}"` after the framework-provided env
			// pairs. A name colliding with a reserved variable would
			// silently shadow the framework value (or expand to empty when
			// no --build-arg is supplied), so reject the collision at
			// decode time. Mirrors the [install.extra_versions].<k>.env
			// collision check.
			a.AddCode("err_pval_build_arg_reserved", []any{b},
				"build_args", fmt.Sprintf("%d", idx))
		}
	}
	if config.HasDuplicates(i.Volumes) {
		a.AddCode("err_pval_duplicate_entries", []any{"install.volumes"}, "volumes")
	}
	for idx, v := range i.Volumes {
		if !rxPluginVolume.MatchString(v) {
			a.AddCode("err_pval_volume_pattern", []any{rxPluginVolume.String()}, "volumes", fmt.Sprintf("%d", idx))
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
	keys := slices.Sorted(maps.Keys(i.ExtraVersions))
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
		a.AddCode("err_pval_extra_key_reserved", []any{k}, "extra_versions", k)
		return
	}
	if !rxExtraVersionKey.MatchString(k) {
		a.AddCode("err_pval_extra_key_pattern", []any{rxExtraVersionKey.String()}, "extra_versions", k)
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
		a.AddCode("err_pval_field_required", []any{"env"}, "extra_versions", k, "env")
		return false
	case !rxBuildArg.MatchString(env):
		a.AddCode("err_pval_env_pattern", []any{rxBuildArg.String()}, "extra_versions", k, "env")
		return false
	}
	if _, reserved := reservedExtraVersionEnvs[env]; reserved {
		a.AddCode("err_pval_env_reserved", []any{env}, "extra_versions", k, "env")
		return false
	}
	if _, conflict := buildArgs[env]; conflict {
		a.AddCode("err_pval_env_collides_build_arg", []any{env}, "extra_versions", k, "env")
		return false
	}
	if prev, dup := seenEnv[env]; dup {
		a.AddCode("err_pval_env_dup", []any{env, prev}, "extra_versions", k, "env")
		return false
	}
	return true
}

// checkExtraVersionDefault rejects an empty default or one containing
// runes that would break the Dockerfile RUN-prefix env quoting.
func checkExtraVersionDefault(a *config.Accumulator, k, def string) {
	if def == "" {
		a.AddCode("err_pval_default_required", nil, "extra_versions", k, "default")
		return
	}
	if bad, r := config.UnsafeExtraVersionRune(def); bad {
		a.AddCode("err_field_extra_version_unsafe", []any{"default", r}, "extra_versions", k, "default")
	}
}

func (i *Install) validateMethods(a *config.Accumulator) {
	if len(i.Methods) == 0 {
		if i.DefaultMethod != "" {
			a.AddCode("err_pval_default_method_needs_methods", nil, "default_method")
		}
		return
	}
	if i.DefaultMethod == "" {
		a.AddCode("err_pval_default_method_required", nil, "default_method")
	} else if _, ok := i.Methods[i.DefaultMethod]; !ok {
		a.AddCode("err_pval_default_method_undeclared", []any{i.DefaultMethod}, "default_method")
	}
	// Sort method names so ValidationError.Error()'s "first error"
	// summary stays stable across runs (map iteration is randomised).
	names := slices.Sorted(maps.Keys(i.Methods))
	for _, name := range names {
		m := i.Methods[name]
		if !rxMethodName.MatchString(name) {
			a.AddCode("err_pval_method_name_pattern", []any{rxMethodName.String()}, "methods", name)
		}
		if m.Description == "" {
			a.AddCode("err_pval_field_required", []any{"description"}, "methods", name, "description")
		}
	}
}
