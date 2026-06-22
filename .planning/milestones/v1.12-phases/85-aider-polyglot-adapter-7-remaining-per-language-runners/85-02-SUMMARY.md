---
phase: 85
plan: 02
subsystem: bench/container
tags: [container, network-isolation, security, argv, tdd]
requires:
  - "bench/container engine.go pullArgs/PullByDigest argv + env-allowlist discipline (Phase 84)"
  - "isValidRepo / isHexSHA256 / procGroupAttr / allowlistEnv (bench/container, Phase 84)"
provides:
  - "bench/container.runArgs — fixed-argv builder for --network=none hermetic container run"
  - "bench/container.(*Engine).Run — os/exec executor mirroring PullByDigest (allowlistEnv + procGroupAttr)"
  - "bench/container.isValidImageRef — digest-pinned-ref validator (repo@sha256:hex)"
  - "bench/container.errBadMount + isValidMountPath — mount fail-close"
affects:
  - "Downstream per-language runners (Phase 85 Plans 03-07) that need a --network=none exec seam"
tech-stack:
  added: []
  patterns:
    - "fixed-argv + -- terminator + strict env allowlist (mirror PullByDigest)"
    - "fail-close validation BEFORE crossing os/exec (mirror pullArgs)"
    - "hermetic argv-assertion test as the authoritative proof; live engine test only skips"
key-files:
  created:
    - bench/container/run.go
    - bench/container/run_test.go
  modified:
    - bench/container/engine.go
decisions:
  - "isValidImageRef added (Rule 3): isValidRepo alone forbids '@', so it cannot validate a digest-pinned <repo>@sha256:<hex> run image; isValidImageRef splits on '@sha256:' and reuses isValidRepo+isHexSHA256, preserving the leading-'-' flag-smuggling rejection."
  - "isValidMountPath uses filepath.ToSlash + Clean-equality + ':' ban so the check is host-OS-independent (windows cross-compile) — container mount specs are always '/'-rooted."
  - "Run takes a runShim test seam on Engine so the argv crossing os/exec is hermetically asserted without a live engine; production callers leave it nil."
  - "Task 1 (runArgs) and Task 2 (Run) both land in run.go (tightly coupled); RED was one commit, GREEN one commit."
metrics:
  duration: ~12min
  completed: 2026-06-21
---

# Phase 85 Plan 02: Container --network=none Run Seam Summary

Added the missing `--network=none` container-run seam to `bench/container/`: a fixed-argv `runArgs` builder and an `(*Engine).Run` executor that mirror the existing `pullArgs`/`PullByDigest` discipline exactly (fail-close validation before os/exec, strict env allowlist, process-group ownership, `--` option terminator, no shell, no docker Go SDK). The `--network=none` flag — the SC#3 network-isolation barrier for untrusted Exercism test code — is hermetically asserted in the exact run argv WITHOUT a live engine; the live `Run` leg only `t.Skip`s when no engine is on PATH and is never the sole proof.

## What Was Built

**Task 1 — `runArgs` pure-argv builder (`bench/container/run.go`):**
- `runArgs(image, cmdArgs, mounts, netNone) ([]string, error)` assembles the fixed argv `["run", "--rm", ("--network=none")?, ("-v" "<src>:<dst>")*, "--", image, cmdArgs...]`.
- `--network=none` is present **iff** `netNone` — the flag presence is exactly the network policy, not always-on.
- Mounts emit in **sorted-by-source** order so the argv is byte-stable for the golden assertion.
- Fail-close BEFORE producing any argv: `isValidImageRef` (image) and `isValidMountPath` (every mount source **and** destination). `errBadRepo` for the image case, new `errBadMount` for mounts. Never returns a partial argv.

**Task 2 — `Run` executor + live-skip (`bench/container/run.go`, `engine.go`):**
- `(*Engine).Run(ctx, image, cmdArgs, mounts, netNone) error` mirrors `PullByDigest` exactly: `runArgs` (error verbatim, fail-close), `exec.CommandContext(ctx, e.bin, args...)`, `cmd.SysProcAttr = procGroupAttr()`, `cmd.Env = allowlistEnv()` (PATH/HOME/HELIX_CACHE_DIR only), wrapped error.
- `runShim` field added to `Engine` as the hermetic test seam (nil in production).

