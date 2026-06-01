#!/usr/bin/env bash
# Real Docker round-trip for the cocoon generator. Run from a CI workflow
# (.github/workflows/e2e.yml or scheduled-e2e.yml) after `just build` has
# produced bin/cocoon. Builds a fresh project, generates .devcontainer/,
# `docker buildx bake`s the image with GHA cache, and execs into the
# running container so generation / install-script drift breaks CI before
# it breaks users.
#
# Required env:
#   COCOON          absolute path to the cocoon binary
#   PRESET          one of: minimal | amd64-full | arm64-full
#   IMAGE           base image id (e.g. ubuntu, debian, denoland/deno)
#   IMAGE_VERSION   image tag (e.g. 26.04, debian-2.7.14)
#
# The two presets minimal vs full have different cost / coverage:
#   - minimal (push/PR e2e.yml)  — 0 plugins, runs across every supported
#     base image so generator image-family branching (apt-mirror, OS
#     family) is exercised on every PR.
#   - amd64-full / arm64-full (scheduled-e2e.yml) — every catalog plugin
#     enabled, pinned to the versions in pin_entries below (un-pinned
#     plugins float to LATEST). Catches plugin-vs-base-image and
#     plugin-vs-plugin breakage; bumping a pin_entries entry surfaces here
#     as a deliberate diff.
#     The enabled set is derived from internal/plugin/catalog/ at runtime
#     so a new plugin auto-enrolls. arm64-full subtracts the arm64-unsafe
#     plugins in e2e/arm64-exclude.txt (install.sh hard-codes amd64 or
#     fails fast on arm64).
#   - PIN_MODE=latest (scheduled-e2e.yml weekly) reruns the full presets with
#     NO pins so every plugin floats to its upstream LATEST — the drift canary
#     that catches upstream packaging changes (e.g. a removed checksum
#     manifest) a release before users hit them. Default PIN_MODE=pinned keeps
#     the reproducible baseline.
#
# Local WSL2 cannot run this (gcc missing for `-race`); GHA-hosted
# ubuntu-latest / ubuntu-24.04-arm ship docker + buildx out of the box.

set -euxo pipefail

: "${COCOON:?COCOON unset (path to cocoon binary)}"
: "${PRESET:?PRESET unset (minimal | amd64-full | arm64-full)}"
: "${IMAGE:?IMAGE unset (base image id)}"
: "${IMAGE_VERSION:?IMAGE_VERSION unset}"

# PIN_MODE selects whether the full presets pin versions (reproducible
# baseline) or float every plugin to LATEST (weekly drift canary). The
# minimal preset ignores it (0 plugins). Default keeps the baseline.
PIN_MODE="${PIN_MODE:-pinned}"
case "$PIN_MODE" in
  pinned | latest) ;;
  *)
    echo "unknown PIN_MODE: $PIN_MODE (pinned | latest)" >&2
    exit 1
    ;;
esac

# Resolve the script and catalog dirs to absolute paths up front, before
# the `cd e2e/test-project` below, so they survive the cwd change. $0 may
# be relative (e.g. `bash e2e/docker-roundtrip.sh` from the repo root), so
# resolve it while still at the original cwd or the `cd` here would fail.
script_dir="$(cd "$(dirname "$0")" && pwd)"
catalog_dir="$(cd "$script_dir/../internal/plugin/catalog" && pwd)"

# Wipe any leftover project dir before init so reruns (local debugging,
# self-hosted runner with cached workspace) are idempotent — `cocoon
# init` refuses to overwrite an existing workspace.toml without --force.
rm -rf e2e/test-project
mkdir -p e2e/test-project
cd e2e/test-project
git init -q

# Derive the enabled plugin set from the embedded catalog so adding a
# plugin under internal/plugin/catalog/ auto-enrolls it here (no
# hand-kept list to drift).

