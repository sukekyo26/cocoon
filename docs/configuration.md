# Configuration (`workspace.toml`)

> [!WARNING]
> cocoon is in v0.x (alpha). By using it, please understand and accept that the `workspace.toml` schema, the CLI flags, and the plugin contracts may change before 1.0, and that breaking changes can land in any release. See the [CHANGELOG](../CHANGELOG.md) and the README's "Project status" section.

`workspace.toml` is the single source of truth that drives `cocoon gen`. This page documents every section and field accepted by the schema.

`cocoon init` writes a fresh file with sensible defaults plus commented-out templates for opt-in features. For most projects, editing the generated file is enough; this reference covers every accepted field for when you need it.

## Discovery order

`cocoon gen` walks upward from the current directory looking for `workspace.toml`:

1. `<cwd>/workspace.toml`
2. `<cwd>/.cocoon/workspace.toml`
3. parent directory's `workspace.toml`, then `.cocoon/workspace.toml`, and so on
4. stop at a `.git` boundary or `$HOME`

The first match wins. Pass `--workspace <path>` to `cocoon gen` to override discovery.

## Section index

| Section | Required? | Purpose |
|---|---|---|
| `[workspace]` | optional | Generation-wide knobs (mount range, Dev Container toggle) |
| `[container]` | **required** | Image identity (service name, username, OS / version) |
| `[container.resources]` | optional | Compose resource limits |
| `[container.shell]` | optional | Login shell + per-shell rc injection |
| `[container.hosts]` | optional | Extra `/etc/hosts` entries |
| `[container.dns]` | optional | Custom DNS resolvers and search domains |
| `[container.sysctls]` | optional | Kernel parameters |
| `[container.capabilities]` | optional | Linux capabilities to add / drop |
| `[container.security_opt]` | optional | Compose `security_opt` |
| `[[container.skel]]` | optional | Dotfiles seeded into `/etc/skel` |
| `[plugins]` | **required** | Enabled plugins |
| `[plugins.versions]` | optional | Plugin version pins + checksums |
| `[apt]` | optional | Extra apt packages |
| `[apt.mirror]` | optional | Regional apt mirror |
| `[apt.proxy]` | optional | apt-get HTTP / HTTPS proxy |
| `[[apt.sources]]` | optional | Third-party apt repositories |
| `[ports]` | optional | Host port forwarding |
| `[volumes]` | optional | Named Docker volumes |
| `[env]` | optional | Container environment variables |
| `[[mounts]]` | optional | Extra host bind mounts |
| `[home_files]` | optional | Per-file bind mounts under `~/` |
| `[locale]` | optional | Timezone and language |
| `[certificates]` | optional | Opt into TLS auto-bake from `~/.cocoon/certs/` (default off) |
| `[dockerfile]` | optional | Custom Dockerfile fragments |
| `[services.<name>]` | optional | Sidecar services |
| `[devcontainer.*]` | optional | Pass-through fields for `devcontainer.json` |

