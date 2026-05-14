# Plugins

> [!WARNING]
> cocoon is in v0.x (alpha). By using it, please understand and accept that the plugin contracts (`plugin.toml` schema, install-script env vars, version-pin layout) may change before 1.0, and that breaking changes can land in any release. See the [CHANGELOG](../CHANGELOG.md) and the README's "Project status" section.

This page is the single source of truth for **plugin authoring**:
the `plugin.toml` schema, the rules for `install.sh` and
`install_user.sh`, the environment variables those scripts receive,
and the version-pin contract.

If you are an end user just trying to enable an existing plugin, you
only need one thing: list the plugin's id in `[plugins].enable` in
`workspace.toml`. The rest of this page is for people writing or
modifying a plugin.

## What a plugin is

A plugin is a self-contained installer that the dockerfile generator
folds into the build. It has one required file — `plugin.toml`,
which describes the plugin — and up to two optional shell scripts:

- `install.sh` runs during `docker build` to install the tool. It
  may run as root or as the unprivileged user depending on
  `[install].requires_root`.
- `install_user.sh` runs **always** as the unprivileged user
  after `install.sh`. It exists to handle the small set of cases
  where root-owned install needs to be paired with user-owned
  configuration (see "`install.sh` vs `install_user.sh`" below).

The whole point of plugins is to keep cocoon's generator small while
still letting users add anything that is not in `[apt].packages`.

## The three layers

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

## Directory layout

```
plugins/<id>/
├── plugin.toml         # required
├── install.<name>.sh   # required — one file per declared [install.methods.<name>] entry
└── install_user.sh     # optional (plugin-scoped, runs after whichever install.<name>.sh ran)
```

Every plugin declares one or more `[install.methods.<name>]` entries in
`plugin.toml` and ships one `install.<name>.sh` per entry. **There is
no `install.sh`** — the loader rejects a literal `install.sh` file with
a migration hint pointing at the rename. Plugins with a single method
still declare one `[install.methods.<name>]` entry; pick the category
that matches your install style (see "Method name categories" below).

The category vocabulary is also used by `cocoon plugin scaffold --template <name>`.

A plugin's **install snippet** (the `# Install …` comment + RUN block in
the generated Dockerfile) is emitted only when at least one of these is
present:

- `install.<name>.sh` (at least one — required by the loader)
- `install_user.sh`
- `[install.env]`

`[install.build_args]` on its own does **not** trigger output: the `ARG`
declaration is only emitted alongside a hook, so a plugin built around
`build_args` alone would carry an `ARG` name that nothing references.
`build_args` is meaningful only when consumed by an
`install.<name>.sh` or `install_user.sh`.

## Method name categories

Method names (the `<name>` in `[install.methods.<name>]` and the
corresponding `install.<name>.sh` filename) follow a 4-category
convention so the workspace.toml `[plugins.methods]` table reads the
same way across the catalog. Plugin-specific names (e.g. `gh-cli`,
`bun-sh`) make users look up plugin.toml to figure out what they
picked.

| Category | Pick when … |
|:---|:---|
| `binary`    | The install is "drop a single binary on PATH". Includes the case where you `tar` a single file out of an archive. (kubectl, helm, terraform, shellcheck.) |
| `installer` | The install pipes a vendor-provided shell installer through `bash`/`sh`. Failure mode is "vendor domain blocked". (bun.sh, sh.rustup.rs, mise.run, astral.sh, gh.io.) |
| `apt`       | The install registers an apt repository (or fetches a `.deb`) and runs `apt-get install`. (docker-cli, github-cli, google-chrome.) |
| `archive`   | The install extracts a multi-file tar/zip into a directory tree (bin + lib + share + …) **or** runs a vendor installer that lives inside the archive. (go, node, zig, nerd-fonts, aws-cli.) |

**Catalog plugins must use one of these four words.** Custom plugins
under `~/.cocoon/plugins/<id>/` or `<workspace>/.cocoon/plugins/<id>/`
may use any name matching the validator's pattern
`^[a-z][a-z0-9_-]*$` when a genuinely novel install path doesn't fit,
but the four-word vocabulary above is strongly recommended even for
overlays — anything outside it is an authoring mistake to be flagged
in review.

Plugins that declare only `[install].volumes` (no install hook, no env)
still affect the generated artifacts — `volumes` flows through a
separate path that produces the `mkdir -p` / `chown` block at the top
of the install phase and the named-volume declaration in
`docker-compose.yml`. Just no per-plugin install snippet.

## `plugin.toml` schema

