#!/usr/bin/env bash
# Install Docker CLI (client only, to use host Docker daemon via socket mount)
# https://github.com/docker/cli
#
# This installs the CLI packages only. The container user's access to the
# mounted docker socket is configured at container start by
# docker-entrypoint.sh, which resolves the socket's GID and joins the user
# to a matching group.
set -euo pipefail

mkdir -p /etc/apt/keyrings

# shellcheck disable=SC1091
. /etc/os-release

# Docker publishes its apt repository at /linux/<distro> for both Ubuntu and
# Debian. Use the /etc/os-release ID directly so the same install.sh works
# for either base image without a hard-coded distro path.
docker_repo="https://download.docker.com/linux/${ID}"
curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "${docker_repo}/gpg" | gpg --dearmor -o /etc/apt/keyrings/docker.gpg

echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] ${docker_repo} ${VERSION_CODENAME} stable" \
  >/etc/apt/sources.list.d/docker.list

apt-get update
apt-get install -y --no-install-recommends docker-ce-cli docker-compose-plugin
apt-get clean
rm -rf /var/lib/apt/lists/* /var/tmp/*
