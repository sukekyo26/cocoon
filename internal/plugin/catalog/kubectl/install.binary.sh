#!/usr/bin/env bash
# Install kubectl (https://github.com/kubernetes/kubectl)
#
# Inputs (env):
#   PIN              : kubectl version without leading "v" (e.g. "1.31.0"); empty = latest stable
#   CHECKSUM_AMD64   : sha256 of the amd64 binary; empty = verify against the
#                      upstream kubectl.sha256
#   CHECKSUM_ARM64   : sha256 of the arm64 binary; empty = verify against the
#                      upstream kubectl.sha256
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
  # dl.k8s.io/release/stable.txt is a plain-text "v1.31.0" line refreshed
  # by the Kubernetes release pipeline.
  VERSION=$(curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    https://dl.k8s.io/release/stable.txt | sed 's/^v//')
fi

url="https://dl.k8s.io/release/v${VERSION}/bin/linux/${ARCH}/kubectl"
curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "$url" -o /tmp/kubectl

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/kubectl" | sha256sum -c -
else
  # No user pin: verify against the .sha256 (a bare hash) dl.k8s.io serves
  # next to the binary.
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    "${url}.sha256" -o /tmp/kubectl.sum
  expected="$(cut -d ' ' -f1 /tmp/kubectl.sum)"
  echo "${expected}  /tmp/kubectl" | sha256sum -c -
  rm -f /tmp/kubectl.sum
fi

install -o root -g root -m 0755 /tmp/kubectl /usr/local/bin/kubectl
rm /tmp/kubectl
