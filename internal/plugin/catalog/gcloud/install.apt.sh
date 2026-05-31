#!/usr/bin/env bash
# Install the Google Cloud CLI (https://cloud.google.com/sdk)
# Provides gcloud, gsutil and bq from the official apt repository.
set -euo pipefail

mkdir -p /etc/apt/keyrings
# Normalize the dir mode so apt's sandboxed _apt user can traverse it under a
# restrictive umask; otherwise apt-get update can fail to read the keyring even
# though the key file is go+r. Matches docker-cli / github-cli.
chmod 755 /etc/apt/keyrings

# Store the Google signing key ASCII-armored and reference it directly via
# signed-by, so apt verifies packages without needing the gpg binary (the
# minimal base ships ca-certificates + curl but not gnupg).
curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  https://packages.cloud.google.com/apt/doc/apt-key.gpg -o /etc/apt/keyrings/cloud.google.asc
chmod go+r /etc/apt/keyrings/cloud.google.asc

# The Google Cloud apt repo serves a single fixed distribution ("cloud-sdk"),
# not a per-release codename, so the same source line works on any Debian /
# Ubuntu base without resolving VERSION_CODENAME.
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/cloud.google.asc] https://packages.cloud.google.com/apt cloud-sdk main" \
  >/etc/apt/sources.list.d/google-cloud-sdk.list

apt-get update
apt-get install -y --no-install-recommends google-cloud-cli
apt-get clean
rm -rf /var/lib/apt/lists/* /var/tmp/*
