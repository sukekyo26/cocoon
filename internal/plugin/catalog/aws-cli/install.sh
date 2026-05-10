#!/usr/bin/env bash
# Install AWS CLI v2 (https://github.com/aws/aws-cli)
set -euo pipefail

ARCH="$(dpkg --print-architecture)"
case "$ARCH" in
  amd64) CLI_ARCH="x86_64" ;;
  arm64) CLI_ARCH="aarch64" ;;
  *) CLI_ARCH="x86_64" ;;
esac

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "https://awscli.amazonaws.com/awscli-exe-linux-${CLI_ARCH}.zip" -o awscliv2.zip
unzip awscliv2.zip
sudo ./aws/install
rm -rf aws awscliv2.zip
