#!/bin/sh
# verify.sh for quick-large-edit-001
# Validates that Process was rewritten to a direct computation.
# NOTE: In eval-quick mode, the scripted agent issues the replace_symbol_body MCP call
# but the in-process daemon applies the edit to the sandbox repo. This verify.sh
# checks that the rewrite succeeded.
set -e
cd "$(dirname "$0")"
grep -q "return x \* 2" main.go
go vet ./...
go build ./...
