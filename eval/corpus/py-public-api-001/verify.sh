#!/usr/bin/env bash
set -euo pipefail

# verify.sh for py-public-api-001
# Asserts that process_batch now accepts a timeout_sec parameter.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/repo"

python3 -c "import py_compile; py_compile.compile('main.py')"

grep -q "timeout_sec" main.py

echo "PASS: py-public-api-001"
