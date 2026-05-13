#!/usr/bin/env bash
# Install Helm (https://github.com/helm/helm)
#
# Inputs (env):
#   PIN              : Helm version without leading "v" (e.g. "3.16.0"); empty = latest stable
#   CHECKSUM_AMD64   : sha256 of the amd64 tarball; empty to skip verification
#   CHECKSUM_ARM64   : sha256 of the arm64 tarball; empty to skip verification
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
  # github.com/helm/helm/releases/latest redirects to the newest stable tag
  # (e.g. /releases/tag/v3.16.0). `-I` keeps the body off the wire.
  VERSION=$(curl -fsSLI --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    -o /dev/null -w '%{url_effective}' https://github.com/helm/helm/releases/latest |
    sed 's|.*/tag/v||')
fi

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "https://get.helm.sh/helm-v${VERSION}-linux-${ARCH}.tar.gz" -o /tmp/helm.tar.gz

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/helm.tar.gz" | sha256sum -c -
else
  printf '%sWARNING: SHA256 verification skipped for Helm (no checksum for helm in [plugins.versions])%s\n' "$C_YEL" "$C_RST" >&2
fi

# The tarball ships `linux-<arch>/helm` so --strip-components=1 + selective
# extract drops just the binary into /usr/local/bin without leaking the
# README / LICENSE siblings.
tar -xzf /tmp/helm.tar.gz -C /usr/local/bin --strip-components=1 "linux-${ARCH}/helm"
chmod +x /usr/local/bin/helm
rm /tmp/helm.tar.gz
