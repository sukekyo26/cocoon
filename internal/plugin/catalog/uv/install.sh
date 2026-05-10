#!/usr/bin/env bash
# Install uv (fast Python package and project manager) (https://github.com/astral-sh/uv)
#
# Inputs (env):
#   PIN       : uv version to install; empty = latest
#   RC_FILE   : absolute path to the user's login-shell rc file
#   RC_SYNTAX : "posix" (bash/zsh) or "fish"
set -euo pipefail

if [ -n "$PIN" ]; then
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    "https://astral.sh/uv/${PIN}/install.sh" | bash
else
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    https://astral.sh/uv/install.sh | bash
fi

if [ "${RC_SYNTAX:-posix}" = "fish" ]; then
  # shellcheck disable=SC2016
  echo 'set -gx PATH $HOME/.local/bin $PATH' >>"$RC_FILE"
else
  # shellcheck disable=SC2016
  echo 'export PATH="$HOME/.local/bin:$PATH"' >>"$RC_FILE"
fi
