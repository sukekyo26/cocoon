#!/bin/bash
# ============================================================
# tests/run_all.sh - Test runner for all test suites
# ============================================================
# Usage: ./tests/run_all.sh
# ============================================================

set -uo pipefail

TESTS_DIR="$(cd "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")" && pwd)"
PROJECT_ROOT="$(dirname "$TESTS_DIR")"

SUITE_RESULTS=()

echo ""
echo "╔══════════════════════════════════════════╗"
echo "║   cocoon  Test Runner          ║"
echo "╚══════════════════════════════════════════╝"
echo ""
echo "Project: $PROJECT_ROOT"
echo ""

# Find and run all test_*.sh files recursively
mapfile -t test_files < <(find "$TESTS_DIR" -name 'test_*.sh' ! -name 'test_helper.sh' | sort)

if [[ ${#test_files[@]} -eq 0 ]]; then
  echo "No test files found in $TESTS_DIR"
  exit 1
fi

for test_file in "${test_files[@]}"; do
  suite_name="${test_file#"$TESTS_DIR/"}"
  suite_name="${suite_name%.sh}"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "Running: $suite_name"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

  if timeout 60 bash "$test_file"; then
    SUITE_RESULTS+=("✅ $suite_name")
  else
    SUITE_RESULTS+=("❌ $suite_name")
  fi

  echo ""
done

echo "╔══════════════════════════════════════════╗"
echo "║   Suite Summary                          ║"
echo "╚══════════════════════════════════════════╝"
echo ""

has_failure=false
for result in "${SUITE_RESULTS[@]}"; do
  echo "  $result"
  if [[ "$result" == "❌"* ]]; then
    has_failure=true
  fi
done

echo ""
if [[ "$has_failure" == true ]]; then
  echo "Some test suites failed."
  exit 1
else
  echo "All test suites passed!"
  exit 0
fi
