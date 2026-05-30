#!/usr/bin/env bash
# Install Flutter SDK (https://github.com/flutter/flutter)
#
# Inputs (env):
#   PIN              : Flutter version (e.g. "3.44.0"); empty = latest stable
#   CHECKSUM_AMD64   : sha256 of linux-x64 tarball; empty = verify against the
#                      sha256 recorded in Flutter's releases_linux.json
#   CHECKSUM_ARM64   : unused — Flutter Linux is x86_64 only
#
# Note: Flutter does not publish official Linux/arm64 builds. On arm64 hosts
# the install fails fast and asks the user to run the container under
# `docker --platform linux/amd64` (Docker Desktop does this automatically on
# Apple Silicon, so this only affects native arm64 Linux hosts).
set -euo pipefail

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

# The releases manifest carries both the version->hash mapping and each
# build's sha256, so fetch it once and reuse it for version resolution and
# integrity verification.
MANIFEST=$(curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  https://storage.googleapis.com/flutter_infra_release/releases/releases_linux.json)

if [ -n "$PIN" ]; then
  VERSION="$PIN"
else
  # Cross-reference current_release.stable -> releases[].hash to pick the
  # correct version without depending on jq or the GitHub API.
  STABLE_HASH=$(printf '%s' "$MANIFEST" | tr -d '\n' |
    sed -n 's/.*"current_release"[[:space:]]*:[[:space:]]*{[^}]*"stable"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
  if [ -z "$STABLE_HASH" ]; then
    echo "Failed to read current_release.stable from Flutter manifest" >&2
    exit 1
  fi
  # `|| true` keeps the command substitution non-fatal: under `set -euo
  # pipefail`, a no-match from `grep -oE` (e.g. manifest shape change) would
  # otherwise exit the script via the pipeline's non-zero status before the
  # friendly `-z "$VERSION"` check below could run.
  VERSION=$(printf '%s' "$MANIFEST" | tr -d '\n' |
    grep -oE "\\{[^{}]*\"hash\"[[:space:]]*:[[:space:]]*\"${STABLE_HASH}\"[^{}]*\\}" |
    sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1) || true
  if [ -z "$VERSION" ]; then
    echo "Failed to resolve latest Flutter stable version" >&2
    exit 1
  fi
fi

archive="stable/linux/flutter_linux_${VERSION}-stable.tar.xz"
curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "https://storage.googleapis.com/flutter_infra_release/releases/${archive}" \
  -o /tmp/flutter.tar.xz

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/flutter.tar.xz" | sha256sum -c -
else
  # No user checksum: verify against the sha256 the manifest records for this build.
  # Escape the dots in the archive path so they match literally rather than as
  # ERE wildcards (the path carries version dots like 3.44.0 and .tar.xz).
  archive_re="${archive//./\\.}"
  expected=$(printf '%s' "$MANIFEST" | tr -d '\n' |
    grep -oE "\\{[^{}]*\"archive\"[[:space:]]*:[[:space:]]*\"${archive_re}\"[^{}]*\\}" |
    sed -n 's/.*"sha256"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1) || true
  if [ -z "$expected" ]; then
    echo "Failed to read sha256 for ${archive} from Flutter manifest" >&2
    exit 1
  fi
  echo "${expected}  /tmp/flutter.tar.xz" | sha256sum -c -
fi

# Tarball root is "flutter/" — extract directly into /usr/local.
tar -C /usr/local -xJf /tmp/flutter.tar.xz
rm /tmp/flutter.tar.xz

# Flutter writes inside its own SDK tree at runtime (bin/cache/ for engine
# binaries, dart-sdk stamps, pre-cached artifacts), so the non-root container
# user must own /usr/local/flutter. Chown inside the same RUN keeps the
# ownership flip in a single image layer (no duplicated 700 MB SDK).
chown -R "${USERNAME}:${USERNAME}" /usr/local/flutter
