package plugin

import (
	"fmt"
	"regexp"

	"github.com/sukekyo26/cocoon/internal/config"
)

var (
	rxEnvKey       = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	rxBuildArg     = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
	rxPluginVolume = regexp.MustCompile(`^/home/\$\{USERNAME\}/[^/]+$`)
	rxPluginURL    = regexp.MustCompile(`^https://[^\s]+$`)
)

// accumulator is a trimmed copy of internal/config's errAccumulator so
// the plugin package stays self-contained without exporting that type.
type accumulator struct {
	base []string
	errs *[]config.FieldError
}

func newAccumulator() *accumulator {
	errs := make([]config.FieldError, 0)
	return &accumulator{base: nil, errs: &errs}
}

func (a *accumulator) at(seg ...string) *accumulator {
	out := make([]string, 0, len(a.base)+len(seg))
	out = append(out, a.base...)
	out = append(out, seg...)
	return &accumulator{base: out, errs: a.errs}
}

func (a *accumulator) add(msg string, seg ...string) {
	loc := make([]string, 0, len(a.base)+len(seg))
	loc = append(loc, a.base...)
	loc = append(loc, seg...)
	*a.errs = append(*a.errs, config.FieldError{Loc: loc, Message: msg})
}

// Validate returns a *config.ValidationError on failure so the CLI's error
// renderer treats it identically to a workspace.toml failure.
func (p *Plugin) Validate(path string) error {
	a := newAccumulator()
	p.runValidate(a)
	if len(*a.errs) == 0 {
		return nil
	}
	return &config.ValidationError{Path: path, Errors: *a.errs}
}

func (p *Plugin) runValidate(a *accumulator) {
	p.Metadata.validate(a.at("metadata"))
	p.Install.validate(a.at("install"))
}

func (m *Metadata) validate(a *accumulator) {
	if m.Name == "" {
		a.add("name must not be empty", "name")
	}
	if m.Description == "" {
		a.add("description must not be empty", "description")
	}
	if m.URL == "" {
		a.add("url must not be empty", "url")
	} else if !rxPluginURL.MatchString(m.URL) {
		a.add("url must start with https:// and contain no whitespace", "url")
	}
	if hasDuplicates(m.Conflicts) {
		a.add("metadata.conflicts contains duplicate entries", "conflicts")
	}
}

func (i *Install) validate(a *accumulator) {
	if i.Env != nil {
		checkMapKeys(a.at("env"), i.Env, rxEnvKey, "install.env")
	}
	if hasDuplicates(i.BuildArgs) {
		a.add("install.build_args contains duplicate entries", "build_args")
	}
	for idx, b := range i.BuildArgs {
		if !rxBuildArg.MatchString(b) {
			a.add("build_arg does not match "+rxBuildArg.String(), "build_args", fmt.Sprintf("%d", idx))
		}
	}
	if hasDuplicates(i.Volumes) {
		a.add("install.volumes contains duplicate entries", "volumes")
	}
	for idx, v := range i.Volumes {
		if !rxPluginVolume.MatchString(v) {
			a.add("volume does not match "+rxPluginVolume.String(), "volumes", fmt.Sprintf("%d", idx))
		}
	}
}

func checkMapKeys(a *accumulator, m map[string]string, rx *regexp.Regexp, label string) {
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
