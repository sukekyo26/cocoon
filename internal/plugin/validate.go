package plugin

import (
	"fmt"
	"regexp"
	"slices"

	"github.com/sukekyo26/cocoon/internal/config"
)

var (
	rxEnvKey       = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	rxBuildArg     = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
	rxPluginVolume = regexp.MustCompile(`^/home/\$\{USERNAME\}/[^/]+$`)
	rxPluginURL    = regexp.MustCompile(`^https://[^\s]+$`)
	rxMethodName   = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
)

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
		a.Add("verify has no effect unless version_capable = true", "verify")
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
