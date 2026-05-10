#!/usr/bin/env bash
set -euo pipefail

# verify.sh for ts-rename-001
# Asserts that fetchUserData has been renamed to loadUserProfile.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/repo"

# loadUserProfile must be present.
grep -q "loadUserProfile" index.ts

# fetchUserData must be gone.
if grep -q "fetchUserData" index.ts; then
  echo "FAIL: fetchUserData still present in index.ts" >&2
  exit 1
fi

echo "PASS: ts-rename-001"