| Section | Field | Type | Default | Required | Meaning |
|---|---|---|---|---|---|
| `[metadata]` | `name`            | string             | —     | ✓ | Human-readable display name. |
| `[metadata]` | `description`     | string             | —     | ✓ | Short description. Should not embed the upstream URL — use the dedicated `url` field instead. |
| `[metadata]` | `url`             | string             | —     | ✓ | Upstream project URL (`https://...`, no whitespace). Surfaced by `cocoon init`'s per-plugin version picker, `cocoon plugin show` (`url:` row), and `cocoon plugin list` (`URL` column). |
| `[metadata]` | `default`         | bool               | `false` |   | If true, `cocoon init`'s default plugin set includes this id. |
| `[metadata]` | `conflicts`       | list of strings    | `[]`  |   | Plugin ids that must not be enabled at the same time. |
| `[apt]`      | `packages`        | list of strings    | `[]`  |   | Apt packages installed before the install scripts run. |
| `[install]`  | `requires_root`   | bool               | —     | ✓ | If true, the `install.<name>.sh` scripts run as root; otherwise as the unprivileged user. |
| `[install]`  | `build_args`      | list of strings    | `[]`  |   | Names of build-time variables (e.g. `DOCKER_GID`) the plugin consumes. The generator emits matching `ARG <name>` lines once per plugin (next to whichever of the install scripts runs first) and threads `<name>="${<name>}"` into the per-RUN env prefix of every hook, so both `install.<name>.sh` and `install_user.sh` can read `$<name>` as a normal env var. ARG scope is stage-wide, so a single declaration covers both RUNs. Names match `^[A-Z_][A-Z0-9_]*$`. |
| `[install]`  | `env`             | map<string,string> | `{}`  |   | `ENV` lines emitted after the install runs. Values can reference earlier `ENV`/`ARG` vars. |
| `[install]`  | `volumes`         | list of strings    | `[]`  |   | Per-user paths under `/home/${USERNAME}/<dir>`; each one is `mkdir -p`'d, `chown`'d, and declared as a docker named volume so its contents persist across rebuilds. |
| `[install]`  | `default_method`  | string             | —     | ✓ | Name of the method picked when `workspace.toml`'s `[plugins.methods]` does not pin one. Must be a key under `[install.methods]`. Required because `[install.methods]` is now mandatory (single-method plugins still declare one entry). |
| `[install.methods.<name>]` | `description` | string | — | ✓ | One-line description shown in `cocoon init`'s method picker. `<name>` matches `^[a-z][a-z0-9_-]*$` and must have a matching `install.<name>.sh` file on disk. **At least one method entry is required** per plugin; the legacy `install.sh` shape is no longer supported. Pick `<name>` from the 4 category words (see "Method name categories"). |
| `[version]`  | `version_capable` | bool               | —     | ✓ | If true, the install script accepts `$PIN` and optionally `$CHECKSUM_AMD64` / `$CHECKSUM_ARM64` (see "Versioned plugins" below). |

Strict unmarshal: unknown top-level fields and unknown keys inside
known sections are rejected loud. If you see `unknown field "foo"`
when loading a plugin, something in `plugin.toml` was renamed or
removed — check this page for the current schema.

## `install.sh` vs `install_user.sh`

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

## Multiple install methods (`[install.methods]`)

Every plugin declares at least one `[install.methods.<name>]` entry; a
plugin offering multiple install paths simply declares more entries.
Each declared `<name>` must have a matching `install.<name>.sh` file on
disk (loader-enforced).

```toml
# plugin.toml — copilot-cli with two methods
[install]
requires_root = false
default_method = "installer"

[install.methods.installer]
description = "Install via the upstream gh.io install script (default)"

[install.methods.binary]
description = "Direct binary from GitHub Releases (Zscaler-friendly)"
```

With the matching `install.installer.sh` and `install.binary.sh` files
on disk, the plugin offers two install paths. The selection flows
through `workspace.toml`:

```toml
[plugins.methods]
copilot-cli = "binary"  # plugins absent here use their default_method
```

`cocoon init` shows a per-plugin method picker **only** when the plugin
declares two or more methods (single-method plugins skip the prompt
silently — there is no real choice). The picker's pre-selected row is
the plugin's `default_method`, so accepting the recommendation is just
Enter.

The chosen script receives every standard env var (`$PIN`,
`$CHECKSUM_AMD64`, `$CHECKSUM_ARM64`, `$RC_FILE`, `$RC_SYNTAX`,
`$LOGIN_SHELL`, build-arg variables) plus one additional value:

| Var | Meaning |
|---|---|
| `$COCOON_INSTALL_METHOD` | Selected method name (e.g. `"binary"`). Always set since every plugin declares `[install.methods]`. |

The script should fail-fast when the env is missing (`: "${COCOON_INSTALL_METHOD:?missing}"`)
and reject mismatched values so a renamed script catches a stale
`workspace.toml` reference before it pulls down the wrong artifact.

**Pin/checksum scope.** Pins live under `[plugins.versions]` and are
**workspace-scoped, not method-scoped** — the catalog deliberately keeps
`plugin.toml` free of per-method checksums to bound the diff every
upstream release produces. When switching methods, refresh
`checksum_amd64` / `checksum_arm64` in `workspace.toml` (or use
`cocoon plugin pin --method <name>` to do both at once) so the install
script's `sha256sum -c -` still matches the new artifact.

