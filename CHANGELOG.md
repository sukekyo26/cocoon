# Changelog

All notable changes to cocoon are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.0.0/) and the project
adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Mount a named Docker volume `cocoon` at `/home/<user>/.cocoon` inside the dev container so per-user shell tweaks survive container rebuilds. The container's rc file (bash / zsh / fish) sources `~/.cocoon/.shellrc` (or `~/.cocoon/.shellrc.fish` for fish) automatically; edit those files from inside the container and they persist across `docker compose down && up --build` (only `down -v` resets them).
- Add `cocoon init --plugin-versions=<id>=<ref>,...` so a single command emits both `[plugins] enable` and `[plugins.versions]` blocks. Each `<id>` must already appear in `--plugins`, must be `version_capable`, and may not repeat. Replaces the prior workflow of pasting `cocoon plugin pin` output by hand.
- Add `cocoon plugin pin --write` to insert (or replace) a `[plugins.versions.<id>]` block directly in `workspace.toml`. The line-based mutator preserves comments and blank lines outside the target block; the existing stdout-only behavior remains the default. If `workspace.toml` has any per-id key assignment under `[plugins.versions]` (e.g. `<id> = "..."` or `<id> = { ... }`), `--write` refuses with a usage error instead of appending a duplicate block.
- New `[certificates]` section in `workspace.toml` toggles host TLS certificate auto-bake from `~/.cocoon/certs/*.crt`. **Default `enable = false`** — teams that never deal with corporate CAs commit `Dockerfile` / `docker-compose.yml` / `devcontainer.json` artifacts that contain no cert-related wiring at all (no `additional_contexts`, no `RUN --mount=type=bind`, no `initializeCommand`, no `SSL_CERT_FILE` ENV exports). When enabled, the compose generator declares `additional_contexts: cocoon_user_certs: ${HOME:?…}/.cocoon/certs` and the Dockerfile uses `RUN --mount=type=bind,from=cocoon_user_certs …` to install any certificates into the trust store before other apt operations, which is required for builds on networks that intercept TLS (e.g., Zscaler). The RUN body is a shell-side conditional, so the same enabled Dockerfile builds successfully whether the developer has user certs in `~/.cocoon/certs/` or not.
- `cocoon init --certificates` / `--no-certificates` flags + interactive prompt drive the new section. When enabled, the generated `workspace.toml` carries `[certificates] enable = true` and the live wiring lands in `.devcontainer/*`; when disabled the section is omitted and a commented template documents how to opt in later.
- Generated `.devcontainer/devcontainer.json` carries `initializeCommand: "mkdir -p ${HOME:?…}/.cocoon/certs"` only when `[certificates]` is enabled, so VS Code Dev Containers users in opted-in workspaces automatically have the host directory created before the build, without needing the cocoon binary themselves. Plain `docker compose build` users (CI etc.) bypass that hook and need to run `mkdir -p ~/.cocoon/certs` once on the host before the first build.
- `cocoon gen` creates `~/.cocoon/certs/` on the host (mode 0700) when missing and prints a short notice describing where to drop corporate / private CA `.crt` files — but only when the workspace has `[certificates] enable = true`. Cert-free workspaces cause no host-side side effect and no notice in stdout.
- `${HOME:?…}` parameter expansion in the generated compose `additional_contexts` and devcontainer `initializeCommand` makes both fail fast with a clear error when `HOME` is unset on the host, instead of silently collapsing the path to `/.cocoon/certs`.

### Changed

