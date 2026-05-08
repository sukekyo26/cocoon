#!/usr/bin/env bash
# Install Nerd Fonts (Meslo) (https://github.com/ryanoasis/nerd-fonts)
set -euo pipefail

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "https://github.com/ryanoasis/nerd-fonts/releases/latest/download/Meslo.tar.xz" \
  -o /tmp/Meslo.tar.xz

mkdir -p ~/.fonts/Meslo
tar -xf /tmp/Meslo.tar.xz -C ~/.fonts/Meslo
fc-cache -fv
rm /tmp/Meslo.tar.xz
