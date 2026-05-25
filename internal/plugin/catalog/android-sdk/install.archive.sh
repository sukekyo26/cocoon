#!/usr/bin/env bash
# Install Android SDK command-line tools (https://developer.android.com/tools)
#
# Inputs (env):
#   PIN                       : commandline-tools BUILD_NUMBER (e.g. "11076708"); required
#   CHECKSUM_AMD64            : sha256 of commandline-tools zip; empty to skip verification
#   CHECKSUM_ARM64            : sha256 of commandline-tools zip; empty to skip verification
#                               (the zip is JVM-only / arch-agnostic, so the same SHA pins both)
#   ANDROID_SDK_API_LEVEL     : platform API level (e.g. "35"); supplied via [install.extra_versions]
#   ANDROID_SDK_BUILD_TOOLS   : build-tools version (e.g. "35.0.0"); supplied via [install.extra_versions]
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

# Resolve the per-arch checksum. The Android command-line tools zip is JVM
# bytecode plus shell wrappers — it is the same payload on amd64 and arm64,
# so users pin the same SHA on both halves.
ARCH="$(dpkg --print-architecture)"
case "$ARCH" in
  amd64) CHECKSUM="$CHECKSUM_AMD64" ;;
  arm64) CHECKSUM="$CHECKSUM_ARM64" ;;
  *) CHECKSUM="$CHECKSUM_AMD64" ;;
esac

# PIN is required. Google does not publish a stable "latest" manifest for
# commandline-tools — the download index is HTML and the filename embeds the
# BUILD_NUMBER, so latest-resolution via scraping is brittle.
if [ -z "$PIN" ]; then
  echo "ERROR: android-sdk requires [plugins.versions].android-sdk.pin (commandline-tools BUILD_NUMBER)." >&2
  echo "       See https://developer.android.com/studio#command-line-tools-only for the current build number." >&2
  exit 1
fi

# Local mirror of [install.env].ANDROID_HOME. The Dockerfile emits the ENV
# line AFTER the install RUN, so this script cannot rely on it during the
# build itself — keep the path in lockstep with plugin.toml.
ANDROID_SDK_ROOT="/usr/local/android-sdk"

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "https://dl.google.com/android/repository/commandlinetools-linux-${PIN}_latest.zip" \
  -o /tmp/android-cmdline-tools.zip

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/android-cmdline-tools.zip" | sha256sum -c -
else
  printf '%sWARNING: SHA256 verification skipped for Android SDK (no checksum for android-sdk in [plugins.versions])%s\n' "$C_YEL" "$C_RST" >&2
fi

# The zip ships a top-level "cmdline-tools/" dir, but sdkmanager expects the
# layout "<root>/cmdline-tools/latest/" so it can later co-exist with other
# versioned cmdline-tools releases. Stage into a scratch dir, then relocate.
rm -rf /tmp/android-out "${ANDROID_SDK_ROOT}"
mkdir -p "${ANDROID_SDK_ROOT}/cmdline-tools"
unzip -q /tmp/android-cmdline-tools.zip -d /tmp/android-out
mv /tmp/android-out/cmdline-tools "${ANDROID_SDK_ROOT}/cmdline-tools/latest"
rm -rf /tmp/android-out /tmp/android-cmdline-tools.zip

SDKMANAGER="${ANDROID_SDK_ROOT}/cmdline-tools/latest/bin/sdkmanager"

# Accept all SDK licences non-interactively. `yes` exits with SIGPIPE once
# sdkmanager closes stdin; `|| true` keeps the pipeline non-fatal under
# `set -euo pipefail`. sdkmanager itself surfaces real failures via the next
# `sdkmanager` call's exit status.
yes 2>/dev/null | "${SDKMANAGER}" --sdk_root="${ANDROID_SDK_ROOT}" --licenses >/dev/null || true

"${SDKMANAGER}" --sdk_root="${ANDROID_SDK_ROOT}" \
  "platform-tools" \
  "platforms;android-${ANDROID_SDK_API_LEVEL}" \
  "build-tools;${ANDROID_SDK_BUILD_TOOLS}"

# The non-root container user owns ~/.android (license cache) and ~/.gradle
# via volumes, but the SDK tree itself lives under /usr/local. Chown inside
# the same RUN keeps the ownership flip in a single image layer.
chown -R "${USERNAME}:${USERNAME}" "${ANDROID_SDK_ROOT}"
