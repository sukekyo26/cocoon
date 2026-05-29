#!/usr/bin/env bash
# Install gitleaks (https://github.com/gitleaks/gitleaks)
#
# Inputs (env):
#   PIN              : gitleaks version (without leading "v"); empty = latest
#   CHECKSUM_AMD64   : sha256 of amd64 tarball; empty = verify against the
#                      release checksums file
#   CHECKSUM_ARM64   : sha256 of arm64 tarball; empty = verify against the
#                      release checksums file
set -euo pipefail

# gitleaks asset names use x64 / arm64 (NOT x86_64 / aarch64).
ARCH="$(dpkg --print-architecture)"
case "$ARCH" in
  amd64)
    DOWNLOAD_ARCH="x64"
    CHECKSUM="$CHECKSUM_AMD64"
    ;;
  arm64)
    DOWNLOAD_ARCH="arm64"
    CHECKSUM="$CHECKSUM_ARM64"
    ;;
  *)
    DOWNLOAD_ARCH="x64"
    CHECKSUM="$CHECKSUM_AMD64"
    ;;
esac

if [ -n "$PIN" ]; then
  VERSION="$PIN"
else
  VERSION=$(curl -fsSLI --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    -o /dev/null -w '%{url_effective}' https://github.com/gitleaks/gitleaks/releases/latest |
    sed 's|.*/tag/v||')
fi

base="https://github.com/gitleaks/gitleaks/releases/download/v${VERSION}"
asset="gitleaks_${VERSION}_linux_${DOWNLOAD_ARCH}.tar.gz"

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "${base}/${asset}" -o /tmp/gitleaks.tar.gz

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/gitleaks.tar.gz" | sha256sum -c -
else
  # No user pin: verify against the release's own checksums file.
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    "${base}/gitleaks_${VERSION}_checksums.txt" -o /tmp/gitleaks.sums
  expected="$(grep "${asset}\$" /tmp/gitleaks.sums | cut -d ' ' -f1)"
  echo "${expected}  /tmp/gitleaks.tar.gz" | sha256sum -c -
  rm -f /tmp/gitleaks.sums
fi

tar -xzf /tmp/gitleaks.tar.gz -C /usr/local/bin gitleaks
rm /tmp/gitleaks.tar.gz
