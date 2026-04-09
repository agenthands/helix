---
phase: 08-advanced-testing
plan: 02
subsystem: integration-tests
tags: [testing, profiles, modes, golden-files, security]
requirements: [ADV-01, ADV-02]
dependency_graph:
  requires:
    - "08-01 (Options.Profile/Mode/MaxWorkers, listSessionTools, assertGoldenTools)"
  provides:
    - "ADV-01: 5-profile contract tests against checked-in goldens"
    - "ADV-02: 19-pair (profile, mode) contract tests against checked-in goldens"
    - "Initial session AllowedTools resolution from profile+mode"
    - "cfg.Mode initial mode override wiring"
  affects:
    - "internal/daemon/daemon.go (bootstrap mode/tool filter wiring)"
    - "test/integration/harness.go (switch_mode arg fix, initial mode via cfg)"
tech_stack:
  added: []
  patterns:
    - "Golden-file oracle pattern for access-control contract tests"
    - "Config-level initial mode override bypassing transition rules"
key_files:
  created:
    - test/integration/profile_golden_test.go
    - test/integration/mode_golden_test.go
    - testdata/profiles/*.tools.golden (19 files)
  modified:
    - internal/daemon/daemon.go
    - test/integration/harness.go
decisions:
  - "Initial mode uses cfg.Mode override instead of runtime switch_mode so admin (not reachable via any allowed_mode_transitions entry) is still exercisable"
  - "Initial AllowedTools resolved at daemon bootstrap so profile filtering applies to very first tools/list, not only after first switch_mode"
  - "ci-bot + replace_symbol_body kept as the excluded-tool EoP probe (matches YAML exclude_tools)"
metrics:
  duration: ~15 minutes
  completed: 2026-04-08
  tasks: 2
  files_changed: 5
  commits: 2
---

# Phase 8 Plan 02: Profile & Mode Contract Tests Summary

One-liner: Golden-file contract tests locking the tool surface of every profile and every valid (profile, mode) pair, plus an EoP test that excluded tools error on direct invocation and a freshness test that switch_mode invalidates cached listings.

## What Landed

- **`TestProfile_Contract_Golden`** (ADV-01): one subtest per profile (claude-code, codex, ide-assistant, ci-bot, full), each session started at the profile's `default_mode` and compared against `testdata/profiles/<profile>.<default_mode>.tools.golden`.
- **`TestMode_Contract_Golden`** (ADV-02): 19 subtests covering every valid (profile, mode) pair. ci-bot omits `edit` per its YAML graph; all other profiles get all four modes.
- **`TestMode_SwitchRefreshesToolList`**: runs `full/admin`, calls `switch_mode target_mode=read`, asserts the new tools/list is both different and strictly smaller. Guards against MCP SDK client-side list caching and middleware refilter regressions (Assumption A2, Threat T-08-04).
- **`TestProfile_ExcludedToolNotInvocable`**: starts `ci-bot`, confirms `replace_symbol_body` is absent from tools/list, then invokes it by name and asserts `result.IsError` (Threat T-08-03 — listing filter is not enough, dispatch must also enforce).
- **19 golden files** under `testdata/profiles/`, sorted LF-per-line, ready to diff on any profile/mode YAML drift.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 — Blocker] switch_mode arg name mismatch in harness (08-01 inherited)**
- **Found during:** Task 2 first `-update` run
- **Issue:** Plan 08-01 harness called `switch_mode` with `{"mode": opts.Mode}` but the tool requires `{"target_mode": ...}`, causing every mode test to fail with `missing required parameter: target_mode`.
- **Fix:** Updated `test/integration/harness.go` and `test/integration/mode_golden_test.go` to use `target_mode`.
- **Files modified:** `test/integration/harness.go`, `test/integration/mode_golden_test.go`
- **Commit:** `26012274`

**2. [Rule 2 — Missing critical functionality] Initial session had no AllowedTools**
- **Found during:** Task 2 investigation of mode switching failure
- **Issue:** `daemon.New` initialized `SessionInfo` with only `Profile` and `Mode` but left `AllowedTools` nil. ProfileFilterMiddleware treats nil as "all tools allowed", so profile filtering never fired until a user ran `switch_mode` at least once. This broke ADV-01 entirely (every profile would have shown the same full tool list) and is a real runtime access-control gap.
- **Fix:** Added `resolveAllowedToolsForMode(store, profile, mode)` helper in `internal/daemon/daemon.go` mirroring `profileSkill.ExecuteSwitchMode` resolution, wired into the initial `SessionInfo` construction so the very first `tools/list` is correctly filtered.
- **Files modified:** `internal/daemon/daemon.go`
- **Commit:** `26012274`

**3. [Rule 3 — Blocker] admin mode unreachable via allowed_mode_transitions**
- **Found during:** Task 2 first `-update` run
- **Issue:** Across every profile YAML, `admin` only appears as a FROM key, never as a TO target. That means runtime `switch_mode` can never land on admin, so the harness's strategy of switching modes post-connect can't produce an `admin` session. Without a fix, ADV-02 loses admin coverage for all 5 profiles.
- **Fix:** Wired `cfg.Mode` (the previously-dead config field) as the initial mode override in daemon bootstrap. The harness now sets `cfg.Mode = opts.Mode` and skips the post-connect `switch_mode` call, so any valid seed mode — including admin — is reachable without violating the transition graph. This also eliminates bogus self-transitions (e.g. `edit`→`edit`) for modes matching the profile default.
- **Files modified:** `internal/daemon/daemon.go`, `test/integration/harness.go`
- **Commit:** `26012274`

## Verification

- `go vet ./...` — clean.
- `go test ./internal/daemon/... ./internal/profile/... ./internal/mcp/...` — all pass (no regressions).
- `go test -tags integration ./test/integration/... -run '^TestProfile_Contract_Golden$|^TestMode_Contract_Golden$|^TestMode_SwitchRefreshesToolList$|^TestProfile_ExcludedToolNotInvocable$' -race -count=1` — all 24 subtests pass on a cold run without `-update`.
- 19 golden files present and committed under `testdata/profiles/`; all sorted and LF-terminated.

## Requirements Completed

- **ADV-01**: 5 profiles × default-mode golden tests passing.
- **ADV-02**: 19 (profile, mode) golden tests passing, covering every mode reachable via the initial-mode config path.

## Threat Mitigations Validated

- **T-08-03 (EoP)**: `TestProfile_ExcludedToolNotInvocable` confirms ci-bot cannot invoke `replace_symbol_body` even by direct name.
- **T-08-04 (Info disclosure via stale listing)**: `TestMode_SwitchRefreshesToolList` confirms post-switch_mode tools/list reflects the new, smaller tool set.
- **T-08-05 (Golden tampering)**: `-update` path is explicit, files surface in `git status`, tests error-message points at scoped `-run` update command.

## Self-Check: PASSED

- `test -f test/integration/profile_golden_test.go` → FOUND
- `test -f test/integration/mode_golden_test.go` → FOUND
- `ls testdata/profiles/*.tools.golden | wc -l` → 19 FOUND
- `git log --oneline` contains `96c01daf` (Task 1) and `26012274` (Task 2) → FOUND
- `internal/daemon/daemon.go` contains `resolveAllowedToolsForMode` → FOUND
- `test/integration/harness.go` uses `target_mode` → FOUND
