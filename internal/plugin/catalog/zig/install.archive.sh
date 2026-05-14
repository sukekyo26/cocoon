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

# Color output (yellow WARNING, red ERROR) when stderr is a TTY (and
# NO_COLOR is unset) or FORCE_COLOR is set. NO_COLOR wins per no-color.org.
if [ -n "${NO_COLOR:-}" ]; then
  C_YEL=''
  C_RED=''
  C_RST=''
elif [ -n "${FORCE_COLOR:-}" ] || [ -t 2 ]; then
  C_YEL=$'\033[33m'
  C_RED=$'\033[31m'
  C_RST=$'\033[0m'
else
  C_YEL=''
  C_RED=''
  C_RST=''
fi

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
  printf '%sERROR: Could not resolve Zig tarball URL for version=%s arch=%s%s\n' "$C_RED" "$VERSION_KEY" "$DOWNLOAD_ARCH" "$C_RST" >&2
  exit 1
fi

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "$ZIG_URL" -o /tmp/zig.tar.xz

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/zig.tar.xz" | sha256sum -c -
else
  printf '%sWARNING: SHA256 verification skipped for Zig (no checksum for zig in [plugins.versions])%s\n' "$C_YEL" "$C_RST" >&2
fi

mkdir -p /usr/local/zig
tar -xf /tmp/zig.tar.xz -C /usr/local/zig --strip-components=1
ln -sf /usr/local/zig/zig /usr/local/bin/zig
rm /tmp/zig.tar.xz
