# cocoon — local Go development tasks.
# https://just.systems  (install: `brew install just`, `cargo install just`,
# or download from https://github.com/casey/just/releases)

pkgs       := "./..."
cover_file := "coverage.out"
# Default version is the trimmed contents of the repo-root VERSION file.
# Override at build time with e.g. `VERSION=1.2.3 just build`.
version    := env_var_or_default("VERSION", trim(`cat VERSION 2>/dev/null || echo 0.0.0-dev`))
ldflags    := "-s -w -X github.com/sukekyo26/cocoon/internal/version.Version=" + version

# List available recipes
default:
    @just --list

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

# CI gate: coverage + total threshold (default 85%, override with MIN_COVERAGE)
cover-check:
    #!/usr/bin/env bash
    set -euo pipefail
    min_coverage="${MIN_COVERAGE:-85}"
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

# Run after intentional changes to generators, plugin mutators, or
# `cocoon init` output, then commit the updated golden / snapshot
# files under each package's testdata/ along with the source change
# (covers `*.expected`, `testdata/init/*.workspace.toml`, and
# `testdata/mutator/**/after.toml`). CI runs without -update-golden,
# so any drift fails the test job.
# Regenerate all golden / snapshot files in one shot.
regen-snapshots:
    go test ./internal/generate/dockerfile       -update-golden
    go test ./internal/generate/compose          -update-golden
    go test ./internal/generate/devcontainerjson -update-golden
    go test ./internal/plugin                    -update-golden
    go test ./internal/cli/init                  -update-golden

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

# Composite pre-push gate mirroring the GitHub Actions pipeline.
ci: fmt-check vet lint test vuln mod-verify shellcheck shfmt-check
