#!/usr/bin/env bash
# Install shfmt (https://github.com/mvdan/sh)
#
# Inputs (env):
#   PIN              : shfmt version without leading "v" (e.g. "3.10.0"); empty = latest
#   CHECKSUM_AMD64   : sha256 of the amd64 binary; empty = verify against the
#                      release sha256sums.txt
#   CHECKSUM_ARM64   : sha256 of the arm64 binary; empty = verify against the
#                      release sha256sums.txt
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
  # No user pin: verify against the release's own sha256sums.txt.
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    "${base}/sha256sums.txt" -o /tmp/shfmt.sums
  # Match the asset name literally (awk field compare, not a regex; strip the
  # binary-mode '*' marker some manifests prepend) and fail loudly if it is
  # absent so a manifest-shape change does not collapse into an opaque error.
  expected="$(awk -v f="$asset" '{ n = $2; sub(/^\*/, "", n); if (n == f) { print $1; exit } }' /tmp/shfmt.sums)"
  if [ -z "$expected" ]; then
    echo "shfmt: ${asset} not found in sha256sums.txt" >&2
    exit 1
  fi
  echo "${expected}  /tmp/shfmt" | sha256sum -c -
  rm -f /tmp/shfmt.sums
fi

install -o root -g root -m 0755 /tmp/shfmt /usr/local/bin/shfmt
rm /tmp/shfmt
