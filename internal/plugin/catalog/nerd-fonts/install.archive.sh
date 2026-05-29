#!/usr/bin/env bash
# Install Nerd Fonts (Meslo) (https://github.com/ryanoasis/nerd-fonts)
#
# Inputs (env):
#   PIN              : Nerd Fonts version (without leading "v"); empty = latest
#   CHECKSUM_AMD64   : sha256 of Meslo.tar.xz; empty = verify against the
#                      release SHA-256.txt
#   CHECKSUM_ARM64   : same artifact — Meslo.tar.xz is architecture-independent;
#                      consulted as a fallback when CHECKSUM_AMD64 is empty
set -euo pipefail

if [ -n "$PIN" ]; then
  base="https://github.com/ryanoasis/nerd-fonts/releases/download/v${PIN}"
else
  base="https://github.com/ryanoasis/nerd-fonts/releases/latest/download"
fi

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "${base}/Meslo.tar.xz" -o /tmp/Meslo.tar.xz

# Meslo.tar.xz is architecture-independent, so checksum_amd64 and
# checksum_arm64 are the same hash. Verify against whichever the workspace
# supplied rather than ignoring a checksum the user actually set.
CHECKSUM="${CHECKSUM_AMD64:-$CHECKSUM_ARM64}"
if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/Meslo.tar.xz" | sha256sum -c -
else
  # No user pin: verify against the release's own SHA-256.txt.
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    "${base}/SHA-256.txt" -o /tmp/nerd.sums
  expected="$(grep "Meslo.tar.xz\$" /tmp/nerd.sums | cut -d ' ' -f1)"
  echo "${expected}  /tmp/Meslo.tar.xz" | sha256sum -c -
  rm -f /tmp/nerd.sums
fi

mkdir -p ~/.fonts/Meslo
tar -xf /tmp/Meslo.tar.xz -C ~/.fonts/Meslo
fc-cache -fv
rm /tmp/Meslo.tar.xz
