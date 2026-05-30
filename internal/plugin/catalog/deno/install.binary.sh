#!/usr/bin/env bash
# Install Deno (https://github.com/denoland/deno)
#
# Inputs (env):
#   PIN              : Deno version without leading "v" (e.g. "2.7.14"); empty = latest stable
#   CHECKSUM_AMD64   : sha256 of x86_64-unknown-linux-gnu zip; empty = verify
#                      against the per-asset .sha256sum published with the release
#   CHECKSUM_ARM64   : sha256 of aarch64-unknown-linux-gnu zip; empty = verify
#                      against the per-asset .sha256sum published with the release
set -euo pipefail

ARCH="$(dpkg --print-architecture)"
case "$ARCH" in
  amd64)
    DENO_ARCH="x86_64"
    CHECKSUM="$CHECKSUM_AMD64"
    ;;
  arm64)
    DENO_ARCH="aarch64"
    CHECKSUM="$CHECKSUM_ARM64"
    ;;
  *)
    DENO_ARCH="x86_64"
    CHECKSUM="$CHECKSUM_AMD64"
    ;;
esac

if [ -n "$PIN" ]; then
  VERSION_TAG="v${PIN}"
else
  # `releases/latest` redirects to the newest stable tag (e.g. v2.7.14).
  VERSION_TAG=$(curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    -o /dev/null -w '%{url_effective}' https://github.com/denoland/deno/releases/latest |
    awk -F'/' '{print $NF}')
  if [ -z "$VERSION_TAG" ]; then
    echo "Failed to resolve latest Deno release tag" >&2
    exit 1
  fi
fi

url="https://github.com/denoland/deno/releases/download/${VERSION_TAG}/deno-${DENO_ARCH}-unknown-linux-gnu.zip"
curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "$url" -o /tmp/deno.zip

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/deno.zip" | sha256sum -c -
else
  # No user pin: verify against the per-asset .sha256sum the release publishes
  # next to the zip.
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    "${url}.sha256sum" -o /tmp/deno.sum
  expected="$(cut -d ' ' -f1 /tmp/deno.sum)"
  if [ -z "$expected" ]; then
    echo "deno: empty checksum from ${url}.sha256sum" >&2
    exit 1
  fi
  echo "${expected}  /tmp/deno.zip" | sha256sum -c -
  rm -f /tmp/deno.sum
fi

unzip -q -o /tmp/deno.zip -d /usr/local/bin
chmod 0755 /usr/local/bin/deno
rm /tmp/deno.zip
