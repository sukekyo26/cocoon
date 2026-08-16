# cocoon — local Go development tasks.
# https://just.systems  (install: `brew install just`, `cargo install just`,
# or download from https://github.com/casey/just/releases)

pkgs       := "./..."
cover_file := "coverage.out"
# Default version is the trimmed contents of the repo-root VERSION file.
# Override at build time with e.g. `VERSION=1.2.3 just build`.
version    := env_var_or_default("VERSION", trim(`cat VERSION 2>/dev/null || echo 0.0.0-dev`))
ldflags    := "-s -w -X github.com/sukekyo26/cocoon/internal/version.Version=" + version
# Trivy scan target: the generated .devcontainer/ tree to audit. cocoon's own
# repo has no .devcontainer/, so the default points at the sandbox
# e2e/docker-roundtrip.sh leaves behind (CI's target; its trap tears down
# containers, not the directory). Override per invocation, e.g.
# `just trivy-static ~/myproject/.devcontainer`.
devcontainer_dir := "e2e/test-project/.devcontainer"
trivy_ignore     := justfile_directory() + "/.trivyignore.yaml"

# List available recipes
default:
    @just --list

# Installs into `$(go env GOBIN)`, or the first GOPATH entry's bin when GOBIN is
# unset; `go` and `just` must already be present.
# Tool versions are pinned to match CI — keep them in sync with the workflow
# files noted inline. shellcheck has no `go install` path; install it from your
# OS package manager (the recipe warns if it is missing).
# Install the pinned dev tools `just ci` needs (govulncheck, shfmt, golangci-lint).
setup:
    #!/usr/bin/env bash
    set -euo pipefail
    # Mirror where `go install` lands so all three tools share one directory
    # and the printed path is accurate: $GOBIN if set, else the first GOPATH
    # entry's bin (golangci-lint's `-b` then targets the same dir).
    bindir="$(go env GOBIN)"
    [ -n "${bindir}" ] || bindir="$(go env GOPATH | cut -d: -f1)/bin"
    mkdir -p "${bindir}"
    echo "Installing dev tools into ${bindir} ..."
    # govulncheck — keep in sync with .github/workflows/go-ci.yml
    go install golang.org/x/vuln/cmd/govulncheck@v1.3.0
    # shfmt — keep in sync with SHFMT_VERSION in .github/workflows/shfmt.yml
    go install mvdan.cc/sh/v3/cmd/shfmt@v3.10.0
    # golangci-lint — keep in sync with .github/workflows/go-ci.yml; the pinned
    # install.sh SHA256-verifies the downloaded binary.
    curl -sSfL --proto '=https' --tlsv1.2 \
        https://raw.githubusercontent.com/golangci/golangci-lint/8f3b0c7ed018e57905fbd873c697e0b1ede605a5/install.sh \
        | sh -s -- -b "${bindir}" v2.11.4
    command -v shellcheck >/dev/null 2>&1 \
        || echo >&2 "NOTE: shellcheck not found — install via 'apt-get install shellcheck' or 'brew install shellcheck'"
    command -v trivy >/dev/null 2>&1 \
        || echo >&2 "NOTE: trivy not found — see https://github.com/aquasecurity/trivy/releases or 'brew install trivy' (only needed for the trivy-* recipes; not part of 'just ci')"
    echo "Done. Ensure ${bindir} is on your PATH, then run 'just ci'."

# Format Go source with gofumpt + goimports (via golangci-lint formatters)
fmt:
    golangci-lint fmt {{pkgs}}

# Verify the tree is formatted (CI gate)
fmt-check:
    golangci-lint fmt --diff {{pkgs}}

# Run golangci-lint with the strict config
lint:
    golangci-lint run {{pkgs}}

# Run `go vet`
vet:
    go vet {{pkgs}}

# Run go test with shuffled order (no race; race needs CGO + a C compiler)
test: build
    go test -shuffle=on {{pkgs}}

# Run go test with race detector (CI-only; needs CGO and gcc/clang installed)
test-race: build
    CGO_ENABLED=1 go test -race -shuffle=on {{pkgs}}

