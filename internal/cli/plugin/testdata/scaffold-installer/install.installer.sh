#!/usr/bin/env bash
# Install Demo via the upstream installer script.
# Method category: installer — pipes the vendor's curl-to-bash script.
#
# Inputs (env):
#   PIN : Demo version to install; empty = latest
set -euo pipefail

# TODO: replace https://example.com/install.sh with the upstream installer URL.
if [ -n "$PIN" ]; then
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    "https://example.com/${PIN}/install.sh" | bash
else
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    https://example.com/install.sh | bash
fi
