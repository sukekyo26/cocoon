#!/usr/bin/env bash
# Install ShellCheck (https://github.com/koalaman/shellcheck)
#
# Inputs (env):
#   PIN              : ShellCheck version without leading "v" (e.g. "0.10.0"); empty = latest
#   CHECKSUM_AMD64   : sha256 of the x86_64 tarball; empty to skip verification
#   CHECKSUM_ARM64   : sha256 of the aarch64 tarball; empty to skip verification
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
    DOWNLOAD_ARCH="x86_64"
    CHECKSUM="$CHECKSUM_AMD64"
    ;;
  arm64)
    DOWNLOAD_ARCH="aarch64"
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
    -o /dev/null -w '%{url_effective}' https://github.com/koalaman/shellcheck/releases/latest |
    sed 's|.*/tag/v||')
fi

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "https://github.com/koalaman/shellcheck/releases/download/v${VERSION}/shellcheck-v${VERSION}.linux.${DOWNLOAD_ARCH}.tar.xz" \
  -o /tmp/shellcheck.tar.xz

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/shellcheck.tar.xz" | sha256sum -c -
else
  printf '%sWARNING: SHA256 verification skipped for ShellCheck (no checksum for shellcheck in [plugins.versions])%s\n' "$C_YEL" "$C_RST" >&2
fi

# The tarball ships `shellcheck-v<ver>/shellcheck`; --strip-components=1 +
# selective extract lands just the binary in /usr/local/bin.
tar -xJf /tmp/shellcheck.tar.xz -C /usr/local/bin --strip-components=1 \
  "shellcheck-v${VERSION}/shellcheck"
chmod +x /usr/local/bin/shellcheck
rm /tmp/shellcheck.tar.xz
