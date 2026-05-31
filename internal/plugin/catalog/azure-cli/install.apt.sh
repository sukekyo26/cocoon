#!/usr/bin/env bash
# Install the Azure CLI (https://learn.microsoft.com/cli/azure) — the `az`
# command — from Microsoft's official apt repository.
set -euo pipefail

mkdir -p /etc/apt/keyrings
# Normalize the dir mode so apt's sandboxed _apt user can traverse it under a
# restrictive umask (matches docker-cli / github-cli).
chmod 755 /etc/apt/keyrings

# shellcheck disable=SC1091
. /etc/os-release

repo="https://packages.microsoft.com/repos/azure-cli"

# Microsoft publishes the Azure CLI apt repo per release codename and lags the
# newest distros (e.g. Debian 13 "trixie" / brand-new Ubuntu). Probe the repo
# first and fail fast with an actionable message instead of letting apt-get
# update die on a 404 source the user cannot diagnose.
if ! curl -fsI --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  -o /dev/null "${repo}/dists/${VERSION_CODENAME}/Release"; then
  echo "azure-cli: Microsoft publishes no apt package for '${VERSION_CODENAME}'." >&2
  echo "  Use a base image whose codename Microsoft ships (e.g. ubuntu 22.04/24.04," >&2
  echo "  debian 12) or install the Azure CLI another way (pip / vendor script)." >&2
  exit 1
fi

# Store the Microsoft signing key ASCII-armored and reference it via signed-by,
# so apt verifies packages without needing the gpg binary (the minimal base
# ships ca-certificates + curl but not gnupg).
curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  https://packages.microsoft.com/keys/microsoft.asc -o /etc/apt/keyrings/microsoft.asc
chmod go+r /etc/apt/keyrings/microsoft.asc

echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/microsoft.asc] ${repo} ${VERSION_CODENAME} main" \
  >/etc/apt/sources.list.d/azure-cli.list

apt-get update
apt-get install -y --no-install-recommends azure-cli
apt-get clean
rm -rf /var/lib/apt/lists/* /var/tmp/*
