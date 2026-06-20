#!/usr/bin/env bash
# Install OpenAI Codex CLI via the upstream standalone installer.
# Method category: installer — pipes chatgpt.com/codex/install.sh through sh,
#                  the standalone channel that keeps in-CLI `codex update`
#                  working. Pick install.binary.sh for a reproducible,
#                  version-pinned build (which cannot self-update).
#
# Inputs (env):
#   PIN                   : requested version; the upstream installer cannot pin
#                           a version, so a non-empty PIN is warned about and
#                           ignored (always installs the latest)
#   COCOON_INSTALL_METHOD : selected install method; must equal "installer"
set -euo pipefail

: "${COCOON_INSTALL_METHOD:?missing}"
if [ "$COCOON_INSTALL_METHOD" != "installer" ]; then
  echo "install.installer.sh invoked with COCOON_INSTALL_METHOD=$COCOON_INSTALL_METHOD; expected installer" >&2
  exit 1
fi

# Yellow WARNING when stderr is a TTY (and NO_COLOR is unset) or
# FORCE_COLOR is set. NO_COLOR wins per no-color.org.
if [ -n "${NO_COLOR:-}" ]; then
  C_YEL=''
  C_RST=''
elif [ -n "${FORCE_COLOR:-}" ] || [ -t 2 ]; then
  C_YEL=$'\033[33m'
  C_RST=$'\033[0m'
else
  C_YEL=''
  C_RST=''
fi

if [ -n "$PIN" ]; then
  # The upstream install.sh exposes no version variable (only
  # CODEX_NON_INTERACTIVE / CODEX_INSTALL_DIR), so a pin cannot be honored here.
  printf '%sWARNING: installer method always installs the latest Codex; pinned version %s is ignored. Use the binary method for a reproducible pinned build.%s\n' "$C_YEL" "$PIN" "$C_RST" >&2
fi

# CODEX_INSTALL_DIR defaults to ~/.local/bin; set it explicitly so the binary
# lands in the same user-owned location as the binary method. CODEX_HOME is left
# at its ~/.codex default (a persisted volume) — setting it would require the
# directory to already exist at build time.
curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  https://chatgpt.com/codex/install.sh |
  CODEX_NON_INTERACTIVE=1 CODEX_INSTALL_DIR="$HOME/.local/bin" sh
