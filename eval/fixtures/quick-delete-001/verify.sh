#!/bin/sh
# verify.sh for quick-delete-001
# Validates that unusedHelper has been removed from main.go.
# NOTE: In eval-quick mode, the scripted agent issues the safe_delete_symbol MCP call
# but the in-process daemon applies the edit to the sandbox repo. This verify.sh
# checks that the deletion succeeded.
set -e
cd "$(dirname "$0")"
! grep -q "func unusedHelper" main.go
go vet ./...
