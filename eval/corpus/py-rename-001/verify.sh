#!/usr/bin/env bash
set -euo pipefail

# verify.sh for py-rename-001
# Asserts that validate_token has been renamed to check_auth_token.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/repo"

python3 -c "import py_compile; py_compile.compile('main.py')"

grep -q "check_auth_token" main.py

if grep -q "validate_token" main.py; then
  echo "FAIL: validate_token still present in main.py" >&2
  exit 1
fi

echo "PASS: py-rename-001"
