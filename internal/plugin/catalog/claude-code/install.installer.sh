#!/usr/bin/env bash
# Install Claude Code (https://github.com/anthropics/claude-code)
set -euo pipefail

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  https://claude.ai/install.sh | bash
