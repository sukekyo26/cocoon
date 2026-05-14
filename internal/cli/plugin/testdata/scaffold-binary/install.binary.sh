#!/usr/bin/env bash
# Install Demo Bin from a GitHub Release binary asset.
# Method category: binary — downloads a single binary (or extracts one
#                  from a tarball) and places it on PATH.
#
# Inputs (env):
#   PIN              : version (without leading "v"); empty = latest
#   CHECKSUM_AMD64   : sha256 of amd64 asset; empty to skip verification
#   CHECKSUM_ARM64   : sha256 of arm64 asset; empty to skip verification
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

# TODO: replace OWNER/REPO and the asset name pattern with the upstream layout.
if [ -n "$PIN" ]; then
  VERSION="$PIN"
else
  VERSION=$(curl -fsSLI --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    -o /dev/null -w '%{url_effective}' https://github.com/OWNER/REPO/releases/latest |
    sed 's|.*/tag/v||')
fi

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "https://github.com/OWNER/REPO/releases/download/v${VERSION}/REPO-${DOWNLOAD_ARCH}-unknown-linux-musl" \
  -o /usr/local/bin/demo

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /usr/local/bin/demo" | sha256sum -c -
else
  echo "WARNING: SHA256 verification skipped for Demo Bin (no checksum for demo in [plugins.versions])" >&2
fi

chmod 0755 /usr/local/bin/demo