all_plugins=()
for d in "$catalog_dir"/*/; do
  all_plugins+=("$(basename "$d")")
done

# arm64-unsafe plugins, kept in a shared data file that
# internal/plugin/e2e_arm64_exclude_test.go validates against the catalog.
# Trim each line (plain `read`, default IFS) so this parser matches that
# Go guard, which TrimSpace's before comparing; a trailing space must not
# turn an excluded id into a silent no-op.
arm64_exclude=()
while read -r line || [ -n "$line" ]; do
  case "$line" in
    '' | '#'*) continue ;;
  esac
  arm64_exclude+=("$line")
done <"$script_dir/arm64-exclude.txt"

# Every apt category, kept in a shared data file that
# internal/aptcategories/e2e_categories_test.go validates against
# internal/aptcategories/aptcategories.go. Selecting all of them makes the
# build actually `apt-get install` every catalog package, so a name that is
# missing from the target apt repos breaks CI here instead of for a user.
# Same trimming reader as arm64_exclude above (matches the Go guard).
apt_categories=()
while read -r line || [ -n "$line" ]; do
  case "$line" in
    '' | '#'*) continue ;;
  esac
  apt_categories+=("$line")
done <"$script_dir/apt-categories.txt"

# Manual version pins as "name=version" entries. Plugins absent here
# install LATEST; pins for plugins not enabled in the active preset are
# pruned automatically (see pins_for), so this single list serves both
# amd64-full and arm64-full. Bump an entry to roll a pinned version
# (surfaces as a deliberate diff in the e2e logs).
pin_entries=(
  aws-cli=2.34.48
  aws-sam-cli=1.160.1
  bun=1.3.3
  cocoon=0.7.4
  codex=0.135.0
  copilot-cli=1.0.47
  dart=3.12.0
  deno=2.7.14
  docker-buildx=0.24.0
  flutter=3.44.0
  gitleaks=8.30.1
  go=1.23.4
  golangci-lint=2.12.2
  helm=3.16.0
  just=1.51.0
  kubectl=1.31.0
  lazygit=0.44.1
  mise=2025.12.0
  nerd-fonts=3.4.0
  node=24.15.0
  opentofu=1.9.0
  proto=0.46.1
  shellcheck=0.10.0
  shfmt=3.10.0
  starship=1.21.1
  terraform=1.10.5
  uv=0.5.7
  zig=0.13.0
)

# join_csv prints its args as a single comma-separated string.
join_csv() {
  local IFS=,
  echo "$*"
}

# pins_for prints the name=version CSV for the pin_entries whose plugin is
# in the given enabled set, dropping pins for plugins this preset omits.
pins_for() {
  local enabled=("$@")
  local entry name p out=()
  for entry in "${pin_entries[@]}"; do
    name="${entry%%=*}"
    for p in "${enabled[@]}"; do
      if [ "$p" = "$name" ]; then
        out+=("$entry")
        break
      fi
    done
  done
  join_csv "${out[@]}"
}

# Fail fast on a stale or mistyped pin: every pin_entries id must be a
# real, version-capable catalog plugin. pins_for silently drops a pin
# whose id matches no enabled plugin, so without this check a renamed or
# typo'd id would quietly demote that plugin to LATEST — yet the same id
# passed straight to `cocoon init --plugin-versions` is rejected, so the
# e2e harness must reject it too.
for entry in "${pin_entries[@]}"; do
  name="${entry%%=*}"
  found=""
  for p in "${all_plugins[@]}"; do
    if [ "$p" = "$name" ]; then
      found=1
      break
    fi
  done
  if [ -z "$found" ]; then
    echo "pin_entries: '$name' is not a catalog plugin (typo or renamed?)" >&2
    exit 1
  fi
  if ! grep -Eq '^version_capable[[:space:]]*=[[:space:]]*true[[:space:]]*$' \
    "$catalog_dir/$name/plugin.toml"; then
    echo "pin_entries: '$name' is not version_capable; it cannot be pinned" >&2
    exit 1
  fi
done

case "$PRESET" in
  minimal)
    plugins=""
    pins=""
    methods=""
    ;;
  amd64-full)
    enabled=("${all_plugins[@]}")
    plugins="$(join_csv "${enabled[@]}")"
    pins="$(pins_for "${enabled[@]}")"
    # Exercise the [install.methods]=binary path for copilot-cli on this
    # preset (matches the offline / no-curl|sh use case the method was added
    # for). arm64-full keeps the default gh-cli method so both install
    # paths get real docker-build coverage per release.
    methods="copilot-cli=binary"
    ;;
  arm64-full)
    enabled=()
    for p in "${all_plugins[@]}"; do
      skip=""
      for x in "${arm64_exclude[@]}"; do
        if [ "$p" = "$x" ]; then
          skip=1
          break
        fi
      done
      if [ -z "$skip" ]; then
        enabled+=("$p")
      fi
    done
    plugins="$(join_csv "${enabled[@]}")"
    pins="$(pins_for "${enabled[@]}")"
    # No --plugin-methods on arm64-full — exercises the default method
    # (gh-cli) end-to-end as the counterpart to amd64-full's binary path.
    # Both real-Docker installs run per release.
    methods=""
    ;;
  *)
    echo "unknown preset: $PRESET" >&2
    exit 1
    ;;
esac

# In latest mode the full presets float every plugin to its upstream LATEST
# so the drift canary surfaces upstream packaging changes (e.g. a removed
# checksum manifest) before users hit them. minimal has no plugins to float.
[ "$PIN_MODE" = latest ] && pins=""

extra=()
[ -n "$plugins" ] && extra+=(--plugins "$plugins")
[ -n "$pins" ] && extra+=(--plugin-versions "$pins")
[ -n "$methods" ] && extra+=(--plugin-methods "$methods")

# service-name and the in-container username are kept identical ("dev")
# so the `docker compose exec dev` step below targets the right service.
# Using a different service-name (e.g. "e2e") would silently break exec
# because `exec <service>` resolves against the compose service id, which
# mirrors workspace.toml's [container].service_name.
# Match cocoon's full default-on apt category set so default-on plugins
# (e.g. docker-cli, which calls gpg in install.sh via the vcs category)
# build cleanly. Trimming this list in CI hides plugin-vs-category
# mismatches that real users would hit.
# --certificates opts the workspace into TLS auto-bake from
# ~/.cocoon/certs/. Required here because the e2e matrix exercises the
# cert wiring path end-to-end (docker buildx bake consuming
# additional_contexts, the RUN --mount=type=bind, the downstream
# update-ca-certificates). Cert-free flows are covered by Go unit tests;
# the e2e overhead of running both branches through the full matrix is
# not worth it.
"$COCOON" init --yes \
  --service-name dev \
  --username dev \
  --image "$IMAGE" \
  --image-version "$IMAGE_VERSION" \
  --mount-root . \
  --no-devcontainer \
  --certificates \
  --apt-categories="$(join_csv "${apt_categories[@]}")" \
  "${extra[@]}"
"$COCOON" gen

# Surface the generated artifacts to the CI log so PR reviewers can read
# exactly what cocoon emitted for this preset / image (matters most
# after generator changes — e.g. plugin install scripts now land inline
# in the Dockerfile via bash heredocs). The GitHub Actions group markers
# keep the dumps collapsed by default so they don't drown the rest of
# the step.
echo "::group::.devcontainer/Dockerfile"
cat .devcontainer/Dockerfile
echo "::endgroup::"
echo "::group::.devcontainer/docker-compose.yml"
cat .devcontainer/docker-compose.yml
echo "::endgroup::"

# The generated tree must land under .devcontainer/ exclusively; any
# leakage back into config/ or to the project root is a regression.
# The .env file is critical for compose interpolation
# (COMPOSE_PROJECT_NAME / CONTAINER_SERVICE_NAME / USERNAME / IMAGE /
# IMAGE_VERSION); without it `docker compose build` fails with
# `failed to parse stage name ":": invalid reference format`.
test -f .devcontainer/Dockerfile
test -f .devcontainer/docker-compose.yml
test -f .devcontainer/docker-entrypoint.sh
test -f .devcontainer/.env
test ! -d config

# Build via `docker buildx bake` to opt into BuildKit's GHA cache
# backend. `docker compose build` cannot pass cache-from/cache-to
# without baking CI concerns into the generated docker-compose.yml
# (which is the user-visible artifact). Bake reads the same compose
# file and lets us inject cache settings purely on the CI side.
#
# Two compose-side conveniences we have to replicate manually:
#   1. .env loading — compose auto-loads .env from the compose file's
#      directory; bake does not. We read .env into an array and pass
#      it inline to bake's child process via `env KEY=VAL ... cmd`.
#   2. Path resolution — compose resolves `context: ..` relative to
#      the compose file's directory; bake resolves it relative to its
#      own cwd. Without an override bake fails with
#      `lstat ../.devcontainer: no such file or directory`. We pin both
#      paths absolutely via `--set "*.context=$PWD"` and
#      `--set "*.dockerfile=$PWD/.devcontainer/Dockerfile"`.
#
# `arch` is derived from `uname -m` with an explicit case so an
# unexpected runner architecture fails loudly here instead of producing
# a malformed `linux/<garbage>` platform string downstream. Decided
# before cache_scope below because the minimal preset folds arch into
# its scope key.
case "$(uname -m)" in
  x86_64) arch=amd64 ;;
  aarch64) arch=arm64 ;;
  *)
    printf 'unsupported runner arch: %s\n' "$(uname -m)" >&2
    exit 1
    ;;
esac

# Cache scope split:
#   - minimal: per-arch + per-image (`e2e-minimal-<arch>-<image>`). The
#     base FROM layer differs entirely between images AND between
#     architectures of the same image (amd64 ubuntu vs arm64 ubuntu both
#     run in the push matrix), so a shared scope would have ~0% hit
#     rate while paying the write contention. `${IMAGE//\//-}` slugifies
#     "denoland/deno" -> "denoland-deno" so the scope is a flat string.
#   - amd64-full / arm64-full: per-preset (`e2e-<preset>`). Both ride on
#     debian 13 and benefit from sharing the apt prelude across runs
#     of the same preset; the preset name already encodes the arch.
# GHA cache backend further isolates by branch.
# mode=max captures all layers (within the 10GB/repo limit).
# The subsequent `compose up -d` reuses the just-loaded image instead
# of rebuilding.
case "$PRESET" in
  minimal) cache_scope="e2e-minimal-${arch}-${IMAGE//\//-}" ;;
  *) cache_scope="e2e-${PRESET}-${PIN_MODE}" ;;
esac

# The generated docker-compose.yml declares
# `additional_contexts: cocoon_user_certs: ${HOME}/.cocoon/certs` so
# users can drop private CA certs into a host-side directory
# and have them auto-baked at build time.
#
# Two CI-side concerns we have to handle that end users don't:
#   1. Path existence — BuildKit resolves the named context path before
#      the build starts and errors if the directory does not exist.
#      VS Code Dev Containers users get this auto-created via
#      devcontainer.json's initializeCommand; bake (this CI) and
#      `docker compose build` directly don't run that hook, so we mkdir
#      explicitly. The directory stays empty in CI, so the Dockerfile's
#      `if find ... ; then ... fi` branch no-ops and no certs land in
#      the image.
#   2. Filesystem entitlement (bake-only) — `docker buildx bake` treats
#      build contexts outside the bake working dir as privileged reads
#      and refuses to start without an explicit `--allow=fs.read=<path>`
#      grant. `docker compose build` does NOT enforce this, so end
#      users running compose directly are unaffected. We pass the flag
#      below.
mkdir -p "${HOME:?HOME unset}/.cocoon/certs"

env_args=()
while IFS= read -r line || [ -n "$line" ]; do
  case "$line" in
    '' | '#'*) continue ;;
  esac
  env_args+=("$line")
done <.devcontainer/.env

env "${env_args[@]}" docker buildx bake \
  -f .devcontainer/docker-compose.yml \
  --allow="fs.read=${HOME}/.cocoon/certs" \
  --load \
  --set "*.context=$PWD" \
  --set "*.dockerfile=$PWD/.devcontainer/Dockerfile" \
  --set "*.cache-from=type=gha,scope=${cache_scope}" \
  --set "*.cache-to=type=gha,scope=${cache_scope},mode=max" \
  --set "*.platform=linux/${arch}" \
  dev

docker compose -f .devcontainer/docker-compose.yml up -d
# Always tear down (containers + volumes) on exit, including the
# `set -e` exits from the UID/GID poll below. Without the trap a
# mid-test failure leaves containers behind — irrelevant on
# ephemeral GHA runners but matters for local reruns and any
# future self-hosted runner.
trap 'docker compose -f .devcontainer/docker-compose.yml down -v >/dev/null 2>&1 || true' EXIT
docker compose -f .devcontainer/docker-compose.yml exec -T dev bash -lc 'echo cocoon-e2e-ok'

# docker-entrypoint.sh remaps the fixed-uid/gid (1000:1000) container
# user to the host owner of the bind-mounted workspace, then drops
# privileges. `up -d` returns before the entrypoint finishes, so poll
# until the in-container `dev` user resolves to the runner's uid AND
# gid — proof the committed, host-independent .devcontainer/ adapted
# to this host. Asserting gid catches a broken groupmod/primary-group
# remap that a uid-only check would miss.
host_uid="$(id -u)"
host_gid="$(id -g)"
container_uid=""
container_gid=""
for _ in $(seq 1 30); do
  container_uid="$(docker compose -f .devcontainer/docker-compose.yml exec -T -u dev dev id -u 2>/dev/null | tr -d '\r\n' || true)"
  container_gid="$(docker compose -f .devcontainer/docker-compose.yml exec -T -u dev dev id -g 2>/dev/null | tr -d '\r\n' || true)"
  if [ "$container_uid" = "$host_uid" ] && [ "$container_gid" = "$host_gid" ]; then
    break
  fi
  sleep 1
done
test "$host_uid" = "$container_uid" ||
  {
    echo "UID remap failed: host=$host_uid container=$container_uid" >&2
    exit 1
  }
test "$host_gid" = "$container_gid" ||
  {
    echo "GID remap failed: host=$host_gid container=$container_gid" >&2
    exit 1
  }
# trap EXIT (installed after `up -d` above) runs `down -v` on exit.
