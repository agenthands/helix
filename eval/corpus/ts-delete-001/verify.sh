#!/usr/bin/env bash
set -euo pipefail

# verify.sh for ts-delete-001
# Asserts that legacyFormat has been removed.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/repo"

if grep -q "legacyFormat" index.ts; then
  echo "FAIL: legacyFormat still present in index.ts" >&2
  exit 1
fi

echo "PASS: ts-delete-001"
