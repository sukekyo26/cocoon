#!/usr/bin/env bash
# Install golangci-lint (https://github.com/golangci/golangci-lint)
#
# Inputs (env):
#   PIN              : golangci-lint version (without leading "v"); empty = latest
#   CHECKSUM_AMD64   : sha256 of amd64 tarball; empty = verify against the
#                      release checksums.txt
#   CHECKSUM_ARM64   : sha256 of arm64 tarball; empty = verify against the
#                      release checksums.txt
set -euo pipefail

# golangci-lint's linux asset names use the same amd64/arm64 tokens dpkg
# reports, so no architecture remap is needed (unlike lazygit's x86_64).
ARCH="$(dpkg --print-architecture)"
case "$ARCH" in
  amd64)
    DOWNLOAD_ARCH="amd64"
    CHECKSUM="$CHECKSUM_AMD64"
    ;;
  arm64)
    DOWNLOAD_ARCH="arm64"
    CHECKSUM="$CHECKSUM_ARM64"
    ;;
  *)
    DOWNLOAD_ARCH="amd64"
    CHECKSUM="$CHECKSUM_AMD64"
    ;;
esac

if [ -n "$PIN" ]; then
  VERSION="$PIN"
else
  VERSION=$(curl -fsSLI --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    -o /dev/null -w '%{url_effective}' https://github.com/golangci/golangci-lint/releases/latest |
    sed 's|.*/tag/v||')
fi

base="https://github.com/golangci/golangci-lint/releases/download/v${VERSION}"
asset="golangci-lint-${VERSION}-linux-${DOWNLOAD_ARCH}.tar.gz"

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "${base}/${asset}" -o /tmp/golangci-lint.tar.gz

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/golangci-lint.tar.gz" | sha256sum -c -
else
  # No user checksum: verify against the release's own checksums.txt.
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    "${base}/golangci-lint-${VERSION}-checksums.txt" -o /tmp/golangci-lint.sums
  # Match the asset name literally (awk field compare, not a regex); strip the
  # binary-mode '*' marker some manifests prepend. Fail loudly if absent so a
  # shape change does not collapse into an opaque error.
  expected="$(awk -v f="$asset" '{ n = $2; sub(/^\*/, "", n); if (tolower(n) == tolower(f)) { print $1; exit } }' /tmp/golangci-lint.sums)"
  if [ -z "$expected" ]; then
    echo "golangci-lint: ${asset} not found in checksums.txt" >&2
    exit 1
  fi
  echo "${expected}  /tmp/golangci-lint.tar.gz" | sha256sum -c -
  rm -f /tmp/golangci-lint.sums
fi

# The binary lives under a version-named top-level directory inside the tarball
# (golangci-lint-<ver>-linux-<arch>/golangci-lint); strip that component so the
# binary lands directly on PATH.
tar -xzf /tmp/golangci-lint.tar.gz -C /usr/local/bin --strip-components=1 \
  "golangci-lint-${VERSION}-linux-${DOWNLOAD_ARCH}/golangci-lint"
rm /tmp/golangci-lint.tar.gz
