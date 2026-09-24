#!/usr/bin/env bash
# Install Android Studio (https://developer.android.com/studio)
#
# Inputs (env):
#   PIN            : "<version>-<codename>" (e.g. "2026.1.4.8-quail4-patch1");
#                    empty = latest (scraped from https://developer.android.com/studio)
#   CHECKSUM_AMD64 : sha256 of the Linux tarball; empty = use the SHA-256 the
#                    /studio page lists when it offers the same file, else warn
#   CHECKSUM_ARM64 : unused (Android Studio for Linux is x86_64 only)
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
if [ "$ARCH" != "amd64" ]; then
  echo "ERROR: Android Studio for Linux is x86_64 only (this image is ${ARCH})." >&2
  echo "       Remove android-studio from [plugins].enable on this architecture." >&2
  exit 1
fi

# The /studio page is the only upstream index: it links the current Linux
# tarball and lists its SHA-256 in the download table. It is fetched even for
# a pinned version, to reuse the listed checksum when the pin is current.
STUDIO_HTML=$(curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  https://developer.android.com/studio | tr -d '\n') || STUDIO_HTML=""

# The tarball name carries the release codename, not the version, so both are
# needed to build the URL (the version-only name 404s). The version has no
# "-", so the first "-" of the pin separates the two.
if [ -n "$PIN" ]; then
  if ! printf '%s' "$PIN" | grep -qE '^[0-9.]+-[a-z0-9-]+$'; then
    echo "ERROR: android-studio pin \"${PIN}\" is not <version>-<codename>." >&2
    echo "       Pin as \"android-studio=2026.1.4.8-quail4-patch1\": the version and codename" >&2
    echo "       are the two parts of .../ide-zips/<version>/android-studio-<codename>-linux.tar.gz" >&2
    echo "       at https://developer.android.com/studio/archive." >&2
    exit 1
  fi
  VERSION="${PIN%%-*}"
  CODENAME="${PIN#*-}"
else
  LATEST=$(printf '%s' "$STUDIO_HTML" |
    grep -oE 'ide-zips/[0-9.]+/android-studio-[a-z0-9-]+-linux\.tar\.gz' | head -n 1 |
    sed -n 's#ide-zips/\([0-9.]*\)/android-studio-\([a-z0-9-]*\)-linux\.tar\.gz#\1-\2#p') || true
  if [ -z "$LATEST" ]; then
    echo "ERROR: failed to resolve the latest Android Studio Linux tarball from https://developer.android.com/studio." >&2
    echo "       Pin explicitly in the enable array: \"android-studio=<version>-<codename>\"" >&2
    echo "       (e.g. \"android-studio=2026.1.4.8-quail4-patch1\"; see https://developer.android.com/studio/archive)." >&2
    exit 1
  fi
  VERSION="${LATEST%%-*}"
  CODENAME="${LATEST#*-}"
fi
TARBALL="android-studio-${CODENAME}-linux.tar.gz"

# Checksum precedence: the user's checksum_amd64, else the SHA-256 the /studio
# download table lists for this exact file name (only the current release).
CHECKSUM="${CHECKSUM_AMD64:-}"
if [ -z "$CHECKSUM" ]; then
  CHECKSUM=$(printf '%s' "$STUDIO_HTML" |
    grep -oE ">${TARBALL//./\\.}</button>[[:space:]]*</td>[[:space:]]*<td>[^<]*</td>[[:space:]]*<td>[0-9a-f]{64}</td>" |
    head -n 1 | grep -oE '[0-9a-f]{64}') || true
fi

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "https://dl.google.com/android/studio/ide-zips/${VERSION}/${TARBALL}" \
  -o /tmp/android-studio.tar.gz

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/android-studio.tar.gz" | sha256sum -c -
else
  if [ -z "$STUDIO_HTML" ]; then
    REASON="could not fetch https://developer.android.com/studio"
  else
    REASON="https://developer.android.com/studio lists no SHA-256 for ${TARBALL}: not the current release, or the page layout changed"
  fi
  printf '%sWARNING: SHA256 verification skipped for Android Studio %s (%s; set checksum_amd64 in [plugins.options].android-studio)%s\n' \
    "$C_YEL" "$VERSION" "$REASON" "$C_RST" >&2
fi

rm -rf /opt/android-studio
tar -xzf /tmp/android-studio.tar.gz -C /opt
rm -f /tmp/android-studio.tar.gz
test -x /opt/android-studio/bin/studio.sh

# Launcher. Display forwarding (DISPLAY / WAYLAND_DISPLAY and the socket
# mounts) is host-specific and left to the user's cocoon.toml.
# shellcheck disable=SC2016 # the launcher expands $@ at run time
printf '%s\n' \
  '#!/bin/sh' \
  'exec /opt/android-studio/bin/studio.sh "$@"' \
  >/usr/local/bin/android-studio
chmod 0755 /usr/local/bin/android-studio
