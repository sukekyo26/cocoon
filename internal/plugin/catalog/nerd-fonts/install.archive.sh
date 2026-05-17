#!/usr/bin/env bash
# Install Nerd Fonts (Meslo) (https://github.com/ryanoasis/nerd-fonts)
#
# Inputs (env):
#   PIN              : Nerd Fonts version (without leading "v"); empty = latest
#   CHECKSUM_AMD64   : sha256 of Meslo.tar.xz; empty to skip verification
#   CHECKSUM_ARM64   : same artifact — Meslo.tar.xz is architecture-independent;
#                      consulted as a fallback when CHECKSUM_AMD64 is empty
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

if [ -n "$PIN" ]; then
  URL="https://github.com/ryanoasis/nerd-fonts/releases/download/v${PIN}/Meslo.tar.xz"
else
  URL="https://github.com/ryanoasis/nerd-fonts/releases/latest/download/Meslo.tar.xz"
fi

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "$URL" -o /tmp/Meslo.tar.xz

# Meslo.tar.xz is architecture-independent, so checksum_amd64 and
# checksum_arm64 are the same hash. Verify against whichever the workspace
# supplied rather than ignoring a checksum the user actually set.
CHECKSUM="${CHECKSUM_AMD64:-$CHECKSUM_ARM64}"
if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/Meslo.tar.xz" | sha256sum -c -
else
  printf '%sWARNING: SHA256 verification skipped for Nerd Fonts (no checksum for nerd-fonts in [plugins.versions])%s\n' "$C_YEL" "$C_RST" >&2
fi

mkdir -p ~/.fonts/Meslo
tar -xf /tmp/Meslo.tar.xz -C ~/.fonts/Meslo
fc-cache -fv
rm /tmp/Meslo.tar.xz
