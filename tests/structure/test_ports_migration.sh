#!/bin/bash
# ============================================================
# tests/structure/test_ports_migration.sh
# Verifies that the legacy `forward = [3000]` int-array form is rejected
# with a migration-friendly error, and that the new short / long forms
# pass validation. Acts as a guard against accidental backwards-compat
# shims being reintroduced.
# ============================================================

set -uo pipefail

TESTS_DIR="$(cd "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")"/.. && pwd)"
# shellcheck source=../test_helper.sh
source "$TESTS_DIR/test_helper.sh"

WSD="$PROJECT_ROOT/bin/wsd-dispatch.sh"

echo ""
echo "[ test_ports_migration.sh ]"

if [[ ! -x "$WSD" ]]; then
  echo "  ⏭️ SKIP: $WSD not executable"
  exit 0
fi

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

write_workspace() {
  cat > "$tmpdir/workspace.toml" <<EOF
[container]
service_name = "dev"
username = "dev"
os = "ubuntu"
os_version = "24.04"

[plugins]
enable = []

[ports]
forward = $1

[apt]
packages = []
EOF
}

run_validate() {
  "$WSD" config validate-workspace "$tmpdir/workspace.toml" 2>&1
}

section "Legacy int form rejected"

write_workspace "[3000]"
output=$(run_validate)
exit_code=$?

if [[ "$exit_code" -eq 0 ]]; then
  assert_eq "rejects forward = [3000]" "non-zero exit" "exit 0"
  echo "      output: $output"
else
  assert_eq "rejects forward = [3000]" "non-zero exit" "non-zero exit"
fi

if [[ "$output" == *"int form was removed"* ]]; then
  assert_eq "error mentions migration" "present" "present"
else
  assert_eq "error mentions migration" "present" "absent"
  echo "      output: $output"
fi

section "New short form accepted"

write_workspace '["3000:3000", "127.0.0.1:5432:5432/tcp"]'
if run_validate > /dev/null 2>&1; then
  assert_eq "accepts short-form strings" "ok" "ok"
else
  assert_eq "accepts short-form strings" "ok" "failed"
  run_validate | head -10 | sed 's/^/      /'
fi

section "New long form accepted"

write_workspace '[{ target = 5432, published = 5432, host_ip = "127.0.0.1", protocol = "tcp" }]'
if run_validate > /dev/null 2>&1; then
  assert_eq "accepts long-form table" "ok" "ok"
else
  assert_eq "accepts long-form table" "ok" "failed"
  run_validate | head -10 | sed 's/^/      /'
fi

section "Invalid long-form key rejected"

write_workspace '[{ target = 3000, foo = "bar" }]'
output=$(run_validate)
exit_code=$?
if [[ "$exit_code" -ne 0 && "$output" == *"unknown key"* ]]; then
  assert_eq "rejects unknown long-form key" "ok" "ok"
else
  assert_eq "rejects unknown long-form key" "ok" "failed"
  echo "      output: $output"
fi

print_summary
