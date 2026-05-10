#!/usr/bin/env bash
# Install bun (all-in-one JavaScript runtime & toolkit) (https://github.com/oven-sh/bun)
#
# Inputs (env):
#   PIN       : bun version without leading "v" (e.g. "1.3.3"); empty = latest
#   RC_FILE   : absolute path to the user's login-shell rc file
#   RC_SYNTAX : "posix" (bash/zsh) or "fish"
set -euo pipefail

if [ -n "$PIN" ]; then
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    https://bun.com/install | bash -s "bun-v${PIN}"
else
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    https://bun.sh/install | bash
fi

if [ "${RC_SYNTAX:-posix}" = "fish" ]; then
  # shellcheck disable=SC2016
  echo 'set -gx BUN_INSTALL $HOME/.bun' >>"$RC_FILE"
  # shellcheck disable=SC2016
  echo 'set -gx PATH $BUN_INSTALL/bin $PATH' >>"$RC_FILE"
else
  # shellcheck disable=SC2016
  echo 'export BUN_INSTALL="$HOME/.bun"' >>"$RC_FILE"
  # shellcheck disable=SC2016
  echo 'export PATH="$BUN_INSTALL/bin:$PATH"' >>"$RC_FILE"
fi
