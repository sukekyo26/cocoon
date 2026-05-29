package plugincli

import (
	"fmt"

	"github.com/sukekyo26/cocoon/internal/generate/tmplx"
)

// templateKind selects which install.<category>.sh body is generated AND
// determines the file's category suffix. The four kinds correspond 1:1 to
// the catalog method-name vocabulary (see docs/plugins.md):
//
//	binary    — single ELF binary placed on PATH
//	installer — vendor `curl ... | bash` installer script
//	apt       — apt repository / .deb package
//	archive   — multi-file tar/zip extracted to a tree (or unzip+run-installer)
type templateKind string

const (
	tmplInstaller templateKind = "installer"
	tmplBinary    templateKind = "binary"
	tmplApt       templateKind = "apt"
	tmplArchive   templateKind = "archive"
)

// methodDescriptions seeds the one-line label written into
// [install.methods.<name>].description by the scaffolder. Authors are
// expected to refine the text after the file is generated; the default
// gives just enough context for `cocoon plugin show` to display
// something useful immediately.
//
//nolint:gochecknoglobals // tabular default, read-only.
var methodDescriptions = map[templateKind]string{
	tmplInstaller: "Upstream installer script piped to bash",
	tmplBinary:    "Single binary download",
	tmplApt:       "Apt repository / .deb package",
	tmplArchive:   "Multi-file archive extracted to a directory tree",
}

// scaffoldData drives all three render templates (plugin.toml,
// install.<category>.sh, install_user.sh). MethodDesc is derived from
// Template via methodDescription() so callers populate Template only and
// the constructor fills MethodDesc.
type scaffoldData struct {
	ID             string
	Name           string
	Description    string
	URL            string
	Default        bool
	RequiresRoot   bool
	VersionCapable bool
	Template       templateKind
	MethodDesc     string
	WithUserHook   bool
}

// installScriptName returns the catalog-conforming file name for the
// install script of the chosen template (e.g. "install.binary.sh"). Kept
// as a helper so scaffold.go has a single source of truth for the
// install.<category>.sh naming rule.
func installScriptName(t templateKind) string {
	return "install." + string(t) + ".sh"
}

// methodDescription returns the seed description text for the chosen
// template's [install.methods.<name>] entry.
func methodDescription(t templateKind) string {
	if d, ok := methodDescriptions[t]; ok {
		return d
	}
	return string(t)
}

var pluginTOMLTmpl = tmplx.MustParse("plugin.toml", `[metadata]
name = "{{ .Name }}"
description = "{{ .Description }}"
url = "{{ .URL }}"
default = {{ .Default }}

[install]
requires_root = {{ .RequiresRoot }}
default_method = "{{ .Template }}"

[install.methods.{{ .Template }}]
description = "{{ .MethodDesc }}"

[version]
version_capable = {{ .VersionCapable }}
`, nil)

var installInstallerTmpl = tmplx.MustParse("install.installer.sh", `#!/usr/bin/env bash
# Install {{ .Name }} via the upstream installer script.
# Method category: installer — pipes the vendor's curl-to-bash script.
{{- if .VersionCapable }}
#
# Inputs (env):
#   PIN : {{ .Name }} version to install; empty = latest
{{- end }}
set -euo pipefail

# TODO: replace https://example.com/install.sh with the upstream installer URL.
{{ if .VersionCapable -}}
if [ -n "$PIN" ]; then
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    "https://example.com/${PIN}/install.sh" | bash
else
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    https://example.com/install.sh | bash
fi
{{- else -}}
curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  https://example.com/install.sh | bash
{{- end }}
`, nil)

