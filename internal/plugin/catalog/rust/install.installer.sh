#!/usr/bin/env bash
# Install Rust toolchain via rustup (https://github.com/rust-lang/rustup)
#
# Inputs (env):
#   RC_FILE   : absolute path to the user's login-shell rc file
#   RC_SYNTAX : "posix" (bash/zsh) or "fish"
set -euo pipefail

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  https://sh.rustup.rs | sh -s -- -y --default-toolchain stable --component clippy --component rustfmt

if [ "${RC_SYNTAX:-posix}" = "fish" ]; then
  # rustup writes both env (POSIX) and env.fish. Fall back to a literal PATH
  # extension if env.fish is somehow absent so the plugin stays robust across
  # rustup versions.
  if [ -f "$HOME/.cargo/env.fish" ]; then
    # shellcheck disable=SC2016
    echo 'source "$HOME/.cargo/env.fish"' >>"$RC_FILE"
  else
    # shellcheck disable=SC2016
    echo 'set -gx PATH $HOME/.cargo/bin $PATH' >>"$RC_FILE"
  fi
else
  # shellcheck disable=SC2016
  echo '. "$HOME/.cargo/env"' >>"$RC_FILE"
fi
