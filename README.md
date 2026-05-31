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

A ~30-line `workspace.toml` is the source of truth. `cocoon gen` regenerates the whole `.devcontainer/` from it deterministically, so configuration "magic" never accumulates and every change is a re-run of the generator. The generated artifacts are host-independent, so you can either keep `workspace.toml` as the only checked-in file and regenerate per host, or commit `.devcontainer/` once and have every teammate build it as-is.

## What you get

`cocoon gen` writes the following under `.devcontainer/`:

| File | Role |
|---|---|
| `Dockerfile` | Multi-stage build with every enabled plugin inlined as `bash` heredocs |
| `docker-compose.yml` | Service + named volumes + ports + optional sidecars |
| `devcontainer.json` | VS Code Reopen-in-Container support (skippable) |
| `docker-entrypoint.sh` | Remaps the container user to the host UID/GID, then restores image-baked binaries, on each start |
| `manage.sh` | Project-scoped Docker clean / rebuild helper (run on the host) |
| `.env` | `COMPOSE_PROJECT_NAME`, `CONTAINER_SERVICE_NAME`, `USERNAME`, IMAGE / IMAGE_VERSION — host-independent, safe to commit |

The same artifacts power both `docker compose up` from the CLI and VS Code's "Reopen in Container".

### Cleaning up and rebuilding

Docker accumulates unused images, volumes, and build cache that eat disk. `.devcontainer/manage.sh` cleans up or rebuilds **only this project's** resources — scoping is automatic because the script drives `docker compose` against the generated compose file.

```bash
./.devcontainer/manage.sh clean             # containers + networks + volumes + built image
./.devcontainer/manage.sh clean containers  # containers only (networks, volumes, image kept)
./.devcontainer/manage.sh clean image       # containers + networks + built image (volume data kept)
./.devcontainer/manage.sh clean volumes     # containers + networks + volumes (built image kept — fast rebuild)
./.devcontainer/manage.sh rebuild           # rebuild the image with --no-cache and recreate the container
./.devcontainer/manage.sh prune-cache       # prune the GLOBAL Docker build cache (affects every project)
```

Destructive commands ask for confirmation first; pass `-y` to skip it. Build cache cannot be scoped to one project, so `prune-cache` is global by nature — it is deliberately separate from `clean`. Run `./.devcontainer/manage.sh -h` for the full command list.

## Requirements

- Linux, macOS, or WSL2
- Docker 23+ with BuildKit, and `docker compose` v2.18+
- Go 1.26+ (only when building from source)

## Install

```bash
# Default: prebuilt binary with SHA256 verification.
curl -fsSL https://raw.githubusercontent.com/sukekyo26/cocoon/main/install.sh | sh

# Alternative: same binary served from the GitHub Pages mirror (`*.github.io`).
# Use this if your environment can reach `*.github.io` but not
# `raw.githubusercontent.com` / `api.github.com`.
curl -fsSL https://sukekyo26.github.io/cocoon/install.sh | \
  COCOON_PAGES_BASE=https://sukekyo26.github.io/cocoon sh

# From source (Go 1.26+)
go install github.com/sukekyo26/cocoon/cmd/cocoon@latest
```

The Pages mirror publishes every release tag at `https://sukekyo26.github.io/cocoon/v<tag>/` (e.g. `/v0.7.4/`) and re-points `/latest/` + `/VERSION` on each release, so `COCOON_VERSION=0.7.4 sh` (or any other published version) still works to pin a version. If you fork the repo and want to host your own mirror, enable Pages once via **Settings → Pages → Source: GitHub Actions** — the `pages.yml` workflow then deploys on every release.

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
4. **User-local install prefix / PATH** (language images only) — auto-injects `[container.shell.env]` so `npm install -g` / `pip` / `go install` / `cargo install` / `deno install` work without `sudo`; defaults to yes for `node` / `python` / `golang` / `rust` / `denoland/deno`. See [`docs/configuration.md#language-image-path-auto-injection`](docs/configuration.md#language-image-path-auto-injection).
5. **Login shell** — `bash`, `zsh`, or `fish`
6. **Alias bundles** — `git`, `ls`, `docker` shortcut sets (multi-select)
7. **Mount range** — cwd only, or its parent (for fat workspaces where sibling repos must be visible)
8. **Container workdir name** — the parent directory under `/home/<user>/` (defaults to `workspace`; slashes allowed for nested paths like `work/myproject`, useful when a tool such as AWS SAM expects the in-container path to mirror a specific host layout)
9. **VS Code Dev Containers** support — emit `devcontainer.json` or skip
10. **Corporate CA auto-bake** — opt in to picking up `.crt` / `.cer` files from `~/.cocoon/certs/` at build time (off by default; see below)
11. **Port forwards** — comma-separated docker-compose short forms (e.g. `3000:3000,5432:5432`); blank to skip and the `[ports]` template stays as a commented-out hint
12. **apt categories** — text-editors, vcs, utilities, build, network, … (multi-select)
13. **Plugins** to enable from the embedded catalog (multi-select)

Each answer becomes a self-documenting line in `workspace.toml`. Pass `--yes` together with the value flags (`--service-name`, `--username`, `--image`, `--dir`, `--plugins`, `--certificates`, `--ports`, …) to drive it from CI without a TTY.

## Plugins

cocoon ships a catalog of plugins embedded in the binary via `go:embed`. Run `cocoon plugin list` for the full catalog and `cocoon plugin show <id>` for a single plugin's details — the command is the authoritative source, so this README does not duplicate (and drift from) the list.

Override or add your own under `~/.cocoon/plugins/<id>/` (user scope) or `<project>/.cocoon/plugins/<id>/` (project scope, checked into the repo) — both layers win over the embedded catalog. See [`docs/plugins.md`](docs/plugins.md) for the authoring guide.

## Corporate CA support

Need to trust a private CA inside the container (a TLS-intercepting proxy, a dev self-signed cert, etc.)? Run `cocoon init --certificates` (or set `[certificates] enable = true` in `workspace.toml`), then drop the `.crt` / `.cer` files into `~/.cocoon/certs/` on the host. They are picked up automatically at container build time. Cert-free workspaces stay cert-free — no wiring lands in the generated artifacts unless you opt in. See [`[certificates]`](docs/configuration.md#certificates) for the team workflow.

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
| Plugin authoring (`plugin.toml`, `install.<category>.sh`, `install_user.sh`) | [plugins.md](docs/plugins.md) | [plugins.ja.md](docs/plugins.ja.md) |
| Changelog | [CHANGELOG.md](CHANGELOG.md) | [CHANGELOG.ja.md](docs/CHANGELOG.ja.md) |

## Developing

`just ci` is the single pre-push gate (Go fmt/vet/lint/test/vuln/mod-verify + `shellcheck` + `shfmt-check`). Optional pre-commit integration runs the fast subset on each `git commit` — `shellcheck`, `shfmt`, `golangci-lint fmt --diff`, `go vet`, `golangci-lint run`, `go mod verify`, and a `go mod tidy` drift check. Slow gates (test / coverage / govulncheck / cross-compile) stay in `just ci`.

```bash
pip install pre-commit  # or `brew install pre-commit`
pre-commit install      # hooks fire on each `git commit`
```

Required on `$PATH`: `shellcheck`, `shfmt`, `go`, `golangci-lint`. macOS: `brew install shellcheck shfmt golangci-lint`. Linux/WSL: `apt-get install shellcheck`, download `shfmt` from <https://github.com/mvdan/sh/releases>, install `golangci-lint` per <https://golangci-lint.run/welcome/install/>.
