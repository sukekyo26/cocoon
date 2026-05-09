#!/usr/bin/env bash
# Configure Starship for the user's login shell.
#
# Inputs (env):
#   RC_FILE     : absolute path to the user's login-shell rc file
#   RC_SYNTAX   : "posix" (bash/zsh) or "fish"
#   LOGIN_SHELL : "bash" / "zsh" / "fish" — picks `starship init <shell>`
set -euo pipefail

shell="${LOGIN_SHELL:-bash}"
if [ "${RC_SYNTAX:-posix}" = "fish" ]; then
  echo "starship init ${shell} | source" >> "$RC_FILE"
else
  # shellcheck disable=SC2016
  echo "eval \"\$(starship init ${shell})\"" >> "$RC_FILE"
fi