[`[git]`](#deprecated-sections) and [`[repositories]`](#deprecated-sections) are still accepted by the parser but are deprecated; new projects should not use them.

---

## `[workspace]`

Generation-wide knobs. All fields optional; defaults apply when omitted.

| Field | Type | Default | Description |
|---|---|---|---|
| `mount_root` | string | `"."` | `"."` mounts cwd as the project, `".."` mounts the parent directory so sibling repos are visible. |
| `devcontainer` | bool | `true` | Emit `.devcontainer/devcontainer.json` for VS Code Reopen-in-Container. |

```toml
[workspace]
mount_root = "."
devcontainer = true
```

---

## `[container]` (required)

Image identity. `service_name`, `username`, `image`, `image_version` are all required.

| Field | Type | Validation | Description |
|---|---|---|---|
| `service_name` | string | `^[a-z][a-z0-9_-]*$` | Compose `services:` key. Used as `docker compose exec <service_name>`. |
| `username` | string | `^[a-z_][a-z0-9_-]*$` | Linux user created inside the container. |
| `image` | string | `ubuntu` \| `debian` \| `node` \| `python` \| `golang` \| `rust` \| `denoland/deno` | Base image for `FROM`, written **verbatim** as DockerHub's canonical image name — `golang` (not `go`) and `denoland/deno` (vendor namespace) — so a reader can recreate the FROM line from workspace.toml alone, with no cocoon-side alias resolution. |
| `image_version` | string | plain Docker tag: first character must be alnum or `_`; trailing characters add `.` / `-`; no slash, no colon | Image tag (e.g. `26.04`, `24-bookworm-slim`, `1.26.3-bookworm`, `debian-2.7.14`). The table below is the curated suggestion list cocoon offers in `cocoon init`; **any well-formed tag the upstream registry publishes is accepted**, so you can pin a patch or new minor (e.g. `1.26.4-bookworm` the day it ships) without waiting for a cocoon release. |
| `docker_socket` | bool | — | Mount `/var/run/docker.sock` for docker-in-docker. Pair with the `docker-cli` plugin so the container has a client to use it. Default `false`. |
| `group_add` | `[]string` | each entry: group name (`^[a-z_][a-z0-9_-]*\$?$`) or numeric GID | Supplementary groups the container user joins (Compose `group_add:`). Required because the user runs as a numeric `UID:GID`, so groups baked into the image's `/etc/group` are not applied at runtime. A group **name** must already exist in the image's `/etc/group`; a numeric GID needs no matching entry. |
| `devices` | `[]string` | `HOST:CONTAINER[:rwm]`; both paths absolute | Host devices mapped into the container (Compose `devices:`), e.g. `/dev/dri` for GPU rendering. CDI device syntax is not supported. |
| `ipc` | string | `none` \| `host` \| `private` \| `shareable` \| `service:<name>` \| `container:<name>` | IPC namespace mode (Compose `ipc:`). `host` exposes a large shared-memory segment, often needed by ML workloads. |
| `gpus` | string | `all` | Request GPU access (Compose `gpus:`). Only the literal `all` is currently supported. |

**Suggested image / version pairs** (not exhaustive — any well-formed tag is accepted):

| `image` | `image_version` (suggestions) | FROM line emitted |
|---|---|---|
| `ubuntu` | `26.04`, `24.04`, `22.04` | `FROM ubuntu:<v>` |
| `debian` | `13`, `12` | `FROM debian:<v>` |
| `node` | `26-bookworm-slim`, `24-bookworm-slim`, `22-bookworm-slim` | `FROM node:<v>` |
| `python` | `3.14-slim-bookworm`, `3.13-slim-bookworm`, `3.12-slim-bookworm` | `FROM python:<v>` |
| `golang` | `1.26.3-bookworm`, `1.26-bookworm`, `1.25-bookworm`, `1.24-bookworm` | `FROM golang:<v>` |
| `rust` | `1.95-bookworm`, `1.94-bookworm`, `1.93-bookworm` | `FROM rust:<v>` |
| `denoland/deno` | `debian-2.7.14`, `debian-2.6.10`, `debian-2.5.7` | `FROM denoland/deno:<v>` |

`cocoon init` exposes these as **Tab-completion suggestions** on the version input — press Tab to cycle through them or type any other tag directly. `--image-version <tag>` accepts the same set on the non-interactive path. Validation only enforces the tag format (no slash, no colon); whether the tag actually exists in the upstream registry is left to `docker pull` at build time.

Every supported image is apt-based, so the existing plugin catalog works the same across all of them. `ubuntu` pulls from Ubuntu archives (archive.ubuntu.com); the other six are Debian (bookworm) variants and pull from deb.debian.org. apt-mirror rewriting branches on this in `aptMirrorOriginHosts` (see `internal/generate/dockerfile/dockerfile.go`).

**Image vs plugin (mutually exclusive pairs):** picking a language-runtime image that overlaps with an existing cocoon plugin is rejected at validation time, because the plugin would either overwrite the base layer (go) or shadow it on `$PATH` (rust). Either drop the plugin from `[plugins].enable`, or switch back to `image = "ubuntu" / "debian"` and pin the version via `[plugins.versions]`.

| Picking `image = …` | …and enabling plugin | Outcome |
|---|---|---|
| `golang` | `go` | **rejected** — base already provides Go |
| `rust` | `rust` | **rejected** — base already provides Rust |
| `python` | `uv` | accepted — uv adds a binary, leaves Python alone |
| `node`, `denoland/deno`, `python` | (no matching plugin) | n/a |

```toml
[container]
service_name = "myapp"
username = "dev"
image = "ubuntu"
image_version = "26.04"

# Or pick a language-runtime image and skip the plugin entirely:
# image = "node"
# image_version = "24-bookworm-slim"

# group_add = ["audio", "dialout"]
# devices   = ["/dev/dri:/dev/dri"]
# ipc       = "host"
# gpus      = "all"
```

### `[container.resources]`

Compose resource limits. Omit any field to inherit Docker defaults (unlimited).

| Field | Type | Example |
|---|---|---|
| `shm_size` | string | `"2gb"` |
| `pids_limit` | int | `2048` |
| `cpus` | float | `2.0` |
| `memory` | string | `"4gb"` |

### `[container.shell]`

Login shell plus per-shell rc injection. Aliases / env are appended to the rc file inside the image at build time; bash and zsh share POSIX syntax (`alias k='v'`, `export K=V`), and fish is translated automatically.

| Field | Type | Default | Notes |
|---|---|---|---|
| `default` | string | `"bash"` | One of `bash`, `zsh`, `fish`. |
| `aliases` | inline table | — | Alias keys must match `^[a-zA-Z_][a-zA-Z0-9_-]*$`. |
| `env` | inline table | — | Env keys must match `^[A-Z_][A-Z0-9_]*$` (uppercase). |

```toml
[container.shell]
default = "bash"
aliases = { ll = "ls -lah", gs = "git status" }
env     = { EDITOR = "vim", PAGER = "less -R" }
```

> `EDITOR=vim` / `nano` requires the `text-editors` apt category. `EDITOR=code` works when the container is launched from VS Code Dev Containers (which injects the `code` shim). `PAGER=less` requires the `utilities` apt category.

`[container.shell]` is for project-level settings checked into the repo. For **per-user, container-rebuild-persistent** edits, the rc file also sources `~/.cocoon/.shellrc` (or `~/.cocoon/.shellrc.fish` for fish) on every shell start; that path is backed by a Docker named volume so edits survive `docker compose down && up --build` and are reset only by `docker compose down -v`. See ["Shell injection" in `architecture.md`](architecture.md#shell-injection) for how the rc file is composed at build time and how the in-container `~/.cocoon/` differs from the host's cocoon CLI working area.

### `[container.hosts]`

Extra `/etc/hosts` entries. Keys are hostnames (RFC 1123); values are IPv4 / IPv6 addresses or the literal `"host-gateway"` (resolves to the host machine).

```toml
[container.hosts]
"db.local"     = "host-gateway"
"corp.example" = "10.0.0.42"
```

### `[container.dns]`

Custom DNS configuration.

| Field | Type | Notes |
|---|---|---|
| `servers` | array of strings | Validated as IPv4 / IPv6. |
| `search` | array of strings | Validated as RFC 1123 hostnames. |

### `[container.sysctls]`

Kernel parameters passed verbatim to Compose. Keys match `^[a-z][a-z0-9._-]*$`; values may be int or string.

```toml
[container.sysctls]
"vm.max_map_count" = 262144
```

### `[container.capabilities]`

Linux capability changes. Names are uppercase without the `CAP_` prefix.

```toml
[container.capabilities]
add  = ["SYS_PTRACE"]
drop = ["AUDIT_WRITE"]
```

### `[container.security_opt]`

| Field | Type | Notes |
|---|---|---|
| `seccomp` | string | e.g. `"unconfined"`. Must be non-empty if set. |
| `apparmor` | string | Same. |
| `no_new_privileges` | bool | Block setuid privilege escalation. |

### `[[container.skel]]`

Dotfiles seeded into the new user's home via `/etc/skel`. Repeatable.

| Field | Type | Validation |
|---|---|---|
| `source` | string | Workspace-root-relative; no leading `/`, no `..`, no whitespace. |
| `target` | string | `/etc/skel`-relative; same rules. |

```toml
[[container.skel]]
source = ".cocoon/skel/example.bashrc"
target = ".bashrc"
```

---

## `[plugins]` (required)

| Field | Type | Validation |
|---|---|---|
| `enable` | array of strings | Plugin IDs must match `^[a-z][a-z0-9-]*$`. Duplicates rejected. |

```toml
[plugins]
enable = ["go", "uv", "github-cli"]
```

Run `cocoon plugin list` to see every available plugin (embedded + user / project overlays).

### `[plugins.versions]`

Pin specific versions for `version_capable` plugins. Optional checksums (64 lowercase hex chars) verify install tarballs.

```toml
[plugins.versions]
go = { pin = "1.22.5" }
uv = { pin = "0.5.7", checksum_amd64 = "<sha256>", checksum_arm64 = "<sha256>" }
```

---

## `[apt]`

| Field | Type | Description |
|---|---|---|
| `packages` | array of strings | Extra Debian packages installed on top of cocoon's minimal base + selected init categories. |

### `[apt.mirror]`

| Field | Type | Validation |
|---|---|---|
| `url` | string | Must start with `http://` or `https://`. May not contain whitespace, `'`, `|`, `&`, or `\` (to avoid sed corruption in the generated Dockerfile). |

### `[apt.proxy]`

| Field | Type | Notes |
|---|---|---|
| `http` | string | Validated as `http://` or `https://`. |
| `https` | string | Same. |

### `[[apt.sources]]`

Third-party apt repositories with signed-by GPG keys. The key is fetched from `key_url` and dearmored under `/etc/apt/keyrings/`.

| Field | Type | Notes |
|---|---|---|
| `name` | string | Pattern `^[a-z][a-z0-9-]*$`. Must be unique. |
| `suite` | string | Pattern `^[a-z][a-z0-9._-]*$`. |
| `components` | array of strings | At least one entry; pattern `^[a-z][a-z0-9_-]*$` per entry. |
| `url` | string | `http://` or `https://`. |
| `key_url` | string | `http://` or `https://`. |
| `arch` | string | `amd64` \| `arm64` \| `i386` \| `armhf` \| `ppc64el` \| `s390x` (optional). |

---

## `[ports]`

| Field | Type | Description |
|---|---|---|
| `forward` | array | Either Compose short-form strings or long-form tables. Short form covers `[HOST_IP:][HOST:]CONTAINER[/PROTOCOL]` with numeric ranges (`N-M`) and `tcp`/`udp` protocols. Long form uses the keys `target`, `published`, `host_ip`, `protocol`, `mode`. |

Short-form accepted patterns (all eight are validated by both `cocoon init --ports` and `cocoon gen`):

```toml
[ports]
forward = [
    "3000",                            # container port only
    "3000-3005",                       # container range
    "8000:8000",                       # host:container
    "9090-9091:8080-8081",             # host range:container range
    "49100:22",                        # host:container
    "127.0.0.1:8001:8001",             # IPv4 bind:host:container
    "127.0.0.1:5000-5010:5000-5010",   # IPv4 bind:host range:container range
    "6060:6060/udp",                   # host:container/protocol
]
```

IPv6 binds are also accepted as `[::1]:80:80`. Each numeric component must be in `[1, 65535]`.

---

## `[volumes]`

Named Docker volumes mapped under the container's home. Format: `<volume-name> = <path inside container>`.

```toml
[volumes]
my-data = "/home/${USERNAME}/.my-tool"
```

---

## `[env]`

Environment variables passed into the container. Keys match `^[A-Za-z_][A-Za-z0-9_]*$`. Values may include `${VAR}` references which resolve against the host's environment at `cocoon gen` time.

```toml
[env]
OPENAI_API_KEY = "${OPENAI_API_KEY}"
DEBUG          = "1"
```

---

## `[[mounts]]`

Extra bind mounts from host to container. Repeatable.

| Field | Type | Required | Description |
|---|---|---|---|
| `source` | string | yes | Host path. May include `~`. Must not be empty. |
| `target` | string | yes | Container path. Must be absolute; only `[A-Za-z0-9._/-]` and the `${USERNAME}` placeholder are allowed. Quotes, `:`, `$`, backticks, and whitespace are rejected because the target is interpolated unquoted into the generated Dockerfile and the docker-compose volume spec. |
| `readonly` | bool | no | Default `false`. |

```toml
[[mounts]]
source   = "~/.ssh"
target   = "/home/${USERNAME}/.ssh"
readonly = true
```

---

## `[certificates]`

Opt in to TLS certificate auto-bake from `~/.cocoon/certs/` on the host.

| Field | Type | Default | Notes |
|---|---|---|---|
| `enable` | bool | `false` | When `true`, the generators wire host TLS certificates into the build. |

```toml
[certificates]
enable = true
```

When the section is absent or `enable = false`, the generated `Dockerfile`,
`docker-compose.yml`, and `devcontainer.json` contain **no cert-related wiring
at all** — no `additional_contexts`, no `RUN --mount=type=bind`, no
`initializeCommand`, no `SSL_CERT_FILE` / `CURL_CA_BUNDLE` /
`REQUESTS_CA_BUNDLE` / `NODE_EXTRA_CA_CERTS` ENV exports. Cert-free teams
commit artifacts that have zero corp-CA machinery.

When enabled, drop PEM-formatted `.crt` files into **`~/.cocoon/certs/`** on
the host. They are picked up at container build time and merged into the
trust store automatically.

```sh
mkdir -p ~/.cocoon/certs
cp /path/to/corp-ca.crt ~/.cocoon/certs/
docker compose -f .devcontainer/docker-compose.yml build
```

This directory is a host-side global location, not a `workspace.toml` section.
**Multiple cocoon projects share the same corp CA bundle** — there is no need
to copy the cert into each project.

### Team workflow

The generated `.devcontainer/` is host-independent and meant to be committed:
`.env` carries no `UID`/`GID`/`DOCKER_GID`, and `docker-entrypoint.sh` resolves
the container user's identity from the bind-mounted workspace at container
start. Commit the whole directory and every teammate builds it as-is — no
per-host regeneration, no shared cocoon binary. The corporate-CA wiring below
is the one feature that still needs a host-side step.

Generated `.devcontainer/*` **cert artifacts** depend on whether the workspace
opted in. Opted-in workspaces share the same cert-wired artifacts across the
team; opted-out workspaces share cert-free artifacts.

| Member | cocoon binary | `~/.cocoon/certs/` creation | Required action |
|---|---|---|---|
| Generator (workspace opted in) | yes | created by `cocoon gen` (mode 0700) | `cocoon gen && commit` |
| Generator (workspace opted out) | yes | not created — section is absent | nothing cert-related to do |
| VS Code user (no cert needed) | not needed | created by `initializeCommand` when section enabled | nothing — just open dev container |
| VS Code user (cert needed) | not needed | created by `initializeCommand` when section enabled | `cp corp.crt ~/.cocoon/certs/` then Rebuild Container |
| Plain `docker compose` / CI (workspace opted in) | not needed | **manual `mkdir -p ~/.cocoon/certs`** | one-time mkdir, drop cert if needed, build |

> **Note**: If you build the dev container without VS Code (e.g. `docker compose build` directly, CI), run `mkdir -p ~/.cocoon/certs` once on the host before the first build. VS Code Dev Containers users get this automatically via `initializeCommand`. In CI add a single `mkdir -p ~/.cocoon/certs` to the setup step.

### How it works (when enabled)

- `.devcontainer/docker-compose.yml`: declares `additional_contexts: cocoon_user_certs: ${HOME:?…}/.cocoon/certs` so the host directory is exposed to BuildKit as a named build context (no copy). The `${HOME:?…}` form fails fast if `HOME` is unset on the host.
- `.devcontainer/Dockerfile`: emits a `RUN --mount=type=bind,from=cocoon_user_certs … if find … ; then … update-ca-certificates ; fi` block so any `*.crt` files are installed into the trust store at build time, before the main apt install. This lets the build complete on TLS-intercepting networks like Zscaler. The block also sets `SSL_CERT_FILE` / `CURL_CA_BUNDLE` / `REQUESTS_CA_BUNDLE` / `NODE_EXTRA_CA_CERTS` to the merged system bundle (`/etc/ssl/certs/ca-certificates.crt`) so language runtimes that read those env vars find the new CAs without further configuration.
- `.devcontainer/devcontainer.json`: emits `initializeCommand: "mkdir -p ${HOME:?…}/.cocoon/certs"` so VS Code Dev Containers users have the host directory created before the build, with no cocoon binary required.

### Caveats

- **All** files in `~/.cocoon/certs/` flow through to BuildKit as the build context, not just `.crt` files. Do not store private keys (`.key`) or other sensitive material in this directory.
- After updating a certificate, rebuild the container. BuildKit hashes the bind-mount content into the layer cache key, so the relevant RUN layer rebuilds automatically.

---

## `[home_files]`

Files persisted via per-file bind mounts. Each path is relative to `~/` (no leading `/`, no `~`, no `..`). Per-segment characters are restricted to `[A-Za-z0-9._-]` because the path is interpolated verbatim into the generated `initializeCommand` shell snippet — anything with shell-special meaning (`$`, backticks, `;`, `&`, `|`, `<`, `>`, `*`, `?`, `!`, quotes, backslashes, whitespace) is rejected so a repo-provided `workspace.toml` cannot inject commands into the host shell. Use this to share host-side configs like `~/.gitconfig` into the container.

```toml
[home_files]
files = [".gitconfig", ".claude.json"]
```

**How the host file is prepared.** The generated `docker-compose.yml` references the host source as `${HOME:?HOME must be set on the host}/<rel>`, so the bind path is resolved at `docker compose up` time on whichever host the user actually runs it from (the gen environment and the up environment can differ). To avoid Docker auto-creating the bind source as an empty directory when the file is missing, two safeguards `touch` the file on the host with mode `0600`:

- `cocoon gen` itself touches each entry under `~/` (idempotent — existing files are left untouched, existing directories are surfaced as an error pointing at `rm -rf <path>` for recovery, symlinks are trusted as-is).
- The generated `devcontainer.json` runs the same `touch` via its `initializeCommand`, so VS Code "Reopen in Container" users do not need to invoke `cocoon gen` separately.

**Running `cocoon gen` inside a container.** If `/.dockerenv` is detected, `cocoon gen` emits a warning to stderr and still proceeds — the compose source is `${HOME:?…}` so the file does resolve correctly when `docker compose up` is later invoked from the host, but the host-side `touch` will have run inside the inner container, not on the Docker host. Run `cocoon gen` on the host before `docker compose up` to actually create the host files.

---

## `[locale]`

| Field | Type | Default | Notes |
|---|---|---|---|
| `timezone` | string | host's | IANA timezone (e.g. `"Asia/Tokyo"`). |
| `lang` | string | `"en_US.UTF-8"` | Pattern `^[a-z]{2,3}_[A-Z]{2}\.UTF-8$`. |

---

## `[dockerfile]`

Inject custom Dockerfile fragments at well-defined hook points. The injected content is passed through verbatim; verify your own RUN commands.

| Field | Type | When |
|---|---|---|
| `pre_user_setup` | string | Runs before `useradd`. |
| `post_plugins` | string | Runs after every plugin's `install.sh`. |

```toml
[dockerfile]
pre_user_setup = """
RUN apt-get update && apt-get install -y my-extra-pkg
"""
```

---

## `[services.<name>]`

Sidecar services on the same Compose network (e.g. postgres, redis). Each `<name>` must match `^[a-z][a-z0-9_-]*$` and must not collide with `[container].service_name`.

| Field | Type | Required | Notes |
|---|---|---|---|
| `image` | string | yes | OCI image. Must not be empty. |
| `ports` | array | no | Compose port specifications. |
| `env` | map | no | Keys match `^[A-Za-z_][A-Za-z0-9_]*$`. |
| `volumes` | map | no | Values must be absolute paths. The reserved key `local` is rejected. |
| `mounts` | array of tables | no | Same shape as `[[mounts]]`. |
| `command` | string or array | no | Override the image's CMD. |
| `depends_on` | array of strings | no | Other sidecar names. May not reference itself or the main service. |
| `healthcheck` | table | no | Forwarded to Compose. |
| `restart` | string | no | One of `no`, `always`, `on-failure`, `unless-stopped`. |

```toml
[services.postgres]
image       = "postgres:16-alpine"
environment = { POSTGRES_PASSWORD = "dev" }
ports       = ["5432:5432"]
```

---

## `[devcontainer.*]`

Anything under `[devcontainer.*]` is merged into the generated `devcontainer.json` verbatim. Ignored when `[workspace] devcontainer = false`.

```toml
[devcontainer.customizations.vscode]
extensions = [
    "ms-azuretools.vscode-docker",
    "eamodio.gitlens",
]
```

---

## Deprecated sections

These sections are still accepted by the parser for backward compatibility but may be removed in a future major release.

### `[git]`

Use [`[home_files]`](#home_files) — bind-mounting the host's `~/.gitconfig` keeps git identity in one place.

### `[repositories]`

For multi-repo "fat" workspaces, `git clone` the sibling repos under the parent directory and set `mount_root = ".."`.
