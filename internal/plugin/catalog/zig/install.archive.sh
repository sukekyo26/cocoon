#!/usr/bin/env bash
# Install Zig (required for cargo-lambda cross-compilation) (https://github.com/ziglang/zig)
#
# Inputs (env):
#   PIN              : Zig version (e.g. "0.14.0") or "master"; empty = master
#   CHECKSUM_AMD64   : sha256 of amd64 tarball; empty = verify against the
#                      shasum in ziglang.org's index.json
#   CHECKSUM_ARM64   : sha256 of arm64 tarball; empty = verify against the
#                      shasum in ziglang.org's index.json
#
# The download URL and its shasum are resolved through ziglang.org's
# index.json so we stay compatible with both pre-0.14 and 0.14+ asset naming.
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

VERSION_KEY="${PIN:-master}"

INDEX=$(curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  https://ziglang.org/download/index.json)
ZIG_URL=$(printf '%s' "$INDEX" | jq -r --arg v "$VERSION_KEY" --arg a "$DOWNLOAD_ARCH" '.[$v][$a + "-linux"].tarball')
ZIG_SHA=$(printf '%s' "$INDEX" | jq -r --arg v "$VERSION_KEY" --arg a "$DOWNLOAD_ARCH" '.[$v][$a + "-linux"].shasum')

if [ -z "$ZIG_URL" ] || [ "$ZIG_URL" = "null" ]; then
  echo "ERROR: Could not resolve Zig tarball URL for version=${VERSION_KEY} arch=${DOWNLOAD_ARCH}" >&2
  exit 1
fi

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "$ZIG_URL" -o /tmp/zig.tar.xz

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/zig.tar.xz" | sha256sum -c -
else
  # No user pin: verify against the shasum index.json publishes for this build.
  if [ -z "$ZIG_SHA" ] || [ "$ZIG_SHA" = "null" ]; then
    echo "ERROR: index.json has no shasum for version=${VERSION_KEY} arch=${DOWNLOAD_ARCH}" >&2
    exit 1
  fi
  echo "${ZIG_SHA}  /tmp/zig.tar.xz" | sha256sum -c -
fi

mkdir -p /usr/local/zig
tar -xf /tmp/zig.tar.xz -C /usr/local/zig --strip-components=1
ln -sf /usr/local/zig/zig /usr/local/bin/zig
rm /tmp/zig.tar.xz
