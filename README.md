# cocoon

[![Go CI](https://github.com/sukekyo26/cocoon/actions/workflows/go-ci.yml/badge.svg)](https://github.com/sukekyo26/cocoon/actions/workflows/go-ci.yml)
[![E2E](https://github.com/sukekyo26/cocoon/actions/workflows/e2e.yml/badge.svg)](https://github.com/sukekyo26/cocoon/actions/workflows/e2e.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

[日本語版 README](docs/README.ja.md)

> [!WARNING]
> **Project status: Alpha (v0.x).** cocoon is under active development. By using it, please understand and accept that the CLI flags, `workspace.toml` schema, and plugin contracts may change before 1.0, and that breaking changes can land in any release. Read the **BREAKING** lines in the [CHANGELOG](CHANGELOG.md) before upgrading.

## Why cocoon?

**A tool for people who don't want to write `Dockerfile` or `docker-compose.yml` themselves.**

Standing up a Docker-based dev environment by hand means writing all of this for every project:

- `Dockerfile` (60–120 lines) — base image, apt, user creation, per-CLI install steps
- `docker-compose.yml` (30–80 lines) — service / mounts / volumes / env / ports
- `devcontainer.json` (20–40 lines) — VS Code Dev Containers wiring

cocoon turns that into:

```bash
cocoon init   # answer "Which base image? Which shell? Which CLIs?"
cocoon gen    # .devcontainer/ is regenerated from scratch
docker compose -f .devcontainer/docker-compose.yml up -d
```

The only file checked into your repository is a ~30-line `workspace.toml`. The `Dockerfile`, the compose file, and `devcontainer.json` are all regenerated on demand, so configuration "magic" never accumulates in the repo and every change is a deterministic re-run of the generator.

## What you get

`cocoon gen` writes the following under `.devcontainer/`:

| File | Role |
|---|---|
| `Dockerfile` | Multi-stage build with every enabled plugin inlined as `bash` heredocs |
| `docker-compose.yml` | Service + named volumes + ports + optional sidecars |
| `devcontainer.json` | VS Code Reopen-in-Container support (skippable) |
| `docker-entrypoint.sh` | Remaps the container user to the host UID/GID, then restores image-baked binaries, on each start |
| `.env` | `COMPOSE_PROJECT_NAME`, `CONTAINER_SERVICE_NAME`, `USERNAME`, IMAGE / IMAGE_VERSION — host-independent, safe to commit |

The same artifacts power both `docker compose up` from the CLI and VS Code's "Reopen in Container".

## Requirements

- Linux, macOS, or WSL2
- Docker 23+ with BuildKit, and `docker compose` v2.18+
- Go 1.26+ (only when building from source)

## Install

```bash
# Recommended: prebuilt binary with SHA256 verification
curl -fsSL https://raw.githubusercontent.com/sukekyo26/cocoon/main/install.sh | sh

# Alternative: from source (Go 1.26+)
go install github.com/sukekyo26/cocoon/cmd/cocoon@latest
```

## Quick start

```bash
cd ~/projects/my-api
cocoon init                                              # answer the prompts
cocoon gen                                               # generate .devcontainer/
docker compose -f .devcontainer/docker-compose.yml up -d # or VS Code → "Reopen in Container"
```

## What `cocoon init` asks you

1. **Service name** and **username** for the container
2. **Base image** — `ubuntu` / `debian` / `node` / `python` / `golang` / `rust` / `denoland/deno` (DockerHub canonical names)
3. **Image version** — pick a curated suggestion or type any Docker tag directly
4. **Login shell** — `bash`, `zsh`, or `fish`
5. **Alias bundles** — `git`, `ls`, `docker` shortcut sets (multi-select)
6. **Mount range** — cwd only, or its parent (for fat workspaces where sibling repos must be visible)
7. **VS Code Dev Containers** support — emit `devcontainer.json` or skip
8. **Corporate CA auto-bake** — opt in to picking up `.crt` files from `~/.cocoon/certs/` at build time (off by default; see below)
9. **Port forwards** — comma-separated docker-compose short forms (e.g. `3000:3000,5432:5432`); blank to skip and the `[ports]` template stays as a commented-out hint
10. **apt categories** — text-editors, vcs, utilities, build, network, … (multi-select)
11. **Plugins** to enable from the embedded catalog (multi-select, 26 to choose from)

Each answer becomes a self-documenting line in `workspace.toml`. Pass `--yes` together with the value flags (`--service-name`, `--username`, `--image`, `--plugins`, `--certificates`, `--ports`, …) to drive it from CI without a TTY.

## Plugins

26 plugins ship inside the binary via `go:embed`:

`aws-cli`, `aws-sam-cli`, `bun`, `claude-code`, `copilot-cli`, `deno`, `docker-buildx`, `docker-cli`, `github-cli`, `go`, `google-chrome`, `helm`, `kubectl`, `lazygit`, `mise`, `nerd-fonts`, `node`, `opentofu`, `proto`, `rust`, `shellcheck`, `shfmt`, `starship`, `terraform`, `uv`, `zig`

Override or add your own under `~/.cocoon/plugins/<id>/` (user scope) or `<project>/.cocoon/plugins/<id>/` (project scope, checked into the repo) — both layers win over the embedded catalog. See [`docs/plugins.md`](docs/plugins.md) for the authoring guide.

## Corporate CA support

Need a corporate CA inside the container (Zscaler, dev self-signed, etc.)? Run `cocoon init --certificates` (or set `[certificates] enable = true` in `workspace.toml`), then drop the `.crt` files into `~/.cocoon/certs/` on the host. They are picked up automatically at container build time. Cert-free workspaces stay cert-free — no wiring lands in the generated artifacts unless you opt in. See [`[certificates]`](docs/configuration.md#certificates) for the team workflow.

## Persistent personal shellrc

cocoon mounts a named Docker volume at `~/.cocoon/` inside the container so per-user shell tweaks survive container rebuilds. The container's rc file (bash / zsh / fish) automatically sources `~/.cocoon/.shellrc` (or `~/.cocoon/.shellrc.fish` for fish), so editing those files from inside the container persists across `docker compose down && up --build` (only `docker compose down -v` resets them).

## i18n

Every prompt, error message, and inline `workspace.toml` comment renders in English or Japanese. The locale is detected from `WORKSPACE_LANG`, then `LC_ALL` / `LC_MESSAGES` / `LANG` — any value starting with `ja` selects Japanese.

## Documentation

| Topic | English | 日本語 |
|---|---|---|
| Architecture | [architecture.md](docs/architecture.md) | [architecture.ja.md](docs/architecture.ja.md) |
| Configuration (`workspace.toml`) | [configuration.md](docs/configuration.md) | [configuration.ja.md](docs/configuration.ja.md) |
| Commands | [commands.md](docs/commands.md) | [commands.ja.md](docs/commands.ja.md) |
| Plugin authoring (`plugin.toml`, `install.sh`, `install_user.sh`) | [plugins.md](docs/plugins.md) | [plugins.ja.md](docs/plugins.ja.md) |
| Changelog | [CHANGELOG.md](CHANGELOG.md) | [CHANGELOG.ja.md](docs/CHANGELOG.ja.md) |

## Developing

`just ci` is the single pre-push gate (Go fmt/vet/lint/test/vuln/mod-verify + `shellcheck` + `shfmt-check`). Optional pre-commit integration runs the same shell hooks at commit time:

```bash
pip install pre-commit  # or `brew install pre-commit`
pre-commit install      # shellcheck + shfmt fire on each `git commit`
```

`shellcheck` and `shfmt` must be on `$PATH`. macOS: `brew install shellcheck shfmt`. Linux/WSL: `apt-get install shellcheck` + download `shfmt` from <https://github.com/mvdan/sh/releases>.
