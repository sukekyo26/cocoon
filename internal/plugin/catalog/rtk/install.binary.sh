#!/usr/bin/env bash
# Install rtk from the GitHub Releases binary tarball
# (https://github.com/rtk-ai/rtk/releases). Picked when
# raw.githubusercontent.com is unreachable (e.g. Zscaler) or policy
# forbids the `curl | sh` pattern that install.installer.sh uses.
#
# Inputs (env):
#   PIN                   : rtk version (without leading "v"); empty = latest
#   CHECKSUM_AMD64        : sha256 of rtk-x86_64-unknown-linux-musl.tar.gz;
#                           empty = verify against the release checksums.txt
#   CHECKSUM_ARM64        : sha256 of rtk-aarch64-unknown-linux-gnu.tar.gz;
#                           empty = verify against the release checksums.txt
#   COCOON_INSTALL_METHOD : selected install method; must equal "binary"
set -euo pipefail

: "${COCOON_INSTALL_METHOD:?missing}"
if [ "$COCOON_INSTALL_METHOD" != "binary" ]; then
  echo "install.binary.sh invoked with COCOON_INSTALL_METHOD=$COCOON_INSTALL_METHOD; expected binary" >&2
  exit 1
fi

REPO="rtk-ai/rtk"
# Upstream release asset names are asymmetric: amd64 ships a musl static
# build, arm64 ships a gnu build. Map each Debian arch to its full target
# triple so the tarball name resolves correctly.
ARCH="$(dpkg --print-architecture)"
case "$ARCH" in
  amd64)
    TARGET="x86_64-unknown-linux-musl"
    CHECKSUM="$CHECKSUM_AMD64"
    ;;
  arm64)
    TARGET="aarch64-unknown-linux-gnu"
    CHECKSUM="$CHECKSUM_ARM64"
    ;;
  *)
    echo "rtk: unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

if [ -n "$PIN" ]; then
  VERSION="$PIN"
else
  VERSION=$(curl -fsSLI --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    -o /dev/null -w '%{url_effective}' https://github.com/${REPO}/releases/latest |
    sed 's|.*/tag/v||')
fi

base="https://github.com/${REPO}/releases/download/v${VERSION}"
TARBALL="rtk-${TARGET}.tar.gz"
curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "${base}/${TARBALL}" \
  -o "/tmp/${TARBALL}"

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/${TARBALL}" | sha256sum -c -
else
  # No user checksum: verify against the release's own checksums.txt.
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    "${base}/checksums.txt" -o /tmp/rtk.sums
  # Match the asset name literally (awk field compare, not a regex; strip the
  # binary-mode '*' marker some manifests prepend) and fail loudly if it is
  # absent so a manifest-shape change does not collapse into an opaque error.
  expected="$(awk -v f="$TARBALL" '{ n = $2; sub(/^\*/, "", n); if (n == f) { print $1; exit } }' /tmp/rtk.sums)"
  if [ -z "$expected" ]; then
    echo "rtk: ${TARBALL} not found in checksums.txt" >&2
    exit 1
  fi
  echo "${expected}  /tmp/${TARBALL}" | sha256sum -c -
  rm -f /tmp/rtk.sums
fi

# The release tarball is a single self-contained binary at the archive
# root (no leading directory), so a straight `tar -xzf` into ~/.local/bin
# lands the binary directly on PATH (install.env adds ~/.local/bin to
# PATH). NB: do NOT add --strip-components=1 — that would silently skip
# the file because there is no leading path component to strip.
mkdir -p "$HOME/.local/bin"
tar -C "$HOME/.local/bin" -xzf "/tmp/${TARBALL}"
# Tarball entry already ships an executable mode, but re-assert it in case
# a future release ships a more permissive umask-dependent mode.
chmod 0755 "$HOME/.local/bin/rtk"

rm "/tmp/${TARBALL}"