var installBinaryTmpl = tmplx.MustParse("install.binary.sh", `#!/usr/bin/env bash
# Install {{ .Name }} from a GitHub Release binary asset.
# Method category: binary — downloads a single binary (or extracts one
#                  from a tarball) and places it on PATH.
#
# Inputs (env):
#   PIN              : version (without leading "v"); empty = latest
#   CHECKSUM_AMD64   : sha256 of amd64 asset; empty = verify against the
#                      upstream-published checksum (see the else branch below)
#   CHECKSUM_ARM64   : sha256 of arm64 asset; empty = verify against the
#                      upstream-published checksum (see the else branch below)
set -euo pipefail

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

# TODO: replace OWNER/REPO and the asset name pattern with the upstream layout.
if [ -n "$PIN" ]; then
  VERSION="$PIN"
else
  VERSION=$(curl -fsSLI --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    -o /dev/null -w '%{url_effective}' https://github.com/OWNER/REPO/releases/latest |
    sed 's|.*/tag/v||')
fi

curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "https://github.com/OWNER/REPO/releases/download/v${VERSION}/REPO-${DOWNLOAD_ARCH}-unknown-linux-musl" \
  -o /usr/local/bin/{{ .ID }}

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /usr/local/bin/{{ .ID }}" | sha256sum -c -
else
  # No user pin: verify against the checksum the upstream publishes with the
  # release. Replace the URL + asset name to match your upstream's layout
  # (a "<asset>.sha256" sidecar, a "checksums.txt", or "SHA256SUMS"); see
  # docs/plugins.md. Fall back to a loud "WARNING: ... skipped" only if the
  # upstream ships no fetchable checksum.
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    "https://github.com/OWNER/REPO/releases/download/v${VERSION}/checksums.txt" -o /tmp/{{ .ID }}.sums
  expected="$(grep "REPO-${DOWNLOAD_ARCH}-unknown-linux-musl\$" /tmp/{{ .ID }}.sums | cut -d ' ' -f1)"
  echo "${expected}  /usr/local/bin/{{ .ID }}" | sha256sum -c -
  rm -f /tmp/{{ .ID }}.sums
fi

chmod 0755 /usr/local/bin/{{ .ID }}
`, nil)

var installAptTmpl = tmplx.MustParse("install.apt.sh", `#!/usr/bin/env bash
# Install {{ .Name }} via apt.
# Method category: apt — registers an upstream apt repository (or fetches
#                  a .deb directly) and runs apt-get install.
set -euo pipefail

# TODO: pick ONE of the patterns below and remove the others.
#
# Pattern A — third-party apt repository:
#   install -d -m 0755 /etc/apt/keyrings
#   curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
#     https://example.com/keyring.asc | gpg --dearmor -o /etc/apt/keyrings/{{ .ID }}.gpg
#   echo "deb [signed-by=/etc/apt/keyrings/{{ .ID }}.gpg] https://example.com/repo stable main" \
#     > /etc/apt/sources.list.d/{{ .ID }}.list
#   apt-get update
#   DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends {{ .ID }}
#
# Pattern B — direct .deb:
#   curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
#     https://example.com/{{ .ID }}.deb -o /tmp/{{ .ID }}.deb
#   DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends /tmp/{{ .ID }}.deb
#   rm /tmp/{{ .ID }}.deb
`, nil)

