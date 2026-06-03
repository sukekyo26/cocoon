# Commands

> [!WARNING]
> cocoon is in v0.x (alpha). By using it, please understand and accept that the CLI flags and subcommands may change before 1.0, and that breaking changes can land in any release. See the [CHANGELOG](../CHANGELOG.md) and the README's "Project status" section.

Reference for every command exposed by the `cocoon` binary.

## Quick reference

| Command | Purpose |
|---|---|
| `cocoon init` | Generate `workspace.toml` interactively |
| `cocoon gen` | Generate `.devcontainer/` artifacts |
| `cocoon gen workspace` | Generate `<name>.code-workspace` at the project root |
| `cocoon lock` | Resolve plugin versions and write `cocoon.lock` for reproducible builds |
| `cocoon plugin list` | List every available plugin (embedded + overlays) |
| `cocoon plugin show <id>` | Print the resolved manifest for one plugin |
| `cocoon plugin pin <id> <ref>` | Emit a `[plugins].enable` array element pinning the version (stdout, or in-place with `--write`) |
| `cocoon plugin scaffold <id>` | Create a new `<id>/` directory from a template |
| `cocoon self-update` | Replace this binary with the latest GitHub release |
| `cocoon version` | Print binary version |
| `cocoon help [command]` | Print help (Cobra builtin) |
| `cocoon completion {bash,zsh,fish,powershell}` | Generate shell completion scripts (Cobra builtin) |

---

## `cocoon init`

Generate `workspace.toml` in the current directory.

### Flags

| Flag | Type | Description |
|---|---|---|
| `--yes` | bool | Skip prompts. `--service-name` and `--username` become required. |
| `--service-name <name>` | string | Compose service name (required with `--yes`). |
| `--username <name>` | string | In-container user (required with `--yes`). |
| `--image <id>` | string | Base image (DockerHub canonical name): `ubuntu` \| `debian` \| `node` \| `python` \| `golang` \| `rust` \| `denoland/deno`. Defaults to `debian` when omitted. |
| `--image-version <ver>` | string | Base image tag. Any well-formed Docker tag is accepted (first character alnum or `_`; `.` / `-` allowed in trailing positions; no slash, no colon); existence in the upstream registry is left to `docker pull`. Requires `--image` to be set. When omitted, defaults to the image's first suggested tag (`debian` → `12`). |
| `--shell <id>` | string | Container login shell: `bash` \| `zsh` \| `fish`. |
| `--mount-root <path>` | string | Mount range: `"."` (cwd) or `".."` (parent). |
| `--devcontainer` | bool | Force-enable `.devcontainer/devcontainer.json` output. |
| `--no-devcontainer` | bool | Skip `.devcontainer/devcontainer.json`. |
| `--certificates` | bool | Force-enable `[certificates] enable = true` (host TLS auto-bake from `~/.cocoon/certs/`). |
| `--no-certificates` | bool | Force-disable; omit the `[certificates]` section (default). |
| `--sudo <mode>` | string | In-container sudo policy: `nopasswd` (default, passwordless), `password` (requires `SUDO_PASSWORD` from `.devcontainer/.env.local`, applied via a build secret), or `none` (`no_new_privileges = true`, disables sudo). Interactive `password` prompts for the password and writes `.env.local` (0600). |
| `--apt-categories <ids>` | string | Comma-separated apt category IDs (skips the prompt). |
| `--plugins <ids>` | string | Comma-separated plugin IDs to enable. |
| `--plugin-versions <id>=<ref>,...` | string | Comma-separated `<id>=<ref>` pins for `version_capable` plugins. Each `<id>` must also appear in `--plugins`, must be `version_capable`, and may not repeat. The version is written inline in the generated `workspace.toml`'s `[plugins].enable` array (e.g. `--plugin-versions go=1.23.4` → the element `"go=1.23.4"`); no checksums. |
| `--alias-bundles <ids>` | string | Comma-separated shell-alias bundle IDs (e.g. `git,ls`). |
| `--ports <values>` | string | Comma-separated docker-compose short-form port mappings (e.g. `3000:3000,5432:5432`). Accepts every form documented for `[ports].forward`: container-only `3000`, ranges `3000-3005:3000-3005`, IPv4/IPv6 binds `127.0.0.1:8001:8001` / `[::1]:80:80`, and protocols `6060:6060/udp`. Skips the prompt. Empty / omitted = no active `[ports]` block (the commented-out template stays so the section is discoverable). |
| `--force` | bool | Overwrite an existing `workspace.toml`. |

### Interactive flow

