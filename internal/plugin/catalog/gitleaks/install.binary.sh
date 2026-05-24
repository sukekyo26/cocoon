#!/usr/bin/env bash
# Install gitleaks (https://github.com/gitleaks/gitleaks)
#
# Inputs (env):
#   PIN              : gitleaks version (without leading "v"); empty = latest
#   CHECKSUM_AMD64   : sha256 of amd64 tarball; empty to skip verification
#   CHECKSUM_ARM64   : sha256 of arm64 tarball; empty to skip verification
set -euo pipefail

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

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "https://github.com/gitleaks/gitleaks/releases/download/v${VERSION}/gitleaks_${VERSION}_linux_${DOWNLOAD_ARCH}.tar.gz" \
  -o /tmp/gitleaks.tar.gz

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/gitleaks.tar.gz" | sha256sum -c -
else
  printf '%sWARNING: SHA256 verification skipped for gitleaks (no checksum for gitleaks in [plugins.versions])%s\n' "$C_YEL" "$C_RST" >&2
fi

tar -xzf /tmp/gitleaks.tar.gz -C /usr/local/bin gitleaks
rm /tmp/gitleaks.tar.gz
