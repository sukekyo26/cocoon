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
#   CHECKSUM_AMD64   : sha256 of amd64 binary; empty to skip verification
#   CHECKSUM_ARM64   : sha256 of arm64 binary; empty to skip verification
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

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "https://sukekyo26.github.io/cocoon/v${VERSION}/cocoon-linux-${DOWNLOAD_ARCH}" \
  -o /tmp/cocoon

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/cocoon" | sha256sum -c -
else
  printf '%sWARNING: SHA256 verification skipped for cocoon (no checksum for cocoon in [plugins.versions])%s\n' "$C_YEL" "$C_RST" >&2
fi

install -m 0755 /tmp/cocoon /usr/local/bin/cocoon
rm /tmp/cocoon
