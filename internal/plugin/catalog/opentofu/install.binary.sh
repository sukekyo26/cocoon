#!/usr/bin/env bash
# Install OpenTofu (https://github.com/opentofu/opentofu)
#
# Integrity: the SHA256SUMS manifest is verified with gpg against OpenTofu's
# release signing key (bundled below) before the downloaded tarball is checked
# against it. The key ships with cocoon — a different trust domain than the
# GitHub release CDN — so a poisoned mirror cannot supply both a malicious
# tarball and a matching signature. This plugin declares verify = "pgp": it
# takes no per-workspace CHECKSUM_*, and signature verification covers every
# version (pinned or latest) without per-release checksum maintenance.
#
# Key maintenance: the bundled key is OpenTofu <core@opentofu.org>, primary
# fingerprint E3E6 E43D 84CB 852E ADB0 051D 0C0A F313 E5FD 9F80. An expired
# key still verifies past signatures (gpg exits 0); only an actual key
# replacement breaks installs. If OpenTofu rotates the key, refresh the block
# below from https://get.opentofu.org/opentofu.asc and update the fingerprint
# check.
#
# Inputs (env):
#   PIN : OpenTofu version without leading "v" (e.g. "1.10.6"); empty = latest
set -euo pipefail

ARCH="$(dpkg --print-architecture)"

if [ -n "$PIN" ]; then
  VERSION="$PIN"
else
  VERSION=$(curl -fsSLI --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    -o /dev/null -w '%{url_effective}' https://github.com/opentofu/opentofu/releases/latest |
    sed 's|.*/tag/v||')
fi

base="https://github.com/opentofu/opentofu/releases/download/v${VERSION}"
asset="tofu_${VERSION}_linux_${ARCH}.tar.gz"
sums="tofu_${VERSION}_SHA256SUMS"

workdir="$(mktemp -d)"
GNUPGHOME="$(mktemp -d)"
export GNUPGHOME
trap 'rm -rf "$workdir" "$GNUPGHOME"' EXIT

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "${base}/${asset}" -o "${workdir}/${asset}"
curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "${base}/${sums}" -o "${workdir}/${sums}"
curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "${base}/${sums}.gpgsig" -o "${workdir}/${sums}.sig"

gpg --batch --quiet --import <<'OPENTOFU_PGP_KEY'
-----BEGIN PGP PUBLIC KEY BLOCK-----

