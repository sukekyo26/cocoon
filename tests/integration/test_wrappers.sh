#!/bin/bash
# ============================================================
# tests/integration/test_wrappers.sh
# Verifies that the five host entry shell scripts are thin wrappers around
# bin/wsd-dispatch.sh. All behavioural coverage lives in Go tests; this file
# only checks the wrappers' shape, syntax, and dispatch contract.
# ============================================================

set -uo pipefail

TESTS_DIR="$(cd "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")"/.. && pwd)"
# shellcheck source=../test_helper.sh
source "$TESTS_DIR/test_helper.sh"

echo ""
echo "[ test_wrappers.sh ]"

# script -> expected dispatch subcommand fragment(s)
declare -A WRAPPERS=(
  [clean-volumes.sh]="clean volumes"
  [clean-docker.sh]="clean docker"
  [generate-workspace.sh]="workspace"
  [rebuild-container.sh]="rebuild"
  [setup-docker.sh]="setup"
)

test_wrapper_shape() {
  section "Wrapper shape"

  local script
  for script in "${!WRAPPERS[@]}"; do
    local path="$PROJECT_ROOT/$script"
    assert_file_exists "$script exists" "$path"
    assert_true "$script is executable" test -x "$path"
    assert_true "$script bash syntax valid" bash -n "$path"

    # Each wrapper must invoke bin/wsd-dispatch.sh with its subcommand.
    assert_file_contains "$script delegates to wsd-dispatch.sh" "$path" 'wsd-dispatch.sh'
    assert_file_contains "$script invokes '${WRAPPERS[$script]}'" "$path" "${WRAPPERS[$script]}"

    # Each wrapper must forward --lang to WORKSPACE_LANG for Go-side i18n.
    assert_file_contains "$script forwards --lang" "$path" 'WORKSPACE_LANG'
  done
}

# When run inside a container the host-side scripts that mutate Docker state
# (clean-volumes, clean-docker, rebuild-container) must refuse to run. The
# guard lives in the Go binary; we exercise the dispatch path here.
test_container_guard() {
  section "Container execution guard"

  if [[ ! -f /.dockerenv ]] && ! grep -qsE 'docker|containerd' /proc/1/cgroup 2>/dev/null; then
    skip_test "container guard" "not running inside a container"
    return
  fi

  local script
  for script in clean-volumes.sh clean-docker.sh rebuild-container.sh; do
    local output
    output=$(WORKSPACE_LANG=en bash "$PROJECT_ROOT/$script" 2>&1 || true)
    assert_file_contains "$script blocks in-container execution" \
      <(echo "$output") 'cannot be run from inside a container'
  done
}

test_wrapper_shape
test_container_guard

print_summary
