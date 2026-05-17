# Commands

> [!WARNING]
> cocoon is in v0.x (alpha). By using it, please understand and accept that the CLI flags and subcommands may change before 1.0, and that breaking changes can land in any release. See the [CHANGELOG](../CHANGELOG.md) and the README's "Project status" section.

Reference for every command exposed by the `cocoon` binary.

## Quick reference

| Command | Purpose |
|---|---|
| `cocoon init` | Generate `workspace.toml` interactively |
| `cocoon gen` | Generate `.devcontainer/` artifacts |
| `cocoon plugin list` | List every available plugin (embedded + overlays) |
| `cocoon plugin show <id>` | Print the resolved manifest for one plugin |
| `cocoon plugin pin <id> <ref>` | Emit an inline-table line for `[plugins.versions]` (stdout, or in-place with `--write`) |
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
| `--image <id>` | string | Base image (DockerHub canonical name): `ubuntu` \| `debian` \| `node` \| `python` \| `golang` \| `rust` \| `denoland/deno`. |
| `--image-version <ver>` | string | Base image tag. Any well-formed Docker tag is accepted (first character alnum or `_`; `.` / `-` allowed in trailing positions; no slash, no colon); existence in the upstream registry is left to `docker pull`. Requires `--image` to be set. |
| `--shell <id>` | string | Container login shell: `bash` \| `zsh` \| `fish`. |
| `--mount-root <path>` | string | Mount range: `"."` (cwd) or `".."` (parent). |
| `--devcontainer` | bool | Force-enable `.devcontainer/devcontainer.json` output. |
| `--no-devcontainer` | bool | Skip `.devcontainer/devcontainer.json`. |
| `--certificates` | bool | Force-enable `[certificates] enable = true` (host TLS auto-bake from `~/.cocoon/certs/`). |
| `--no-certificates` | bool | Force-disable; omit the `[certificates]` section (default). |
| `--apt-categories <ids>` | string | Comma-separated apt category IDs (skips the prompt). |
| `--plugins <ids>` | string | Comma-separated plugin IDs to enable. |
| `--plugin-versions <id>=<ref>,...` | string | Comma-separated `<id>=<ref>` pins for `version_capable` plugins. Each `<id>` must also appear in `--plugins`, must be `version_capable`, and may not repeat. Emits a `[plugins.versions]` block directly in the generated `workspace.toml`. |
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
10. port forwards (comma-separated docker-compose short forms; blank to skip — the commented-out `[ports]` template stays for later opt-in)
11. apt categories (multi-select)
12. plugins (multi-select)

### Examples

```bash
# Fully interactive
cocoon init

# Non-interactive
cocoon init --yes \
    --service-name myapp --username dev \
    --image ubuntu --image-version 26.04 \
    --shell bash --mount-root . --devcontainer \
    --apt-categories text-editors,vcs,utilities,compression,build \
    --plugins go,uv,github-cli \
    --alias-bundles git,ls \
    --ports 3000:3000,5432:5432
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

The generated `Dockerfile` / `docker-compose.yml` / `devcontainer.json` carry cert auto-bake wiring **only when the workspace opts in** via `[certificates] enable = true` (or `cocoon init --certificates`). Cert-free workspaces commit cert-free artifacts (no `additional_contexts`, no `RUN --mount=type=bind`, no `initializeCommand`). When opted in, the trust store ingests any `~/.cocoon/certs/*.crt` files at build time. See [`[certificates]` in `configuration.md`](configuration.md#certificates) for the full setup and team workflow.

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

**Purpose:** record an upstream version (and optional per-arch checksums) for a `version_capable` plugin in `workspace.toml` under `[plugins.versions]`. The entry is emitted as a single inline-table line — `<id> = { pin = "<ref>", checksum_amd64 = "...", checksum_arm64 = "..." }` — that the plugin's install script reads via `$PIN` and `$CHECKSUM_AMD64` / `$CHECKSUM_ARM64`.

**Example (default — stdout, manual paste):**

```console
$ cocoon plugin pin go 1.23.4 --amd64-checksum abc123 --arm64-checksum def456
# Add the following line under [plugins.versions] in workspace.toml:

go = { pin = "1.23.4", checksum_amd64 = "abc123", checksum_arm64 = "def456" }
```

**Example (`--write` — in-place mutation):**

```console
$ cocoon plugin pin go 1.23.4 --write
Updated /home/alice/proj/workspace.toml: [plugins.versions] go
```

`--write` parses `workspace.toml` line-by-line, replaces the existing `<id> = { ... }` line under `[plugins.versions]` (if any) or appends a new one to that section. Comments and blank lines outside the target line are preserved.

**Flags:**

| Flag | Description |
|---|---|
| `--amd64-checksum <sha256>` | SHA256 of the amd64 artifact. |
| `--arm64-checksum <sha256>` | SHA256 of the arm64 artifact. |
| `--write` | Insert (or replace) the inline-table line in `workspace.toml` (auto-discovered from cwd). |

**Gotchas:**

- `pin` only makes sense for plugins whose `[version].version_capable = true`. The pin entry is ignored at `gen` time for non-version-capable plugins.
- Checksum flags only matter when the plugin's install script actually reads `$CHECKSUM_AMD64` / `$CHECKSUM_ARM64` — typically the `binary` and `archive` methods that verify a downloaded artifact. They are silently inert for `installer` and `apt` methods that delegate integrity to the vendor.
- `--write` requires a discoverable `workspace.toml` from cwd; without `--write`, the command works from anywhere because it only resolves the layered FS for id validation.
- `--write` only edits the inline-table form (`<id> = { pin = "..." }` lines under a single `[plugins.versions]` section). If `workspace.toml` still contains legacy `[plugins.versions.<id>]` subsection blocks, `--write` refuses with a usage error rather than appending a duplicate entry. Convert each legacy block to an inline-table line under `[plugins.versions]` first, or edit `workspace.toml` manually.

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

---

## Removed commands

The following noun groups and subcommands existed in earlier releases and have been **removed**. They are documented here so readers searching for an old command know where they went; full migration notes live in the [CHANGELOG](../CHANGELOG.md).

- **`cocoon config` (entire noun group)** — `get`, `list`, `volumes`, `plugin-get`, `plugin-list`, `plugin-volumes`, `plugins-table`, `validate-workspace`, `validate-plugins`, `has-section`, `list-sidecars`, `dump-devcontainer`, `dump-repositories`, `repositories`, `format-repositories`. These were low-level TOML accessors used by retired bash entry-point scripts. External scripts that scraped `workspace.toml` via `cocoon config` should switch to a dedicated TOML parser (`tomlq`, `taplo`, or a small Go / Python helper).
- **`cocoon plugin add`** — replaced by listing the id under `[plugins].enable` (the embedded catalog is exposed through LayeredFS without a copy step). To customise an embedded plugin, run `cocoon plugin scaffold <new-id>` and adapt logic from there.
- **`cocoon plugin remove`** — replaced by `rm -rf ~/.cocoon/plugins/<id>` (user scope) or `rm -rf <workspace>/.cocoon/plugins/<id>` (project scope).