xsFNBGVUyIwBEADPg6jUJm5liMTiDndyprnwXQ23GdyQm/kW9MFOhYDRksmmbsz0
DCfqntFpuoKxPXzA+JTrZlWZONtU+leZjIOlAVZiz0rwz5EJq7uIrkueWtUk6AYk
BLN+zMtbui0z3HCPVNnR5BlVNyXQeW3jlrQtzuKevjZWzI0gbQGgEKNpj+lfyRFu
6q3u/T0o3p/6bOOlQHwCMtnFlWpjr6f/J2EdUVO/6NYHQzImPj4LINXF/+eqo7v6
svFtaVTtREG2V2V7We7bu/cJ+NgJYH7ro7UhB1RQH2k09NdpSCt9F60PVERnORpx
GBkM/VKZzgMSzRvdpxUWwrLxfAxinu5ddbBm3y0bzaU80OT3i1qrWIqW73fmdGHQ
71gbJxRrroyLMWehjcJ/9WJDxkHqsfPKqBifYsp6/J9npczDfSU+zYBVGpR73a4E
dbeIRWqwbH0LWhlbi1IM5aFDaZMFNkY+AWyP+OHn8Kehu6DOIh1AVM7v7vLxaX9h
t1jVJbswjvPFYquv1DvUdc7VP2QHz3xctQS1GZJQ1ekcgTv9rRYXUOOwknInjtkM
9kQDtyBkVLcEc8ha3Cfh6PJscIP5VHwaNMgAPr9tsl3xqdz56l5UPjFSFuel98jS
Bqn83VrT0uKwM0PnDVHd/7q8+Dg1EtOggMwZ830KORFNdjfv6ydsBvl7fwARAQAB
zUpPcGVuVG9mdSAoVGhpcyBrZXkgaXMgdXNlZCB0byBzaWduIG9wZW50b2Z1IHBy
b3ZpZGVycykgPGNvcmVAb3BlbnRvZnUub3JnPsLBjAQTAQgAQQUCZVTIjAkQDArz
E+X9n4AWIQTj5uQ9hMuFLq2wBR0MCvMT5f2fgAIbAwIeAQIZAQMLCQcCFQgDFgAC
BScJAgcCAABwAg/1HZnTvPHZDWf5OluYOaQ7ADX/oyjUO85VNUmKhmBZkLr5mTqr
LO72k9fg+101hbggbhtK431z3Ca6ZqDAG/3DBi0BC1ag0rw83TEApkPGYnfX1DWS
1ZvyH1PkV0aqCkXAtMrte2PlUiieaKAsiYOIXqfZwszd07gch14wxMOw1B6Au/Xz
Nrv2omnWSgGIyR6WOsG4QQ8R5AMVz3K8Ftzl6520wBgtr3osA3uM/xconnGVukMn
9NLQqKx5oeaJwONZpyZL5bg2ke9MVZM2+bG30UGZKoxrzOtQ//OTOYlhPCqm1ffR
hYrUytwsWzDnJvXJF1QhnDu8whP3tSrcHyKxYZ9xUNzeu2AmjYfvkKHSdK2DFmOf
DafaRs3c1VYnC7J7aRi6kVF/t+vWeOEVpPylyK7vSbPFc6XVoQrsE07hbN/BjWjm
s8voK5U6oJRgEugXtSQKFypfOq8R99nXwbMHdhqY8aGyOCj++cuvRCUBDZAQqPEW
AuD0X7+9Trnfin47MK+n18wsTAL4w6PJhtCrwK4e0cVuQ5u4M/PMid5W6hEA27PX
x506Jpe8iRmcIP/cCR6pvhgOUMC36bIkAqZ5dJ545kDQju0lf8gLdVIQpig45udn
ZM2KgyApGqhsS7yCUrbLDrtNmQ31TSYdKc8IU+/jXkfy2RYbZ+wNgfloKM7BTQRl
VMiMARAAwRZUyMIc5TNbcFg3WGKxhaNC9hDZ4zBfXlb5jONzZOx3rDi2lD4UQOH+
NpG7CF98co//kryS/4AsDdp2jzhh+VMgyx6KJIhSkBP6kqhriy9eWRmgfrnLbUf4
6kkTkzLVkjYnMNeyHt+mi9I7EKtsDuF/EvjlwF5E81+DEOteCO/un/Qt1q3e1Slf
vTpLkPvr1FiQ3VqzaBeBBI3MAMb/ycwL6hQE1l4Lg34T43Zu+9zkE1uzvjeNIlIW
ucjB4q1htEjJl2CLAv+8cGHdmCcV2ZO3WM8M9Omq1CE7jhak4NE/YuGylJYCBd+B
S7tuDPDu6+o4Nx+axxcwMvgyfr07FteEr1Lopaw2ci8b/xzQie/gkI0CByQMwD5V
gnJpiMBnjP4d6UF6HEVldCQ7a3T1T80bKj5JjtFbR9P85Qntuheqn3Pge89YexMc
E/00VA3blrj+GeYpO9ZGFu7DR/x4sjnTEhfjXEoLv1C4AdgGHCIjW9wU6HkcWnla
X7akKlwIWEUP/BFLkcWPpmUrtClhWx9wq1GHFvKAN/qp//VWnv4IfRU6RjmVPOWB
efvTu/cpsfBHLyp15goOYPboahIdTUTNQIXh4Vid7E1NoKnWZUMu50n3/zAbjSds
mNmifi4g01MYJ3TVoU2Q01P7NiD3IRmaw72nLmf9cM9/7QMdGn0AEQEAAcLBdgQY
AQgAKgUCZVTIjAkQDArzE+X9n4AWIQTj5uQ9hMuFLq2wBR0MCvMT5f2fgAIbDAAA
SUoP/2ExsUoGbxjuZ76QUnYtfzDoz+o218UWd3gZCsBQ6/hGam5kMq+EUEabF3lV
7QLDyn/1v5sqrkmYg0u5cfjtY3oimCPvr6E0WTuqMIwYl0fdlkmdNttDpMqvCazq
bzLK5dDVWbh/EYTiEN1xKXM6rlAquYv8I16uWL8QHanMb6yexNmDYhC4fXWqCi+s
5sXxWrPrd+fGz8CR/fEYahPXj8uY6dwN9DlWyek9QtKW2PsqrkBn5vCOm2IyZW6d
t/Kn70tYtxMxJND2otk47mpG/Fv3sYK2bTGJ+k/5+E5IrjWqIX2lVB3G1+TCoZ5s
cc16zls32mOlRh81fTAqcwkDFxICxcOeNHGLt3N+UvoPSUafYKD96rn5mWFao4xb
cFniaYv2PdqH8HDjvXZXqHypRMXvYMbXXOgydLL+tSUSBpMTd4afjq8x2gNSWOEL
I1jT5FWbKTKan0ycKi37bSqGHhDjlg4HRGvC3IK0EuVjdX3r+8uIVgFbqLwNhXk4
GAIL03vl689TQ7/oPW75XCQIevFai0kcJPl6qIRvi9/S/v5EPRy9UDCGY/MPmc5f
H1an0ebU4I4TlYfBoEUkYYqBDxvxWW0I/Q01rDebcd6mrGw8lW1EiNZlClLwx9Bv
/+MNnIT9m1f8KeqmweoAgbIQRUI7EkJSzxYN4DNuy2XoKmF9
=VhyH
-----END PGP PUBLIC KEY BLOCK-----
OPENTOFU_PGP_KEY

# Fail closed if the bundled key block is mangled or replaced: only a
# successful listing that lacks the expected fingerprint reaches the mismatch
# branch, so it stays distinguishable from a gpg crash (which aborts via set -e).
fpr_listing="$(gpg --batch --with-colons --fingerprint)"
if ! printf '%s\n' "$fpr_listing" |
  grep -q '^fpr:::::::::E3E6E43D84CB852EADB0051D0C0AF313E5FD9F80:'; then
  echo "ERROR: bundled OpenTofu signing key did not yield the expected fingerprint" >&2
  echo "       (want E3E6E43D84CB852EADB0051D0C0AF313E5FD9F80)" >&2
  exit 1
fi

gpg --batch --verify "${workdir}/${sums}.sig" "${workdir}/${sums}"

# Extract the asset's line from the now-trusted manifest and check the tarball.
expected="$(grep "  ${asset}\$" "${workdir}/${sums}" | cut -d ' ' -f1)"
echo "${expected}  ${workdir}/${asset}" | sha256sum -c -

tar -xzf "${workdir}/${asset}" -C /usr/local/bin tofu
chmod +x /usr/local/bin/tofu
