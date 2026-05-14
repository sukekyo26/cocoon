#!/usr/bin/env bash
# Install AWS SAM CLI (https://github.com/aws/aws-sam-cli)
set -euo pipefail

ARCH="$(dpkg --print-architecture)"
case "$ARCH" in
  amd64) DOWNLOAD_ARCH="x86_64" ;;
  arm64) DOWNLOAD_ARCH="aarch64" ;;
  *) DOWNLOAD_ARCH="x86_64" ;;
esac
echo "Detected architecture: $ARCH -> $DOWNLOAD_ARCH"

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "https://github.com/aws/aws-sam-cli/releases/latest/download/aws-sam-cli-linux-${DOWNLOAD_ARCH}.zip" \
  -o aws-sam-cli.zip
unzip aws-sam-cli.zip -d sam-installation
sudo ./sam-installation/install
rm -rf sam-installation aws-sam-cli.zip
