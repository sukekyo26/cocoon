package generate

// Cert build-context constants shared by the dockerfile / compose /
// devcontainerjson generators and the gen mkdir helper. Centralized so
// the four emit sites stay in lockstep.

const (
	// CertsBuildContextName must match between compose's
	// `additional_contexts` key and the Dockerfile's
	// `RUN --mount=type=bind,from=…` name.
	CertsBuildContextName = "cocoon_user_certs"

	// CertsHostPath uses `${HOME:?…}` so Compose / sh fail fast when
	// HOME is unset, instead of silently collapsing to `/.cocoon/certs`.
	CertsHostPath = "${HOME:?HOME must be set on the host}/.cocoon/certs"

	// CertsHostPathRelative is the os.UserHomeDir()-relative form gen
	// uses for a real mkdir; Compose interpolation does not run there.
	CertsHostPathRelative = ".cocoon/certs"
)
