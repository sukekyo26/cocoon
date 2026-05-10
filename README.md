# cocoon

[![Go CI](https://github.com/sukekyo26/cocoon/actions/workflows/go-ci.yml/badge.svg)](https://github.com/sukekyo26/cocoon/actions/workflows/go-ci.yml)
[![E2E](https://github.com/sukekyo26/cocoon/actions/workflows/e2e.yml/badge.svg)](https://github.com/sukekyo26/cocoon/actions/workflows/e2e.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

[日本語版 README](docs/README.ja.md)

Project-aware container workspace **generator**. Run `cocoon init && cocoon gen` from any project directory to produce a `.devcontainer/` stack tailored to that repository, then start it with `docker compose` or VS Code's "Reopen in Container".

## Highlights

- **Single static binary** — every plugin ships inside the binary via `go:embed`; install once and use anywhere
- **Generates** `.devcontainer/{Dockerfile, docker-compose.yml, devcontainer.json, docker-entrypoint.sh, .env}` from a single `workspace.toml`
- VS Code Dev Containers and raw `docker compose` consume the same output
- **Layered plugin overrides** — `<project>/.cocoon/plugins/` > `~/.cocoon/plugins/` > embedded catalog (20 plugins out of the box)
- **Interactive `cocoon init`** — picks mount range, login shell, apt categories, plugins, alias bundles, and writes a self-documenting `workspace.toml`
- **Persistent personal shellrc** — `~/.cocoon/.shellrc` (and `~/.cocoon/.shellrc.fish`) inside the container is backed by a named volume, so per-user aliases / PATH tweaks survive `docker compose down && up --build`
- **i18n** — every prompt, error message, and inline `workspace.toml` comment renders in English or Japanese based on `$LANG`

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
cocoon init                                              # generate workspace.toml interactively
cocoon gen                                               # generate .devcontainer/
docker compose -f .devcontainer/docker-compose.yml up -d # or open in VS Code → "Reopen in Container"
```

> **Need a corporate CA inside the container** (Zscaler, dev self-signed, etc.)? Run `cocoon init --certificates` (or set `[certificates] enable = true` in `workspace.toml`), then drop the `.crt` files into `~/.cocoon/certs/` on the host. They are picked up automatically at container build time. Cert-free workspaces stay cert-free — no wiring lands in the generated artifacts unless you opt in. See [`[certificates]`](docs/configuration.md#certificates) for the team workflow.

## Documentation

| Topic | English | 日本語 |
|---|---|---|
| Architecture | [architecture.md](docs/architecture.md) | [architecture.ja.md](docs/architecture.ja.md) |
| Configuration (`workspace.toml`) | [configuration.md](docs/configuration.md) | [configuration.ja.md](docs/configuration.ja.md) |
| Commands | [commands.md](docs/commands.md) | [commands.ja.md](docs/commands.ja.md) |
| Changelog | [CHANGELOG.md](CHANGELOG.md) | [CHANGELOG.ja.md](docs/CHANGELOG.ja.md) |

## Developing

`just ci` is the single pre-push gate (Go fmt/vet/lint/test/vuln/mod-verify + `shellcheck` + `shfmt-check`). Optional pre-commit integration runs the same shell hooks at commit time:

```bash
pip install pre-commit  # or `brew install pre-commit`
pre-commit install      # shellcheck + shfmt fire on each `git commit`
```

`shellcheck` and `shfmt` must be on `$PATH`. macOS: `brew install shellcheck shfmt`. Linux/WSL: `apt-get install shellcheck` + download `shfmt` from <https://github.com/mvdan/sh/releases>.

## License

MIT — see [LICENSE](LICENSE).
