#!/usr/bin/env bash
# Install mise (polyglot runtime version manager) (https://github.com/jdx/mise)
#
# Inputs (env):
#   PIN         : mise version without leading "v" (e.g. "2025.12.0"); empty = latest
#   RC_FILE     : absolute path to the user's login-shell rc file
#   RC_SYNTAX   : "posix" (bash/zsh) or "fish"
#   LOGIN_SHELL : "bash" / "zsh" / "fish" — picks `mise activate <shell>`
set -euo pipefail

if [ -n "$PIN" ]; then
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    https://mise.run | MISE_VERSION="v${PIN}" sh
else
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    https://mise.run | sh
fi

shell="${LOGIN_SHELL:-bash}"
if [ "$shell" = "fish" ]; then
  # shellcheck disable=SC2016
  echo 'set -gx PATH $HOME/.local/bin $PATH' >> "$RC_FILE"
  # shellcheck disable=SC2016
  echo '$HOME/.local/bin/mise activate fish | source' >> "$RC_FILE"
else
  # shellcheck disable=SC2016
  echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$RC_FILE"
  echo "eval \"\$(\$HOME/.local/bin/mise activate ${shell})\"" >> "$RC_FILE"
fi
