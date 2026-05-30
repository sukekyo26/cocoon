#!/usr/bin/env bash
# Install Docker Buildx (https://github.com/docker/buildx)
#
# Pair with the docker-cli plugin (or a docker-bundled base image) so the
# host Docker daemon is reachable; buildx is a client-side CLI plugin that
# drives the daemon's BuildKit, and is useless without one.
#
# Inputs (env):
#   PIN              : Buildx version without leading "v" (e.g. "0.24.0"); empty = latest
#   CHECKSUM_AMD64   : sha256 of the amd64 binary; empty = verify against the
#                      release checksums.txt
#   CHECKSUM_ARM64   : sha256 of the arm64 binary; empty = verify against the
#                      release checksums.txt
set -euo pipefail

ARCH="$(dpkg --print-architecture)"
case "$ARCH" in
  amd64) CHECKSUM="$CHECKSUM_AMD64" ;;
  arm64) CHECKSUM="$CHECKSUM_ARM64" ;;
  *) CHECKSUM="$CHECKSUM_AMD64" ;;
esac

if [ -n "$PIN" ]; then
  VERSION="$PIN"
else
  VERSION=$(curl -fsSLI --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    -o /dev/null -w '%{url_effective}' https://github.com/docker/buildx/releases/latest |
    sed 's|.*/tag/v||')
fi

base="https://github.com/docker/buildx/releases/download/v${VERSION}"
asset="buildx-v${VERSION}.linux-${ARCH}"

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "${base}/${asset}" -o /tmp/docker-buildx

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/docker-buildx" | sha256sum -c -
else
  # No user pin: verify against the release's own checksums.txt.
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    "${base}/checksums.txt" -o /tmp/docker-buildx.sums
  # Match the asset name literally (awk field compare, not a regex; strip the
  # binary-mode '*' marker docker-buildx's checksums.txt prepends) and fail
  # loudly if it is absent so a manifest-shape change does not collapse into
  # an opaque sha256sum error.
  expected="$(awk -v f="$asset" '{ n = $2; sub(/^\*/, "", n); if (n == f) { print $1; exit } }' /tmp/docker-buildx.sums)"
  if [ -z "$expected" ]; then
    echo "docker-buildx: ${asset} not found in checksums.txt" >&2
    exit 1
  fi
  echo "${expected}  /tmp/docker-buildx" | sha256sum -c -
  rm -f /tmp/docker-buildx.sums
fi

# /usr/libexec/docker/cli-plugins/ is the Docker CLI plugin lookup path
# (see `docker --help`). The directory may not exist on a fresh base image
# if the docker-cli plugin has not run yet, so create it before install.
mkdir -p /usr/libexec/docker/cli-plugins
install -o root -g root -m 0755 /tmp/docker-buildx /usr/libexec/docker/cli-plugins/docker-buildx
rm /tmp/docker-buildx