**3-layer overlay.** Method resolution is currently **same-layer only**:
a plugin's `plugin.toml` and every declared `install.<name>.sh` must
live in the same layer (embedded / user / project). Cross-layer
distribution (e.g. one method script in the user layer and another in
the project layer) is not supported yet.

## Environment variables passed to install scripts

Both scripts run inside `bash <<'COCOON_PLUGIN_EOF' … COCOON_PLUGIN_EOF`.
The single-quoted heredoc terminator means the body passes through
BuildKit and `/bin/sh` verbatim — no `${VAR}` substitution happens
at parse / heredoc-read time. **bash, executing the body, expands
`$VAR` references at runtime using its environment.** The plugin
author writes `${USERNAME}`, `$RC_FILE`, etc. directly in the script;
they are resolved when bash runs the body.

bash's environment for that step is composed from two sources:

- **Per-RUN env prefix**: the generator emits `NAME="value"` pairs on
  the `bash …` line in the Dockerfile (e.g. `RC_FILE="…" bash <<'EOF'`).
  These bind `NAME` for that single RUN step only.
- **Dockerfile `ARG` promotion**: BuildKit promotes every `ARG`
  declared in the Dockerfile to a real environment variable for
  subsequent `RUN` commands (with a couple of internal-only
  exceptions). So `ARG USERNAME` makes `$USERNAME` available to the
  bash body without any per-RUN prefix.

| Variable | Source | What it is |
|---|---|---|
| `RC_FILE`        | per-RUN env, always | Absolute path to the user's login-shell rc file (`/home/<user>/.bashrc`, `/home/<user>/.zshrc`, or `~/.config/fish/config.fish`). |
| `RC_SYNTAX`      | per-RUN env, always | `posix` (bash/zsh) or `fish`. Use this to branch when emitting rc lines. |
| `LOGIN_SHELL`    | per-RUN env, always | `bash`, `zsh`, or `fish`. |
| `USERNAME`       | Dockerfile `ARG`, always | The unprivileged container user name (declared via `ARG USERNAME` near the top of the generated Dockerfile and promoted to env by BuildKit). |
| `UID` / `GID`    | Dockerfile `ARG`, always | Numeric UID and GID of the unprivileged user (same `ARG` mechanism as `USERNAME`). |
| `PIN`            | per-RUN env, only when `[version].version_capable = true` | Version string from `[plugins.versions]`'s `<id> = { pin = "..." }` entry in `workspace.toml`. Empty means "use upstream latest". |
| `CHECKSUM_AMD64` | per-RUN env, only when `[version].version_capable = true` | `sha256` of the amd64 artifact, or empty (script must skip verification with a warning). |
| `CHECKSUM_ARM64` | same as above | `sha256` of the arm64 artifact. |
| `<BUILD_ARG>`    | per-RUN env (also declared as `ARG`), only when listed in `[install].build_args` | The generator emits one `ARG <name>` line per plugin (next to whichever hook runs first) and threads `<name>="${<name>}"` into the per-RUN prefix of every hook. The Dockerfile substitutes the value on each prefix line at build time. Example: `docker-cli` reads `DOCKER_GID`. |

Nothing on the developer's host machine evaluates the script — bash
runs the body inside the build environment, with the env composed as
above.

The literal string `COCOON_PLUGIN_EOF` must not appear on a line by
itself anywhere in `install.sh` / `install_user.sh` — that would
close the heredoc early. The generator detects this and fails loud
(`ErrHeredocCollision`).

## Versioned plugins (`version_capable = true`)

A versioned plugin agrees to:

- Read `$PIN` and use it as the version. If `$PIN` is empty, fall
  back to upstream "latest" (curl-redirect, GitHub API, etc.).
- Verify the downloaded artifact against `$CHECKSUM_AMD64` /
  `$CHECKSUM_ARM64` when those are non-empty, and **warn loudly**
  when they are empty (do not silently skip).
- Use `dpkg --print-architecture` (or equivalent) to choose the
  right artifact and the right checksum variable.

Users record pins in `workspace.toml` under a single
`[plugins.versions]` section, one inline-table line per plugin:

```toml
[plugins.versions]
go = { pin = "1.23.4", checksum_amd64 = "abc...", checksum_arm64 = "def..." }
```

`cocoon plugin pin <id> <ref> --write` upserts this line for you.
See `docs/commands.md` for the full flag list.

Plugins where `version_capable = false` ignore `$PIN` entirely;
the pin entry has no effect at `gen` time for those.

## Catalog tour

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

## Troubleshooting

- **`unknown field "<x>"` from `cocoon plugin show` / `gen`** — your
  `plugin.toml` carries a field that was renamed or removed. Cross
  reference the "`plugin.toml` schema" section above for the current
  schema. Common offenders are old fields that were dropped during
  refactors.
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
