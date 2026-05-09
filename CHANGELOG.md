# Changelog

All notable changes to cocoon are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.0.0/) and the project
adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Mount a named Docker volume `cocoon` at `/home/<user>/.cocoon` inside the dev container so per-user shell tweaks survive container rebuilds. The container's rc file (bash / zsh / fish) sources `~/.cocoon/.shellrc` (or `~/.cocoon/.shellrc.fish` for fish) automatically; edit those files from inside the container and they persist across `docker compose down && up --build` (only `down -v` resets them).
- Add `cocoon init --plugin-versions=<id>=<ref>,...` so a single command emits both `[plugins] enable` and `[plugins.versions]` blocks. Each `<id>` must already appear in `--plugins`, must be `version_capable`, and may not repeat. Replaces the prior workflow of pasting `cocoon plugin pin` output by hand.
- Add `cocoon plugin pin --write` to insert (or replace) a `[plugins.versions.<id>]` block directly in `workspace.toml`. The line-based mutator preserves comments and blank lines outside the target block; the existing stdout-only behavior remains the default. If `workspace.toml` uses the equivalent inline-table form (`[plugins.versions]` + `<id> = { pin = "..." }`), `--write` refuses with a usage error instead of appending a duplicate block.

### Changed

- **BREAKING**: `cocoon plugin scaffold` now defaults `--plugins-dir` to `<workspace>/.cocoon/plugins` (auto-discovered from `workspace.toml`) instead of `./plugins`. Without `--plugins-dir` and outside a cocoon project, scaffold refuses with an actionable error rather than writing to `./plugins/<id>/`. Pass `--plugins-dir <path>` explicitly to override.

### Fixed

- Catalog `claude-code` and `copilot-cli` plugins now export `~/.local/bin` to `PATH` via `[install.env]` so the installed CLIs are reachable in interactive shells without depending on another plugin (e.g. `uv`) to set the same PATH.
- Catalog `go` plugin now installs `build-essential` (gcc / make) so cgo builds and `go install` of native-dependent tools work out of the box.

### Docs

- Expand `docs/commands.md` plugin section with purpose, runnable example, and gotchas for each subcommand. Document the layered FS (project > user > embedded) and the typical `add → edit → enable → gen` workflow at the top of the section.

### Removed

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
