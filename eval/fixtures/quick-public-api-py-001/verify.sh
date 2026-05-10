#!/bin/sh
# verify.sh for quick-public-api-py-001
# Validates that compute returns tuple[int, str].
# NOTE: In eval-quick mode, the scripted agent issues the replace_symbol_body MCP call
# but the in-process daemon applies the edit to the sandbox repo. This verify.sh
# checks that the edit succeeded.
set -e
cd "$(dirname "$0")"
python3 -m py_compile main.py
grep -q "tuple\[int, str\]" main.py
