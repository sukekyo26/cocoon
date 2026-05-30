#!/usr/bin/env bash
# Install Starship (https://github.com/starship/starship)
#
# Inputs (env):
#   PIN              : Starship version (without leading "v"); empty = latest
#   CHECKSUM_AMD64   : sha256 of amd64 tarball; empty = verify against the
#                      per-asset .sha256 published with the release
#   CHECKSUM_ARM64   : sha256 of arm64 tarball; empty = verify against the
#                      per-asset .sha256 published with the release
set -euo pipefail

ARCH="$(dpkg --print-architecture)"
case "$ARCH" in
  amd64)
    DOWNLOAD_ARCH="x86_64"
    CHECKSUM="$CHECKSUM_AMD64"
    ;;
  arm64)
    DOWNLOAD_ARCH="aarch64"
    CHECKSUM="$CHECKSUM_ARM64"
    ;;
  *)
    DOWNLOAD_ARCH="x86_64"
    CHECKSUM="$CHECKSUM_AMD64"
    ;;
esac

if [ -n "$PIN" ]; then
  VERSION="$PIN"
else
  VERSION=$(curl -fsSLI --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    -o /dev/null -w '%{url_effective}' https://github.com/starship/starship/releases/latest |
    sed 's|.*/tag/v||')
fi

url="https://github.com/starship/starship/releases/download/v${VERSION}/starship-${DOWNLOAD_ARCH}-unknown-linux-musl.tar.gz"
curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "$url" -o /tmp/starship.tar.gz

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/starship.tar.gz" | sha256sum -c -
else
  # No user checksum: verify against the per-asset .sha256 (a bare hash) the
  # release publishes next to the tarball.
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    "${url}.sha256" -o /tmp/starship.sum
  expected="$(cut -d ' ' -f1 /tmp/starship.sum)"
  if [ -z "$expected" ]; then
    echo "starship: empty checksum from ${url}.sha256" >&2
    exit 1
  fi
  echo "${expected}  /tmp/starship.tar.gz" | sha256sum -c -
  rm -f /tmp/starship.sum
fi

tar -xzf /tmp/starship.tar.gz -C /usr/local/bin starship
rm /tmp/starship.tar.gz
