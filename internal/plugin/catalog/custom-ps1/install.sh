#!/usr/bin/env bash
# Custom PS1 with Docker container name, user/host, working directory, and git status.
#
# Bash-only: the prompt uses bash-specific escapes (\h, \u, $(__git_ps1 ...))
# that don't translate cleanly to zsh's PROMPT or fish's fish_prompt function.
# Use the `starship` plugin for a cross-shell prompt instead.
#
# Inputs (env):
#   RC_FILE     : absolute path to the user's login-shell rc file
#   LOGIN_SHELL : "bash" / "zsh" / "fish"
set -euo pipefail

if [ "${LOGIN_SHELL:-bash}" != "bash" ]; then
  echo "WARN: custom-ps1 plugin is bash-only; skipping on ${LOGIN_SHELL}." >&2
  echo "      Use the 'starship' plugin for a cross-shell prompt." >&2
  exit 0
fi

{
  echo 'GIT_PS1_SHOWDIRTYSTATE=1'
  echo 'GIT_PS1_SHOWUNTRACKEDFILES=1'
  echo 'GIT_PS1_SHOWUPSTREAM="auto"'
  # shellcheck disable=SC2016,SC2028
  echo 'PS1='\''\[\033[01;35m\][Docker $CONTAINER_SERVICE_NAME]\[\033[00m\] \[\033[01;32m\]\u@\h:\[\033[01;34m\]\w\[\033[00m\]$(__git_ps1 " \[\033[01;33m\](%s)\[\033[00m\]" 2>/dev/null) \$ '\'''
} >>"$RC_FILE"
