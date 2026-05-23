#!/usr/bin/env bash
# Install Flutter SDK (https://github.com/flutter/flutter)
#
# Inputs (env):
#   PIN              : Flutter version (e.g. "3.44.0"); empty = latest stable
#   CHECKSUM_AMD64   : sha256 of linux-x64 tarball; empty to skip verification
#   CHECKSUM_ARM64   : unused — Flutter Linux is x86_64 only
#
# Note: Flutter does not publish official Linux/arm64 builds. On arm64 hosts
# the install fails fast and asks the user to run the container under
# `docker --platform linux/amd64` (Docker Desktop does this automatically on
# Apple Silicon, so this only affects native arm64 Linux hosts).
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
    CHECKSUM="$CHECKSUM_AMD64"
    ;;
  *)
    echo "ERROR: Flutter does not publish official Linux/${ARCH} builds." >&2
    echo "       Re-run the container under 'docker --platform linux/amd64'" >&2
    echo "       (Docker Desktop does this automatically on Apple Silicon)." >&2
    exit 1
    ;;
esac

if [ -n "$PIN" ]; then
  VERSION="$PIN"
else
  # Resolve latest stable from the official releases manifest. Cross-reference
  # current_release.stable -> releases[].hash to pick the correct version
  # without depending on jq or the GitHub API.
  MANIFEST=$(curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    https://storage.googleapis.com/flutter_infra_release/releases/releases_linux.json)
  STABLE_HASH=$(printf '%s' "$MANIFEST" | tr -d '\n' |
    sed -n 's/.*"current_release"[[:space:]]*:[[:space:]]*{[^}]*"stable"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
  if [ -z "$STABLE_HASH" ]; then
    echo "Failed to read current_release.stable from Flutter manifest" >&2
    exit 1
  fi
  VERSION=$(printf '%s' "$MANIFEST" | tr -d '\n' |
    grep -oE "\\{[^{}]*\"hash\"[[:space:]]*:[[:space:]]*\"${STABLE_HASH}\"[^{}]*\\}" |
    sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)
  if [ -z "$VERSION" ]; then
    echo "Failed to resolve latest Flutter stable version" >&2
    exit 1
  fi
fi

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "https://storage.googleapis.com/flutter_infra_release/releases/stable/linux/flutter_linux_${VERSION}-stable.tar.xz" \
  -o /tmp/flutter.tar.xz

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/flutter.tar.xz" | sha256sum -c -
else
  printf '%sWARNING: SHA256 verification skipped for Flutter (no checksum for flutter in [plugins.versions])%s\n' "$C_YEL" "$C_RST" >&2
fi

# Tarball root is "flutter/" — extract directly into /usr/local.
tar -C /usr/local -xJf /tmp/flutter.tar.xz
rm /tmp/flutter.tar.xz
