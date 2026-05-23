#!/usr/bin/env bash
# Install Dart SDK (https://github.com/dart-lang/sdk)
#
# Inputs (env):
#   PIN              : Dart SDK version (e.g. "3.12.0"); empty = latest stable
#   CHECKSUM_AMD64   : sha256 of linux-x64 zip; empty to skip verification
#   CHECKSUM_ARM64   : sha256 of linux-arm64 zip; empty to skip verification
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
    DART_ARCH="x64"
    CHECKSUM="$CHECKSUM_AMD64"
    ;;
  arm64)
    DART_ARCH="arm64"
    CHECKSUM="$CHECKSUM_ARM64"
    ;;
  *)
    DART_ARCH="x64"
    CHECKSUM="$CHECKSUM_AMD64"
    ;;
esac

if [ -n "$PIN" ]; then
  VERSION="$PIN"
else
  # Resolve latest stable from the official channel manifest. The endpoint
  # returns a small JSON document with a "version" field; extract it with
  # sed to avoid a jq dependency.
  VERSION=$(curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    https://storage.googleapis.com/dart-archive/channels/stable/release/latest/VERSION |
    sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)
  if [ -z "$VERSION" ]; then
    echo "Failed to resolve latest Dart stable version" >&2
    exit 1
  fi
fi

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "https://storage.googleapis.com/dart-archive/channels/stable/release/${VERSION}/sdk/dartsdk-linux-${DART_ARCH}-release.zip" \
  -o /tmp/dart.zip

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/dart.zip" | sha256sum -c -
else
  printf '%sWARNING: SHA256 verification skipped for Dart (no checksum for dart in [plugins.versions])%s\n' "$C_YEL" "$C_RST" >&2
fi

# ZIP root is "dart-sdk/" — extract into a scratch dir then move to /usr/local/dart.
rm -rf /tmp/dart-out
unzip -q /tmp/dart.zip -d /tmp/dart-out
mv /tmp/dart-out/dart-sdk /usr/local/dart
rm -rf /tmp/dart-out /tmp/dart.zip
