// Package plugin owns the plugin.toml schema, its loader, conflict
// detection, and the volume helpers consumed by the dockerfile/compose
// generators.
package plugin

// Plugin mirrors the plugin.toml manifest under plugins/<id>/plugin.toml.
type Plugin struct {
	Metadata Metadata `toml:"metadata"`
	Apt      *Apt     `toml:"apt,omitempty"`
	Install  Install  `toml:"install"`
	Version  Version  `toml:"version"`
}

// Metadata mirrors plugin.toml [metadata].
type Metadata struct {
	Name        string   `toml:"name"`
	Description string   `toml:"description"`
	URL         string   `toml:"url"`
	Default     bool     `toml:"default"`
	Conflicts   []string `toml:"conflicts,omitempty"`
}

// Apt mirrors plugin.toml [apt].
type Apt struct {
	Packages []string `toml:"packages"`
}

// Install mirrors plugin.toml [install].
//
// Methods and DefaultMethod are optional. When Methods is empty the
// plugin uses a single install.sh file (legacy form). When Methods is
// non-empty the plugin must provide install.<name>.sh for every
// declared method and must not have install.sh (exclusive). The
// active method is selected at install time via workspace.toml's
// [plugins.methods] map; absent overrides fall back to DefaultMethod.
type Install struct {
	RequiresRoot  bool                     `toml:"requires_root"`
	BuildArgs     []string                 `toml:"build_args,omitempty"`
	Env           map[string]string        `toml:"env,omitempty"`
	Volumes       []string                 `toml:"volumes,omitempty"`
	DefaultMethod string                   `toml:"default_method,omitempty"`
	Methods       map[string]InstallMethod `toml:"methods,omitempty"`
}

// InstallMethod mirrors plugin.toml [install.methods.<name>]. The
// script file lives at <plugin-dir>/install.<name>.sh.
type InstallMethod struct {
	Description string `toml:"description"`
}

// Version mirrors plugin.toml [version].
type Version struct {
	VersionCapable bool `toml:"version_capable"`
}