## Tests (TDD RED → GREEN)

- `TestRunArgsNetworkNoneFixedArgv` — authoritative SC#3 proof: exact argv with `--network=none`/`--rm`/`-v /srv/repo:/work`/`--` terminator, no live engine.
- `TestRunArgsNetworkFlagIsExactlyThePolicy` — `--network=none` present iff `netNone`.
- `TestRunArgsDeterministicMountOrder` — multi-mount sorted-by-source emission.
- `TestRunArgsRejectsFlagSmugglingImage` — leading-`-`/tag-shaped/uppercase/empty image → `errBadRepo` before argv.
- `TestRunArgsRejectsBadMount` — `..`, non-absolute, `-`-leading, `:`-bearing, empty mount ends → `errBadMount` before argv.
- `TestRunInvokesRunArgsArgv` — hermetic Run argv-equivalence via recorded `runShim`; bad image fail-closes and never reaches the shim.
- `TestRunLiveSkipsWithoutEngine` — `t.Skip` on `errEngineUnavailable` (this env), never the sole proof.

## Verification

- `go test ./bench/container/ -count=1` — green.
- `go vet ./bench/container/` — clean.
- `go build ./...` — clean.
- `GOOS=windows GOARCH=amd64 go build ./bench/container/...` — clean.
- `make verify-no-docker-sdk` — green (no Engine SDK; the indirect `docker-credential-helpers` registry-auth dep is not `github.com/docker/docker`).
- `make vet` (full gate incl. verify-no-docker-sdk + 6 leakage vettools) — exit 0.
- TDD gates present: `test(85-02)` (90982676) before `feat(85-02)` (aaff7a20).

## Threat Mitigations Applied

| Threat ID | Mitigation |
|-----------|------------|
| T-85-02-01 | `isValidImageRef` (rejects leading `-`) + `isValidMountPath` (rejects `..`/`:`/non-absolute) + `--` terminator — fail-close before os/exec |
| T-85-02-02 | `--network=none` asserted present in the run argv golden (`TestRunArgsNetworkNoneFixedArgv`) |
| T-85-02-03 | `allowlistEnv()` reused verbatim — only PATH/HOME/HELIX_CACHE_DIR, never `os.Environ()` |
| T-85-02-04 | fixed argv via `exec.CommandContext`, never a shell |
| T-85-02-SC | os/exec only; `make verify-no-docker-sdk` green |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `isValidImageRef` added for digest-pinned run images**
- **Found during:** Task 1 (GREEN)
- **Issue:** The plan said to validate the image with the existing `isValidRepo`, but `isValidRepo`'s allowed alphabet excludes `@`, so it rejects a digest-pinned run image `<repo>@sha256:<hex>` (the standard run-target form). All argv tests failed with `errBadRepo`.
- **Fix:** Added `isValidImageRef` which splits on `@sha256:` and reuses `isValidRepo` (repo half) + `isHexSHA256` (digest half); a bare repo with no pin still validates via `isValidRepo`. This preserves the leading-`-` flag-smuggling rejection while spanning the full pinned-image alphabet `isValidRepo` alone cannot express.
- **Files modified:** bench/container/run.go
- **Commit:** aaff7a20

## TDD Gate Compliance

RED commit (90982676) precedes GREEN commit (aaff7a20). No test passed unexpectedly during RED (the test file did not compile against the absent `runArgs`/`Run`/`errBadMount`/`runShim`, the canonical RED state). REFACTOR gate not needed.

## Known Stubs

None. The toolchain-IMAGE build (offline dep pre-resolution) is the larger toolchain-host sub-effort and is recorded as **toolchain-gated** in 85-VALIDATION.md, not a stub — this plan delivers the hermetically-verifiable run seam + `--network=none` argv proof, which is the complete contract for this plan.

## Self-Check: PASSED

- FOUND: bench/container/run.go
- FOUND: bench/container/run_test.go
- FOUND: .planning/phases/85-aider-polyglot-adapter-7-remaining-per-language-runners/85-02-SUMMARY.md
- FOUND: 90982676 (test gate)
- FOUND: aaff7a20 (feat gate)
