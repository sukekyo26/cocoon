#!/usr/bin/env sh
# install.sh — fetch the latest cocoon release and drop it into your PATH.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/sukekyo26/cocoon/main/install.sh | sh
#
# Environment overrides:
#   COCOON_VERSION       Pin to a specific version (e.g. "0.1.0"); default "latest".
#   COCOON_INSTALL_DIR   Where to drop the binary; default "$HOME/.local/bin".
#   COCOON_REPO          GitHub repository slug; default "sukekyo26/cocoon".
#   COCOON_API_BASE      GitHub API base URL; default "https://api.github.com".
#                        Useful for GitHub Enterprise Server or test mocks.
#   COCOON_RELEASE_BASE  GitHub release host; default "https://github.com".
#
# The script verifies the binary's SHA-256 against the SHA256SUMS asset
# from the same release before installing.

set -eu

REPO="${COCOON_REPO:-sukekyo26/cocoon}"
INSTALL_DIR="${COCOON_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${COCOON_VERSION:-latest}"
API_BASE="${COCOON_API_BASE:-https://api.github.com}"
RELEASE_BASE="${COCOON_RELEASE_BASE:-https://github.com}"

err() { printf "cocoon-install: %s\n" "$*" >&2; }
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
  # Capture curl output first so a network/API failure dies with a specific
  # message instead of being masked by the pipeline's last-command exit
  # status (POSIX sh has no `pipefail`).
  releases_url="$API_BASE/repos/$REPO/releases/latest"
  api_response=$(curl -fsSL "$releases_url") ||
    die "failed to fetch release metadata: $releases_url"
  tag=$(printf '%s' "$api_response" |
    sed -n 's/.*"tag_name": *"\(v\{0,1\}[^"]*\)".*/\1/p' |
    head -n1)
  [ -n "$tag" ] || die "could not parse tag_name from GitHub API response for $REPO"
else
  case "$VERSION" in
    v*) tag="$VERSION" ;;
    *) tag="v$VERSION" ;;
  esac
fi

# tag_name is "v0.1.0"; strip the "v" so the asset filename matches.
version=${tag#v}

asset="cocoon-$os-$arch"
base="$RELEASE_BASE/$REPO/releases/download/$tag"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

err "downloading $asset@$version"
curl -fsSL "$base/$asset" -o "$tmp/$asset" || die "download failed: $base/$asset"
curl -fsSL "$base/SHA256SUMS" -o "$tmp/SHA256SUMS" || die "download failed: $base/SHA256SUMS"

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
