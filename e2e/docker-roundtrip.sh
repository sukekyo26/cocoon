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
#   - amd64-full / arm64-full (scheduled-e2e.yml) — every shipped plugin
#     enabled with all version_capable plugins pinned. Catches
#     plugin-vs-base-image and plugin-vs-plugin breakage; bumping a pinned
#     version surfaces here as a deliberate diff. arm64-full is the
#     arm64-safe subset (excludes lazygit / zig / starship / google-chrome
#     / flutter where install.sh hard-codes amd64 or fails fast on arm64).
#
# Local WSL2 cannot run this (gcc missing for `-race`); GHA-hosted
# ubuntu-latest / ubuntu-24.04-arm ship docker + buildx out of the box.

set -euxo pipefail

: "${COCOON:?COCOON unset (path to cocoon binary)}"
: "${PRESET:?PRESET unset (minimal | amd64-full | arm64-full)}"
: "${IMAGE:?IMAGE unset (base image id)}"
: "${IMAGE_VERSION:?IMAGE_VERSION unset}"

# Wipe any leftover project dir before init so reruns (local debugging,
# self-hosted runner with cached workspace) are idempotent — `cocoon
# init` refuses to overwrite an existing workspace.toml without --force.
rm -rf e2e/test-project
mkdir -p e2e/test-project
cd e2e/test-project
git init -q

# Resolve --plugins / --plugin-versions per preset. The lists are inlined
# (vs. a fixture TOML) so a pin bump is a one-line edit. Keep this
# case-stmt and the testdata snapshots in
# internal/cli/init/testdata/init/ in lockstep — they share the same
# plugin sets so the goldens act as a structural check on whatever this
# script generates at runtime.
case "$PRESET" in
  minimal)
    plugins=""
    pins=""
    methods=""
    ;;
  amd64-full)
    plugins="docker-cli,docker-buildx,aws-cli,aws-sam-cli,github-cli,claude-code,copilot-cli,proto,mise,uv,bun,node,deno,dart,flutter,zig,rust,go,lazygit,starship,nerd-fonts,google-chrome,terraform,opentofu,kubectl,helm,shellcheck,shfmt"
    pins="aws-cli=2.34.48,aws-sam-cli=1.160.1,bun=1.3.3,copilot-cli=1.0.47,dart=3.12.0,deno=2.7.14,docker-buildx=0.24.0,flutter=3.44.0,go=1.23.4,helm=3.16.0,kubectl=1.31.0,lazygit=0.44.1,mise=2025.12.0,nerd-fonts=3.4.0,node=24.15.0,opentofu=1.9.0,proto=0.46.1,shellcheck=0.10.0,shfmt=3.10.0,starship=1.21.1,terraform=1.10.5,uv=0.5.7,zig=0.13.0"
    # Exercise the [install.methods]=binary path for copilot-cli on this
    # preset (matches the Zscaler-style use case the method was added
    # for). arm64-full keeps the default gh-cli method so both install
    # paths get real docker-build coverage per release.
    methods="copilot-cli=binary"
    ;;
  arm64-full)
    # Excluded vs amd64-full (install.sh hard-codes amd64 or fails fast on arm64):
    #   lazygit, zig, starship, google-chrome, flutter.
    plugins="docker-cli,docker-buildx,aws-cli,aws-sam-cli,github-cli,claude-code,copilot-cli,proto,mise,uv,bun,node,deno,dart,rust,go,nerd-fonts,terraform,opentofu,kubectl,helm,shellcheck,shfmt"
    pins="aws-cli=2.34.48,aws-sam-cli=1.160.1,bun=1.3.3,copilot-cli=1.0.47,dart=3.12.0,deno=2.7.14,docker-buildx=0.24.0,go=1.23.4,helm=3.16.0,kubectl=1.31.0,mise=2025.12.0,nerd-fonts=3.4.0,node=24.15.0,opentofu=1.9.0,proto=0.46.1,shellcheck=0.10.0,shfmt=3.10.0,terraform=1.10.5,uv=0.5.7"
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
  --apt-categories=text-editors,vcs,utilities,compression,build \
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
#     ubuntu 22.04 and benefit from sharing the apt prelude across runs
#     of the same preset; the preset name already encodes the arch.
# GHA cache backend further isolates by branch.
# mode=max captures all layers (within the 10GB/repo limit).
# The subsequent `compose up -d` reuses the just-loaded image instead
# of rebuilding.
case "$PRESET" in
  minimal) cache_scope="e2e-minimal-${arch}-${IMAGE//\//-}" ;;
  *) cache_scope="e2e-${PRESET}" ;;
esac

# The generated docker-compose.yml declares
# `additional_contexts: cocoon_user_certs: ${HOME}/.cocoon/certs` so
# users can drop corp CA certs (e.g. Zscaler) into a host-side directory
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
