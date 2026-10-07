#!/usr/bin/env bash
# Install the AWS Session Manager plugin (https://github.com/aws/session-manager-plugin)
#
# Integrity: AWS publishes no SHA256 checksums for the .deb — only a detached
# PGP signature (.deb.sig). This script verifies the download against the
# Session Manager plugin signing key bundled below. The key ships with cocoon
# (a different trust domain than s3.amazonaws.com), so a poisoned bucket
# cannot supply both a malicious .deb and a matching key.
#
# Key maintenance: the bundled key is AWS SSM Session Manager
# <session-manager-plugin-signer@amazon.com>, fingerprint
# 7959 6371 24CE 093A D501 D47A 2C4D 4AFF 6F67 57EE (no expiry). If AWS
# rotates it, refresh the block below from
# https://docs.aws.amazon.com/systems-manager/latest/userguide/install-plugin-linux-verify-signature.html
# and update the fingerprint check.
#
# Inputs (env):
#   PIN : plugin version (e.g. "1.2.835.0"); empty = latest. AWS signs
#         releases from 1.2.707.0 onward; older versions have no signature
#         and are rejected.
set -euo pipefail

ARCH="$(dpkg --print-architecture)"
case "$ARCH" in
  amd64) DEB_ARCH="ubuntu_64bit" ;;
  arm64) DEB_ARCH="ubuntu_arm64" ;;
  *)
    echo "ERROR: session-manager-plugin has no .deb for architecture '${ARCH}' (amd64 / arm64 only)" >&2
    exit 1
    ;;
esac

base="https://s3.amazonaws.com/session-manager-downloads/plugin"

if [ -n "$PIN" ]; then
  VERSION="$PIN"
else
  VERSION="$(curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    "${base}/latest/VERSION" | head -n 1 | tr -d '[:space:]')"
  if [ -z "$VERSION" ]; then
    echo "ERROR: ${base}/latest/VERSION returned no version; pin one in [plugins].enable" >&2
    echo "       (e.g. \"session-manager-plugin=<version>\")." >&2
    exit 1
  fi
fi

workdir="$(mktemp -d)"
GNUPGHOME="$(mktemp -d)"
export GNUPGHOME
trap 'rm -rf "$workdir" "$GNUPGHOME"' EXIT

# A 403 from the bucket means the object is missing, so a failure here is
# either a wrong version, an unsigned (pre-1.2.707.0) release, or the network.
fetch() {
  if ! curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    "$1" -o "$2"; then
    echo "ERROR: failed to download $1" >&2
    echo "       Check that session-manager-plugin ${VERSION} exists" >&2
    echo "       (https://github.com/aws/session-manager-plugin/releases), that it is" >&2
    echo "       1.2.707.0 or newer (older releases are unsigned), and that" >&2
    echo "       s3.amazonaws.com is reachable." >&2
    exit 1
  fi
}

deb="${workdir}/session-manager-plugin.deb"
fetch "${base}/${VERSION}/${DEB_ARCH}/session-manager-plugin.deb" "$deb"
fetch "${base}/${VERSION}/${DEB_ARCH}/session-manager-plugin.deb.sig" "${deb}.sig"

gpg --batch --quiet --import <<'SESSION_MANAGER_PGP_KEY'
-----BEGIN PGP PUBLIC KEY BLOCK-----

mFIEZ5ERQxMIKoZIzj0DAQcCAwQjuZy+IjFoYg57sLTGhF3aZLBaGpzB+gY6j7Ix
P7NqbpXyjVj8a+dy79gSd64OEaMxUb7vw/jug+CfRXwVGRMNtIBBV1MgU1NNIFNl
c3Npb24gTWFuYWdlciA8c2Vzc2lvbi1tYW5hZ2VyLXBsdWdpbi1zaWduZXJAYW1h
em9uLmNvbT4gKEFXUyBTeXN0ZW1zIE1hbmFnZXIgU2Vzc2lvbiBNYW5hZ2VyIFBs
dWdpbiBMaW51eCBTaWduZXIgS2V5KYkBAAQQEwgAqAUCZ5ERQ4EcQVdTIFNTTSBT
ZXNzaW9uIE1hbmFnZXIgPHNlc3Npb24tbWFuYWdlci1wbHVnaW4tc2lnbmVyQGFt
YXpvbi5jb20+IChBV1MgU3lzdGVtcyBNYW5hZ2VyIFNlc3Npb24gTWFuYWdlciBQ
bHVnaW4gTGludXggU2lnbmVyIEtleSkWIQR5WWNxJM4JOtUB1HosTUr/b2dX7gIe
AwIbAwIVCAAKCRAsTUr/b2dX7rO1AQCa1kig3lQ78W/QHGU76uHx3XAyv0tfpE9U
oQBCIwFLSgEA3PDHt3lZ+s6m9JLGJsy+Cp5ZFzpiF6RgluR/2gA861M=
=2DQm
-----END PGP PUBLIC KEY BLOCK-----
SESSION_MANAGER_PGP_KEY

# Fail closed if the bundled key block is mangled or replaced: only a
# successful listing that lacks the expected fingerprint reaches the mismatch
# branch, so it stays distinguishable from a gpg crash (which aborts via set -e).
fpr_listing="$(gpg --batch --with-colons --fingerprint)"
if ! printf '%s\n' "$fpr_listing" |
  grep -q '^fpr:::::::::7959637124CE093AD501D47A2C4D4AFF6F6757EE:'; then
  echo "ERROR: bundled Session Manager plugin signing key did not yield the expected fingerprint" >&2
  echo "       (want 7959637124CE093AD501D47A2C4D4AFF6F6757EE)" >&2
  exit 1
fi

gpg --batch --verify "${deb}.sig" "$deb"

# The package depends only on libc6, so dpkg needs no apt index. Its postinst
# symlinks /usr/local/bin/session-manager-plugin onto PATH for the AWS CLI.
dpkg -i "$deb"
