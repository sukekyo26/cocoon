#!/usr/bin/env bash
# Install AWS CLI v2 (https://github.com/aws/aws-cli)
#
# Integrity: AWS publishes no SHA256 checksums for the installer zip — only
# a detached PGP signature (.zip.sig). This script verifies the download
# against the AWS CLI Team public key bundled below. The key ships with
# cocoon (a different trust domain than awscli.amazonaws.com), so a poisoned
# CDN cannot supply both a malicious zip and a matching key.
#
# Key maintenance: the bundled key (fingerprint
# FB5DB77FD5C118B80511ADA8A6310ACC4672475C) expires 2026-07-07. After AWS
# extends the expiry, refresh the block below from the AWS CLI install guide
# (https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html).
# A stale key does not break installs — gpg --verify still exits 0 for an
# expired-key signature — it only weakens the freshness of the check.
#
# Inputs (env):
#   PIN : AWS CLI v2 version (without leading "v"); empty = latest.
#         This plugin declares verify = "pgp"; CHECKSUM_AMD64 / CHECKSUM_ARM64
#         are not passed and not consulted.
set -euo pipefail

ARCH="$(dpkg --print-architecture)"
case "$ARCH" in
  amd64) CLI_ARCH="x86_64" ;;
  arm64) CLI_ARCH="aarch64" ;;
  *) CLI_ARCH="x86_64" ;;
esac

if [ -n "$PIN" ]; then
  BASE="awscli-exe-linux-${CLI_ARCH}-${PIN}.zip"
else
  BASE="awscli-exe-linux-${CLI_ARCH}.zip"
fi

GNUPGHOME="$(mktemp -d)"
export GNUPGHOME
trap 'rm -rf "$GNUPGHOME" awscliv2.zip awscliv2.sig aws' EXIT

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "https://awscli.amazonaws.com/${BASE}" -o awscliv2.zip
curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "https://awscli.amazonaws.com/${BASE}.sig" -o awscliv2.sig

gpg --batch --quiet --import <<'AWS_CLI_PGP_KEY'
-----BEGIN PGP PUBLIC KEY BLOCK-----

mQINBF2Cr7UBEADJZHcgusOJl7ENSyumXh85z0TRV0xJorM2B/JL0kHOyigQluUG
ZMLhENaG0bYatdrKP+3H91lvK050pXwnO/R7fB/FSTouki4ciIx5OuLlnJZIxSzx
PqGl0mkxImLNbGWoi6Lto0LYxqHN2iQtzlwTVmq9733zd3XfcXrZ3+LblHAgEt5G
TfNxEKJ8soPLyWmwDH6HWCnjZ/aIQRBTIQ05uVeEoYxSh6wOai7ss/KveoSNBbYz
gbdzoqI2Y8cgH2nbfgp3DSasaLZEdCSsIsK1u05CinE7k2qZ7KgKAUIcT/cR/grk
C6VwsnDU0OUCideXcQ8WeHutqvgZH1JgKDbznoIzeQHJD238GEu+eKhRHcz8/jeG
94zkcgJOz3KbZGYMiTh277Fvj9zzvZsbMBCedV1BTg3TqgvdX4bdkhf5cH+7NtWO
lrFj6UwAsGukBTAOxC0l/dnSmZhJ7Z1KmEWilro/gOrjtOxqRQutlIqG22TaqoPG
fYVN+en3Zwbt97kcgZDwqbuykNt64oZWc4XKCa3mprEGC3IbJTBFqglXmZ7l9ywG
EEUJYOlb2XrSuPWml39beWdKM8kzr1OjnlOm6+lpTRCBfo0wa9F8YZRhHPAkwKkX
XDeOGpWRj4ohOx0d2GWkyV5xyN14p2tQOCdOODmz80yUTgRpPVQUtOEhXQARAQAB
tCFBV1MgQ0xJIFRlYW0gPGF3cy1jbGlAYW1hem9uLmNvbT6JAlQEEwEIAD4CGwMF
CwkIBwIGFQoJCAsCBBYCAwECHgECF4AWIQT7Xbd/1cEYuAURraimMQrMRnJHXAUC
aGveYQUJDMpiLAAKCRCmMQrMRnJHXKBYD/9Ab0qQdGiO5hObchG8xh8Rpb4Mjyf6
0JrVo6m8GNjNj6BHkSc8fuTQJ/FaEhaQxj3pjZ3GXPrXjIIVChmICLlFuRXYzrXc
Pw0lniybypsZEVai5kO0tCNBCCFuMN9RsmmRG8mf7lC4FSTbUDmxG/QlYK+0IV/l
uJkzxWa+rySkdpm0JdqumjegNRgObdXHAQDWlubWQHWyZyIQ2B4U7AxqSpcdJp6I
S4Zds4wVLd1WE5pquYQ8vS2cNlDm4QNg8wTj58e3lKN47hXHMIb6CHxRnb947oJa
pg189LLPR5koh+EorNkA1wu5mAJtJvy5YMsppy2y/kIjp3lyY6AmPT1posgGk70Z
CmToEZ5rbd7ARExtlh76A0cabMDFlEHDIK8RNUOSRr7L64+KxOUegKBfQHb9dADY
qqiKqpCbKgvtWlds909Ms74JBgr2KwZCSY1HaOxnIr4CY43QRqAq5YHOay/mU+6w
hhmdF18vpyK0vfkvvGresWtSXbag7Hkt3XjaEw76BzxQH21EBDqU8WJVjHgU6ru+
DJTs+SxgJbaT3hb/vyjlw0lK+hFfhWKRwgOXH8vqducF95NRSUxtS4fpqxWVaw3Q
V2OWSjbne99A5EPEySzryFTKbMGwaTlAwMCwYevt4YT6eb7NmFhTx0Fis4TalUs+
j+c7Kg92pDx2uQ==
=OBAt
-----END PGP PUBLIC KEY BLOCK-----
AWS_CLI_PGP_KEY

# Fail closed if the bundled key block is ever mangled or replaced. A gpg
# failure here aborts via set -e with gpg's own stderr intact; only a
# successful listing that lacks the expected fingerprint reaches the
# mismatch branch, so the two failure modes stay distinguishable.
fpr_listing="$(gpg --batch --with-colons --fingerprint)"
if ! printf '%s\n' "$fpr_listing" |
  grep -q '^fpr:::::::::FB5DB77FD5C118B80511ADA8A6310ACC4672475C:'; then
  echo "ERROR: bundled AWS CLI signing key did not yield the expected fingerprint" >&2
  echo "       (want FB5DB77FD5C118B80511ADA8A6310ACC4672475C); gpg --fingerprint gave:" >&2
  printf '%s\n' "$fpr_listing" >&2
  exit 1
fi

gpg --batch --verify awscliv2.sig awscliv2.zip

unzip awscliv2.zip
sudo ./aws/install
