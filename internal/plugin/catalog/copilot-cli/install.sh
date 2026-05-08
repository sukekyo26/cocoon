#!/usr/bin/env bash
# Install GitHub Copilot CLI (https://github.com/github/copilot-cli)
#
# Inputs (env):
#   PIN              : Copilot CLI version (without leading "v"); empty = latest
set -euo pipefail

if [ -n "$PIN" ]; then
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    https://gh.io/copilot-install | VERSION="v${PIN}" PREFIX="$HOME/.local" bash
else
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    https://gh.io/copilot-install | PREFIX="$HOME/.local" bash
fi
