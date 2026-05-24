#!/usr/bin/env bash
# Install just (https://github.com/casey/just)
#
# Inputs (env):
#   PIN              : just version (without leading "v"); empty = latest
#   CHECKSUM_AMD64   : sha256 of amd64 tarball; empty to skip verification
#   CHECKSUM_ARM64   : sha256 of arm64 tarball; empty to skip verification
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

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "https://github.com/casey/just/releases/download/${VERSION}/just-${VERSION}-${DOWNLOAD_ARCH}-unknown-linux-musl.tar.gz" \
  -o /tmp/just.tar.gz

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/just.tar.gz" | sha256sum -c -
else
  printf '%sWARNING: SHA256 verification skipped for just (no checksum for just in [plugins.versions])%s\n' "$C_YEL" "$C_RST" >&2
fi

tar -xzf /tmp/just.tar.gz -C /usr/local/bin just
rm /tmp/just.tar.gz