- `cocoon plugin scaffold` now shows a multi-paragraph description on the interactive `Add install_user.sh?` prompt that explains the root + user split, when to opt in vs skip, and points to starship as the canonical example. Both EN and JA prompt catalogs are updated.
- `cocoon gen` no longer materializes the plugin catalog into `~/.cocoon/cache/build-context/`. Each enabled plugin's `install.sh` (and `install_user.sh` when present) is now inlined directly into the generated `.devcontainer/Dockerfile` via a single-quoted bash heredoc, and the `additional_contexts: plugins:` entry is dropped from `docker-compose.yml`. The build needs no external Docker context beyond the project tree, which means `cocoon gen` works the same way from inside the dev container as from the host (the previous flow assumed the build always ran on the host because the cache lived under the host `$HOME`). Existing `~/.cocoon/cache/build-context/` directories are no longer recreated and can be removed manually with `rm -rf ~/.cocoon/cache/build-context`.
- **BREAKING**: `cocoon plugin scaffold` now defaults `--plugins-dir` to `<workspace>/.cocoon/plugins` (auto-discovered from `workspace.toml`) instead of `./plugins`. Without `--plugins-dir` and outside a cocoon project, scaffold refuses with an actionable error rather than writing to `./plugins/<id>/`. Pass `--plugins-dir <path>` explicitly to override.
- **BREAKING**: TLS certificate auto-bake source path moved from `<project>/certs/*.crt` to `~/.cocoon/certs/*.crt`, and the feature is now opt-in via `[certificates] enable = true` in `workspace.toml` (default off). Migration: drop `[certificates]\nenable = true` into `workspace.toml`, run `mkdir -p ~/.cocoon/certs && mv ./certs/*.crt ~/.cocoon/certs/`, then re-run `cocoon gen`. Project-level `certs/` directories are no longer scanned, and cert wiring is no longer emitted unless the workspace opts in.

### Fixed

- `[install].build_args` is now honoured symmetrically across `install.sh` and `install_user.sh`. Previously, the generator emitted the matching `ARG <name>` Dockerfile line only next to the `install.sh` RUN, so a plugin shipped with only `install_user.sh` (no `install.sh`) plus `build_args` ended up with `${<name>}` resolving to the empty string at build time. The generator now emits one `ARG <name>` line per plugin, attached to whichever hook runs first, so both hooks see the build-arg value via the per-RUN env prefix. ARGs are stage-scoped, so a single declaration is sufficient and no redundant duplicate is produced when both hooks exist.
- `[workspace] mount_root` resolution in the generated `docker-compose.yml`. Compose resolves bind-mount relative paths against the compose file's directory (`.devcontainer/`), so the previous output was one level too shallow: `mount_root = ".."` mounted the project root instead of its parent (sibling repos were not visible) and `mount_root = "."` mounted `.devcontainer/` itself instead of the project. Both cases now emit one extra `..` so they resolve to the user-facing target.
- Plugin authors that ship an `[install.env]` table without an `install.sh` (env-only plugins) no longer have the `ENV` directives silently dropped from the generated Dockerfile; the env block is emitted as its own snippet so the variables still land in the image.
- Catalog `claude-code` and `copilot-cli` plugins now export `~/.local/bin` to `PATH` via `[install.env]` so the installed CLIs are reachable in interactive shells without depending on another plugin (e.g. `uv`) to set the same PATH.
- Catalog `go` plugin now installs `build-essential` (gcc / make) so cgo builds and `go install` of native-dependent tools work out of the box.

### Docs

- Add `docs/plugins.md` (English) and `docs/plugins.ja.md` (Japanese) as the single source of truth for plugin authoring. They document the 3-layer LayeredFS, the full `plugin.toml` schema, when to write `install.sh` vs `install_user.sh` (with a decision matrix and concrete examples covering starship plus hypothetical fzf / oh-my-zsh / miniconda cases), the env vars passed to install scripts, the version-pin contract, a catalog tour, and troubleshooting recipes. The plugin-authoring SKILL.md is slimmed to agent workflow only and delegates spec to these docs.
- Expand `docs/commands.md` plugin section with purpose, runnable example, and gotchas for each subcommand. Document the layered FS (project > user > embedded) at the top, link to `docs/plugins.md` for authoring details, and replace the prior `add → edit → enable → gen` workflow with "list the id under `[plugins].enable`; cp -r or scaffold to customise".

### Removed

