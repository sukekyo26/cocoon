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
		spec := i.ExtraVersions[k]
		if !rxExtraVersionKey.MatchString(k) {
			a.Add("extra_versions key does not match "+rxExtraVersionKey.String(), "extra_versions", k)
		}
		if spec.Env == "" {
			a.Add("env must not be empty", "extra_versions", k, "env")
			continue
		}
		if !rxBuildArg.MatchString(spec.Env) {
			a.Add("env does not match "+rxBuildArg.String(), "extra_versions", k, "env")
			continue
		}
		if _, reserved := reservedExtraVersionEnvs[spec.Env]; reserved {
			a.Add(fmt.Sprintf("env %q collides with a cocoon-reserved variable", spec.Env),
				"extra_versions", k, "env")
			continue
		}
		if _, conflict := buildArgs[spec.Env]; conflict {
			a.Add(fmt.Sprintf("env %q collides with an install.build_args entry", spec.Env),
				"extra_versions", k, "env")
			continue
		}
		if prev, dup := seenEnv[spec.Env]; dup {
			a.Add(fmt.Sprintf("env %q is also used by extra_versions.%s", spec.Env, prev),
				"extra_versions", k, "env")
			continue
		}
		seenEnv[spec.Env] = k
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
