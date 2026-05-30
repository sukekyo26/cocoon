#!/usr/bin/env bash
# Install cocoon (https://github.com/sukekyo26/cocoon)
#
# Downloads the cocoon binary from the GitHub Pages mirror at
# https://sukekyo26.github.io/cocoon/ rather than the GitHub Releases
# CDN. Pages-only by design — no fallback to the releases/download
# path — so the plugin works in environments that can reach
# *.github.io but not GitHub's raw / API hosts.
#
# Inputs (env):
#   PIN              : cocoon version (without leading "v"); empty = latest
#   CHECKSUM_AMD64   : sha256 of amd64 binary; empty = verify against the
#                      SHA256SUMS published in the Pages mirror
#   CHECKSUM_ARM64   : sha256 of arm64 binary; empty = verify against the
#                      SHA256SUMS published in the Pages mirror
set -euo pipefail

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
    # The Pages mirror only publishes linux-amd64 / linux-arm64. Fail
    # loud here rather than silently download the amd64 binary and trip
    # `exec format error` at first run (the rest of the catalog defaults
    # to amd64 for unknown arch, but those plugins target upstreams that
    # publish more architectures — cocoon's mirror is strictly two).
    echo "cocoon plugin: unsupported architecture '${ARCH}' — Pages mirror publishes linux-amd64 and linux-arm64 only" >&2
    exit 1
    ;;
esac

# Pages mirror VERSION file is a single line "X.Y.Z" (no leading "v");
# strip any whitespace so the URL below stays well-formed regardless of
# how the mirror serves it. When PIN is set, accept a leading "v"
# defensively so users who copy a tag name don't end up with a
# /vv0.7.4/ path.
if [ -n "$PIN" ]; then
  VERSION="${PIN#v}"
else
  VERSION=$(curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    https://sukekyo26.github.io/cocoon/VERSION | tr -d '[:space:]')
fi

base="https://sukekyo26.github.io/cocoon/v${VERSION}"
curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "${base}/cocoon-linux-${DOWNLOAD_ARCH}" -o /tmp/cocoon

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/cocoon" | sha256sum -c -
else
  # No user checksum: verify against the SHA256SUMS the Pages mirror publishes
  # alongside the binaries for this version.
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    "${base}/SHA256SUMS" -o /tmp/cocoon.sums
  # Match the asset name literally (awk field compare, not a regex; strip the
  # binary-mode '*' marker some manifests prepend) and fail loudly if it is
  # absent so a mirror-layout change does not collapse into an opaque error.
  asset="cocoon-linux-${DOWNLOAD_ARCH}"
  expected="$(awk -v f="$asset" '{ n = $2; sub(/^\*/, "", n); if (n == f) { print $1; exit } }' /tmp/cocoon.sums)"
  if [ -z "$expected" ]; then
    echo "cocoon: ${asset} not found in SHA256SUMS" >&2
    exit 1
  fi
  echo "${expected}  /tmp/cocoon" | sha256sum -c -
  rm -f /tmp/cocoon.sums
fi

install -m 0755 /tmp/cocoon /usr/local/bin/cocoon
rm /tmp/cocoon
