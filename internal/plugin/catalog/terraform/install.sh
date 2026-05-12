#!/usr/bin/env bash
# Install Terraform (https://github.com/hashicorp/terraform)
#
# Inputs (env):
#   PIN              : Terraform version without leading "v" (e.g. "1.10.5"); empty = latest
#   CHECKSUM_AMD64   : sha256 of amd64 zip; empty to skip verification
#   CHECKSUM_ARM64   : sha256 of arm64 zip; empty to skip verification
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
  amd64) CHECKSUM="$CHECKSUM_AMD64" ;;
  arm64) CHECKSUM="$CHECKSUM_ARM64" ;;
  *) CHECKSUM="$CHECKSUM_AMD64" ;;
esac

if [ -n "$PIN" ]; then
  VERSION="$PIN"
else
  VERSION=$(curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    https://checkpoint-api.hashicorp.com/v1/check/terraform |
    sed -n 's/.*"current_version":"\([^"]*\)".*/\1/p')
fi

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "https://releases.hashicorp.com/terraform/${VERSION}/terraform_${VERSION}_linux_${ARCH}.zip" \
  -o /tmp/terraform.zip

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/terraform.zip" | sha256sum -c -
else
  printf '%sWARNING: SHA256 verification skipped for Terraform (no checksum provided in [plugins.versions.terraform])%s\n' "$C_YEL" "$C_RST" >&2
fi

unzip -q -o /tmp/terraform.zip -d /usr/local/bin/
chmod +x /usr/local/bin/terraform
rm /tmp/terraform.zip
