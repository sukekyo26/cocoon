#!/bin/bash
# ============================================================
# tests/structure/test_snapshot.sh
# Snapshot tests: compare generated files against expected output across
# multiple workspace.toml fixtures.
# ============================================================
# Usage:
#   ./tests/test_snapshot.sh           # Compare snapshots
#   ./tests/test_snapshot.sh --update  # Update snapshot files
# ============================================================

set -uo pipefail

TESTS_DIR="$(cd "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")"/.. && pwd)"
# shellcheck source=../test_helper.sh
source "$TESTS_DIR/test_helper.sh"

# Invoke the Go binary directly. Snapshot tests do not need any of the
# behavioural wrappers from lib/ — they only verify that `wsd generate-all`
# emits byte-identical artefacts for each pinned fixture.
#
# Note: bin/wsd-dispatch.sh falls back to a GitHub Releases download when
# no in-tree binary exists. CI runs `go build -o bin/wsd-linux-amd64 ./cmd/wsd`
# first (see ci.yml :: test). For local runs, do the same via `just build-all`
# or set WSD_BINARY to your dev build.
generate_all() {
  local workspace_toml="$1"
  local output_dir="$2"
  local plugins_dir
  plugins_dir="$(cd "$(dirname "$workspace_toml")" && pwd)/plugins"
  mkdir -p "$output_dir"
  "$PROJECT_ROOT/bin/wsd-dispatch.sh" generate-all "$workspace_toml" "$plugins_dir" "$output_dir"
}

echo ""
echo "[ test_snapshot.sh ]"

SNAPSHOT_DIR="$TESTS_DIR/snapshots"

# Fixtures whose generated artefacts are byte-compared against
# tests/snapshots/<name>/. Add a new entry to fan out coverage.
declare -A SNAPSHOT_FIXTURES=(
  ["default"]="$TESTS_DIR/fixtures/snapshot.workspace.toml"
  ["pinned"]="$TESTS_DIR/fixtures/ci/pinned.workspace.toml"
  ["arm64-smoke"]="$TESTS_DIR/fixtures/ci/arm64-smoke.workspace.toml"
  ["ports-comprehensive"]="$TESTS_DIR/fixtures/ci/ports-comprehensive.workspace.toml"
  ["certs-with-https-mirror"]="$TESTS_DIR/fixtures/ci/certs-with-https-mirror.workspace.toml"
  ["debian-bookworm"]="$TESTS_DIR/fixtures/ci/debian-bookworm.workspace.toml"
  ["debian-trixie"]="$TESTS_DIR/fixtures/ci/debian-trixie.workspace.toml"
)

# Fixtures that drift with upstream (e.g. latest versions) and only need a
# generation-success smoke check, not a byte-equal compare.
declare -A SMOKE_FIXTURES=(
  ["latest"]="$TESTS_DIR/fixtures/ci/latest.workspace.toml"
)

UPDATE_MODE=false
[[ "${1:-}" == "--update" ]] && UPDATE_MODE=true

# ============================================================
# Snapshot file targets (relative to the generated workspace dir)
# ============================================================
SNAPSHOT_FILES=(
  "Dockerfile"
  "docker-compose.yml"
  ".devcontainer/devcontainer.json"
  ".devcontainer/docker-compose.yml"
)

# Map a generated rel-path to the on-disk snapshot filename.
# Snapshot paths mirror the generated layout — `.devcontainer/*` artefacts live
# under `.devcontainer/` in the snapshot tree as well.
snapshot_filename_for() {
  case "$1" in
    ".devcontainer/devcontainer.json") echo ".devcontainer/devcontainer.json.expected" ;;
    "Dockerfile")                       echo "Dockerfile.expected" ;;
    "docker-compose.yml")               echo "docker-compose.yml.expected" ;;
    ".devcontainer/docker-compose.yml") echo ".devcontainer/docker-compose.yml.expected" ;;
    *) return 1 ;;
  esac
}

# ============================================================
# Helper: stage a temporary workspace from a fixture and run generate-all.
# Sets WORK_DIR to the generated workspace path.
# ============================================================
WORK_DIR=""
_SNAPSHOT_TMPDIR=""

generate_for_fixture() {
  local fixture_toml="$1"
  _SNAPSHOT_TMPDIR=$(mktemp -d)
  WORK_DIR="$_SNAPSHOT_TMPDIR/cocoon"
  mkdir -p "$WORK_DIR"

  cp -r "$PROJECT_ROOT/plugins" "$WORK_DIR/"
  cp -r "$PROJECT_ROOT/config" "$WORK_DIR/"
  mkdir -p "$WORK_DIR/certs"

  # Copy fixture-specific certs if a sibling <basename>.certs/ dir exists.
  # Used by certs-with-https-mirror to lock down the cert install RUN order.
  local fixture_certs="${fixture_toml%.workspace.toml}.certs"
  if [[ -d "$fixture_certs" ]]; then
    cp -r "$fixture_certs"/. "$WORK_DIR/certs/"
  fi

  cp "$fixture_toml" "$WORK_DIR/workspace.toml"

  (
    cd "$WORK_DIR" || exit 1
    generate_all "workspace.toml" "."
  )
}

cleanup() {
  [[ -n "$_SNAPSHOT_TMPDIR" && -d "$_SNAPSHOT_TMPDIR" ]] && rm -rf "$_SNAPSHOT_TMPDIR"
  WORK_DIR=""
  _SNAPSHOT_TMPDIR=""
}

