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
#
# The script verifies the binary's SHA-256 against the SHA256SUMS asset
# from the same release before installing.

set -eu

REPO="${COCOON_REPO:-sukekyo26/cocoon}"
INSTALL_DIR="${COCOON_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${COCOON_VERSION:-latest}"

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
require sha256sum
require mktemp

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
  tag=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" |
    sed -n 's/.*"tag_name": *"\(v\{0,1\}[^"]*\)".*/\1/p' |
    head -n1)
  [ -n "$tag" ] || die "could not resolve latest release for $REPO"
else
  case "$VERSION" in
    v*) tag="$VERSION" ;;
    *) tag="v$VERSION" ;;
  esac
fi

# tag_name is "v0.1.0"; strip the "v" so the asset filename matches.
version=${tag#v}

asset="cocoon-$os-$arch"
base="https://github.com/$REPO/releases/download/$tag"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

err "downloading $asset@$version"
curl -fsSL "$base/$asset" -o "$tmp/$asset" || die "download failed: $base/$asset"
curl -fsSL "$base/SHA256SUMS" -o "$tmp/SHA256SUMS" || die "download failed: $base/SHA256SUMS"

expected=$(grep "  $asset\$" "$tmp/SHA256SUMS" | awk '{print $1}')
[ -n "$expected" ] || die "$asset not listed in SHA256SUMS"
actual=$(sha256sum "$tmp/$asset" | awk '{print $1}')
[ "$expected" = "$actual" ] || die "checksum mismatch ($actual != $expected)"

mkdir -p "$INSTALL_DIR"
mv "$tmp/$asset" "$INSTALL_DIR/cocoon"
chmod +x "$INSTALL_DIR/cocoon"

err "installed cocoon $version to $INSTALL_DIR/cocoon"
case ":$PATH:" in
  *:"$INSTALL_DIR":*) ;;
  *) err "note: $INSTALL_DIR is not in PATH; add it to ~/.profile or ~/.zshrc" ;;
esac
