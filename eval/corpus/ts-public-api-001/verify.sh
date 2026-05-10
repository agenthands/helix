#!/usr/bin/env bash
set -euo pipefail

# verify.sh for ts-public-api-001
# Asserts that createSession now returns a Promise.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/repo"

# createSession must now return a Promise<Session>.
grep -q "Promise<Session>\|async.*createSession" index.ts

echo "PASS: ts-public-api-001"
