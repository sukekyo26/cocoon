#!/bin/bash
# ============================================================
# docker-entrypoint.sh — sync image files to volume
# ============================================================
# Docker named volumes only populate from the image on first
# creation. When new plugins are added and the image is rebuilt,
# the existing volume hides newly installed files.
#
# This entrypoint copies image-installed files from the staging
# directory (~/.image-local) into the volume-mounted ~/.local
# on every container start.
# ============================================================

if [ -d "$HOME/.image-local" ]; then
  mkdir -p "$HOME/.local"
  cp -a "$HOME/.image-local/." "$HOME/.local/"
fi

exec "$@"
