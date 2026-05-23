package initcli

// pathFixEnvEntry is one `K = "V"` pair the auto-path-fix adds to
// [container.shell.env]. Slice order is preserved as the workspace.toml
// emit order; shellrc.go sorts alphabetically at render time so runtime
// order is independent of this list.
type pathFixEnvEntry struct {
	Key   string
	Value string
}

// imagePathFix describes the auto-injection cocoon offers for one language
// base image. Command is the user-facing example command (e.g. `npm install
// -g <pkg>`) that the entries make work without sudo; it is spliced into
// the prompt description and the workspace.toml auto-comment so both
// surfaces explain the *why* in concrete terms.
type imagePathFix struct {
	Entries []pathFixEnvEntry
	Command string
}

// imagePathFixes lists every base image whose official Docker variant
// writes user installs to a root-owned directory (or to a directory that
// is not on PATH). Keys match config.SupportedImages exactly so the prompt
// gate can rely on map lookup. Images absent from this map (ubuntu,
// debian) do not surface the prompt or the auto-comment.
//
// For rust, CARGO_INSTALL_ROOT is preferred over overriding CARGO_HOME so
// rustup and `cargo build`'s registry cache stay on the image-default
// /usr/local/cargo while only `cargo install` writes under $HOME.
//
//nolint:gochecknoglobals // tabular configuration data, file-scoped by design.
var imagePathFixes = map[string]imagePathFix{
	"node": {
		Entries: []pathFixEnvEntry{
			{Key: "NPM_CONFIG_PREFIX", Value: "$HOME/.npm-global"},
			{Key: "PATH", Value: "$HOME/.npm-global/bin:$PATH"},
		},
		Command: "npm install -g <pkg>",
	},
	"python": {
		Entries: []pathFixEnvEntry{
			{Key: "PATH", Value: "$HOME/.local/bin:$PATH"},
		},
		Command: "pip install --user <pkg>",
	},
	"golang": {
		Entries: []pathFixEnvEntry{
			{Key: "PATH", Value: "$HOME/go/bin:$PATH"},
		},
		Command: "go install <pkg>@latest",
	},
	"rust": {
		Entries: []pathFixEnvEntry{
			{Key: "CARGO_INSTALL_ROOT", Value: "$HOME/.cargo"},
			{Key: "PATH", Value: "$HOME/.cargo/bin:$PATH"},
		},
		Command: "cargo install <pkg>",
	},
	"denoland/deno": {
		Entries: []pathFixEnvEntry{
			{Key: "PATH", Value: "$HOME/.deno/bin:$PATH"},
		},
		Command: "deno install <script>",
	},
}

// imagePathFixApplies reports whether the prompt and auto-injection are
// relevant for the given base image. Returns false for any image not in
// imagePathFixes (ubuntu, debian, future additions) so callers can use it
// as a single gate.
func imagePathFixApplies(image string) bool {
	_, ok := imagePathFixes[image]
	return ok
}

// imagePathFixFor returns the entries and example command for image, or
// the zero value when the image has no fix. Callers must gate on
// imagePathFixApplies before treating the result as meaningful.
func imagePathFixFor(image string) imagePathFix {
	return imagePathFixes[image]
}
