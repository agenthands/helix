#!/bin/sh
# verify.sh for IT-go-call-graph-1 (D-10 runner-less fallback).
#
# Runs the fixture module's tests in the cloned working copy and exits with
# `go test`'s exit code (0 = pass, non-zero = fail). Hermetic: go.mod has NO
# external dependencies, so this runs offline.
set -e
cd "$(dirname "$0")"
exec go test ./...
