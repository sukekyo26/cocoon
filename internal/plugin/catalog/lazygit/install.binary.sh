#!/usr/bin/env bash
# Install lazygit (https://github.com/jesseduffield/lazygit)
#
# Inputs (env):
#   PIN              : lazygit version (without leading "v"); empty = latest
#   CHECKSUM_AMD64   : sha256 of amd64 tarball; empty = verify against the
#                      release checksums.txt
#   CHECKSUM_ARM64   : sha256 of arm64 tarball; empty = verify against the
#                      release checksums.txt
set -euo pipefail

ARCH="$(dpkg --print-architecture)"
case "$ARCH" in
  amd64)
    DOWNLOAD_ARCH="x86_64"
    CHECKSUM="$CHECKSUM_AMD64"
    ;;
  arm64)
    DOWNLOAD_ARCH="arm64"
    CHECKSUM="$CHECKSUM_ARM64"
    ;;
  *)
    DOWNLOAD_ARCH="x86_64"
    CHECKSUM="$CHECKSUM_AMD64"
    ;;
esac

if [ -n "$PIN" ]; then
  VERSION="$PIN"
else
  VERSION=$(curl -fsSLI --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    -o /dev/null -w '%{url_effective}' https://github.com/jesseduffield/lazygit/releases/latest |
    sed 's|.*/tag/v||')
fi

base="https://github.com/jesseduffield/lazygit/releases/download/v${VERSION}"
asset="lazygit_${VERSION}_linux_${DOWNLOAD_ARCH}.tar.gz"

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "${base}/${asset}" -o /tmp/lazygit.tar.gz

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/lazygit.tar.gz" | sha256sum -c -
else
  # No user pin: verify against the release's own checksums.txt, fetched from
  # the same release.
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    "${base}/checksums.txt" -o /tmp/lazygit.sums
  # Match the asset name literally (awk field compare, not a regex); tolower()
  # tolerates the manifest's "Linux" capitalisation and we strip the binary-mode
  # '*' marker some manifests prepend. Fail loudly if absent so a shape change
  # does not collapse into an opaque error.
  expected="$(awk -v f="$asset" '{ n = $2; sub(/^\*/, "", n); if (tolower(n) == tolower(f)) { print $1; exit } }' /tmp/lazygit.sums)"
  if [ -z "$expected" ]; then
    echo "lazygit: ${asset} not found in checksums.txt" >&2
    exit 1
  fi
  echo "${expected}  /tmp/lazygit.tar.gz" | sha256sum -c -
  rm -f /tmp/lazygit.sums
fi

tar -xzf /tmp/lazygit.tar.gz -C /usr/local/bin lazygit
rm /tmp/lazygit.tar.gz
