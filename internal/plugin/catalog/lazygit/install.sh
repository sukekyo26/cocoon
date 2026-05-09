#!/usr/bin/env bash
# Install lazygit (https://github.com/jesseduffield/lazygit)
#
# Inputs (env):
#   PIN              : lazygit version (without leading "v"); empty = latest
#   CHECKSUM_AMD64   : sha256 of amd64 tarball; empty to skip verification
#   CHECKSUM_ARM64   : sha256 of arm64 tarball; empty to skip verification
set -euo pipefail

ARCH="$(dpkg --print-architecture)"
case "$ARCH" in
  amd64) DOWNLOAD_ARCH="x86_64" ; CHECKSUM="$CHECKSUM_AMD64" ;;
  arm64) DOWNLOAD_ARCH="arm64"  ; CHECKSUM="$CHECKSUM_ARM64" ;;
  *)     DOWNLOAD_ARCH="x86_64" ; CHECKSUM="$CHECKSUM_AMD64" ;;
esac

if [ -n "$PIN" ]; then
  VERSION="$PIN"
else
  VERSION=$(curl -fsSLI --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    -o /dev/null -w '%{url_effective}' https://github.com/jesseduffield/lazygit/releases/latest \
    | sed 's|.*/tag/v||')
fi

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "https://github.com/jesseduffield/lazygit/releases/download/v${VERSION}/lazygit_${VERSION}_linux_${DOWNLOAD_ARCH}.tar.gz" \
  -o /tmp/lazygit.tar.gz

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/lazygit.tar.gz" | sha256sum -c -
else
  echo "WARNING: SHA256 verification skipped for lazygit (no checksum provided in [plugins.versions.lazygit])" >&2
fi

tar -xzf /tmp/lazygit.tar.gz -C /usr/local/bin lazygit
rm /tmp/lazygit.tar.gz
