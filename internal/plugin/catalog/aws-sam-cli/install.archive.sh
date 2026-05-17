#!/usr/bin/env bash
# Install AWS SAM CLI (https://github.com/aws/aws-sam-cli)
#
# Inputs (env):
#   PIN              : AWS SAM CLI version (without leading "v"); empty = latest
#   CHECKSUM_AMD64   : sha256 of the amd64 zip; empty to skip verification
#   CHECKSUM_ARM64   : sha256 of the arm64 zip; empty to skip verification
set -euo pipefail

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

ARCH="$(dpkg --print-architecture)"
case "$ARCH" in
  amd64)
    DOWNLOAD_ARCH="x86_64"
    CHECKSUM="$CHECKSUM_AMD64"
    ;;
  arm64)
    DOWNLOAD_ARCH="aarch64"
    CHECKSUM="$CHECKSUM_ARM64"
    ;;
  *)
    DOWNLOAD_ARCH="x86_64"
    CHECKSUM="$CHECKSUM_AMD64"
    ;;
esac

if [ -n "$PIN" ]; then
  URL="https://github.com/aws/aws-sam-cli/releases/download/v${PIN}/aws-sam-cli-linux-${DOWNLOAD_ARCH}.zip"
else
  URL="https://github.com/aws/aws-sam-cli/releases/latest/download/aws-sam-cli-linux-${DOWNLOAD_ARCH}.zip"
fi

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "$URL" -o aws-sam-cli.zip

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  aws-sam-cli.zip" | sha256sum -c -
else
  printf '%sWARNING: SHA256 verification skipped for AWS SAM CLI (no checksum for aws-sam-cli in [plugins.versions])%s\n' "$C_YEL" "$C_RST" >&2
fi

unzip aws-sam-cli.zip -d sam-installation
sudo ./sam-installation/install
rm -rf sam-installation aws-sam-cli.zip
