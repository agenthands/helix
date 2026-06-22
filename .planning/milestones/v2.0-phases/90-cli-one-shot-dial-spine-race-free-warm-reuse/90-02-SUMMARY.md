---
phase: 90-cli-one-shot-dial-spine-race-free-warm-reuse
plan: 02
subsystem: cli
tags: [cli, cobra, help, forwarder, routing]
requires:
  - "internal/cli/root.go runRoot dispatch (pre-existing)"
  - "cobra v1.10.2 AddGroup/Group API"
provides:
  - "no-arg `helix` → grouped help + exit 0 (CLI-04), no stdio MCP session"
  - "cobra command groups (workspace/runtime/maintenance) as a scaffold for Phase 91 verb groups"
  - "overridable runForwarderFn/runDaemonFn routing seams for hermetic CLI tests"
affects:
  - "Phase 91 (registers generated verb commands into these stable group IDs)"
  - "Phase 94 (full forwarder-head deletion; explicit --mode=stdio path intentionally preserved here)"
tech-stack:
  added: []
  patterns:
    - "package-level function seams (runForwarderFn/runDaemonFn) for routing assertions without daemon spawn"
    - "centralized GroupID assignment at AddCommand time via an addGrouped closure"
key-files:
  created:
    - "internal/cli/root_test.go"
  modified:
    - "internal/cli/root.go"
decisions:
  - "Distinguish explicit --mode=stdio (→ forwarder, preserved for Phase 94) from default --mode=auto (→ grouped help + exit 0). The old switch collapsed both into runForwarder."
  - "Three capability groups (workspace/runtime/maintenance) chosen to leave room for Phase 91's generated verb groups (navigation/edit/fileops/diagnostics/repomap/memory) without renaming."
  - "Routed --version output through cmd.OutOrStdout() (was fmt.Printf to os.Stdout) so it honors cobra's writer and is testable."
metrics:
  duration: 3min
  completed: 2026-06-21
---

# Phase 90 Plan 02: No-Arg Helix → Grouped Help (CLI-04) Summary

Bare `helix` now prints grouped command help and exits 0 instead of silently opening a stdio MCP session; cobra command groups scaffold the help screen for Phase 91's generated verbs, with explicit `--serve`/`--mode=http`/`--mode=stdio`/`--version` routings preserved.

## What Was Built

**Task 1 (TDD): No-arg helix → grouped help, exit 0, no stdio session (CLI-04)**

- `runRoot` (`internal/cli/root.go`) split the former `case "stdio", "auto"` branch:
  - `--mode=stdio` (explicit) → still calls the forwarder (Phase 94 owns the head deletion).
  - `--mode=auto` (default, bare `helix`) → returns `cmd.Help()` and exits 0, opening **no** stdio MCP session.
  - `--serve` / `--mode=http` → daemon (unchanged); unknown mode → error (unchanged).
- Added cobra command groups via `AddGroup`: `workspace` (activate/deactivate/nudge), `runtime` (setup/status), `maintenance` (update/upgrade). All seven visible subcommands are assigned to a group through a centralized `addGrouped` closure at registration.
- Introduced overridable `runForwarderFn`/`runDaemonFn` package-level seams so `root_test.go` can assert which entry point each invocation reaches without spawning a daemon or opening an MCP session.
- `--version` now writes through `cmd.OutOrStdout()` for testability.

## Verification

- `go vet ./internal/cli/...` clean; `go vet ./...` clean.
- `go test ./internal/cli/ -run Root -count=1` green (6 tests: no-arg help/exit-0/no-forwarder, serve→daemon, http→daemon, stdio→forwarder, version, unknown-mode error, plus group assignment test).
- `go test ./internal/cli/...` green; full `go test ./...` green.
- `go build ./...` succeeds.
- Acceptance greps: `cmd.Help()` = 1, `AddGroup` = 3, `func runForwarder` = 1 (head NOT deleted).
- `git diff --exit-code api/proto/` empty (zero-proto invariant honored).

## TDD Gate Compliance

- RED commit `f0485033` (`test(90-02): ...`) — test failed to compile (seams undefined) before implementation.
- GREEN commit `bd390e23` (`feat(90-02): ...`) — all routing + group tests pass.
- No REFACTOR commit needed; implementation was clean as written.

## Threat Model

- **T-90-04 (EoP — no-arg silently opening stdio MCP session): mitigated.** The no-arg path now returns help and exits 0; `TestRunRoot_NoArgPrintsGroupedHelpExitsZero` asserts the forwarder is not entered.
- **T-90-05 (info disclosure — accidental network surface): accept (per plan).** No-arg path opens neither stdio nor TCP; explicit `--mode=http` is unchanged and removed entirely in Phase 94.

## Deviations from Plan

**1. [Rule 1 - Bug] `--version` output not captured by cobra writer**
- **Found during:** Task 1 GREEN (Test 5 failed: version line absent from captured output).
- **Issue:** `runRoot` printed the version via `fmt.Printf` directly to `os.Stdout`, bypassing cobra's configured output writer — untestable and inconsistent with cobra conventions.
- **Fix:** Changed to `fmt.Fprintf(cmd.OutOrStdout(), ...)`. Default behavior (writing to stdout) is unchanged for end users.
- **Files modified:** internal/cli/root.go
- **Commit:** bd390e23

## Self-Check: PASSED

- FOUND: internal/cli/root.go
- FOUND: internal/cli/root_test.go
- FOUND commit: f0485033
- FOUND commit: bd390e23
