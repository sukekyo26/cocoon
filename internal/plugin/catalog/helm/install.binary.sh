#!/usr/bin/env bash
# Install Helm (https://github.com/helm/helm)
#
# Inputs (env):
#   PIN              : Helm version without leading "v" (e.g. "3.16.0"); empty = latest stable
#   CHECKSUM_AMD64   : sha256 of the amd64 tarball; empty = verify against the
#                      per-tarball .sha256sum published on get.helm.sh
#   CHECKSUM_ARM64   : sha256 of the arm64 tarball; empty = verify against the
#                      per-tarball .sha256sum published on get.helm.sh
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
  # github.com/helm/helm/releases/latest redirects to the newest stable tag
  # (e.g. /releases/tag/v3.16.0). `-I` keeps the body off the wire.
  VERSION=$(curl -fsSLI --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    -o /dev/null -w '%{url_effective}' https://github.com/helm/helm/releases/latest |
    sed 's|.*/tag/v||')
fi

url="https://get.helm.sh/helm-v${VERSION}-linux-${ARCH}.tar.gz"
curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "$url" -o /tmp/helm.tar.gz

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/helm.tar.gz" | sha256sum -c -
else
  # No user pin: verify against the per-tarball .sha256sum on get.helm.sh.
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    "${url}.sha256sum" -o /tmp/helm.sum
  expected="$(cut -d ' ' -f1 /tmp/helm.sum)"
  echo "${expected}  /tmp/helm.tar.gz" | sha256sum -c -
  rm -f /tmp/helm.sum
fi

# The tarball ships `linux-<arch>/helm` so --strip-components=1 + selective
# extract drops just the binary into /usr/local/bin without leaking the
# README / LICENSE siblings.
tar -xzf /tmp/helm.tar.gz -C /usr/local/bin --strip-components=1 "linux-${ARCH}/helm"
chmod +x /usr/local/bin/helm
rm /tmp/helm.tar.gz