# Run go test with coverage and emit coverage.out + coverage.html
cover:
    go test -shuffle=on -covermode=atomic -coverpkg=./internal/... -coverprofile={{cover_file}} {{pkgs}}
    go tool cover -html={{cover_file}} -o coverage.html
    go tool cover -func={{cover_file}} | tail -1

# CI gate: coverage + total threshold (default 90%, override with MIN_COVERAGE)
cover-check:
    #!/usr/bin/env bash
    set -euo pipefail
    min_coverage="${MIN_COVERAGE:-90}"
    go test -shuffle=on -covermode=atomic \
        -coverpkg=./internal/... -coverprofile={{cover_file}} {{pkgs}}
    go tool cover -func={{cover_file}} | tail -1
    total=$(go tool cover -func={{cover_file}} | awk '/^total:/ {gsub("%","",$3); print $3}')
    echo "Total coverage: ${total}% (threshold: ${min_coverage}%)"
    awk -v t="$total" -v m="$min_coverage" 'BEGIN { exit (t+0 >= m+0) ? 0 : 1 }' || {
        echo "::error::Coverage ${total}% is below threshold ${min_coverage}%"
        exit 1
    }

# Run govulncheck against the module
vuln:
    govulncheck ./...

# Run after intentional changes to generators or `cocoon init` output,
# then commit the updated golden / snapshot files under each package's
# testdata/ along with the source change (covers `*.expected` and
# `testdata/init/*.cocoon.toml`). CI runs without -update-golden,
# so any drift fails the test job.
# Regenerate all golden / snapshot files in one shot.
regen-snapshots:
    go test ./internal/generate/dockerfile       -update-golden
    go test ./internal/generate/compose          -update-golden
    go test ./internal/generate/devcontainerjson -update-golden
    go test ./internal/generate/codeworkspace    -update-golden
    go test ./internal/cli/init                  -update-golden
    go test ./internal/cli                       -run TestHelpGolden -update-golden

# Build a cocoon binary for the host OS/arch.
build:
    @mkdir -p bin
    CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags "{{ldflags}}" -o bin/cocoon ./cmd/cocoon

# Cross-compile bin/cocoon-{linux,darwin}-{amd64,arm64}.
build-all:
    @mkdir -p bin
    CGO_ENABLED=0 GOOS=linux  GOARCH=amd64 go build -trimpath -buildvcs=false -ldflags "{{ldflags}}" -o bin/cocoon-linux-amd64  ./cmd/cocoon
    CGO_ENABLED=0 GOOS=linux  GOARCH=arm64 go build -trimpath -buildvcs=false -ldflags "{{ldflags}}" -o bin/cocoon-linux-arm64  ./cmd/cocoon
    CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -buildvcs=false -ldflags "{{ldflags}}" -o bin/cocoon-darwin-amd64 ./cmd/cocoon
    CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -buildvcs=false -ldflags "{{ldflags}}" -o bin/cocoon-darwin-arm64 ./cmd/cocoon

# Build all release binaries and write bin/SHA256SUMS for GitHub Releases.
release-assets: build-all
    cd bin && sha256sum cocoon-linux-amd64 cocoon-linux-arm64 \
                        cocoon-darwin-amd64 cocoon-darwin-arm64 > SHA256SUMS
    @echo "wrote bin/SHA256SUMS"

# Verify go.sum integrity (supply-chain check)
mod-verify:
    go mod verify

# Run shellcheck across all *.sh files in the repo (severity=style)
shellcheck:
    @command -v shellcheck >/dev/null 2>&1 || { echo >&2 "shellcheck not installed; install via 'apt-get install shellcheck' / 'brew install shellcheck'"; exit 1; }
    shellcheck --severity=style $(find . -type f -name '*.sh' -not -path './.git/*' -not -path './bin/*')

# Format all *.sh files in-place with shfmt (gofmt-style)
shfmt:
    @command -v shfmt >/dev/null 2>&1 || { echo >&2 "shfmt not installed; see https://github.com/mvdan/sh/releases or 'brew install shfmt'"; exit 1; }
    shfmt -i 2 -ci -w $(find . -type f -name '*.sh' -not -path './.git/*' -not -path './bin/*')

