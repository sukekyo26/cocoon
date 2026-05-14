#!/usr/bin/env bash
# Install Go (https://github.com/golang/go)
#
# Inputs (env):
#   PIN              : Go version to install (e.g. "1.23.0"); empty = latest
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
  amd64) CHECKSUM="$CHECKSUM_AMD64" ;;
  arm64) CHECKSUM="$CHECKSUM_ARM64" ;;
  *) CHECKSUM="$CHECKSUM_AMD64" ;;
esac

if [ -n "$PIN" ]; then
  VERSION="$PIN"
else
  VERSION=$(curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    https://go.dev/VERSION?m=text | head -1 | sed 's/^go//')
fi

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "https://go.dev/dl/go${VERSION}.linux-${ARCH}.tar.gz" -o /tmp/go.tar.gz

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/go.tar.gz" | sha256sum -c -
else
  printf '%sWARNING: SHA256 verification skipped for Go (no checksum for go in [plugins.versions])%s\n' "$C_YEL" "$C_RST" >&2
fi

tar -C /usr/local -xzf /tmp/go.tar.gz
rm /tmp/go.tar.gz
