#!/usr/bin/env bash
# Install Demo Bin from a GitHub Release binary asset.
# Method category: binary — downloads a single binary (or extracts one
#                  from a tarball) and places it on PATH.
#
# Inputs (env):
#   PIN              : version (without leading "v"); empty = latest
#   CHECKSUM_AMD64   : sha256 of amd64 asset; empty = verify against the
#                      upstream-published checksum (see the else branch below)
#   CHECKSUM_ARM64   : sha256 of arm64 asset; empty = verify against the
#                      upstream-published checksum (see the else branch below)
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
  # No user pin: verify against the checksum the upstream publishes with the
  # release. Replace the URL + asset name to match your upstream's layout
  # (a "<asset>.sha256" sidecar, a "checksums.txt", or "SHA256SUMS"); see
  # docs/plugins.md. Fall back to a loud "WARNING: ... skipped" only if the
  # upstream ships no fetchable checksum.
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    "https://github.com/OWNER/REPO/releases/download/v${VERSION}/checksums.txt" -o /tmp/demo.sums
  expected="$(grep "REPO-${DOWNLOAD_ARCH}-unknown-linux-musl\$" /tmp/demo.sums | cut -d ' ' -f1)"
  echo "${expected}  /usr/local/bin/demo" | sha256sum -c -
  rm -f /tmp/demo.sums
fi

chmod 0755 /usr/local/bin/demo
