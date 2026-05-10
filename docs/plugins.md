# Plugins

This page is the single source of truth for **plugin authoring**:
the `plugin.toml` schema, the rules for `install.sh` and
`install_user.sh`, the environment variables those scripts receive,
and the version-pin contract.

If you are an end user just trying to enable an existing plugin, you
only need one thing: list the plugin's id in `[plugins].enable` in
`workspace.toml`. The rest of this page is for people writing or
modifying a plugin.

## 1. What a plugin is

A plugin is a self-contained installer that the dockerfile generator
folds into the build. It has one required file — `plugin.toml`,
which describes the plugin — and up to two optional shell scripts:

- `install.sh` runs during `docker build` to install the tool. It
  may run as root or as the unprivileged user depending on
  `[install].requires_root`.
- `install_user.sh` runs **always** as the unprivileged user
  after `install.sh`. It exists to handle the small set of cases
  where root-owned install needs to be paired with user-owned
  configuration (see §5).

The whole point of plugins is to keep cocoon's generator small while
still letting users add anything that is not in `[apt].packages`.

## 2. The three layers

Plugins are read from a layered filesystem that resolves
**project > user > embedded** priority:

| Layer | Path | Notes |
|---|---|---|
| project  | `<workspace>/.cocoon/plugins/<id>/`              | overrides everything else; on disk |
| user     | `~/.cocoon/plugins/<id>/`                        | overrides embedded; on disk |
| embedded | `internal/plugin/catalog/<id>/` (in the cocoon source repo, compiled into the binary via `go:embed`) | shipped inside the binary; **not present on a single-binary install** |

Same-id directories are **not merged**: the highest-priority layer
wins completely. Inspect what wins with `cocoon plugin list` (the
`SOURCE` column shows the layer) and `cocoon plugin show <id>`.

To customise an embedded plugin, the supported workflow is to
**scaffold a new id with `cocoon plugin scaffold <new-id>`** and
adapt logic from there. If you have a clone of the cocoon repo,
you can also copy the embedded source directly into your overlay:
`cp -r internal/plugin/catalog/<id> ~/.cocoon/plugins/<id>/`. A
single-binary install does not include the embedded source on
disk, so this shortcut requires either a `git clone` of cocoon
or unpacking a source tarball (e.g. the GitHub Release source
archive).

## 3. Directory layout

```
plugins/<id>/
├── plugin.toml         # required
├── install.sh          # optional (skip if [install.env] alone is enough)
└── install_user.sh     # optional (only when §5 applies)
```

## 4. `plugin.toml` schema

| Section | Field | Type | Default | Required | Meaning |
|---|---|---|---|---|---|
| `[metadata]` | `name`            | string             | —     | ✓ | Human-readable display name. |
| `[metadata]` | `description`     | string             | —     | ✓ | Short description; **must include the upstream URL in parentheses**. |
| `[metadata]` | `default`         | bool               | `false` |   | If true, `cocoon init`'s default plugin set includes this id. |
| `[metadata]` | `conflicts`       | list of strings    | `[]`  |   | Plugin ids that must not be enabled at the same time. |
| `[apt]`      | `packages`        | list of strings    | `[]`  |   | Apt packages installed before `install.sh` runs. |
| `[install]`  | `requires_root`   | bool               | —     | ✓ | If true, `install.sh` runs as root; otherwise as the unprivileged user. |
| `[install]`  | `build_args`      | list of strings    | `[]`  |   | Names of build-time variables (e.g. `DOCKER_GID`) the build must accept. The generator emits matching `ARG <name>` lines and threads `<name>="${<name>}"` into the per-RUN env prefix, so `install.sh` can read `$<name>` as a normal env var. Names match `^[A-Z_][A-Z0-9_]*$`. |
| `[install]`  | `env`             | map<string,string> | `{}`  |   | `ENV` lines emitted after the install runs. Values can reference earlier `ENV`/`ARG` vars. |
| `[install]`  | `volumes`         | list of strings    | `[]`  |   | Per-user paths under `/home/${USERNAME}/<dir>`; each one is `mkdir -p`'d, `chown`'d, and declared as a docker named volume so its contents persist across rebuilds. |
| `[version]`  | `version_capable` | bool               | —     | ✓ | If true, `install.sh` accepts `$PIN` and optionally `$CHECKSUM_AMD64` / `$CHECKSUM_ARM64` (see §7). |

