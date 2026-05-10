#!/usr/bin/env bash
# Install Zig (required for cargo-lambda cross-compilation) (https://github.com/ziglang/zig)
#
# Inputs (env):
#   PIN              : Zig version (e.g. "0.14.0") or "master"; empty = master
#   CHECKSUM_AMD64   : sha256 of amd64 tarball; empty to skip verification
#   CHECKSUM_ARM64   : sha256 of arm64 tarball; empty to skip verification
#
# The download URL is resolved through ziglang.org's index.json so we stay
# compatible with both pre-0.14 and 0.14+ asset naming.
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
echo "Detected architecture: $ARCH -> $DOWNLOAD_ARCH"

VERSION_KEY="${PIN:-master}"

ZIG_URL=$(curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  https://ziglang.org/download/index.json |
  jq -r --arg v "$VERSION_KEY" --arg a "$DOWNLOAD_ARCH" '.[$v][$a + "-linux"].tarball')

if [ -z "$ZIG_URL" ] || [ "$ZIG_URL" = "null" ]; then
  echo "ERROR: Could not resolve Zig tarball URL for version=$VERSION_KEY arch=$DOWNLOAD_ARCH" >&2
  exit 1
fi

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "$ZIG_URL" -o /tmp/zig.tar.xz

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/zig.tar.xz" | sha256sum -c -
else
  echo "WARNING: SHA256 verification skipped for Zig (no checksum provided in [plugins.versions.zig])" >&2
fi

mkdir -p /usr/local/zig
tar -xf /tmp/zig.tar.xz -C /usr/local/zig --strip-components=1
ln -sf /usr/local/zig/zig /usr/local/bin/zig
rm /tmp/zig.tar.xz
