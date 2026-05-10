# Commands

Reference for every command exposed by the `cocoon` binary.

## Quick reference

| Command | Purpose |
|---|---|
| `cocoon init` | Generate `workspace.toml` interactively |
| `cocoon gen` | Generate `.devcontainer/` artifacts |
| `cocoon plugin list` | List every available plugin (embedded + overlays) |
| `cocoon plugin show <id>` | Print the resolved manifest for one plugin |
| `cocoon plugin add <id>` | Copy a plugin into a user / project overlay |
| `cocoon plugin remove <id>` | Delete a user / project overlay copy |
| `cocoon plugin pin <id> <ref>` | Emit a `[plugins.versions.<id>]` block (stdout, or in-place with `--write`) |
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
| `--os <id>` | string | Base OS: `ubuntu` \| `debian`. |
| `--os-version <ver>` | string | Base OS version (must match `--os`). |
| `--shell <id>` | string | Container login shell: `bash` \| `zsh` \| `fish`. |
| `--mount-root <path>` | string | Mount range: `"."` (cwd) or `".."` (parent). |
| `--devcontainer` | bool | Force-enable `.devcontainer/devcontainer.json` output. |
| `--no-devcontainer` | bool | Skip `.devcontainer/devcontainer.json`. |
| `--apt-categories <ids>` | string | Comma-separated apt category IDs (skips the prompt). |
| `--plugins <ids>` | string | Comma-separated plugin IDs to enable. |
| `--alias-bundles <ids>` | string | Comma-separated shell-alias bundle IDs (e.g. `git,ls`). |
| `--force` | bool | Overwrite an existing `workspace.toml`. |

### Interactive flow

When run without `--yes`, prompts are shown one screen at a time:

1. service name
2. username
3. OS
4. OS version (filtered by the chosen OS)
5. login shell
6. alias bundles (multi-select)
7. mount range
8. devcontainer y/n
9. apt categories (multi-select)
10. plugins (multi-select)

### Examples

```bash
# Fully interactive
cocoon init

# Non-interactive
cocoon init --yes \
    --service-name myapp --username dev \
    --os ubuntu --os-version 26.04 \
    --shell bash --mount-root . --devcontainer \
    --apt-categories text-editors,vcs,utilities,compression,build \
    --plugins go,uv,github-cli \
    --alias-bundles git,ls
```

---

## `cocoon gen`

Read `workspace.toml`, resolve the layered plugin catalog (project ∪ user ∪ embedded), and write `.devcontainer/`. Plugin install scripts are inlined directly into the generated Dockerfile, so the build needs no external context beyond the project tree.

### Flags

| Flag | Type | Description |
|---|---|---|
| `--workspace <path>` | string | Path to `workspace.toml` (default: discovered from cwd). |
| `--output <dir>` | string | Project root to write artifacts under (default: directory of `workspace.toml`). |

### Examples

```bash
# From the project root
cocoon gen

# Specify a workspace.toml elsewhere
cocoon gen --workspace ./infra/workspace.toml --output ./infra
```

### TLS certificates

