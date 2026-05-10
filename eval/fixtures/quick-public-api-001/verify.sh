#!/bin/sh
# verify.sh for quick-public-api-001
# Validates that Greeter.Hello returns (string, error).
# NOTE: In eval-quick mode, the scripted agent issues the replace_symbol_body MCP call
# but the in-process daemon applies the edit to the sandbox repo. This verify.sh
# checks that the edit succeeded.
set -e
cd "$(dirname "$0")"
grep -q "Hello() (string, error)" main.go
go vet ./...
go build ./...
