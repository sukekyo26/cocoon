#!/usr/bin/env bash
# Install Android SDK command-line tools (https://developer.android.com/tools)
#
# Inputs (env):
#   PIN                       : commandline-tools BUILD_NUMBER (e.g. "11076708"); empty = latest
#                               (scraped from https://developer.android.com/studio)
#   CHECKSUM_AMD64            : sha256 of commandline-tools zip; falls back to
#                               CHECKSUM_ARM64 when empty (verification is skipped only
#                               when both are empty)
#   CHECKSUM_ARM64            : same artifact — the zip is JVM-only / arch-agnostic,
#                               so the same SHA pins both; consulted as a fallback
#                               when CHECKSUM_AMD64 is empty
#   ANDROID_SDK_API_LEVEL     : platform API level (e.g. "35"); supplied via [install.extra_versions]
#   ANDROID_SDK_BUILD_TOOLS   : build-tools version (e.g. "35.0.0"); supplied via [install.extra_versions]
set -euo pipefail

# The two extra_versions inputs are always emitted by the generator (with
# the plugin.toml default when [plugins.versions] omits them). Guard at the
# entry rather than at the sdkmanager call: ${VAR:?msg} identifies the
# offending plugin input by name, instead of bash's generic "unbound" or
# sdkmanager's confusing "platforms;android-" / "build-tools;" error on
# an empty workspace override.
: "${ANDROID_SDK_API_LEVEL:?empty/unset — set api_level on android-sdk in [plugins.versions], or restore the plugin.toml default}"
: "${ANDROID_SDK_BUILD_TOOLS:?empty/unset — set build_tools on android-sdk in [plugins.versions], or restore the plugin.toml default}"

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
# so users typically pin the same SHA on both halves. When only one half is
# set, fall back to the other rather than silently skipping verification: the
# SHA the user supplied still validates this download.
ARCH="$(dpkg --print-architecture)"
case "$ARCH" in
  amd64) CHECKSUM="${CHECKSUM_AMD64:-$CHECKSUM_ARM64}" ;;
  arm64) CHECKSUM="${CHECKSUM_ARM64:-$CHECKSUM_AMD64}" ;;
  *) CHECKSUM="${CHECKSUM_AMD64:-$CHECKSUM_ARM64}" ;;
esac

# Resolve the BUILD_NUMBER. Google does not publish a stable JSON / XML
# manifest for commandline-tools, but the /studio page embeds the current
# download URL with BUILD_NUMBER baked into the filename. Scrape it as the
# "upstream latest" the version_capable contract (docs/plugins.md) requires
# when $PIN is empty. If the page shape changes and scraping fails, fail
# loud rather than guessing a stale default.
if [ -n "$PIN" ]; then
  BUILD_NUMBER="$PIN"
else
  STUDIO_HTML=$(curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    https://developer.android.com/studio)
  BUILD_NUMBER=$(printf '%s' "$STUDIO_HTML" | tr -d '\n' |
    grep -oE 'commandlinetools-linux-[0-9]+_latest\.zip' | head -n 1 |
    sed -n 's/commandlinetools-linux-\([0-9]*\)_latest\.zip/\1/p') || true
  if [ -z "$BUILD_NUMBER" ]; then
    echo "ERROR: failed to resolve latest commandline-tools BUILD_NUMBER from https://developer.android.com/studio." >&2
    echo "       Pin explicitly via [plugins.versions].android-sdk.pin = \"<BUILD_NUMBER>\"" >&2
    echo "       (find the current BUILD_NUMBER under \"Command line tools only\" at the URL above)." >&2
    exit 1
  fi
fi

# Local mirror of [install.env].ANDROID_HOME. The Dockerfile emits the ENV
# line AFTER the install RUN, so this script cannot rely on it during the
# build itself — keep the path in lockstep with plugin.toml.
ANDROID_SDK_ROOT="/usr/local/android-sdk"

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "https://dl.google.com/android/repository/commandlinetools-linux-${BUILD_NUMBER}_latest.zip" \
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