Strict unmarshal: unknown top-level fields and unknown keys inside
known sections are rejected loud. If you see `unknown field "foo"`
when loading a plugin, something in `plugin.toml` was renamed or
removed — check this page for the current schema.

## 5. `install.sh` vs `install_user.sh`

Most plugins only need `install.sh`. Reach for `install_user.sh`
when **all** of these are true:

1. `[install].requires_root = true` (so `install.sh` runs as root), and
2. The plugin must touch user-owned files (`~/.bashrc`,
   `~/.config/<tool>/`, `~/.local/share/`, …) **or** run a setup
   command that has to execute as the unprivileged user (`<tool> init`,
   `git clone ~/.<tool>`, `conda init bash`, …).

The reason for the split is straightforward: writing to `~/.bashrc`
while still root makes the file root-owned, and the user can't edit
it after the container starts. Splitting the user-owned half into a
second hook keeps that boundary explicit and obvious.

### Decision matrix

| Situation | Pick |
|---|---|
| `requires_root = false`; no rc edit needed | `install.sh` only |
| `requires_root = false`; rc edit needed | `install.sh` only — write to `$RC_FILE` from inside `install.sh` |
| `requires_root = true`; only `apt-get` / `/usr/local/bin` work | `install.sh` only |
| `requires_root = true`; rc edit / `~/.config/<tool>` write / `<tool> init` | `install.sh` **and** `install_user.sh` |

### Concrete examples

| Plugin | `install.sh` does | `install_user.sh` does |
|---|---|---|
| starship (real)              | downloads & places `/usr/local/bin/starship` (root)        | appends `eval "$(starship init …)"` to `$RC_FILE` (user) |
| fzf (hypothetical)           | `apt-get install fzf` (root)                               | `git clone https://github.com/junegunn/fzf.git ~/.fzf && ~/.fzf/install --bash` (user) |
| oh-my-zsh (hypothetical)     | `apt-get install zsh` (root)                               | runs the upstream installer to populate `~/.oh-my-zsh` (user) |
| miniconda (hypothetical)     | extracts `/opt/conda` (root)                               | `conda init bash` writes a multi-line block into `$RC_FILE` (user) |

If there is no rc edit and no user-owned config to write, you don't
need `install_user.sh`. The current catalog reflects this: starship
is the only plugin that uses it.

## 6. Environment variables passed to install scripts

Both scripts run inside `bash <<'COCOON_PLUGIN_EOF' … EOF`. The
following names resolve in the script body, with two distinct
mechanisms:

- **Per-RUN env prefix (real bash env vars)**: emitted as
  `NAME="value"` before the `bash` invocation in the generated
  Dockerfile. Visible to bash via `$NAME`.
- **Dockerfile ARG (build-time substitution)**: BuildKit
  substitutes the value into the RUN body before bash sees it,
  so the script ends up with the literal value baked in (it is
  not a runtime env var and `unset NAME` has no effect).

| Variable | How provided | What it is |
|---|---|---|
| `RC_FILE`        | per-RUN env, always | Absolute path to the user's login-shell rc file (`/home/<user>/.bashrc`, `/home/<user>/.zshrc`, or `~/.config/fish/config.fish`). |
| `RC_SYNTAX`      | per-RUN env, always | `posix` (bash/zsh) or `fish`. Use this to branch when emitting rc lines. |
| `LOGIN_SHELL`    | per-RUN env, always | `bash`, `zsh`, or `fish`. |
| `USERNAME`       | Dockerfile `ARG` (declared at the top of the generated Dockerfile), always | The unprivileged container user name. Reference it as `${USERNAME}` in `install.sh`; BuildKit substitutes the literal value before bash runs the body. |
| `PIN`            | per-RUN env, only when `[version].version_capable = true` | Version string from `[plugins.versions.<id>].pin` in `workspace.toml`. Empty means "use upstream latest". |
| `CHECKSUM_AMD64` | per-RUN env, only when `[version].version_capable = true` | `sha256` of the amd64 artifact, or empty (script must skip verification with a warning). |
| `CHECKSUM_ARM64` | same as above | `sha256` of the arm64 artifact. |
| `<BUILD_ARG>`    | per-RUN env, only when listed in `[install].build_args` | The Dockerfile declares an `ARG` and the per-RUN prefix passes the value through, so the script sees a real env var. Example: `docker-cli` reads `DOCKER_GID`. |

