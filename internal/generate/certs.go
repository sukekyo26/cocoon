package generate

// User-cert build context constants shared by the dockerfile, compose,
// and devcontainerjson generators (and the cocoon gen command's host-side
// mkdir helper). They are exported as package-level constants so the four
// emit sites stay in lockstep — changing the BuildKit context name or
// the host path here updates every consumer.

const (
	// CertsBuildContextName is the BuildKit named build context that
	// exposes the host's user certificate directory to the Dockerfile.
	// It must match between docker-compose.yml's `additional_contexts`
	// (left side) and the Dockerfile's `RUN --mount=type=bind,from=…`.
	CertsBuildContextName = "cocoon_user_certs"

	// CertsHostPath is the host-side path with `${HOME}` kept symbolic
	// so docker-compose interpolation expands it per developer at build
	// time. Used by:
	//   - compose.go's `additional_contexts` value
	//   - devcontainer.json's `initializeCommand` mkdir argument
	// The `${HOME:?…}` form makes Compose / sh fail fast with a clear
	// message if HOME is unset (rare on Linux/macOS/WSL2 but possible
	// in stripped-down CI shells), preventing a silent path collapse to
	// `/.cocoon/certs`.
	CertsHostPath = "${HOME:?HOME must be set on the host so cocoon can " +
		"resolve ~/.cocoon/certs (the user TLS certificate directory)}/.cocoon/certs"

	// CertsHostPathRelative is the same path expressed relative to the
	// user home directory. Used by `cocoon gen` together with
	// `os.UserHomeDir()` to perform a real `mkdir` on disk; Compose
	// interpolation does not run there, so we cannot reuse CertsHostPath.
	CertsHostPathRelative = ".cocoon/certs"
)
