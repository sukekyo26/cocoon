#!/usr/bin/env bash
# Install kubectl (https://github.com/kubernetes/kubectl)
#
# Inputs (env):
#   PIN              : kubectl version without leading "v" (e.g. "1.31.0"); empty = latest stable
#   CHECKSUM_AMD64   : sha256 of the amd64 binary; empty to skip verification
#   CHECKSUM_ARM64   : sha256 of the arm64 binary; empty to skip verification
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

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "https://dl.k8s.io/release/v${VERSION}/bin/linux/${ARCH}/kubectl" -o /tmp/kubectl

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/kubectl" | sha256sum -c -
else
  printf '%sWARNING: SHA256 verification skipped for kubectl (no checksum for kubectl in [plugins.versions])%s\n' "$C_YEL" "$C_RST" >&2
fi

install -o root -g root -m 0755 /tmp/kubectl /usr/local/bin/kubectl
rm /tmp/kubectl
