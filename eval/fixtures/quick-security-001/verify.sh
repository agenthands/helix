#!/bin/sh
# verify.sh for quick-security-001
# Validates that handleRequest no longer uses shell interpolation.
# NOTE: In eval-quick mode, the scripted agent issues the replace_symbol_body MCP call
# but the in-process daemon applies the edit to the sandbox repo. This verify.sh
# checks that the rewrite succeeded.
set -e
cd "$(dirname "$0")"
! grep -q '"sh", "-c"' main.go
go vet ./...
go build ./...
