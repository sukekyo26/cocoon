#!/usr/bin/env bash
# Install Deno (https://github.com/denoland/deno)
#
# Inputs (env):
#   PIN              : Deno version without leading "v" (e.g. "2.7.14"); empty = latest stable
#   CHECKSUM_AMD64   : sha256 of x86_64-unknown-linux-gnu zip; empty to skip verification
#   CHECKSUM_ARM64   : sha256 of aarch64-unknown-linux-gnu zip; empty to skip verification
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

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "https://github.com/denoland/deno/releases/download/${VERSION_TAG}/deno-${DENO_ARCH}-unknown-linux-gnu.zip" -o /tmp/deno.zip

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/deno.zip" | sha256sum -c -
else
  printf '%sWARNING: SHA256 verification skipped for Deno (no checksum for deno in [plugins.versions])%s\n' "$C_YEL" "$C_RST" >&2
fi

unzip -q -o /tmp/deno.zip -d /usr/local/bin
chmod 0755 /usr/local/bin/deno
rm /tmp/deno.zip