# Verify all *.sh files are shfmt-clean (CI gate)
shfmt-check:
    @command -v shfmt >/dev/null 2>&1 || { echo >&2 "shfmt not installed; see https://github.com/mvdan/sh/releases or 'brew install shfmt'"; exit 1; }
    shfmt -i 2 -ci -d $(find . -type f -name '*.sh' -not -path './.git/*' -not -path './bin/*')

# Trivy misconfiguration scan of a generated .devcontainer/. Only the
# Dockerfile is in scope: Trivy ships no misconfig checks for Compose, so
# docker-compose.yml is a no-op here (compose posture is asserted instead by
# the SUDO_MODE checks in e2e/docker-roundtrip.sh). CI gate: any finding not
# justified in .trivyignore.yaml exits 1.
# Scan a generated .devcontainer/ for Dockerfile misconfigurations.
trivy-config dir=devcontainer_dir:
    @command -v trivy >/dev/null 2>&1 || { echo >&2 "trivy not installed; see https://github.com/aquasecurity/trivy/releases or 'brew install trivy'"; exit 1; }
    trivy config --quiet --exit-code 1 --misconfig-scanners dockerfile --ignorefile "{{trivy_ignore}}" "{{dir}}"

# Trivy secret scan of a generated .devcontainer/. Matters most for the
# Dockerfile: plugin install scripts are inlined verbatim as quoted heredocs,
# so a token committed into a catalog install.sh surfaces here. Needs no
# vulnerability DB. CI gate: any hit exits 1.
# Scan a generated .devcontainer/ for leaked secrets.
trivy-secret dir=devcontainer_dir:
    @command -v trivy >/dev/null 2>&1 || { echo >&2 "trivy not installed; see https://github.com/aquasecurity/trivy/releases or 'brew install trivy'"; exit 1; }
    trivy fs --quiet --scanners secret --exit-code 1 --ignorefile "{{trivy_ignore}}" "{{dir}}"

# Both build-free gates in one shot. They read the generated Dockerfile alone,
# which is what makes them usable from plugin-e2e.yml, where the single preset
# forces BUILD_ONLY and never loads an image.
# Run both build-free Trivy gates over a generated .devcontainer/.
trivy-static dir=devcontainer_dir: (trivy-config dir) (trivy-secret dir)

# Report-only CVE scan of a built image. Deliberately never gates: base image
# CVEs are upstream and unfixable here, so a red check would train reviewers
# to ignore it.
# Report HIGH/CRITICAL CVEs in a built image (never fails).
trivy-image image:
    @command -v trivy >/dev/null 2>&1 || { echo >&2 "trivy not installed; see https://github.com/aquasecurity/trivy/releases or 'brew install trivy'"; exit 1; }
    trivy image --quiet --exit-code 0 --scanners vuln --severity HIGH,CRITICAL --ignore-unfixed "{{image}}"

# Same misconfig gate against the committed generator snapshots. Needs no
# docker and no `cocoon gen`, so it runs on a bare checkout — the local smoke
# test before touching the Dockerfile generator. Covers the shell variants
# only; the CI gates above cover real output across every plugin.
# Run the misconfig gate against the committed Dockerfile snapshots.
trivy-golden:
    @command -v trivy >/dev/null 2>&1 || { echo >&2 "trivy not installed; see https://github.com/aquasecurity/trivy/releases or 'brew install trivy'"; exit 1; }
    trivy config --quiet --exit-code 1 --misconfig-scanners dockerfile --ignorefile "{{trivy_ignore}}" --file-patterns 'dockerfile:.*\.expected' internal/generate/dockerfile/testdata

# Composite pre-push gate mirroring the GitHub Actions pipeline. The trivy-*
# recipes are deliberately excluded: they audit `cocoon gen` OUTPUT, which
# this repo does not carry and CI produces via e2e/docker-roundtrip.sh (which
# local WSL2 cannot run). Use `just trivy-golden` for the build-free local
# smoke test.
ci: fmt-check vet lint test cover-check vuln mod-verify shellcheck shfmt-check
