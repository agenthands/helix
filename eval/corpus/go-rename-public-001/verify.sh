#!/usr/bin/env bash
set -euo pipefail

# verify.sh for go-rename-public-001
# Asserts that AuthMiddleware has been renamed to AuthGuard.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/repo"

# The code must compile and vet clean.
go vet ./...

# AuthGuard must be present.
grep -q "AuthGuard" main.go

# AuthMiddleware must be gone.
if grep -q "AuthMiddleware" main.go; then
  echo "FAIL: AuthMiddleware still present in main.go" >&2
  exit 1
fi

echo "PASS: go-rename-public-001"