Inside `install.sh` and `install_user.sh`, `$VAR` references resolve in
two stages:

1. **BuildKit** substitutes Dockerfile-level `ARG` references
   (`${USERNAME}`, `${UID}`, …) in the heredoc body before bash starts.
   This is how plugins reach `${USERNAME}` without any per-RUN prefix.
2. **bash** then expands per-RUN env vars (`$RC_FILE`, `$DOCKER_GID`, …)
   while it executes the script.

The single-quoted heredoc terminator (`<<'COCOON_PLUGIN_EOF'`) prevents
the outer shell from interfering during the heredoc-read phase, so the
text passes through unchanged and both stages work as expected. Nothing
on the developer's host machine evaluates the script — both stages run
inside the build environment.

The literal string `COCOON_PLUGIN_EOF` must not appear on a line by
itself anywhere in `install.sh` / `install_user.sh` — that would
close the heredoc early. The generator detects this and fails loud
(`ErrHeredocCollision`).

## 7. Versioned plugins (`version_capable = true`)

A versioned plugin agrees to:

- Read `$PIN` and use it as the version. If `$PIN` is empty, fall
  back to upstream "latest" (curl-redirect, GitHub API, etc.).
- Verify the downloaded artifact against `$CHECKSUM_AMD64` /
  `$CHECKSUM_ARM64` when those are non-empty, and **warn loudly**
  when they are empty (do not silently skip).
- Use `dpkg --print-architecture` (or equivalent) to choose the
  right artifact and the right checksum variable.

Users record pins in `workspace.toml` under `[plugins.versions.<id>]`:

```toml
[plugins.versions.go]
pin            = "1.23.4"
checksum_amd64 = "abc..."
checksum_arm64 = "def..."
```

`cocoon plugin pin <id> <ref> --write` generates this block and
inserts (or replaces) it for you. See `docs/commands.md` for the
full flag list.

Plugins where `version_capable = false` ignore `$PIN` entirely;
the pin block has no effect at `gen` time for those.

## 8. Catalog tour

Use these embedded plugins as templates when writing your own:

- **`go`** — `tarball` template, `[install.env]` heavy. Good
  reference for `$PIN` + `$CHECKSUM_*` + arch switch.
- **`docker-cli`** — only catalog plugin using `[install].build_args`
  to receive `DOCKER_GID`. Read this when you need to thread a
  host-derived value into `install.sh`.
- **`proto`** — `curl-pipe` template; minimal `install.sh` because
  the upstream installer does the work, but with a `$PIN`-respecting
  version selection.
- **`starship`** — the only plugin in the catalog with
  `install_user.sh`. Read both files together to understand the
  root → user split.
- **`lazygit`** — `tarball` template, no `[install.env]`. Smallest
  versioned plugin in the catalog.

## 9. Troubleshooting

- **`unknown field "<x>"` from `cocoon plugin show` / `gen`** — your
  `plugin.toml` carries a field that was renamed or removed. Cross
  reference §4 for the current schema. Common offenders are old
  fields that were dropped during refactors.
- **`ErrHeredocCollision: plugin "x" contains the literal
  COCOON_PLUGIN_EOF`** — your `install.sh` has a line that exactly
  matches the heredoc terminator. Rename the marker inside the
  script.
- **`ErrNilPluginsFS`** — internal: a caller built a
  `WorkspaceContext` without wiring up the layered plugin FS. Surface
  this only happens when cocoon's own integration code is being
  modified.
- **`cocoon plugin show <id>` says "not found in any layer"** — the
  id isn't in the embedded catalog and there is no overlay for it.
  Double-check the directory name matches the id you're passing.
- **A volume path is rejected** — `[install].volumes` entries must
  match `^/home/\$\{USERNAME\}/[^/]+$`: a single segment under
  `/home/${USERNAME}/`, no nested paths. Split nested paths into the
  install script (`mkdir -p $HOME/.cache/foo/bar`) instead of
  declaring them as volumes.

## Related docs

- [`architecture.md`](architecture.md) — why the layered FS and
  inline-heredoc design exist.
- [`configuration.md`](configuration.md) — the `[plugins]` and
  `[plugins.versions]` sections of `workspace.toml`.
- [`commands.md`](commands.md) — `cocoon plugin <subcmd>` reference.
