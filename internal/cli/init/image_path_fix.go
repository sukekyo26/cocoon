package initcli

// pathFixEnvEntry is one `K = "V"` pair the auto-path-fix adds to
// [container.shell.env]. Slice order is preserved as the cocoon.toml
// emit order; shellrc.go sorts alphabetically at render time so runtime
// order is independent of this list.
type pathFixEnvEntry struct {
	Key   string
	Value string
}

// pathFixVolume is one `<name> = "<path>"` entry the auto-path-fix adds to
// the top-level [volumes] section so the env-targeted install destination
// survives `docker compose down && up --build`. Name follows
// plugin.DeriveVolumeName conventions (basename of Path with leading dot
// stripped) so image-path-fix and the equivalent catalog plugin emit the
// same compose volume key — swapping image⇄plugin yields a structurally
// equivalent compose snapshot.
type pathFixVolume struct {
	Name string
	Path string
}

// imagePathFix describes the auto-injection cocoon offers for one language
// base image. Command is the user-facing example command (e.g. `npm install
// -g <pkg>`) that the entries make work without sudo; it is spliced into
// the prompt description and the cocoon.toml auto-comment so both
// surfaces explain the *why* in concrete terms. Volumes pairs with Entries
// to persist the install destinations across container rebuilds — the two
// are emitted together (same prompt answer / same flag) because env
// without volumes leaves rebuilds losing user installs, and volumes
// without env mounts a directory the runtime never writes to.
type imagePathFix struct {
	Entries []pathFixEnvEntry
	Volumes []pathFixVolume
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
// /usr/local/cargo while only `cargo install` writes under $HOME. The
// rust image keeps rustup state at /usr/local/rustup, so $HOME/.rustup is
// not used — only $HOME/.cargo needs a named volume.
//
// Python carries no Volumes: $HOME/.local/bin (the install target for
// `pip install --user`) lives under $HOME/.local, which cocoon already
// mounts unconditionally via the reserved `local:` named volume
// (internal/generate/compose/compose.go). Adding a redundant entry would
// collide with that reservation.
//
//nolint:gochecknoglobals // tabular configuration data, file-scoped by design.
var imagePathFixes = map[string]imagePathFix{
	"node": {
		Entries: []pathFixEnvEntry{
			{Key: "NPM_CONFIG_PREFIX", Value: "$HOME/.npm-global"},
			{Key: "PATH", Value: "$HOME/.npm-global/bin:$PATH"},
		},
		Volumes: []pathFixVolume{
			{Name: "npm-global", Path: "/home/${USERNAME}/.npm-global"},
			{Name: "npm", Path: "/home/${USERNAME}/.npm"},
		},
		Command: "npm install -g <pkg>",
	},
	"python": {
		Entries: []pathFixEnvEntry{
			{Key: "PATH", Value: "$HOME/.local/bin:$PATH"},
		},
		// Volumes intentionally nil: $HOME/.local is already covered by
		// the reserved `local:` named volume, so no extra entry is needed.
		Volumes: nil,
		Command: "pip install --user <pkg>",
	},
	"golang": {
		Entries: []pathFixEnvEntry{
			{Key: "PATH", Value: "$HOME/go/bin:$PATH"},
		},
		Volumes: []pathFixVolume{
			{Name: "go", Path: "/home/${USERNAME}/go"},
		},
		Command: "go install <pkg>@latest",
	},
	"rust": {
		Entries: []pathFixEnvEntry{
			{Key: "CARGO_INSTALL_ROOT", Value: "$HOME/.cargo"},
			{Key: "PATH", Value: "$HOME/.cargo/bin:$PATH"},
		},
		Volumes: []pathFixVolume{
			{Name: "cargo", Path: "/home/${USERNAME}/.cargo"},
		},
		Command: "cargo install <pkg>",
	},
	"denoland/deno": {
		Entries: []pathFixEnvEntry{
			{Key: "PATH", Value: "$HOME/.deno/bin:$PATH"},
		},
		Volumes: []pathFixVolume{
			{Name: "deno", Path: "/home/${USERNAME}/.deno"},
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