When run without `--yes`, prompts are shown one screen at a time:

1. service name
2. username
3. base image
4. image version (filtered by the chosen image)
5. login shell
6. alias bundles (multi-select)
7. mount range
8. devcontainer y/n
9. certificates y/n (opt in to host TLS auto-bake from `~/.cocoon/certs/`; default no)
10. secure y/n (preset `no_new_privileges = true`, disabling in-container `sudo`; default no)
11. port forwards (comma-separated docker-compose short forms; blank to skip — the commented-out `[ports]` template stays for later opt-in)
12. apt categories (multi-select)
13. plugins (multi-select)

### Examples

```bash
# Fully interactive
cocoon init

# Non-interactive
cocoon init --yes \
    --service-name myapp --username dev \
    --image debian --image-version 12 \
    --shell bash --mount-root . --devcontainer \
    --apt-categories text-editors,vcs,utilities,compression,build \
    --plugins go,uv,github-cli \
    --alias-bundles git,ls \
    --ports 3000:3000,5432:5432
```

---

## `cocoon gen`

Read `workspace.toml`, resolve the layered plugin catalog (project ∪ user ∪ embedded), and write `.devcontainer/`. Plugin install scripts are inlined directly into the generated Dockerfile, so the build needs no external context beyond the project tree. Generation is fully offline: when a [`cocoon.lock`](#cocoon-lock) is present, each locked plugin's resolved version and per-arch checksums are baked into the Dockerfile (`PIN` / `CHECKSUM_*`) so the build is reproducible.

### Flags

| Flag | Type | Description |
|---|---|---|
| `--workspace <path>` | string | Path to `workspace.toml` (default: discovered from cwd). |
| `--output <dir>` | string | Project root to write artifacts under (default: directory of `workspace.toml`). |
| `--locked` | bool | Fail if any enabled plugin uses `"latest"` without a `cocoon.lock` entry (reproducible CI). Without it, such plugins warn and fall back to resolving the latest version at build time. |

### Examples

```bash
# From the project root
cocoon gen

# Specify a workspace.toml elsewhere
cocoon gen --workspace ./infra/workspace.toml --output ./infra
```

### TLS certificates

The generated `Dockerfile` / `docker-compose.yml` / `devcontainer.json` carry cert auto-bake wiring **only when the workspace opts in** via `[certificates] enable = true` (or `cocoon init --certificates`). Cert-free workspaces commit cert-free artifacts (no `additional_contexts`, no `RUN --mount=type=bind`, no `initializeCommand`). When opted in, the trust store ingests any `~/.cocoon/certs/*.crt` and `*.cer` files at build time. See [`[certificates]` in `configuration.md`](configuration.md#certificates) for the full setup and team workflow.

### `cocoon gen workspace`

Generate a VS Code `.code-workspace` file from the `[code_workspace]` section of `workspace.toml`. By default the file is written next to `workspace.toml` (**not** under `.devcontainer/`), so VS Code can open it with `code <name>.code-workspace` and treat it as the project's entry point. Pass `--output <dir>` to redirect the file elsewhere — folder paths are always relativized against the directory the file is actually written to, so VS Code resolves them correctly from that location. Paths in `folders[]` are `~`-expanded as well, which is what lets entries like `"~/.claude"` resolve to a path VS Code can traverse upward.

`cocoon gen` itself does **not** emit this file — the subcommand is opt-in.

#### Flags

| Flag | Type | Description |
|---|---|---|
| `--workspace <path>` | string | Path to `workspace.toml` (default: discovered from cwd). |
| `--output <dir>` | string | Project root to write the `.code-workspace` under (default: directory of `workspace.toml`). |
| `--name <basename>` | string | Output file basename without `.code-workspace` (default: `[code_workspace].name` or project directory basename). Validated as a single path segment. |
| `--folder <path>[=<name>]` | repeatable | Append a folder after `[code_workspace].folders`. Supports `~` expansion. Pass `=<name>` to override the auto-derived display name. |

#### Examples

```bash
# Use [code_workspace] from workspace.toml as-is
cocoon gen workspace

# Add ad-hoc folders without editing workspace.toml
cocoon gen workspace --folder ~/.config/nvim --folder ../sibling-repo=Sibling

# Override the file name
cocoon gen workspace --name my-stack
```

See [`[code_workspace]` in `configuration.md`](configuration.md#code_workspace) for the TOML schema and path-resolution rules.

---

## `cocoon lock`

Resolve every enabled `version_capable` plugin's `[plugins].enable` version pin to a concrete version (plus per-arch SHA256 checksums) over the network, and write `cocoon.lock` at the workspace root (next to `workspace.toml`). `cocoon gen` then consumes `cocoon.lock` offline so the generated `.devcontainer/` is reproducible — same plugin versions, same checksums, no network call at generation time.

- A `"latest"` constraint is frozen to the newest release. An `"=x.y.z"` exact pin keeps its version and gains recorded per-arch checksums.
- The lock file is named `cocoon.lock` by default; set [`[lockfile].name`](configuration.md#lockfile) in `workspace.toml` to use a different basename (both `cocoon lock` and `cocoon gen` honor it).
- Re-running is idempotent: already-locked entries are reused with **no network call** unless `--upgrade` is passed. `--upgrade` re-resolves `"latest"` constraints to the current newest release; exact pins never change.

### `cocoon.lock`

A generated, committed TOML file — machine-owned, so do **not** hand-edit it; re-run `cocoon lock` instead. It carries a top-level `lock_version` (lock-format version) and `inputs_hash` (a digest of the enabled plugins and their constraints, used by `--check` and `cocoon gen --locked` to detect drift from `workspace.toml`), then one `[[plugins]]` entry per resolved plugin:

| Field | Meaning |
|---|---|
| `id` | Plugin id. |
| `requested` | The `workspace.toml` constraint that produced the entry (`"latest"` or `"=x.y.z"`). |
| `version` | The concrete resolved version. |
| `checksum_amd64` / `checksum_arm64` | Per-arch SHA256 of the downloaded artifact. Omitted for plugins that publish no fetchable per-arch hash (e.g. `verify = "pgp"` or `| bash` installers). |
| `extra` | Frozen subcomponent selectors, when the plugin has them (e.g. android-sdk's `api_level`). |

### Flags

| Flag | Type | Description |
|---|---|---|
| `--workspace <path>` | string | Path to `workspace.toml` (default: discovered from cwd). |
| `--check` | bool | Verify `cocoon.lock` matches `workspace.toml` **without resolving** (no network). Exits non-zero on drift — a missing lock, a changed `inputs_hash`, or any enabled plugin whose recorded `requested` no longer matches. For CI. |
| `--upgrade` | bool | Re-resolve `"latest"` constraints to the current newest release. Exact pins are untouched. |

### Exact-only plugins

A few plugins' upstreams expose no machine-readable "latest": **`aws-cli`** (unversioned download alias), **`android-sdk`** (HTML-scraped build number), and **`flutter`** (release keyed by a commit hash). These cannot resolve `"latest"`; they must be pinned to an exact version inline in the `[plugins].enable` array (e.g. `"flutter=3.44.1"`). Left unpinned or on `latest`, `cocoon lock` errors with a hint to pin them (`"<id>=<version>"`).

### Examples

```console
$ cocoon lock
OK: Locked go 1.23.4
OK: Locked uv 0.5.11
OK: Wrote /home/alice/proj/cocoon.lock (2 plugin(s))
```

Resulting `cocoon.lock` (excerpt):

```toml
# cocoon.lock — generated by `cocoon lock`; do not edit by hand.
# Records resolved plugin versions + per-arch checksums for reproducible builds.

lock_version = 1
inputs_hash = "…"

[[plugins]]
id = "go"
requested = "latest"
version = "1.23.4"
checksum_amd64 = "…"
checksum_arm64 = "…"

[[plugins]]
id = "uv"
requested = "latest"
version = "0.5.11"
```

```bash
# Refresh "latest" constraints to the newest releases
cocoon lock --upgrade

# CI gate: fail if the committed lock no longer matches workspace.toml
cocoon lock --check
```

---

## `cocoon plugin`

Inspect and author cocoon plugins. Plugins live in three layers, resolved with priority **project > user > embedded**:

| Layer | Path | Source |
|---|---|---|
| project | `<workspace>/.cocoon/plugins/<id>/` | overlay scaffolded in place or copied from another overlay |
| user | `~/.cocoon/plugins/<id>/` | overlay scaffolded or copied from another overlay |
| embedded | `internal/plugin/catalog/<id>/` (in the cocoon source repo, compiled into the binary via `go:embed`) | shipped with cocoon; **not on disk** for single-binary installs |

To enable an embedded plugin, just add its id to `[plugins].enable` in `workspace.toml`. To customise an embedded plugin, the supported workflow is `cocoon plugin scaffold <new-id>` and adapting logic. If you have a clone of the cocoon repo (or an unpacked source tarball), `cp -r internal/plugin/catalog/<id> ~/.cocoon/plugins/<id>/` works as a shortcut; on a single-binary install the embedded source is not present on disk.

Overlays are read at `gen` time only; placing files in `~/.cocoon/plugins/<id>/` does not enable a plugin on its own — `[plugins].enable` is the activation list.

> Authoring a plugin? See [`docs/plugins.md`](plugins.md) for the full `plugin.toml` schema, the `install.sh` / `install_user.sh` rules, and the version-pin contract.

### `cocoon plugin list`

**Purpose:** show every plugin id reachable through the layered view, with the source layer that wins for each id.

**Example:**

```console
$ cocoon plugin list
ID            SOURCE    DEFAULT  DESCRIPTION                                  URL
claude-code   embedded  false    Claude Code — AI-powered coding assistant... https://github.com/anthropics/claude-code
go            embedded  false    Go programming language ...                  https://github.com/golang/go
my-internal   user      true     internal CLI ...                             https://git.example.com/team/internal-cli
```

**Flags:**

| Flag | Description |
|---|---|
| `--source <embedded\|user\|project>` | Filter to plugins resolved from a single layer (single value only). |

**Gotchas:** when the same id exists in multiple layers, only the highest-priority layer is shown. Delete the overlay directory directly (e.g. `rm -rf ~/.cocoon/plugins/<id>`) to reveal the next layer.

### `cocoon plugin show <id>`

**Purpose:** print the resolved `plugin.toml` (parsed and re-rendered into a stable diff-friendly form) plus the source layer that owns it.

**Example:**

```console
$ cocoon plugin show go
id: go
source: embedded
name: Go
description: Go programming language ...
url: https://github.com/golang/go
default: false
requires_root: true
version_capable: true
apt_packages: [build-essential]
env:
  GOPATH=/home/${USERNAME}/go
  PATH=/usr/local/go/bin:$GOPATH/bin:$PATH
volumes: [/home/${USERNAME}/go]
```

**Gotchas:** errors with `plugin "<id>" not found in any layer` if the id is unknown. `apt_packages` and `env` are sorted alphabetically for stable diffs — the order does not match `plugin.toml` source order.

### `cocoon plugin pin <id> <ref>`

**Purpose:** pin a version for a `version_capable` plugin in `workspace.toml`'s `[plugins].enable` array. The pin is emitted as an array element — `"<id>=<ref>"` — that the plugin's install script reads via `$PIN`. A bare `<ref>` (e.g. `1.23.4`) is written with the version spelled bare (`"go=1.23.4"`); passing `latest` writes `"go=latest"`; a range (`>=`, `^`, …) is rejected with a usage error.

**Example (default — stdout, manual paste):**

```console
$ cocoon plugin pin go 1.23.4
# Add (or update) this entry in the [plugins].enable array in workspace.toml:

"go=1.23.4"
```

**Example (`--write` — in-place mutation):**

```console
$ cocoon plugin pin go 1.23.4 --write
Updated /home/alice/proj/workspace.toml: [plugins].enable "go=1.23.4"
```

`--write` upserts the `<id>` element in the `[plugins].enable` array — replacing the existing `"<id>"` / `"<id>=..."` element if the id is already enabled, or appending a new one — and re-emits the array in canonical multi-line style (one element per line). Comments and blank lines elsewhere in the file are preserved.

**Flags:**

| Flag | Description |
|---|---|
| `--method <name>` | Install method to validate the ref against (when the plugin declares more than one). |
| `--write` | Upsert the pin element in `workspace.toml`'s `[plugins].enable` array (auto-discovered from cwd). |

**Gotchas:**

- A pin only makes sense for plugins whose `[version].version_capable = true`. The element's version is ignored at `gen` time for non-version-capable plugins.
- Checksums are not pinned here. They are recorded in `cocoon.lock` by `cocoon lock`; until then the install script's fallback verifies each download against the checksum the upstream publishes with the release.
- `--write` requires a discoverable `workspace.toml` from cwd; without `--write`, the command works from anywhere because it only resolves the layered FS for id validation.
- `--write` refuses with a usage error if `workspace.toml` still contains a `[plugins.versions]` section (the removed schema). Migrate each pin into the `[plugins].enable` array first — turn an inline-table pin like `go = { pin = "1.23.4" }` into the element `"go=1.23.4"` and delete the `[plugins.versions]` section — then re-run, or edit `workspace.toml` manually.

### `cocoon plugin scaffold <id>`

**Purpose:** create a new `<id>/` directory containing a `plugin.toml` and an `install.<category>.sh` skeleton from one of four templates (catalog method-name vocabulary). Use it to bootstrap a new project- or user-scope plugin without copying boilerplate by hand.

**Example:**

```console
$ cd ~/projects/myapp
$ cocoon plugin scaffold gh-cli \
    --template installer --version-capable \
    --name "GitHub CLI" --description "GitHub CLI" \
    --url "https://cli.github.com" \
    --non-interactive
OK: scaffolded /home/alice/projects/myapp/.cocoon/plugins/gh-cli (2 files)
```

**Templates:**

| Template | Use when | Boilerplate |
|---|---|---|
| `installer` | upstream ships `curl ... \| bash` (uv, proto, mise) | `$PIN` env-var-driven version; no checksum verification |
| `binary` | upstream ships a single binary you drop on PATH (helm, kubectl, terraform) | `$PIN` + `$CHECKSUM_AMD64` / `$CHECKSUM_ARM64` + `dpkg --print-architecture` switch |
| `apt` | apt repository or `.deb` package (docker-cli, github-cli, google-chrome) | apt keyring + sources.list scaffold; no `$PIN` plumbing |
| `archive` | upstream ships multi-file tar/zip extracted to a tree (go, node, zig) | `$PIN` + `$CHECKSUM_AMD64` / `$CHECKSUM_ARM64` + `tar --strip-components=1` |

**Flags:**

| Flag | Description |
|---|---|
| `--plugins-dir <path>` | Output directory. Default: `<workspace>/.cocoon/plugins` (auto-discovered from `workspace.toml`). |
| `--name <name>` | Display name (e.g. `"GitHub CLI"`). |
| `--description <text>` | Short description. Do not embed the upstream URL — pass it via `--url`. |
| `--url <url>` | Upstream project URL (`https://...`, no whitespace). Required under `--non-interactive`. |
| `--default` | Mark plugin enabled by default. |
| `--requires-root` | The install script runs as root. |
| `--version-capable` | Generate `$PIN` / `$CHECKSUM_*` boilerplate. |
| `--template <kind>` | `installer` \| `binary` \| `apt` \| `archive`. |
| `--with-install-user` | Also generate `install_user.sh` (runs as the unprivileged user after `install.<category>.sh`). |
| `--non-interactive` | Skip prompts; require all fields above. |
| `--force` | Overwrite `<plugins-dir>/<id>/` if it already exists. |

**Gotchas:**

- Without `--plugins-dir` and outside a cocoon project (no discoverable `workspace.toml`), scaffold refuses with an actionable error rather than silently writing to `./plugins/<id>/`.
- `--template binary` implies `--version-capable`; the scaffold rejects `binary` without `--version-capable`.
- After scaffolding, the generated `plugin.toml` is reloaded under the same strict validator the runtime uses; if it fails (bad name, missing or malformed `url`, etc.), the directory is rolled back.
- Like overlays from `add`, a scaffolded plugin still needs to be listed in `[plugins].enable` to take effect at `gen` time.

---

## `cocoon self-update`

Replace the running binary with the latest GitHub release. The new binary is fetched together with `SHA256SUMS`, verified, and swapped in via atomic rename (with a cross-device fallback).

| Flag | Description |
|---|---|
| `--check-only` | Report whether an update is available without downloading. Exit 100 means an update exists. |
| `--force` | Re-install even when already at the latest version. |

---

## `cocoon version`

Print the cocoon binary version (set at build time via `-X main.Version`).

---

## `cocoon help [command]`

Cobra builtin. Prints help for the given command. `cocoon help <command>` is equivalent to `cocoon <command> --help`.

---

## `cocoon completion {bash,zsh,fish,powershell}`

Cobra builtin. Generates shell completion scripts:

```bash
# bash (system-wide)
cocoon completion bash | sudo tee /etc/bash_completion.d/cocoon

# zsh (user-local)
cocoon completion zsh > "${fpath[1]}/_cocoon"

# fish
cocoon completion fish > ~/.config/fish/completions/cocoon.fish

# PowerShell
cocoon completion powershell | Out-String | Invoke-Expression
```

---

## Environment variables

| Variable | Purpose |
|---|---|
| `WORKSPACE_LANG` | Highest-priority locale override for cocoon prompts and inline TOML comments. |
| `LC_ALL` / `LC_MESSAGES` / `LANG` | Fallback locale chain. Any value starting with `ja` selects Japanese. |
| `COCOON_INSTALL_DIR` | `install.sh`: target directory (default: `$HOME/.local/bin`). |

---

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | Failure (`ErrFailure`-wrapped) |
| `2` | Usage error (`ErrUsage`-wrapped) |
| `100` | `cocoon self-update --check-only`: an update is available |
