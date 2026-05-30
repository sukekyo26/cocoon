#!/usr/bin/env bash
# Install Dart SDK (https://github.com/dart-lang/sdk)
#
# Inputs (env):
#   PIN              : Dart SDK version (e.g. "3.12.0"); empty = latest stable
#   CHECKSUM_AMD64   : sha256 of linux-x64 zip; empty = verify against the
#                      upstream .sha256sum served next to the zip
#   CHECKSUM_ARM64   : sha256 of linux-arm64 zip; empty = verify against the
#                      upstream .sha256sum served next to the zip
set -euo pipefail

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

url="https://storage.googleapis.com/dart-archive/channels/stable/release/${VERSION}/sdk/dartsdk-linux-${DART_ARCH}-release.zip"
curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "$url" -o /tmp/dart.zip

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/dart.zip" | sha256sum -c -
else
  # No user pin: verify against the .sha256sum dart-archive serves next to
  # the zip.
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    "${url}.sha256sum" -o /tmp/dart.sum
  expected="$(cut -d ' ' -f1 /tmp/dart.sum)"
  if [ -z "$expected" ]; then
    echo "dart: empty checksum from ${url}.sha256sum" >&2
    exit 1
  fi
  echo "${expected}  /tmp/dart.zip" | sha256sum -c -
  rm -f /tmp/dart.sum
fi

# ZIP root is "dart-sdk/" — extract into a scratch dir then move to /usr/local/dart.
# Clear /usr/local/dart up front so a rerun (e.g. a base image layer that
# already carries a dart-sdk/ tree) overwrites the toolchain instead of
# nesting the new SDK at /usr/local/dart/dart-sdk and leaving PATH pointing
# at the stale install.
rm -rf /tmp/dart-out /usr/local/dart
unzip -q /tmp/dart.zip -d /tmp/dart-out
mv /tmp/dart-out/dart-sdk /usr/local/dart
rm -rf /tmp/dart-out /tmp/dart.zip
