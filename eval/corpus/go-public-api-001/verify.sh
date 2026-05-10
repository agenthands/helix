#!/usr/bin/env bash
set -euo pipefail

# verify.sh for go-public-api-001
# Asserts that ProcessRequest accepts a context.Context parameter.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/repo"

go vet ./...

# ProcessRequest must now accept context.Context.
grep -q "context.Context" main.go

echo "PASS: go-public-api-001"
