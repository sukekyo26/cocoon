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
// Methods and DefaultMethod are required for any plugin loaded via the
// public Load / LoadEnabled[FromFS] entry points: the loader's
// validateMethodScripts rejects an empty Methods map and a literal
// install.sh file. Each declared method must have a matching
// install.<name>.sh on disk. The in-memory Validate() method, by
// contrast, tolerates an empty Methods so test code can build *Plugin
// literals without filling in Methods just to exercise an unrelated
// field. The active method is selected at install time via
// workspace.toml's [plugins.methods] map; absent overrides fall back
// to DefaultMethod.
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

// Verification methods for [version].verify.
const (
	// VerifyChecksum verifies downloads against $CHECKSUM_AMD64 /
	// $CHECKSUM_ARM64 recorded per workspace in [plugins.versions].
	VerifyChecksum = "checksum"
	// VerifyPGP verifies downloads in-script against a bundled signing
	// key; it takes no per-workspace checksum.
	VerifyPGP = "pgp"
)

// Version mirrors plugin.toml [version].
type Version struct {
	VersionCapable bool   `toml:"version_capable"`
	Verify         string `toml:"verify,omitempty"`
}

// VerifiesByChecksum reports whether the plugin's install script verifies
// downloads against $CHECKSUM_AMD64 / $CHECKSUM_ARM64. An empty Verify
// defaults to the checksum mechanism; VerifyPGP plugins verify in-script
// against a bundled signing key and take no per-workspace checksum.
func (v Version) VerifiesByChecksum() bool {
	return v.Verify == "" || v.Verify == VerifyChecksum
}
