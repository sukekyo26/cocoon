#!/usr/bin/env bash
# Install Go (https://github.com/golang/go)
#
# Inputs (env):
#   PIN              : Go version to install (e.g. "1.23.0"); empty = latest
#   CHECKSUM_AMD64   : sha256 of amd64 tarball; empty = verify against the
#                      upstream .sha256 served next to the tarball
#   CHECKSUM_ARM64   : sha256 of arm64 tarball; empty = verify against the
#                      upstream .sha256 served next to the tarball
set -euo pipefail

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

# dl.google.com/go is the canonical CDN go.dev/dl redirects to; it also
# serves a .sha256 sidecar (a bare hash) next to each tarball.
url="https://dl.google.com/go/go${VERSION}.linux-${ARCH}.tar.gz"
curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "$url" -o /tmp/go.tar.gz

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/go.tar.gz" | sha256sum -c -
else
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    "${url}.sha256" -o /tmp/go.sum
  expected="$(cut -d ' ' -f1 /tmp/go.sum)"
  echo "${expected}  /tmp/go.tar.gz" | sha256sum -c -
  rm -f /tmp/go.sum
fi

tar -C /usr/local -xzf /tmp/go.tar.gz
rm /tmp/go.tar.gz
