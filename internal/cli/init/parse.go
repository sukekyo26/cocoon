package initcli

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/sukekyo26/cocoon/internal/aliasbundles"
	"github.com/sukekyo26/cocoon/internal/aptcategories"
	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

var (
	rxServiceName = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	rxUsername    = regexp.MustCompile(`^[a-z_][a-z0-9_-]*$`)
	// rxImageVersionInput must stay in lockstep with config.rxImageVersion
	// so prompts reject the same set `cocoon gen` does (Docker tag spec:
	// alnum / underscore can lead, period / hyphen cannot).
	rxImageVersionInput = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]*$`)
)

// versionStringValidator enforces the TOML-safe charset via rxImageVersionInput.
// Upstream existence is intentionally NOT verified. latestSentinel, when
// non-empty, is accepted unconditionally (the plugin picker uses "LATEST" for
// "leave unpinned"). Empty input is always rejected so a stray Enter does not
// silently encode as a sentinel.
func versionStringValidator(cat *i18n.Catalog, formatErrKey, latestSentinel string) func(string) error {
	return func(s string) error {
		s = strings.TrimSpace(s)
		if latestSentinel != "" && s == latestSentinel {
			return nil
		}
		if s == "" {
			return errors.New(cat.Msg("init_err_required")) //nolint:err113 // user-facing prompt
		}
		if !rxImageVersionInput.MatchString(s) {
			return errors.New(cat.Msg(formatErrKey)) //nolint:err113 // user-facing prompt
		}
		return nil
	}
}

// makeStrictValidator surfaces a localized sentence instead of the raw
// regex. Strict-on-empty is safe only because each Input is a standalone
// huh.Form with no previous group, so the validator cannot trap the user
// in a blurred-with-error field.
func makeStrictValidator(pattern *regexp.Regexp, charsKey string, cat *i18n.Catalog) func(string) error {
	return func(s string) error {
		if s == "" {
			return errors.New(cat.Msg("init_err_required")) //nolint:err113 // user-facing prompt
		}
		if !pattern.MatchString(s) {
			return errors.New(cat.Msg(charsKey)) //nolint:err113 // user-facing prompt
		}
		return nil
	}
}

// parseAptCategories rejects unknown ids. Used by --apt-categories.
func parseAptCategories(raw string) ([]string, error) {
	var ids []string
	for _, part := range strings.Split(raw, ",") {
		id := strings.TrimSpace(part)
		if id == "" {
			continue
		}
		if aptcategories.AptCategoryByID(id) == nil {
			return nil, fmt.Errorf("%w: unknown apt category %q (run `cocoon init --help` for the list)",
				clihelpers.ErrUsage, id)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// parsePorts validates entries through config.ValidateShortForm so init
// accepts the same set `cocoon gen` does. (nil, nil) for blank input;
// the renderer treats nil as "user opted out" and emits the commented
// [ports] template.
func parsePorts(raw string) ([]string, error) {
	var ports []string
	for _, part := range strings.Split(raw, ",") {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		if err := config.ValidateShortForm(p); err != nil {
			return nil, fmt.Errorf("%w: --ports %w", clihelpers.ErrUsage, err)
		}
		ports = append(ports, p)
	}
	return ports, nil
}

// parseAliasBundles rejects unknown ids. Used by --alias-bundles.
func parseAliasBundles(raw string) ([]string, error) {
	var ids []string
	for _, part := range strings.Split(raw, ",") {
		id := strings.TrimSpace(part)
		if id == "" {
			continue
		}
		if aliasbundles.AliasBundleByID(id) == nil {
			return nil, fmt.Errorf("%w: unknown alias bundle %q", clihelpers.ErrUsage, id)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// parsePlugins leaves the conflict check to validatePluginConflicts so the
// same logic covers both flag and prompt paths.
func parsePlugins(raw string, plugins map[string]*plugin.Plugin) ([]string, error) {
	var ids []string
	for _, part := range strings.Split(raw, ",") {
		id := strings.TrimSpace(part)
		if id == "" {
			continue
		}
		if _, ok := plugins[id]; !ok {
			return nil, fmt.Errorf("%w: unknown plugin %q (run `cocoon plugin list` for the catalog)",
				clihelpers.ErrUsage, id)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// parsePluginVersions parses `--plugin-versions=<id>=<ref>,…` into a map.
// Each id must be a known plugin, listed in enabled (--plugins), and
// version_capable; any violation returns ErrUsage. Empty input returns a
// non-nil empty map (nilnil lint). Duplicate ids are rejected so a typo
// can't silently pick the last value.
func parsePluginVersions(raw string, plugins map[string]*plugin.Plugin, enabled []string) (map[string]string, error) {
	enabledSet := make(map[string]struct{}, len(enabled))
	for _, id := range enabled {
		enabledSet[id] = struct{}{}
	}
	out := map[string]string{}
	for _, part := range strings.Split(raw, ",") {
		token := strings.TrimSpace(part)
		if token == "" {
			continue
		}
		// Reject typos like "go==1.23" instead of silently using "=1.23"
		// as the pin ref. Real pin refs never contain '='.
		if strings.Count(token, "=") != 1 {
			return nil, fmt.Errorf(
				"%w: --plugin-versions token %q must be <id>=<ref>", clihelpers.ErrUsage, token)
		}
		eq := strings.IndexByte(token, '=')
		id := strings.TrimSpace(token[:eq])
		ref := strings.TrimSpace(token[eq+1:])
		if id == "" || ref == "" {
			return nil, fmt.Errorf(
				"%w: --plugin-versions token %q must be <id>=<ref>", clihelpers.ErrUsage, token)
		}
		p, ok := plugins[id]
		if !ok {
			return nil, fmt.Errorf(
				"%w: --plugin-versions: unknown plugin %q (run `cocoon plugin list`)",
				clihelpers.ErrUsage, id)
		}
		if !p.Version.VersionCapable {
			return nil, fmt.Errorf(
				"%w: --plugin-versions: plugin %q is not version_capable",
				clihelpers.ErrUsage, id)
		}
		if _, on := enabledSet[id]; !on {
			return nil, fmt.Errorf(
				"%w: --plugin-versions: plugin %q must also appear in --plugins",
				clihelpers.ErrUsage, id)
		}
		if _, dup := out[id]; dup {
			return nil, fmt.Errorf(
				"%w: --plugin-versions: duplicate id %q", clihelpers.ErrUsage, id)
		}
		out[id] = ref
	}
	// Returning an empty (non-nil) map for an all-whitespace input is fine —
	// the writer falls back to the commented template when len == 0, and
	// keeping a non-nil sentinel keeps `nilnil` happy without forcing a
	// custom error for "I parsed your input but it was empty."
	return out, nil
}

// parsePluginMethods parses `--plugin-methods=<id>=<method>,…` into a map.
// Each id must be a known plugin, listed in enabled (--plugins), and declare
// [install.methods] in plugin.toml. The method must be one of the declared
// keys. Empty input returns a non-nil empty map (nilnil lint); duplicate ids
// are rejected so a typo cannot silently pick the last value.
func parsePluginMethods(raw string, plugins map[string]*plugin.Plugin, enabled []string) (map[string]string, error) {
	enabledSet := make(map[string]struct{}, len(enabled))
	for _, id := range enabled {
		enabledSet[id] = struct{}{}
	}
	out := map[string]string{}
	for _, part := range strings.Split(raw, ",") {
		token := strings.TrimSpace(part)
		if token == "" {
			continue
		}
		if strings.Count(token, "=") != 1 {
			return nil, fmt.Errorf(
				"%w: --plugin-methods token %q must be <id>=<method>", clihelpers.ErrUsage, token)
		}
		eq := strings.IndexByte(token, '=')
		id := strings.TrimSpace(token[:eq])
		method := strings.TrimSpace(token[eq+1:])
		if id == "" || method == "" {
			return nil, fmt.Errorf(
				"%w: --plugin-methods token %q must be <id>=<method>", clihelpers.ErrUsage, token)
		}
		p, ok := plugins[id]
		if !ok {
			return nil, fmt.Errorf(
				"%w: --plugin-methods: unknown plugin %q (run `cocoon plugin list`)",
				clihelpers.ErrUsage, id)
		}
		if _, on := enabledSet[id]; !on {
			return nil, fmt.Errorf(
				"%w: --plugin-methods: plugin %q must also appear in --plugins",
				clihelpers.ErrUsage, id)
		}
		if len(p.Install.Methods) == 0 {
			return nil, fmt.Errorf(
				"%w: --plugin-methods: plugin %q has no [install.methods] — drop it from --plugin-methods",
				clihelpers.ErrUsage, id)
		}
		if _, declared := p.Install.Methods[method]; !declared {
			return nil, fmt.Errorf(
				"%w: --plugin-methods: plugin %q has no method %q in [install.methods]",
				clihelpers.ErrUsage, id, method)
		}
		if _, dup := out[id]; dup {
			return nil, fmt.Errorf(
				"%w: --plugin-methods: duplicate id %q", clihelpers.ErrUsage, id)
		}
		out[id] = method
	}
	return out, nil
}