The generated `Dockerfile` / `docker-compose.yml` / `devcontainer.json` always contain the wiring that bakes any `~/.cocoon/certs/*.crt` files into the container trust store at build time. The artifacts are byte-identical regardless of whether the developer has any certs configured, so teams can commit them and share. See [the TLS certificates section in `configuration.md`](configuration.md#tls-certificates-cocooncerts) for the full setup.

Manage cocoon plugins. Plugins live in three layers, resolved with priority **project > user > embedded**:

| Layer | Path | Source |
|---|---|---|
| project | `<workspace>/.cocoon/plugins/<id>/` | overlay copied via `add --scope project` or scaffolded in place |
| user | `~/.cocoon/plugins/<id>/` | overlay copied via `add --scope user` (default) |
| embedded | `internal/plugin/catalog/<id>/` (compiled into the binary) | shipped with cocoon |

The typical workflow is:

1. `cocoon plugin add <id>` → copy an embedded plugin into a writable overlay
2. edit `plugin.toml` / `install.sh` in that overlay
3. add `<id>` to `[plugins].enable` in `workspace.toml`
4. `cocoon gen && docker compose up -d --build`

Overlays are read at `gen` time only; placing files in `~/.cocoon/plugins/<id>/` does not enable a plugin on its own — `[plugins].enable` is the activation list.

### `cocoon plugin list`

**Purpose:** show every plugin id reachable through the layered view, with the source layer that wins for each id.

**Example:**

```console
$ cocoon plugin list
ID            SOURCE    DEFAULT  DESCRIPTION
claude-code   embedded  false    Claude Code — AI-powered coding assistant ...
go            embedded  false    Go programming language ...
my-internal   user      true     internal CLI ...
```

**Flags:**

| Flag | Description |
|---|---|
| `--source <embedded\|user\|project>` | Filter to plugins resolved from a single layer (single value only). |

**Gotchas:** when the same id exists in multiple layers, only the highest-priority layer is shown. Use `cocoon plugin remove --scope <layer>` to peel back overlays and reveal the next layer.

### `cocoon plugin show <id>`

**Purpose:** print the resolved `plugin.toml` (parsed and re-rendered into a stable diff-friendly form) plus the source layer that owns it.

**Example:**

```console
$ cocoon plugin show go
id: go
source: embedded
name: Go
description: Go programming language ...
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

### `cocoon plugin add <id>`

**Purpose:** copy an embedded plugin into a writable overlay so its `plugin.toml` / `install.sh` can be edited locally.

**Example:**

```console
$ cocoon plugin add starship --scope user
Plugin "starship" copied to /home/alice/.cocoon/plugins/starship (user overlay)
$ $EDITOR ~/.cocoon/plugins/starship/install.sh
```

**Flags:**

| Flag | Description |
|---|---|
| `--scope <user\|project>` | Where to copy. Default `user` (`~/.cocoon/plugins/<id>/`); `project` uses `<workspace>/.cocoon/plugins/<id>/`. |
| `--force` | Overwrite an existing overlay copy (without `--force` an existing target is an error). |

**Gotchas:**

- An overlay copy is **not auto-enabled** — add `<id>` to `[plugins].enable` in `workspace.toml` and re-run `cocoon gen` for the change to take effect.
- `--scope project` requires a discoverable `workspace.toml` from cwd; otherwise the command refuses with a usage error.
- `*.sh` files are restored to mode `0755` even if the source umask was stricter.

### `cocoon plugin remove <id>`

**Purpose:** delete a user- or project-scope overlay copy. The embedded catalog is never touched.

**Example:**

```console
$ cocoon plugin remove starship --scope user
Plugin "starship" removed from /home/alice/.cocoon/plugins/starship
```

**Flags:**

| Flag | Description |
|---|---|
| `--scope <user\|project>` | Which overlay to delete (**required**, no default). |

**Gotchas:**

- `--scope` is mandatory so you always confirm whether the user or project overlay is being deleted.
- After removal, `cocoon plugin list` will surface the next-priority layer (or the embedded version) for the same id.

### `cocoon plugin pin <id> <ref>`

**Purpose:** record an upstream version (and optional per-arch checksums) for a `version_capable` plugin in `workspace.toml` under `[plugins.versions.<id>]`. The block declares `pin = "<ref>"` plus optional `checksum_amd64` / `checksum_arm64` lines that `install.sh` reads via `$PIN` and `$CHECKSUM_AMD64` / `$CHECKSUM_ARM64`.

**Example (default — stdout, manual paste):**

```console
$ cocoon plugin pin go 1.23.4 --amd64-checksum abc123 --arm64-checksum def456
# Append the following block to workspace.toml under [plugins.versions]:

[plugins.versions.go]
pin = "1.23.4"
checksum_amd64 = "abc123"
checksum_arm64 = "def456"
```

**Example (`--write` — in-place mutation):**

```console
$ cocoon plugin pin go 1.23.4 --write
Updated /home/alice/proj/workspace.toml: [plugins.versions.go]
```

`--write` parses `workspace.toml` line-by-line and replaces the existing block (if any) or appends a new one after the last `[plugins.versions.*]` block. Comments and blank lines outside the target block are preserved.

**Flags:**

| Flag | Description |
|---|---|
| `--amd64-checksum <sha256>` | SHA256 of the amd64 artifact. |
| `--arm64-checksum <sha256>` | SHA256 of the arm64 artifact. |
| `--write` | Insert (or replace) the block in `workspace.toml` (auto-discovered from cwd). |

**Gotchas:**

- `pin` only makes sense for plugins whose `[version].version_capable = true`. The pin block is ignored at `gen` time for non-version-capable plugins.
- Checksum flags only matter when the plugin's `install.sh` actually reads `$CHECKSUM_AMD64` / `$CHECKSUM_ARM64` (i.e. `tarball` template plugins). They are silently inert for `curl-pipe` / `generic` templates.
- `--write` requires a discoverable `workspace.toml` from cwd; without `--write`, the command works from anywhere because it only resolves the layered FS for id validation.
- `--write` only edits the multi-line `[plugins.versions.<id>]` form. If `workspace.toml` has any per-id key assignment directly under `[plugins.versions]` — `<id> = "1.23.4"`, `<id> = [..]`, or the inline-table `<id> = { pin = "..." }` style the `init` template suggests in commented-out lines — `--write` refuses with a usage error rather than appending a duplicate block. Convert each entry to a `[plugins.versions.<id>]` block first, or edit `workspace.toml` manually.

### `cocoon plugin scaffold <id>`

**Purpose:** create a new `<id>/` directory containing a `plugin.toml` and an `install.sh` skeleton from one of three templates. Use it to bootstrap a new project- or user-scope plugin without copying boilerplate by hand.

**Example:**

```console
$ cd ~/projects/myapp
$ cocoon plugin scaffold gh-cli \
    --template curl-pipe --version-capable \
    --name "GitHub CLI" --description "GitHub CLI (https://cli.github.com)" \
    --non-interactive
OK: scaffolded /home/alice/projects/myapp/.cocoon/plugins/gh-cli (2 files)
```

**Templates:**

| Template | Use when | Boilerplate |
|---|---|---|
| `curl-pipe` | upstream ships `curl ... \| bash` (uv, proto) | `$PIN` env-var-driven version; no checksum verification |
| `tarball` | upstream ships GitHub Release tarballs (starship, go) | `$PIN` + `$CHECKSUM_AMD64` / `$CHECKSUM_ARM64` + `dpkg --print-architecture` switch (forces `--version-capable`) |
| `generic` | apt packages or freeform install | minimal skeleton, no `$PIN` plumbing |

**Flags:**

| Flag | Description |
|---|---|
| `--plugins-dir <path>` | Output directory. Default: `<workspace>/.cocoon/plugins` (auto-discovered from `workspace.toml`). |
| `--name <name>` | Display name (e.g. `"GitHub CLI"`). |
| `--description <text>` | Short description. Must embed an upstream URL in parentheses. |
| `--default` | Mark plugin enabled by default. |
| `--requires-root` | `install.sh` runs as root. |
| `--version-capable` | Generate `$PIN` / `$CHECKSUM_*` boilerplate. |
| `--template <kind>` | `curl-pipe` \| `tarball` \| `generic`. |
| `--with-install-user` | Also generate `install_user.sh` (runs as the unprivileged user after `install.sh`). |
| `--non-interactive` | Skip prompts; require all fields above. |
| `--force` | Overwrite `<plugins-dir>/<id>/` if it already exists. |

**Gotchas:**

- Without `--plugins-dir` and outside a cocoon project (no discoverable `workspace.toml`), scaffold refuses with an actionable error rather than silently writing to `./plugins/<id>/`.
- `--template tarball` implies `--version-capable`; the scaffold rejects the combination of `tarball` without `--version-capable`.
- After scaffolding, the generated `plugin.toml` is reloaded under the same strict validator the runtime uses; if it fails (bad name, missing URL in description, etc.), the directory is rolled back.
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
