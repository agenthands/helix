#!/bin/sh
# verify.sh for quick-rename-ts-001
# Validates that Foo has been renamed to Bar in index.ts.
# NOTE: In eval-quick mode, the scripted agent issues the rename_symbol MCP call
# but the in-process daemon applies the edit to the sandbox repo. This verify.sh
# checks that the rename succeeded.
# tsc check is skipped gracefully if tsc is not on PATH.
set -e
cd "$(dirname "$0")"
grep -q "class Bar" index.ts
! grep -q "class Foo" index.ts
if command -v tsc >/dev/null 2>&1; then
  tsc --noEmit
fi
