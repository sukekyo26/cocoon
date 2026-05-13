#!/usr/bin/env bash
# Install Demo Tar
#
# Inputs (env):
#   PIN              : version (without leading "v"); empty = latest
#   CHECKSUM_AMD64   : sha256 of amd64 tarball; empty to skip verification
#   CHECKSUM_ARM64   : sha256 of arm64 tarball; empty to skip verification
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
  "https://github.com/OWNER/REPO/releases/download/v${VERSION}/REPO-${DOWNLOAD_ARCH}-unknown-linux-musl.tar.gz" \
  -o /tmp/demo.tar.gz

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/demo.tar.gz" | sha256sum -c -
else
  echo "WARNING: SHA256 verification skipped for Demo Tar (no checksum for demo in [plugins.versions])" >&2
fi

# TODO: extract to the right destination.
tar -xzf /tmp/demo.tar.gz -C /usr/local/bin
rm /tmp/demo.tar.gz
