#!/usr/bin/env bash
# Install Google Chrome (https://www.google.com/chrome/)
#
# Registers Google's signed apt repository via the same signed-by keyring
# mechanism as docker-cli / github-cli: apt verifies every Chrome package
# against the pinned Google signing key, so the install no longer trusts TLS
# alone. The key is stored ASCII-armored and referenced directly by
# signed-by, so no gpg binary is required (the minimal base ships
# ca-certificates + curl but not gnupg). Chrome for Linux ships amd64 only
# (this plugin is in e2e/arm64-exclude.txt).
set -euo pipefail

mkdir -p /etc/apt/keyrings
# Normalize the dir mode so apt's sandboxed _apt user can traverse it under a
# restrictive umask; otherwise apt-get update can fail to read the keyring even
# though the key file is go+r. Matches github-cli / docker-cli's keyless flow.
chmod 755 /etc/apt/keyrings
curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  https://dl.google.com/linux/linux_signing_key.pub \
  -o /etc/apt/keyrings/google-chrome.asc
chmod go+r /etc/apt/keyrings/google-chrome.asc

echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/google-chrome.asc] https://dl.google.com/linux/chrome/deb/ stable main" \
  >/etc/apt/sources.list.d/google-chrome.list

apt-get update
apt-get install -y --no-install-recommends google-chrome-stable
apt-get clean
rm -rf /var/lib/apt/lists/* /var/tmp/*
