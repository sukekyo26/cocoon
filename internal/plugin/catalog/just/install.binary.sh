#!/usr/bin/env bash
# Install just (https://github.com/casey/just)
#
# Inputs (env):
#   PIN              : just version (without leading "v"); empty = latest
#   CHECKSUM_AMD64   : sha256 of amd64 tarball; empty = verify against the
#                      release SHA256SUMS
#   CHECKSUM_ARM64   : sha256 of arm64 tarball; empty = verify against the
#                      release SHA256SUMS
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

# just tags are bare semver (e.g. "1.51.0") with no leading "v",
# so the latest-redirect path is .../tag/<ver> — strip up to "/tag/".
if [ -n "$PIN" ]; then
  VERSION="$PIN"
else
  VERSION=$(curl -fsSLI --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    -o /dev/null -w '%{url_effective}' https://github.com/casey/just/releases/latest |
    sed 's|.*/tag/||')
fi

base="https://github.com/casey/just/releases/download/${VERSION}"
asset="just-${VERSION}-${DOWNLOAD_ARCH}-unknown-linux-musl.tar.gz"

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "${base}/${asset}" -o /tmp/just.tar.gz

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/just.tar.gz" | sha256sum -c -
else
  # No user pin: verify against the release's own SHA256SUMS.
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    "${base}/SHA256SUMS" -o /tmp/just.sums
  # Match the asset name literally (awk field compare, not a regex) and fail
  # loudly if it is absent so a manifest-shape change does not collapse into
  # an opaque sha256sum error.
  expected="$(awk -v f="$asset" '$2 == f { print $1; exit }' /tmp/just.sums)"
  if [ -z "$expected" ]; then
    echo "just: ${asset} not found in SHA256SUMS" >&2
    exit 1
  fi
  echo "${expected}  /tmp/just.tar.gz" | sha256sum -c -
  rm -f /tmp/just.sums
fi

tar -xzf /tmp/just.tar.gz -C /usr/local/bin just
rm /tmp/just.tar.gz
