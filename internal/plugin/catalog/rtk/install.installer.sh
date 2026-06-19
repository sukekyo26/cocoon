#!/usr/bin/env bash
# Install rtk via the upstream install.sh.
# Method category: installer — pipes the upstream install.sh through sh,
#                  exactly as the documented upstream flow does. Pick
#                  install.binary.sh when raw.githubusercontent.com is
#                  blocked (e.g. Zscaler) or curl|sh is forbidden.
#
# Inputs (env):
#   PIN                   : rtk version (without leading "v"); empty = latest
#   COCOON_INSTALL_METHOD : selected install method; must equal "installer"
set -euo pipefail

: "${COCOON_INSTALL_METHOD:?missing}"
if [ "$COCOON_INSTALL_METHOD" != "installer" ]; then
  echo "install.installer.sh invoked with COCOON_INSTALL_METHOD=$COCOON_INSTALL_METHOD; expected installer" >&2
  exit 1
fi

URL="https://raw.githubusercontent.com/rtk-ai/rtk/refs/heads/master/install.sh"
if [ -n "$PIN" ]; then
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors "$URL" |
    RTK_VERSION="v${PIN}" RTK_INSTALL_DIR="$HOME/.local/bin" sh
else
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors "$URL" |
    RTK_INSTALL_DIR="$HOME/.local/bin" sh
fi
