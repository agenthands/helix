#!/bin/sh
# verify.sh for quick-delete-ts-001
# Validates that legacyHelper has been removed from index.ts.
# NOTE: In eval-quick mode, the scripted agent issues the safe_delete_symbol MCP call
# but the in-process daemon applies the edit to the sandbox repo. This verify.sh
# checks that the deletion succeeded.
# tsc check is skipped gracefully if tsc is not on PATH.
set -e
cd "$(dirname "$0")"
! grep -q "function legacyHelper" index.ts
if command -v tsc >/dev/null 2>&1; then
  tsc --noEmit
fi
