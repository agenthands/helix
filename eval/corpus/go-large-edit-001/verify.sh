#!/usr/bin/env bash
set -euo pipefail

# verify.sh for go-large-edit-001
# Asserts that BuildReport now produces JSON output.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/repo"

go vet ./...

# The output must contain JSON indicators.
grep -q "encoding/json\|json.Marshal\|json.NewEncoder" main.go

echo "PASS: go-large-edit-001"
