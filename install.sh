#!/usr/bin/env sh
# install.sh — fetch the latest cocoon release and drop it into your PATH.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/sukekyo26/cocoon/main/install.sh | sh
#   # GitHub Pages mirror (skips raw.githubusercontent.com + api.github.com):
#   curl -fsSL https://sukekyo26.github.io/cocoon/install.sh | \
#     COCOON_PAGES_BASE=https://sukekyo26.github.io/cocoon sh
#
# Environment overrides:
#   COCOON_VERSION       Pin to a specific version (e.g. "0.1.0"); default "latest".
#   COCOON_INSTALL_DIR   Where to drop the binary; default "$HOME/.local/bin".
#   COCOON_REPO          GitHub repository slug; default "sukekyo26/cocoon".
#   COCOON_API_BASE      GitHub API base URL; default "https://api.github.com".
#                        Useful for GitHub Enterprise Server or test mocks.
#   COCOON_RELEASE_BASE  GitHub release host; default "https://github.com".
#   COCOON_API_TOKEN     Optional bearer token sent to COCOON_API_BASE. Lifts the
#                        anonymous rate limit (60 req/hour per IP → 5000 with a
#                        GitHub token); typically only set in CI / automation.
#   COCOON_PAGES_BASE    GitHub Pages mirror base URL (default empty). When set,
#                        the script reads the latest version from
#                        "$COCOON_PAGES_BASE/VERSION" instead of the GitHub API
#                        and downloads the binary + SHA256SUMS from
#                        "$COCOON_PAGES_BASE/<tag>/..." instead of
#                        github.com/.../releases/download/. Use this in
#                        environments that can reach *.github.io but not
#                        raw.githubusercontent.com / api.github.com.
#
# The script verifies the binary's SHA-256 against the SHA256SUMS asset
# from the same release before installing.

set -eu

# Color output when stderr is a TTY (and NO_COLOR is unset) or when
# FORCE_COLOR is set. NO_COLOR wins over FORCE_COLOR per no-color.org.
if [ -n "${NO_COLOR:-}" ]; then
  C_RED=''
  C_RST=''
elif [ -n "${FORCE_COLOR:-}" ] || [ -t 2 ]; then
  C_RED=$(printf '\033[31m')
  C_RST=$(printf '\033[0m')
else
  C_RED=''
  C_RST=''
fi

REPO="${COCOON_REPO:-sukekyo26/cocoon}"
INSTALL_DIR="${COCOON_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${COCOON_VERSION:-latest}"
API_BASE="${COCOON_API_BASE:-https://api.github.com}"
RELEASE_BASE="${COCOON_RELEASE_BASE:-https://github.com}"
PAGES_BASE="${COCOON_PAGES_BASE:-}"
# Strip a single trailing slash so callers may pass either ".../api/v3" or
# ".../api/v3/" without producing "//<rest>" URLs that some mirrors / GHES
# reverse proxies reject.
API_BASE="${API_BASE%/}"
RELEASE_BASE="${RELEASE_BASE%/}"
PAGES_BASE="${PAGES_BASE%/}"
API_TOKEN="${COCOON_API_TOKEN:-}"

err() { printf "%scocoon-install: %s%s\n" "$C_RED" "$*" "$C_RST" >&2; }
die() {
  err "$*"
  exit 1
}

require() {
  command -v "$1" >/dev/null 2>&1 || die "missing required tool: $1"
}

require curl
require uname
require mktemp

# sha256: portable wrapper. Linux ships coreutils `sha256sum`; macOS ships
# `shasum` (Perl) which prints the same `<hash>  <filename>` format under
# `-a 256`. Both need to work because install.sh declares darwin support.
if command -v sha256sum >/dev/null 2>&1; then
  sha256() { sha256sum "$@"; }
elif command -v shasum >/dev/null 2>&1; then
  sha256() { shasum -a 256 "$@"; }
else
  die "missing required tool: sha256sum or shasum"
fi

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  linux | darwin) ;;
  *) die "unsupported OS: $os (cocoon supports linux and darwin)" ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) die "unsupported architecture: $arch" ;;
esac

if [ "$VERSION" = "latest" ]; then
  if [ -n "$PAGES_BASE" ]; then
    # Pages mirror: a single static "<version>\n" file is the source of
    # truth for "latest". Avoids api.github.com entirely.
    version_url="$PAGES_BASE/VERSION"
    pages_version=$(curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors "$version_url") ||
      die "failed to fetch latest version: $version_url"
    pages_version=$(printf '%s' "$pages_version" | tr -d '[:space:]')
    [ -n "$pages_version" ] || die "empty VERSION file at $version_url"
    tag="v$pages_version"
  else
    # Capture curl output first so a network/API failure dies with a specific
    # message instead of being masked by the pipeline's last-command exit
    # status (POSIX sh has no `pipefail`).
    releases_url="$API_BASE/repos/$REPO/releases/latest"
    if [ -n "$API_TOKEN" ]; then
      api_response=$(curl -fsSL --proto '=https' --tlsv1.2 -H "Authorization: Bearer $API_TOKEN" "$releases_url") ||
        die "failed to fetch release metadata: $releases_url"
    else
      api_response=$(curl -fsSL --proto '=https' --tlsv1.2 "$releases_url") ||
        die "failed to fetch release metadata: $releases_url"
    fi
    tag=$(printf '%s' "$api_response" |
      sed -n 's/.*"tag_name": *"\(v\{0,1\}[^"]*\)".*/\1/p' |
      head -n1)
    [ -n "$tag" ] || die "could not parse tag_name from GitHub API response for $REPO"
  fi
else
  case "$VERSION" in
    v*) tag="$VERSION" ;;
    *) tag="v$VERSION" ;;
  esac
fi

# tag_name is "v0.1.0"; strip the "v" so the asset filename matches.
version=${tag#v}

asset="cocoon-$os-$arch"
if [ -n "$PAGES_BASE" ]; then
  base="$PAGES_BASE/$tag"
else
  base="$RELEASE_BASE/$REPO/releases/download/$tag"
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

err "downloading $asset@$version"
curl -fsSL --proto '=https' --tlsv1.2 "$base/$asset" -o "$tmp/$asset" || die "download failed: $base/$asset"
curl -fsSL --proto '=https' --tlsv1.2 "$base/SHA256SUMS" -o "$tmp/SHA256SUMS" || die "download failed: $base/SHA256SUMS"

expected=$(grep "  $asset\$" "$tmp/SHA256SUMS" | awk '{print $1}')
[ -n "$expected" ] || die "$asset not listed in SHA256SUMS"
actual=$(sha256 "$tmp/$asset" | awk '{print $1}')
[ "$expected" = "$actual" ] || die "checksum mismatch ($actual != $expected)"

mkdir -p "$INSTALL_DIR"
mv "$tmp/$asset" "$INSTALL_DIR/cocoon"
chmod +x "$INSTALL_DIR/cocoon"

err "installed cocoon $version to $INSTALL_DIR/cocoon"
case ":$PATH:" in
  *:"$INSTALL_DIR":*) ;;
  *) err "note: $INSTALL_DIR is not in PATH; add it to ~/.profile or ~/.zshrc" ;;
esac
