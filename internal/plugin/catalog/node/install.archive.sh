#!/usr/bin/env bash
# Install Node.js (https://github.com/nodejs/node)
#
# Inputs (env):
#   PIN              : Node.js version without leading "v" (e.g. "24.15.0"); empty = latest LTS
#   CHECKSUM_AMD64   : sha256 of linux-x64 tarball; empty = verify against the
#                      release SHASUMS256.txt
#   CHECKSUM_ARM64   : sha256 of linux-arm64 tarball; empty = verify against the
#                      release SHASUMS256.txt
set -euo pipefail

ARCH="$(dpkg --print-architecture)"
case "$ARCH" in
  amd64)
    NODE_ARCH="x64"
    CHECKSUM="$CHECKSUM_AMD64"
    ;;
  arm64)
    NODE_ARCH="arm64"
    CHECKSUM="$CHECKSUM_ARM64"
    ;;
  *)
    NODE_ARCH="x64"
    CHECKSUM="$CHECKSUM_AMD64"
    ;;
esac

if [ -n "$PIN" ]; then
  VERSION="$PIN"
else
  # Resolve latest LTS from dist/index.tab. The 10th column is the LTS
  # codename ("-" for non-LTS releases); the first matching row is the
  # newest LTS line because index.tab is sorted newest-first.
  VERSION=$(curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    https://nodejs.org/dist/index.tab |
    awk -F'\t' 'NR>1 && $10 != "-" { sub(/^v/, "", $1); print $1; exit }')
  if [ -z "$VERSION" ]; then
    echo "Failed to resolve latest Node.js LTS version" >&2
    exit 1
  fi
fi

dist="https://nodejs.org/dist/v${VERSION}"
asset="node-v${VERSION}-linux-${NODE_ARCH}.tar.xz"
curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "${dist}/${asset}" -o /tmp/node.tar.xz

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/node.tar.xz" | sha256sum -c -
else
  # No user pin: verify against the release's own SHASUMS256.txt.
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    "${dist}/SHASUMS256.txt" -o /tmp/node.sums
  # Match the asset name literally (awk field compare, not a regex) and fail
  # loudly if it is absent so a manifest-shape change does not collapse into
  # an opaque sha256sum error.
  expected="$(awk -v f="$asset" '$2 == f { print $1; exit }' /tmp/node.sums)"
  if [ -z "$expected" ]; then
    echo "node: ${asset} not found in SHASUMS256.txt" >&2
    exit 1
  fi
  echo "${expected}  /tmp/node.tar.xz" | sha256sum -c -
  rm -f /tmp/node.sums
fi

mkdir -p /usr/local/node
tar -C /usr/local/node --strip-components=1 -xJf /tmp/node.tar.xz
rm /tmp/node.tar.xz
