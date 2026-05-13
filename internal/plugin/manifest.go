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
	Default     bool     `toml:"default"`
	Conflicts   []string `toml:"conflicts,omitempty"`
}

// Apt mirrors plugin.toml [apt].
type Apt struct {
	Packages []string `toml:"packages"`
}

// Install mirrors plugin.toml [install].
type Install struct {
	RequiresRoot bool              `toml:"requires_root"`
	BuildArgs    []string          `toml:"build_args,omitempty"`
	Env          map[string]string `toml:"env,omitempty"`
	Volumes      []string          `toml:"volumes,omitempty"`
}

// Version mirrors plugin.toml [version].
type Version struct {
	VersionCapable bool `toml:"version_capable"`
}
