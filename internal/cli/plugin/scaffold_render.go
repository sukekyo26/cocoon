package plugincli

import (
	"fmt"

	"github.com/sukekyo26/cocoon/internal/generate/tmplx"
)

// templateKind selects which install.sh body is generated.
type templateKind string

const (
	tmplCurlPipe templateKind = "curl-pipe"
	tmplTarball  templateKind = "tarball"
	tmplGeneric  templateKind = "generic"
)

// scaffoldData drives all three render templates (plugin.toml, install.sh,
// install_user.sh).
type scaffoldData struct {
	ID             string
	Name           string
	Description    string
	Default        bool
	RequiresRoot   bool
	VersionCapable bool
	Template       templateKind
	WithUserHook   bool
}

var pluginTOMLTmpl = tmplx.MustParse("plugin.toml", `[metadata]
name = "{{ .Name }}"
description = "{{ .Description }}"
default = {{ .Default }}

[install]
requires_root = {{ .RequiresRoot }}

[version]
version_capable = {{ .VersionCapable }}
`, nil)

var installCurlPipeTmpl = tmplx.MustParse("install.sh.curl-pipe", `#!/usr/bin/env bash
# Install {{ .Name }}
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

var installTarballTmpl = tmplx.MustParse("install.sh.tarball", `#!/usr/bin/env bash
# Install {{ .Name }}
#
# Inputs (env):
#   PIN              : version (without leading "v"); empty = latest
#   CHECKSUM_AMD64   : sha256 of amd64 tarball; empty to skip verification
#   CHECKSUM_ARM64   : sha256 of arm64 tarball; empty to skip verification
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
  "https://github.com/OWNER/REPO/releases/download/v${VERSION}/REPO-${DOWNLOAD_ARCH}-unknown-linux-musl.tar.gz" \
  -o /tmp/{{ .ID }}.tar.gz

if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/{{ .ID }}.tar.gz" | sha256sum -c -
else
  echo "WARNING: SHA256 verification skipped for {{ .Name }} (no checksum for {{ .ID }} in [plugins.versions])" >&2
fi

# TODO: extract to the right destination.
tar -xzf /tmp/{{ .ID }}.tar.gz -C /usr/local/bin
rm /tmp/{{ .ID }}.tar.gz
`, nil)

var installGenericTmpl = tmplx.MustParse("install.sh.generic", `#!/usr/bin/env bash
# Install {{ .Name }}
{{- if .VersionCapable }}
#
# Inputs (env):
#   PIN              : version to install; empty = latest
#   CHECKSUM_AMD64   : sha256 (optional); empty to skip verification
#   CHECKSUM_ARM64   : sha256 (optional); empty to skip verification
{{- end }}
set -euo pipefail

# TODO: implement the install steps. Common patterns:
#   - apt-get update && apt-get install -y <pkg> \
#       && apt-get clean && rm -rf /var/lib/apt/lists/*
#   - dpkg -i <package>.deb
#   - cargo install / go install / npm install -g
#
# Architecture detection (uncomment if needed):
# ARCH="$(dpkg --print-architecture)"   # amd64 | arm64
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

// renderInstallSh returns the rendered install.sh body for the chosen template.
func renderInstallSh(d scaffoldData) (string, error) {
	var (
		body string
		err  error
	)
	switch d.Template {
	case tmplCurlPipe:
		body, err = tmplx.Render(installCurlPipeTmpl, d)
	case tmplTarball:
		body, err = tmplx.Render(installTarballTmpl, d)
	case tmplGeneric:
		body, err = tmplx.Render(installGenericTmpl, d)
	default:
		body, err = tmplx.Render(installGenericTmpl, d)
	}
	if err != nil {
		return "", fmt.Errorf("render install.sh (%s): %w", d.Template, err)
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
