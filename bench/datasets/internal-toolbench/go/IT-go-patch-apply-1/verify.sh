#!/bin/sh
# verify.sh for toolbench-go/sum-doubler (D-03).
#
# Runs the seed module's tests in the cloned working copy and exits with
# `go test`'s exit code so the bench runner can feed it into
# trace.MergeInput.VerifyExitCode (D-04): 0 = pass, non-zero = fail.
#
# Hermetic by construction: go.mod has NO external dependencies, so this runs
# offline with no network access (Assumption A4).
set -e
cd "$(dirname "$0")"
exec go test ./...
