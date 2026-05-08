#!/bin/bash
# ============================================================
# tests/test_helper.sh - Shared test framework
# ============================================================
# Provides assert functions and test tracking used by all test files.
# Source this file at the top of each test script.
# ============================================================

# Test counters
PASS=0
FAIL=0
SKIP=0
ERRORS=()

# Project root (parent of tests/) — exported for use by sourcing test scripts
PROJECT_ROOT="$(cd "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")/.." && pwd)"
export PROJECT_ROOT

# ===== Assert Functions =====

assert_eq() {
  local desc="$1" expected="$2" actual="$3"
  if [[ "$expected" == "$actual" ]]; then
    echo "  ✅ PASS: $desc"
    PASS=$((PASS + 1))
  else
    echo "  ❌ FAIL: $desc"
    echo "      expected: '$expected'"
    echo "      actual:   '$actual'"
    FAIL=$((FAIL + 1))
    ERRORS+=("$desc")
  fi
}

assert_ne() {
  local desc="$1" unexpected="$2" actual="$3"
  if [[ "$unexpected" != "$actual" ]]; then
    echo "  ✅ PASS: $desc"
    PASS=$((PASS + 1))
  else
    echo "  ❌ FAIL: $desc"
    echo "      should NOT be: '$unexpected'"
    FAIL=$((FAIL + 1))
    ERRORS+=("$desc")
  fi
}

assert_true() {
  local desc="$1"
  shift
  if "$@" 2>/dev/null; then
    echo "  ✅ PASS: $desc"
    PASS=$((PASS + 1))
  else
    echo "  ❌ FAIL: $desc"
    FAIL=$((FAIL + 1))
    ERRORS+=("$desc")
  fi
}

assert_false() {
  local desc="$1"
  shift
  if ! "$@" 2>/dev/null; then
    echo "  ✅ PASS: $desc"
    PASS=$((PASS + 1))
  else
    echo "  ❌ FAIL: $desc"
    FAIL=$((FAIL + 1))
    ERRORS+=("$desc")
  fi
}

assert_file_exists() {
  local desc="$1" path="$2"
  if [[ -f "$path" ]]; then
    echo "  ✅ PASS: $desc"
    PASS=$((PASS + 1))
  else
    echo "  ❌ FAIL: $desc (file not found: $path)"
    FAIL=$((FAIL + 1))
    ERRORS+=("$desc")
  fi
}

assert_dir_exists() {
  local desc="$1" path="$2"
  if [[ -d "$path" ]]; then
    echo "  ✅ PASS: $desc"
    PASS=$((PASS + 1))
  else
    echo "  ❌ FAIL: $desc (dir not found: $path)"
    FAIL=$((FAIL + 1))
    ERRORS+=("$desc")
  fi
}

assert_file_contains() {
  local desc="$1" path="$2" pattern="$3"
  if grep -Fq "$pattern" "$path" 2>/dev/null; then
    echo "  ✅ PASS: $desc"
    PASS=$((PASS + 1))
  else
    echo "  ❌ FAIL: $desc (string '$pattern' not found in $path)"
    FAIL=$((FAIL + 1))
    ERRORS+=("$desc")
  fi
}

assert_file_not_contains() {
  local desc="$1" path="$2" pattern="$3"
  if ! grep -Fq "$pattern" "$path" 2>/dev/null; then
    echo "  ✅ PASS: $desc"
    PASS=$((PASS + 1))
  else
    echo "  ❌ FAIL: $desc (string '$pattern' found in $path)"
    FAIL=$((FAIL + 1))
    ERRORS+=("$desc")
  fi
}

assert_file_matches() {
  local desc="$1" path="$2" pattern="$3"
  if grep -Eq "$pattern" "$path" 2>/dev/null; then
    echo "  ✅ PASS: $desc"
    PASS=$((PASS + 1))
  else
    echo "  ❌ FAIL: $desc (regex '$pattern' not matched in $path)"
    FAIL=$((FAIL + 1))
    ERRORS+=("$desc")
  fi
}

assert_file_not_matches() {
  local desc="$1" path="$2" pattern="$3"
  if ! grep -Eq "$pattern" "$path" 2>/dev/null; then
    echo "  ✅ PASS: $desc"
    PASS=$((PASS + 1))
  else
    echo "  ❌ FAIL: $desc (regex '$pattern' matched in $path)"
    FAIL=$((FAIL + 1))
    ERRORS+=("$desc")
  fi
}

assert_exit_code() {
  local desc="$1" expected="$2"
  shift 2
  "$@" >/dev/null 2>&1
  local actual=$?
  if [[ "$expected" -eq "$actual" ]]; then
    echo "  ✅ PASS: $desc"
    PASS=$((PASS + 1))
  else
    echo "  ❌ FAIL: $desc (expected exit $expected, got $actual)"
    FAIL=$((FAIL + 1))
    ERRORS+=("$desc")
  fi
}

skip_test() {
  local desc="$1" reason="$2"
  echo "  ⏭️  SKIP: $desc ($reason)"
  SKIP=$((SKIP + 1))
}

section() {
  echo ""
  echo "━━━ $1 ━━━"
}

# ===== Summary =====

print_summary() {
  local total=$((PASS + FAIL + SKIP))
  echo ""
  echo "━━━ Results ━━━"
  echo ""
  echo "  Total: $total  ✅ PASS: $PASS  ❌ FAIL: $FAIL  ⏭️ SKIP: $SKIP"

  if [[ $FAIL -gt 0 ]]; then
    echo ""
    echo "  Failed tests:"
    local e
    for e in "${ERRORS[@]}"; do
      echo "    - $e"
    done
    echo ""
    return 1
  else
    echo ""
    echo "  All tests passed!"
    echo ""
    return 0
  fi
}
