# cocoon

Project-aware container workspace generator. Run `cocoon init && cocoon up` from any project directory to spin up a Dev Container or plain Docker Compose stack tailored to that repository.

`cocoon` is the successor of [workspace-docker](https://github.com/sukekyo26/workspace-docker). It replaces the "clone the base repo" workflow with a single binary you install once and use anywhere.

## Status

**Pre-release.** Targeting `v0.1.0` MVP.

See [`docs/design.md`](docs/design.md) and the design plan for the full architecture.

## Highlights

- Single binary, installed via `curl | sh` or `go install`
- `workspace.toml` per project (root or `.cocoon/workspace.toml` fallback)
- Generates `.devcontainer/{Dockerfile,docker-compose.yml,devcontainer.json}` — IDE-neutral
- Pluggable language/tool catalog (`go`, `uv`, `rust`, `aws-cli`, ...) shipped inside the binary via `go:embed`; user plugins live in `~/.cocoon/plugins/`
- Mount the current project (`mount_root = "."`) or its parent (`mount_root = ".."`) for sibling-repo workflows — chosen interactively at `cocoon init`
- apt packages picked from grouped checkboxes during `init`, written directly to `[apt] packages` for transparent later editing

## Quick start (planned)

```bash
curl -fsSL https://raw.githubusercontent.com/sukekyo26/cocoon/main/install.sh | sh
cd ~/projects/my-api
cocoon init
cocoon up
cocoon exec
```

## License

MIT — see [`LICENSE`](LICENSE).
