#!/usr/bin/env bash
# Install GitHub Copilot CLI from the GitHub Releases binary tarball
# (https://github.com/github/copilot-cli/releases). Picked when gh.io is
# blocked (e.g. Zscaler) or corporate policy forbids the `curl | bash`
# pattern that install.installer.sh uses.
#
# Inputs (env):
#   PIN                   : Copilot CLI version (without leading "v"); empty = latest
#   CHECKSUM_AMD64        : sha256 of copilot-linux-x64.tar.gz; empty to skip verification
#   CHECKSUM_ARM64        : sha256 of copilot-linux-arm64.tar.gz; empty to skip verification
#   COCOON_INSTALL_METHOD : selected install method; must equal "binary"
set -euo pipefail

: "${COCOON_INSTALL_METHOD:?missing}"
if [ "$COCOON_INSTALL_METHOD" != "binary" ]; then
  echo "install.binary.sh invoked with COCOON_INSTALL_METHOD=$COCOON_INSTALL_METHOD; expected binary" >&2
  exit 1
fi

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
    -o /dev/null -w '%{url_effective}' https://github.com/github/copilot-cli/releases/latest |
    sed 's|.*/tag/v||')
fi

TARBALL="copilot-linux-${DOWNLOAD_ARCH}.tar.gz"
curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "https://github.com/github/copilot-cli/releases/download/v${VERSION}/${TARBALL}" \
  -o "/tmp/${TARBALL}"

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/${TARBALL}" | sha256sum -c -
else
  printf '%sWARNING: SHA256 verification skipped for Copilot CLI (no checksum for copilot-cli in [plugins.versions])%s\n' "$C_YEL" "$C_RST" >&2
fi

# The release tarball is a single self-contained ELF binary at the
# archive root (no leading directory), so a straight `tar -xzf` into
# ~/.local/bin lands the launcher directly on PATH (install.env adds
# ~/.local/bin to PATH). NB: do NOT add --strip-components=1 — that
# would silently skip the file because there is no leading path
# component to strip.
mkdir -p "$HOME/.local/bin"
tar -C "$HOME/.local/bin" -xzf "/tmp/${TARBALL}"
# Tarball entry already ships mode 0755, but re-assert it in case a
# future release ships a more permissive umask-dependent mode.
chmod 0755 "$HOME/.local/bin/copilot"

rm "/tmp/${TARBALL}"
