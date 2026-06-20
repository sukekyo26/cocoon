#!/usr/bin/env bash
# Install OpenAI Codex CLI from a GitHub Release tarball.
# Method category: binary — version-pinnable and reproducible, but the in-CLI
#                  `codex update` cannot self-update this channel (it writes no
#                  standalone package metadata). Pick install.installer.sh when
#                  you want `codex update` to work.
#
# Inputs (env):
#   PIN                   : Codex version without the "rust-v" tag prefix (e.g. "0.135.0"); empty = latest
#   CHECKSUM_AMD64        : sha256 of the x86_64 tarball; empty to skip verification
#   CHECKSUM_ARM64        : sha256 of the aarch64 tarball; empty to skip verification
#   COCOON_INSTALL_METHOD : selected install method; must equal "binary"
set -euo pipefail

: "${COCOON_INSTALL_METHOD:?missing}"
if [ "$COCOON_INSTALL_METHOD" != "binary" ]; then
  echo "install.binary.sh invoked with COCOON_INSTALL_METHOD=$COCOON_INSTALL_METHOD; expected binary" >&2
  exit 1
fi

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
    DOWNLOAD_ARCH="x86_64"
    CHECKSUM="$CHECKSUM_AMD64"
    ;;
  arm64)
    DOWNLOAD_ARCH="aarch64"
    CHECKSUM="$CHECKSUM_ARM64"
    ;;
  *)
    DOWNLOAD_ARCH="x86_64"
    CHECKSUM="$CHECKSUM_AMD64"
    ;;
esac

if [ -n "$PIN" ]; then
  VERSION="$PIN"
else
  # openai/codex tags releases as rust-v<semver>; /releases/latest redirects
  # to .../tag/rust-v<semver>.
  VERSION=$(curl -fsSLI --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    -o /dev/null -w '%{url_effective}' https://github.com/openai/codex/releases/latest |
    sed 's|.*/tag/rust-v||')
fi

triple="${DOWNLOAD_ARCH}-unknown-linux-musl"
TARBALL="codex-${triple}.tar.gz"
curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "https://github.com/openai/codex/releases/download/rust-v${VERSION}/${TARBALL}" \
  -o "/tmp/${TARBALL}"

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/${TARBALL}" | sha256sum -c -
else
  # openai/codex publishes no SHA256SUMS manifest or .sha256 sidecar (only
  # per-asset digests in the GitHub API + Sigstore bundles), so an unpinned
  # build can fetch no upstream checksum. Warn loudly and proceed, matching
  # shfmt / shellcheck. Set checksum_amd64 / checksum_arm64 in [plugins.options].codex for a verified build.
  printf '%sWARNING: SHA256 verification skipped for OpenAI Codex CLI (no upstream checksum; set checksum_amd64/arm64 in [plugins.options].codex)%s\n' "$C_YEL" "$C_RST" >&2
fi

# The tarball holds one entry named with the full target triple
# (codex-<triple>, no leading directory); land it as ~/.local/bin/codex.
mkdir -p "$HOME/.local/bin"
tar -xzf "/tmp/${TARBALL}" -C /tmp "codex-${triple}"
install -m 0755 "/tmp/codex-${triple}" "$HOME/.local/bin/codex"
rm "/tmp/${TARBALL}" "/tmp/codex-${triple}"
