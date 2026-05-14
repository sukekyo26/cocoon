#!/usr/bin/env bash
# Install GitHub Copilot CLI via the upstream gh.io installer.
# Method category: installer — pipes https://gh.io/copilot-install through
#                  bash with VERSION and PREFIX env, exactly as the
#                  documented upstream flow does. Pick install.binary.sh
#                  when gh.io is blocked or curl|sh is forbidden.
#
# Inputs (env):
#   PIN                   : Copilot CLI version (without leading "v"); empty = latest
#   COCOON_INSTALL_METHOD : selected install method; must equal "installer"
set -euo pipefail

: "${COCOON_INSTALL_METHOD:?missing}"
if [ "$COCOON_INSTALL_METHOD" != "installer" ]; then
  echo "install.installer.sh invoked with COCOON_INSTALL_METHOD=$COCOON_INSTALL_METHOD; expected installer" >&2
  exit 1
fi

if [ -n "$PIN" ]; then
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    https://gh.io/copilot-install | VERSION="v${PIN}" PREFIX="$HOME/.local" bash
else
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    https://gh.io/copilot-install | PREFIX="$HOME/.local" bash
fi
