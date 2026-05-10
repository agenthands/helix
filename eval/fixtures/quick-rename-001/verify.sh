#!/bin/sh
# verify.sh for quick-rename-001
# Validates that Foo has been renamed to Bar in main.go.
# NOTE: In eval-quick mode, the scripted agent issues the rename_symbol MCP call
# but the in-process daemon applies the edit to the sandbox repo. This verify.sh
# checks that the rename succeeded.
set -e
cd "$(dirname "$0")"
grep -q "type Bar struct" main.go
! grep -q "type Foo struct" main.go
go vet ./...