var installArchiveTmpl = tmplx.MustParse("install.archive.sh", `#!/usr/bin/env bash
# Install {{ .Name }} from a multi-file archive.
# Method category: archive — extracts a tar/zip with several files
#                  (bin + lib + share, or vendor-bundled installer) into
#                  a directory tree rather than dropping a single binary.
#
# Inputs (env):
{{- if .VersionCapable }}
#   PIN              : version to install; empty = latest
{{- end }}
#   CHECKSUM_AMD64   : sha256 of amd64 archive; empty = verify against the
#                      upstream-published checksum (see the else branch below)
#   CHECKSUM_ARM64   : sha256 of arm64 archive; empty = verify against the
#                      upstream-published checksum (see the else branch below)
set -euo pipefail

ARCH="$(dpkg --print-architecture)"
case "$ARCH" in
  amd64) DOWNLOAD_ARCH="x86_64"; CHECKSUM="$CHECKSUM_AMD64" ;;
  arm64) DOWNLOAD_ARCH="aarch64"; CHECKSUM="$CHECKSUM_ARM64" ;;
  *)     DOWNLOAD_ARCH="x86_64"; CHECKSUM="$CHECKSUM_AMD64" ;;
esac

# TODO: replace the URL pattern and destination directory.
{{ if .VersionCapable -}}
if [ -n "$PIN" ]; then VERSION="$PIN"; else VERSION="LATEST"; fi
{{- end }}
curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
  "https://example.com/{{ .ID }}-${DOWNLOAD_ARCH}.tar.gz" -o /tmp/{{ .ID }}.tar.gz
if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/{{ .ID }}.tar.gz" | sha256sum -c -
else
  # No user pin: verify against the checksum the upstream publishes with the
  # release (a "<asset>.sha256" / ".sha256sum" sidecar, a "checksums.txt" /
  # "SHA256SUMS", or a field in a release JSON); see docs/plugins.md.
  # Replace the URL below to match your upstream's layout.
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors \
    "https://example.com/{{ .ID }}-${DOWNLOAD_ARCH}.tar.gz.sha256" -o /tmp/{{ .ID }}.sum
  expected="$(cut -d ' ' -f1 /tmp/{{ .ID }}.sum)"
  echo "${expected}  /tmp/{{ .ID }}.tar.gz" | sha256sum -c -
  rm -f /tmp/{{ .ID }}.sum
fi
mkdir -p /usr/local/{{ .ID }}
tar -C /usr/local/{{ .ID }} --strip-components=1 -xzf /tmp/{{ .ID }}.tar.gz
rm /tmp/{{ .ID }}.tar.gz
`, nil)

var installUserTmpl = tmplx.MustParse("install_user.sh", `#!/usr/bin/env bash
# Configure {{ .Name }} for the user's bash shell.
#
# TODO: append shell init lines to ~/.bashrc, e.g.:
#   echo 'eval "$(yourtool init bash)"' >> ~/.bashrc
# Add "# shellcheck disable=SC2016" right before the line if you use single
# quotes around an expression that intentionally must not be expanded.
set -euo pipefail

# Replace this no-op with the real configuration step.
:
`, nil)

// renderPluginTOML returns the rendered plugin.toml body.
func renderPluginTOML(d scaffoldData) (string, error) {
	out, err := tmplx.Render(pluginTOMLTmpl, d)
	if err != nil {
		return "", fmt.Errorf("render plugin.toml: %w", err)
	}
	return out, nil
}

// renderInstallSh returns the rendered install.<category>.sh body for the
// chosen template. Unknown templates fall through to the installer body
// (cheapest stub), but the finalizeOpts switch in scaffold.go is the real
// gatekeeper and would reject unknowns before this is called.
func renderInstallSh(d scaffoldData) (string, error) {
	var (
		body string
		err  error
	)
	switch d.Template {
	case tmplInstaller:
		body, err = tmplx.Render(installInstallerTmpl, d)
	case tmplBinary:
		body, err = tmplx.Render(installBinaryTmpl, d)
	case tmplApt:
		body, err = tmplx.Render(installAptTmpl, d)
	case tmplArchive:
		body, err = tmplx.Render(installArchiveTmpl, d)
	default:
		body, err = tmplx.Render(installInstallerTmpl, d)
	}
	if err != nil {
		return "", fmt.Errorf("render %s (%s): %w", installScriptName(d.Template), d.Template, err)
	}
	return body, nil
}

// renderInstallUserSh returns the rendered install_user.sh body.
func renderInstallUserSh(d scaffoldData) (string, error) {
	out, err := tmplx.Render(installUserTmpl, d)
	if err != nil {
		return "", fmt.Errorf("render install_user.sh: %w", err)
	}
	return out, nil
}
