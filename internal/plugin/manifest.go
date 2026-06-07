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
// public Load / LoadEnabledFromFS entry points: the loader's
// validateMethodScripts rejects an empty Methods map and a literal
// install.sh file. Each declared method must have a matching
// install.<name>.sh on disk. The in-memory Validate() method, by
// contrast, tolerates an empty Methods so test code can build *Plugin
// literals without filling in Methods just to exercise an unrelated
// field. The active method is selected at install time via
// cocoon.toml's [plugins.methods] map; absent overrides fall back
// to DefaultMethod.
type Install struct {
	RequiresRoot  bool                        `toml:"requires_root"`
	BuildArgs     []string                    `toml:"build_args,omitempty"`
	Env           map[string]string           `toml:"env,omitempty"`
	Volumes       []string                    `toml:"volumes,omitempty"`
	DefaultMethod string                      `toml:"default_method,omitempty"`
	Methods       map[string]InstallMethod    `toml:"methods,omitempty"`
	ExtraVersions map[string]ExtraVersionSpec `toml:"extra_versions,omitempty"`
}

// InstallMethod mirrors plugin.toml [install.methods.<name>]. The
// script file lives at <plugin-dir>/install.<name>.sh.
type InstallMethod struct {
	Description string `toml:"description"`
}

// ExtraVersionSpec mirrors plugin.toml [install.extra_versions.<key>].
// It declares one user-overridable subcomponent version: cocoon.toml
// can write `<key> = "..."` inside [plugins.options].<id> and the value
// is exported into the install script as the env variable named in Env.
// Default is used when the cocoon.toml override is absent. Both Env
// and Default are required (an empty Env is rejected during validation
// and an empty Default would make the install script unstable across
// invocations).
type ExtraVersionSpec struct {
	Env     string `toml:"env"`
	Default string `toml:"default"`
}

// Verification methods for [version].verify.
const (
	// VerifyChecksum verifies downloads against $CHECKSUM_AMD64 /
	// $CHECKSUM_ARM64 recorded in cocoon.lock.
	VerifyChecksum = "checksum"
	// VerifyPGP verifies downloads in-script against a bundled signing
	// key; it takes no per-workspace checksum.
	VerifyPGP = "pgp"
)

// Version mirrors plugin.toml [version].
type Version struct {
	VersionCapable bool           `toml:"version_capable"`
	Verify         string         `toml:"verify,omitempty"`
	Source         *VersionSource `toml:"source,omitempty"`
}

// VerifiesByChecksum reports whether the plugin's install script verifies
// downloads against $CHECKSUM_AMD64 / $CHECKSUM_ARM64 (recorded in
// cocoon.lock by `cocoon lock`). An empty Verify defaults to the checksum
// mechanism; VerifyPGP plugins verify in-script against a bundled signing
// key and take no checksum.
func (v Version) VerifiesByChecksum() bool {
	return v.Verify == "" || v.Verify == VerifyChecksum
}

// VersionSource declares how `cocoon lock` discovers the latest version of a
// version_capable plugin and where it fetches the per-arch SHA256 checksum.
// The two axes are independent: Latest answers "what is the newest version",
// Checksum answers "what is the SHA256 of the <version>/<arch> artifact". A
// plugin with no Source cannot resolve "latest" (the user must pin an exact
// version) and records no checksum. URLs may use ${version} and ${arch}
// placeholders; ${arch} is substituted via the Arch map (e.g. node needs
// x64/arm64, just needs x86_64/aarch64). ${version} is always the clean
// version with no leading "v"; templates spell out a literal "v${version}"
// where the upstream tag carries one.
type VersionSource struct {
	Latest   LatestSpec        `toml:"latest"`
	Checksum ChecksumSpec      `toml:"checksum"`
	Arch     map[string]string `toml:"arch,omitempty"`
}

// Latest-discovery kinds for [version.source.latest].type.
const (
	// LatestGitHubRelease reads tag_name from the GitHub releases/latest API
	// for Repo (e.g. "casey/just").
	LatestGitHubRelease = "github-release"
	// LatestText GETs URL and takes the first line (e.g. go.dev/VERSION).
	LatestText = "text"
	// LatestJSONField GETs URL, decodes JSON, and reads the dotted Field
	// (e.g. HashiCorp's checkpoint API .current_version).
	LatestJSONField = "json-field"
	// LatestTab GETs a TSV index (Node's dist/index.tab); with LTSOnly it
	// takes the first row whose LTS column is non-"-".
	LatestTab = "tab"
)

// LatestSpec mirrors [version.source.latest].
type LatestSpec struct {
	Type        string `toml:"type"`
	URL         string `toml:"url,omitempty"`
	Repo        string `toml:"repo,omitempty"`
	StripPrefix string `toml:"strip_prefix,omitempty"`
	Field       string `toml:"field,omitempty"`
	LTSOnly     bool   `toml:"lts_only,omitempty"`
}

// Checksum-fetch kinds for [version.source.checksum].type.
const (
	// ChecksumNone records no checksum (pgp plugins, or | bash installers
	// whose per-arch artifact hash cocoon cannot fetch independently).
	ChecksumNone = "none"
	// ChecksumSidecar GETs AssetURL+Suffix; the body is a bare hash
	// (e.g. go's .sha256 sidecar).
	ChecksumSidecar = "sidecar"
	// ChecksumShasumsFile GETs ManifestURL (one SHASUMS-style file) and finds
	// the line whose filename field matches AssetName.
	ChecksumShasumsFile = "shasums-file"
)

// ChecksumSpec mirrors [version.source.checksum].
type ChecksumSpec struct {
	Type        string `toml:"type"`
	AssetURL    string `toml:"asset_url,omitempty"`
	Suffix      string `toml:"suffix,omitempty"`
	ManifestURL string `toml:"manifest_url,omitempty"`
	AssetName   string `toml:"asset_name,omitempty"`
}
