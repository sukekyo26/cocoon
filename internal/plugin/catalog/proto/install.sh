#!/usr/bin/env bash
# Install proto (multi-language version manager) (https://github.com/moonrepo/proto)
#
# Inputs (env):
#   PIN       : proto version to install; empty = latest
#   RC_FILE   : absolute path to the user's login-shell rc file
#   RC_SYNTAX : "posix" (bash/zsh) or "fish"
set -euo pipefail

if [ -n "$PIN" ]; then
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    https://moonrepo.dev/install/proto.sh | bash -s -- "$PIN" --no-profile --yes
else
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    https://moonrepo.dev/install/proto.sh | bash -s -- --no-profile --yes
fi

if [ "${RC_SYNTAX:-posix}" = "fish" ]; then
  {
    # shellcheck disable=SC2016
    echo 'set -gx PROTO_HOME $HOME/.proto'
    # shellcheck disable=SC2016
    echo 'set -gx PATH $PROTO_HOME/shims $PROTO_HOME/bin $PATH'
  } >> "$RC_FILE"
else
  {
    # shellcheck disable=SC2016
    echo 'export PROTO_HOME="$HOME/.proto"'
    # shellcheck disable=SC2016
    echo 'export PATH="$PROTO_HOME/shims:$PROTO_HOME/bin:$PATH"'
  } >> "$RC_FILE"
fi
