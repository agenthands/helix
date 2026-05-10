#!/usr/bin/env bash
set -euo pipefail

# verify.sh for go-security-001
# Asserts that RunQuery no longer uses string concatenation for SQL.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/repo"

go vet ./...

# String concatenation pattern must be gone from RunQuery area.
if grep -q '"SELECT.*"\s*+\|fmt.Sprintf.*SELECT' main.go; then
  echo "FAIL: SQL injection pattern still present" >&2
  exit 1
fi

# Parameterised query marker must be present.
grep -q '\$1\|?\|Prepare\|QueryContext\|ExecContext\|placeholder' main.go

echo "PASS: go-security-001"
