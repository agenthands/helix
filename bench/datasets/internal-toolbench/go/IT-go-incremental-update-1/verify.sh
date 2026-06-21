#!/bin/sh
# verify.sh for internal-toolbench/go/IT-go-incremental-update-1 (incremental_update).
#
# Runs the fixture module's tests in the cloned working copy and exits with
# `go test`'s exit code so the bench runner (or the GoRunner via RunTests) can
# feed it into the cell outcome: 0 = pass, non-zero = fail.
#
# Hermetic by construction: go.mod has NO external dependencies, so this runs
# offline with no network access. The refresh_semantic_graph overlay-drain is
# in-process (Phase 70), so no network is involved end-to-end.
set -e
cd "$(dirname "$0")"
exec go test ./...
