# cocoon

Project-aware container workspace **generator**. Run `cocoon init && cocoon gen` from any project directory to produce a `.devcontainer/` stack tailored to that repository, then start it with `docker compose` (or VS Code's "Reopen in Container") — cocoon never wraps `docker compose` itself.

`cocoon` is the successor of [workspace-docker](https://github.com/sukekyo26/workspace-docker). It replaces the "clone the base repo" workflow with a single binary you install once and use anywhere.

## Status

**Pre-release.** Targeting `v0.1.0` MVP.

See [`docs/design.md`](docs/design.md) and the design plan for the full architecture.

## Highlights

- Single binary generator — emits `.devcontainer/{Dockerfile,docker-compose.yml,devcontainer.json}` and gets out of the way
- IDE-neutral: VS Code reads `.devcontainer/devcontainer.json` automatically; CLI users invoke `docker compose -f .devcontainer/docker-compose.yml ...` directly
- `workspace.toml` per project (root or `.cocoon/workspace.toml` fallback)
- Pluggable language/tool catalog (`go`, `uv`, `rust`, `aws-cli`, ...) shipped inside the binary via `go:embed`; user plugins live in `~/.cocoon/plugins/`
- Mount the current project (`mount_root = "."`) or its parent (`mount_root = ".."`) for sibling-repo workflows — chosen interactively at `cocoon init`
- apt packages picked from grouped checkboxes during `init`, written directly to `[apt] packages` for transparent later editing

## Quick start (planned)

```bash
curl -fsSL https://raw.githubusercontent.com/sukekyo26/cocoon/main/install.sh | sh
cd ~/projects/my-api
cocoon init                                              # writes workspace.toml
cocoon gen                                               # writes .devcontainer/
docker compose -f .devcontainer/docker-compose.yml up -d # or open in VS Code
```

## License

MIT — see [`LICENSE`](LICENSE).
