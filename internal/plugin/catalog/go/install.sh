#!/usr/bin/env bash
# Install Go (https://github.com/golang/go)
#
# Inputs (env):
#   PIN              : Go version to install (e.g. "1.23.0"); empty = latest
#   CHECKSUM_AMD64   : sha256 of amd64 tarball; empty to skip verification
#   CHECKSUM_ARM64   : sha256 of arm64 tarball; empty to skip verification
set -euo pipefail

ARCH="$(dpkg --print-architecture)"
case "$ARCH" in
  amd64) CHECKSUM="$CHECKSUM_AMD64" ;;
  arm64) CHECKSUM="$CHECKSUM_ARM64" ;;
  *)     CHECKSUM="$CHECKSUM_AMD64" ;;
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
  echo "WARNING: SHA256 verification skipped for Go (no checksum provided in [plugins.versions.go])" >&2
fi

tar -C /usr/local -xzf /tmp/go.tar.gz
rm /tmp/go.tar.gz
