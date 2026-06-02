package generate

// Password-sudo build-secret constants shared by the dockerfile / compose
// generators and the gen missing-secret warning. Centralized so the emit
// sites stay in lockstep (the secret id the Dockerfile mounts must match the
// name compose defines).

const (
	// SudoPasswordSecretName is the compose secret id and the Dockerfile
	// `RUN --mount=type=secret,id=…` id; the secret mounts at
	// /run/secrets/<name>.
	SudoPasswordSecretName = "sudo_password"

	// SudoPasswordSecretFile is the compose secret source file, resolved
	// relative to the compose project directory (.devcontainer/). It is
	// gitignored and never committed.
	//nolint:gosec // G101: a gitignored filename, not a hardcoded credential.
	SudoPasswordSecretFile = ".env.local"

	// SudoPasswordEnvKey is the key the Dockerfile parses out of the secret
	// file (an env-file): the line `SUDO_PASSWORD=<value>` supplies the
	// container user's sudo password.
	SudoPasswordEnvKey = "SUDO_PASSWORD"
)

// SudoPasswordGitignoreComment documents the SudoPasswordSecretFile ignore line
// that `cocoon gen` and `cocoon init` upsert into .devcontainer/.gitignore (via
// fsx.EnsureGitignoreEntry). Only the secret file is ignored — the generated
// .env is committable and intentionally not ignored. The upsert preserves any
// existing user rules rather than overwriting the file.
//
//nolint:gosec // G101: a .gitignore comment string, not a hardcoded credential.
const SudoPasswordGitignoreComment = "# cocoon: local sudo password secret (build secret) — never commit"
