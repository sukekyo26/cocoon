#!/usr/bin/env bash
# Install shfmt (https://github.com/mvdan/sh)
#
# Inputs (env):
#   PIN              : shfmt version without leading "v" (e.g. "3.10.0"); empty = latest
#   CHECKSUM_AMD64   : sha256 of the amd64 binary; empty to skip verification
#   CHECKSUM_ARM64   : sha256 of the arm64 binary; empty to skip verification
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
  VERSION=$(curl -fsSLI --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    -o /dev/null -w '%{url_effective}' https://github.com/mvdan/sh/releases/latest |
    sed 's|.*/tag/v||')
fi

base="https://github.com/mvdan/sh/releases/download/v${VERSION}"
asset="shfmt_v${VERSION}_linux_${ARCH}"

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "${base}/${asset}" -o /tmp/shfmt

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/shfmt" | sha256sum -c -
else
  printf '%sWARNING: SHA256 verification skipped for shfmt (no checksum for shfmt in [plugins.versions])%s\n' "$C_YEL" "$C_RST" >&2
fi

install -o root -g root -m 0755 /tmp/shfmt /usr/local/bin/shfmt
rm /tmp/shfmt