- **BREAKING**: drop `cocoon plugin remove` subcommand. The implementation was a thin wrapper around `os.RemoveAll` and is fully equivalent to `rm -rf <overlay>`. Migration: replace `cocoon plugin remove <id> --scope user` with `rm -rf ~/.cocoon/plugins/<id>` (or `<workspace>/.cocoon/plugins/<id>` for project scope).
- **BREAKING**: drop `cocoon plugin add` subcommand. The implementation copied an embedded plugin into a writable overlay; the name "add" misleadingly read as "enable" (LayeredFS already exposes the embedded catalog through `[plugins].enable` alone). Migration: to enable an embedded plugin, just list its id in `[plugins].enable` in `workspace.toml`. To customise an embedded plugin, the supported workflow is `cocoon plugin scaffold <new-id>` and adapting the logic. With a cocoon source checkout (`git clone` of the repo, or an unpacked GitHub Release source tarball), `cp -r internal/plugin/catalog/<id> ~/.cocoon/plugins/<id>/` is a shortcut; single-binary installs do not include the embedded source on disk. The `plugin.Materialize` helper used by `add` is also removed.
- **BREAKING**: drop the `cocoon config` noun group (`get`, `list`, `volumes`, `plugin-get`, `plugin-list`, `plugin-volumes`, `plugins-table`, `validate-workspace`, `validate-plugins`, `has-section`, `list-sidecars`, `dump-devcontainer`, `dump-repositories`, `repositories`, `format-repositories`). The group existed to feed bash entry-point scripts that were retired in v0.1.0; nothing inside cocoon depends on it. External scripts that scraped `workspace.toml` via `cocoon config` should switch to a dedicated TOML parser (e.g., `tomlq`, `taplo`, or a small Go / Python helper).

## [0.1.0] - 2026-05-09

### Added

- Add `cocoon init` for interactive `workspace.toml` generation with prompts for service name, username, OS, OS version, login shell, mount range, devcontainer toggle, alias bundles, apt categories, and plugins.
- Add non-interactive flags (`--yes`, `--service-name`, `--username`, `--os`, `--os-version`, `--shell`, `--mount-root`, `--devcontainer`, `--no-devcontainer`, `--apt-categories`, `--plugins`, `--alias-bundles`, `--force`) so CI and scripts can drive `cocoon init` without TTY interaction.
- Add localized inline comments and 20 commented-out section templates to the generated `workspace.toml` for in-file feature discovery.
- Add `cocoon gen` to emit `.devcontainer/{Dockerfile, docker-compose.yml, docker-entrypoint.sh, .env, devcontainer.json}` from `workspace.toml`.
- Add `cocoon plugin` noun group with `list`, `show`, `add`, `remove`, `pin`, and `scaffold` subcommands backed by an embedded 20-plugin catalog and `LayeredFS` (project > user > embedded) overrides.
- Add `cocoon config` noun group exposing `get`, `list`, `volumes`, `plugin-get`, `plugin-list`, `plugin-volumes`, `plugins-table`, `validate-workspace`, `validate-plugins`, `has-section`, `list-sidecars`, `dump-devcontainer`, `dump-repositories`, `repositories`, and `format-repositories`.
- Add `cocoon self-update` with GitHub release download, SHA256 verification, and atomic-rename swap.
- Add `cocoon version`.
- Add 10 apt categories (`text-editors`, `vcs`, `utilities`, `compression`, `build`, `search`, `network`, `monitoring`, `python3`, `json-yaml`) selectable in `cocoon init`.
- Add 3 alias bundles (`git`, `ls`, `docker`) selectable in `cocoon init`, merged into `[container.shell] aliases`.
- Add Dockerfile-heredoc shell rc injection so `[container.shell] env` and `aliases` flow directly into `~/.bashrc` / `~/.zshrc` / `~/.config/fish/config.fish` at image build time.
- Add `COMPOSE_PROJECT_NAME` derivation from the project directory basename so docker compose namespacing matches the host directory.
- Add i18n catalog (English / Japanese) covering every CLI prompt, error message, and inline `workspace.toml` comment, switched via `WORKSPACE_LANG` / `LC_ALL` / `LC_MESSAGES` / `LANG`.

[Unreleased]: https://github.com/sukekyo26/cocoon/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/sukekyo26/cocoon/releases/tag/v0.1.0