# ============================================================
# Update mode: regenerate every snapshot fixture
# ============================================================
if [[ "$UPDATE_MODE" == true ]]; then
  echo "Updating snapshots..."

  for fixture_name in "${!SNAPSHOT_FIXTURES[@]}"; do
    fixture_toml="${SNAPSHOT_FIXTURES[$fixture_name]}"
    if [[ ! -f "$fixture_toml" ]]; then
      echo "  Skipping $fixture_name (missing fixture: $fixture_toml)"
      continue
    fi
    echo "  Fixture: $fixture_name"
    generate_for_fixture "$fixture_toml"
    out_root="$SNAPSHOT_DIR/$fixture_name"
    for rel in "${SNAPSHOT_FILES[@]}"; do
      gen_path="$WORK_DIR/$rel"
      [[ -f "$gen_path" ]] || continue
      snap_name=$(snapshot_filename_for "$rel")
      snap_path="$out_root/$snap_name"
      mkdir -p "$(dirname "$snap_path")"
      cp "$gen_path" "$snap_path"
      echo "    Updated: $fixture_name/$snap_name"
    done
    cleanup
  done

  echo "Snapshots updated. Please review and commit."
  exit 0
fi

# ============================================================
# Test mode: snapshot fixtures (byte-equal compare)
# ============================================================
section "Snapshot comparison"

missing_snapshots=false
for fixture_name in "${!SNAPSHOT_FIXTURES[@]}"; do
  fixture_toml="${SNAPSHOT_FIXTURES[$fixture_name]}"
  if [[ ! -f "$fixture_toml" ]]; then
    echo "  Missing fixture: $fixture_toml"
    missing_snapshots=true
    continue
  fi
  out_root="$SNAPSHOT_DIR/$fixture_name"
  if [[ ! -d "$out_root" ]]; then
    echo "  Missing snapshot dir: $out_root (run with --update to create)"
    missing_snapshots=true
  fi
done

if [[ "$missing_snapshots" == true ]]; then
  echo "Run: tests/test_snapshot.sh --update"
  exit 1
fi

for fixture_name in "${!SNAPSHOT_FIXTURES[@]}"; do
  fixture_toml="${SNAPSHOT_FIXTURES[$fixture_name]}"
  out_root="$SNAPSHOT_DIR/$fixture_name"
  generate_for_fixture "$fixture_toml"

  for rel in "${SNAPSHOT_FILES[@]}"; do
    gen_path="$WORK_DIR/$rel"
    snap_name=$(snapshot_filename_for "$rel")
    snap_path="$out_root/$snap_name"

    # Some fixtures legitimately do not emit every artefact (e.g. when
    # devcontainer compose is disabled). Skip the comparison if neither a
    # generated file nor a recorded snapshot exists; otherwise report
    # asymmetric state as a regression.
    if [[ ! -f "$gen_path" && ! -f "$snap_path" ]]; then
      continue
    fi
    if [[ ! -f "$gen_path" ]]; then
      assert_eq "$fixture_name: $rel" "exists" "missing"
      echo "      Generator did not emit $rel but a snapshot exists at $snap_path"
      continue
    fi
    if [[ ! -f "$snap_path" ]]; then
      assert_eq "$fixture_name: $rel" "snapshot recorded" "no snapshot"
      echo "      Generator emitted $rel but no snapshot at $snap_path. Run --update."
      continue
    fi
    if diff -u "$snap_path" "$gen_path" > /dev/null 2>&1; then
      assert_eq "$fixture_name: $rel" "match" "match"
    else
      assert_eq "$fixture_name: $rel" "match" "differs"
      echo "      Diff for $fixture_name/$rel:"
      diff -u "$snap_path" "$gen_path" | head -20 | sed 's/^/      /'
      echo "      ..."
      echo "      Run: tests/test_snapshot.sh --update"
    fi
  done

  cleanup
done

# ============================================================
# Runtime parse: feed the comprehensive ports snapshot through `docker
# compose config` to confirm Docker itself accepts every short / long form
# we emit. Skipped (not failed) when docker is unavailable so unit-only
# environments still pass.
# ============================================================
section "docker compose config (ports-comprehensive)"

ports_compose="$SNAPSHOT_DIR/ports-comprehensive/docker-compose.yml.expected"
if [[ ! -f "$ports_compose" ]]; then
  assert_eq "ports-comprehensive snapshot present" "exists" "missing"
elif ! command -v docker > /dev/null 2>&1; then
  echo "  ⏭️ SKIP: docker not available; skipping compose runtime parse"
else
  parse_dir=$(mktemp -d)
  cp "$ports_compose" "$parse_dir/docker-compose.yml"
  cat > "$parse_dir/.env" <<'ENV'
COMPOSE_PROJECT_NAME=ports-test
CONTAINER_SERVICE_NAME=ports
USERNAME=developer
UID=1000
GID=1000
DOCKER_GID=999
OS_IMAGE=ubuntu
OS_VERSION=24.04
ENV
  if (cd "$parse_dir" && docker compose config --quiet > /dev/null 2>&1); then
    assert_eq "ports-comprehensive: docker compose config" "ok" "ok"
  else
    assert_eq "ports-comprehensive: docker compose config" "ok" "failed"
    (cd "$parse_dir" && docker compose config 2>&1 | head -20 | sed 's/^/      /')
  fi
  rm -rf "$parse_dir"
fi

# ============================================================
# Smoke fixtures: generation-success only (no byte compare)
# ============================================================
section "Generation smoke"

for fixture_name in "${!SMOKE_FIXTURES[@]}"; do
  fixture_toml="${SMOKE_FIXTURES[$fixture_name]}"
  if [[ ! -f "$fixture_toml" ]]; then
    assert_eq "$fixture_name fixture present" "exists" "missing"
    continue
  fi
  if generate_for_fixture "$fixture_toml" > /dev/null 2>&1; then
    assert_eq "$fixture_name generates" "ok" "ok"
  else
    assert_eq "$fixture_name generates" "ok" "failed"
  fi
  cleanup
done

print_summary
