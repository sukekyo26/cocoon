#!/usr/bin/env bash
# Install GitHub Copilot CLI from the GitHub Releases binary tarball
# (https://github.com/github/copilot-cli/releases). Picked when gh.io is
# blocked (e.g. Zscaler) or corporate policy forbids the `curl | bash`
# pattern that install.installer.sh uses.
#
# Inputs (env):
#   PIN                   : Copilot CLI version (without leading "v"); empty = latest
#   CHECKSUM_AMD64        : sha256 of copilot-linux-x64.tar.gz; empty = verify
#                           against the release SHA256SUMS.txt
#   CHECKSUM_ARM64        : sha256 of copilot-linux-arm64.tar.gz; empty = verify
#                           against the release SHA256SUMS.txt
#   COCOON_INSTALL_METHOD : selected install method; must equal "binary"
set -euo pipefail

: "${COCOON_INSTALL_METHOD:?missing}"
if [ "$COCOON_INSTALL_METHOD" != "binary" ]; then
  echo "install.binary.sh invoked with COCOON_INSTALL_METHOD=$COCOON_INSTALL_METHOD; expected binary" >&2
  exit 1
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

base="https://github.com/github/copilot-cli/releases/download/v${VERSION}"
TARBALL="copilot-linux-${DOWNLOAD_ARCH}.tar.gz"
curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "${base}/${TARBALL}" \
  -o "/tmp/${TARBALL}"

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/${TARBALL}" | sha256sum -c -
else
  # No user pin: verify against the release's own SHA256SUMS.txt.
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    "${base}/SHA256SUMS.txt" -o /tmp/copilot.sums
  expected="$(grep "${TARBALL}\$" /tmp/copilot.sums | cut -d ' ' -f1)"
  echo "${expected}  /tmp/${TARBALL}" | sha256sum -c -
  rm -f /tmp/copilot.sums
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
