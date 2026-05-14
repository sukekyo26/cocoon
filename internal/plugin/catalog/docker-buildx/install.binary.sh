#!/usr/bin/env bash
# Install Docker Buildx (https://github.com/docker/buildx)
#
# Pair with the docker-cli plugin (or a docker-bundled base image) so the
# host Docker daemon is reachable; buildx is a client-side CLI plugin that
# drives the daemon's BuildKit, and is useless without one.
#
# Inputs (env):
#   PIN              : Buildx version without leading "v" (e.g. "0.24.0"); empty = latest
#   CHECKSUM_AMD64   : sha256 of the amd64 binary; empty to skip verification
#   CHECKSUM_ARM64   : sha256 of the arm64 binary; empty to skip verification
set -euo pipefail

# Yellow WARNING when stderr is a TTY (and NO_COLOR is unset) or
# FORCE_COLOR is set. NO_COLOR wins per no-color.org.
if [ -n "${NO_COLOR:-}" ]; then
  C_YEL=''
  C_RST=''
elif [ -n "${FORCE_COLOR:-}" ] || [ -t 2 ]; then
  C_YEL=$'\033[33m'
  C_RST=$'\033[0m'
else
  C_YEL=''
  C_RST=''
fi

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

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "https://github.com/docker/buildx/releases/download/v${VERSION}/buildx-v${VERSION}.linux-${ARCH}" \
  -o /tmp/docker-buildx

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/docker-buildx" | sha256sum -c -
else
  printf '%sWARNING: SHA256 verification skipped for Docker Buildx (no checksum for docker-buildx in [plugins.versions])%s\n' "$C_YEL" "$C_RST" >&2
fi

# /usr/libexec/docker/cli-plugins/ is the Docker CLI plugin lookup path
# (see `docker --help`). The directory may not exist on a fresh base image
# if the docker-cli plugin has not run yet, so create it before install.
mkdir -p /usr/libexec/docker/cli-plugins
install -o root -g root -m 0755 /tmp/docker-buildx /usr/libexec/docker/cli-plugins/docker-buildx
rm /tmp/docker-buildx
