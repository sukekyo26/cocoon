#!/usr/bin/env bash
# Install Google Chrome (https://www.google.com/chrome/)
set -euo pipefail

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  -o /tmp/google-chrome-stable_current_amd64.deb \
  https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb

apt-get update
apt-get install -y --no-install-recommends /tmp/google-chrome-stable_current_amd64.deb
apt-get clean
rm -rf /var/lib/apt/lists/* /var/tmp/*
