#!/bin/sh
# verify.sh for quick-rename-py-001
# Validates that Foo has been renamed to Bar in main.py.
# NOTE: In eval-quick mode, the scripted agent issues the rename_symbol MCP call
# but the in-process daemon applies the edit to the sandbox repo. This verify.sh
# checks that the rename succeeded.
set -e
cd "$(dirname "$0")"
grep -q "class Bar" main.py
! grep -q "class Foo" main.py
python3 -m py_compile main.py
