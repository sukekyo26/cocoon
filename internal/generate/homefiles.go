package generate

import "errors"

// HomeFilesHostPathPrefix is the compose-source prefix for [home_files]
// entries. Uses ${HOME:?…} so docker compose / sh fail fast when HOME is
// unset on the host instead of silently mounting `/<rel>`. Callers append
// `/<rel>` per file. Identical shape to CertsHostPath; a future refactor
// can unify both.
const HomeFilesHostPathPrefix = "${HOME:?HOME must be set on the host}"

// ErrHomeFileIsDirectory signals that a [home_files] entry exists as a
// directory on the host — typically auto-created by Docker when the file
// was missing at first `docker compose up`. Callers should surface `rm -rf`
// recovery guidance so users can recover the bind mount.
var ErrHomeFileIsDirectory = errors.New("home_files: path exists as a directory")
