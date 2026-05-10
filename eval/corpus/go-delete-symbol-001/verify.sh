#!/usr/bin/env bash
set -euo pipefail

# verify.sh for go-delete-symbol-001
# Asserts that LegacyParser has been removed.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/repo"

go vet ./...

if grep -q "LegacyParser" main.go; then
  echo "FAIL: LegacyParser still present in main.go" >&2
  exit 1
fi

echo "PASS: go-delete-symbol-001"
