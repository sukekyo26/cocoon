# Configuration (`workspace.toml`)

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

Image identity. `service_name`, `username`, `os`, `os_version` are all required.

| Field | Type | Validation | Description |
|---|---|---|---|
| `service_name` | string | `^[a-z][a-z0-9_-]*$` | Compose `services:` key. Used as `docker compose exec <service_name>`. |
| `username` | string | `^[a-z_][a-z0-9_-]*$` | Linux user created inside the container. |
| `os` | string | `ubuntu` \| `debian` | Base distribution for `FROM`. |
| `os_version` | string | matches the chosen `os` (see below) | Distribution version (e.g. `26.04`, `13`). |
| `docker_socket` | bool | — | Mount `/var/run/docker.sock` for docker-in-docker. Default `false`. |

**Supported OS / version pairs:**

| `os` | `os_version` |
|---|---|
| `ubuntu` | `26.04`, `24.04`, `22.04` |
| `debian` | `13`, `12` |

```toml
[container]
service_name = "myapp"
username = "dev"
os = "ubuntu"
os_version = "26.04"
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

`[container.shell]` is for project-level settings checked into the repo. For **per-user, container-rebuild-persistent** edits, the rc file additionally sources `~/.cocoon/.shellrc` (or `~/.cocoon/.shellrc.fish` for fish) on every shell start. That path is backed by a Docker named volume (`cocoon`), so user edits survive `docker compose down && up --build`. Edit it from inside the container:

```bash
docker compose -f .devcontainer/docker-compose.yml exec dev bash -lc 'vim ~/.cocoon/.shellrc'
```

(Pick whichever editor your image has installed; `bash -lc` ensures the in-container `EDITOR` / `PATH` resolve, which would not happen if the editor were named via `"$EDITOR"` at the host shell.)

The volume is reset only by `docker compose down -v`. The path lives **inside the container only** — host `~/.cocoon/` is unrelated (it is cocoon CLI's local working area for plugin overlays, build-context cache, certificates).

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
| `forward` | array | Either Compose short-form strings (`"3000:3000"`, `"127.0.0.1:5432:5432/tcp"`, ranges `"3000-3005:3000-3005"`) or long-form tables with `target`, `published`, `host_ip`, `protocol`, `mode`. |

```toml
[ports]
forward = ["3000:3000", "5432:5432"]
```

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
| `target` | string | yes | Container path. Must be absolute. |
| `readonly` | bool | no | Default `false`. |

```toml
[[mounts]]
source   = "~/.ssh"
target   = "/home/${USERNAME}/.ssh"
readonly = true
```

---

## `[home_files]`

Files persisted via per-file bind mounts. Each path is relative to `~/` (no leading `/`, no `~`, no `..`). Use this to share host-side configs like `~/.gitconfig` into the container.

```toml
[home_files]
files = [".gitconfig", ".claude.json"]
```

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
