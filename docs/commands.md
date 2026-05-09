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
| `cocoon plugin pin <id> <ref>` | Print a `[plugins.versions.<id>]` block |
| `cocoon plugin scaffold <id>` | Create a new `<id>/` directory from a template |
| `cocoon config get <file> <field>` | Print scalar from `workspace.toml` |
| `cocoon config list <file> <field>` | Print array from `workspace.toml` |
| `cocoon config volumes <file>` | Print `[volumes]` entries |
| `cocoon config plugin-get <dir-or-file> <field>` | Print scalar from `plugin.toml` |
| `cocoon config plugin-list <dir-or-file> <field>` | Print array from `plugin.toml` |
| `cocoon config plugin-volumes <dir-or-file>` | Print `plugin.install.volumes` |
| `cocoon config plugins-table <dir>` | Print one plugin row per line |
| `cocoon config validate-workspace <file> [plugins]` | Validate `workspace.toml` |
| `cocoon config validate-plugins <dir>` | Validate every plugin under `<dir>` |
| `cocoon config has-section <file> <section>` | Print true / false |
| `cocoon config list-sidecars <file>` | Print one `[services.<name>]` key per line |
| `cocoon config dump-devcontainer <file>` | Dump `[devcontainer]` as TOML |
| `cocoon config dump-repositories <file>` | Dump `[repositories]` as TOML |
| `cocoon config repositories <file>` | Emit `[repositories].clone` as JSON |
| `cocoon config format-repositories <file\|->` | Format JSON entries as TOML |
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

Read `workspace.toml`, materialize plugins, and write `.devcontainer/`.

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

---

## `cocoon plugin`

Manage cocoon plugins. The `LayeredFS` (project > user > embedded) determines which definition wins for a given plugin ID.

### `cocoon plugin list`

| Flag | Description |
|---|---|
| `--source <embedded\|user\|project>` | Filter to plugins resolved from a single layer. |

### `cocoon plugin show <id>`

Print the resolved `plugin.toml` plus its source layer.

### `cocoon plugin add <id>`

Copy an embedded plugin into a writable overlay so it can be edited.

| Flag | Description |
|---|---|
| `--scope <user\|project>` | Where to copy. Default `user` (`~/.cocoon/plugins/<id>/`). |
| `--force` | Overwrite an existing overlay copy. |

### `cocoon plugin remove <id>`

Delete an overlay copy. The embedded version is never affected.

| Flag | Description |
|---|---|
| `--scope <user\|project>` | Which overlay to delete (required). |

### `cocoon plugin pin <id> <ref>`

Print a `[plugins.versions.<id>]` snippet for `workspace.toml`. Stdout-only — does not edit the file (preserves any existing comments).

| Flag | Description |
|---|---|
| `--amd64-checksum <sha256>` | SHA256 of the amd64 artifact. |
| `--arm64-checksum <sha256>` | SHA256 of the arm64 artifact. |

### `cocoon plugin scaffold <id>`

Create a new `<id>/` directory from a template (`curl-pipe` / `tarball` / `generic`).

| Flag | Description |
|---|---|
| `--plugins-dir <path>` | Output directory. Default `plugins`. |
| `--name <name>` | Display name (e.g. `"GitHub CLI"`). |
| `--description <text>` | Short description. |
| `--default` | Mark plugin enabled by default. |
| `--requires-root` | `install.sh` runs as root. |
| `--version-capable` | Generate `$PIN` / `$CHECKSUM_*` boilerplate. |
| `--template <kind>` | `curl-pipe` \| `tarball` \| `generic`. |
| `--with-install-user` | Also generate `install_user.sh`. |
| `--non-interactive` | Skip prompts; require all fields above. |
| `--force` | Overwrite `<id>/` if it already exists. |

---

## `cocoon config`

Parse and validate `workspace.toml` / `plugin.toml`. These subcommands exist for legacy bash-bridge integration; future versions may trim them.

| Subcommand | Purpose |
|---|---|
| `get <file> <field>` | Print a scalar from `workspace.toml`. |
| `list <file> <field>` | Print an array from `workspace.toml`, one entry per line. |
| `volumes <file>` | Print `[volumes]` entries as `name<TAB>path`. |
| `plugin-get <dir-or-file> <field>` | Print a scalar from `plugin.toml`. |
| `plugin-list <dir-or-file> <field>` | Print an array from `plugin.toml`. |
| `plugin-volumes <dir-or-file>` | Print `plugin.install.volumes` as `name<TAB>path`. |
| `plugins-table <dir>` | Print one plugin row per line: `id<TAB>name<TAB>default<TAB>description`. |
| `validate-workspace <file> [plugins]` | Validate `workspace.toml`; optional plugin dir for cross-checks. |
| `validate-plugins <dir>` | Validate every `plugin.toml` under `<dir>`. |
| `has-section <file> <section>` | Print `true` or `false`. |
| `list-sidecars <file>` | Print one `[services.<name>]` key per line. |
| `dump-devcontainer <file>` | Dump `[devcontainer]` as TOML. |
| `dump-repositories <file>` | Dump `[repositories]` as TOML. |
| `repositories <file>` | Emit `[repositories].clone` as JSON. |
| `format-repositories <file\|->` | Format JSON entries as TOML (`-` reads stdin). |

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
