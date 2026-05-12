#!/usr/bin/env bash
# Install Node.js (https://github.com/nodejs/node)
#
# Inputs (env):
#   PIN              : Node.js version without leading "v" (e.g. "24.15.0"); empty = latest LTS
#   CHECKSUM_AMD64   : sha256 of linux-x64 tarball; empty to skip verification
#   CHECKSUM_ARM64   : sha256 of linux-arm64 tarball; empty to skip verification
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

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "https://nodejs.org/dist/v${VERSION}/node-v${VERSION}-linux-${NODE_ARCH}.tar.xz" -o /tmp/node.tar.xz

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/node.tar.xz" | sha256sum -c -
else
  printf '%sWARNING: SHA256 verification skipped for Node.js (no checksum provided in [plugins.versions.node])%s\n' "$C_YEL" "$C_RST" >&2
fi

mkdir -p /usr/local/node
tar -C /usr/local/node --strip-components=1 -xJf /tmp/node.tar.xz
rm /tmp/node.tar.xz
