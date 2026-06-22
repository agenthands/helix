---
phase: 96-address-v2-0-tech-debt
plan: 01
subsystem: infra
tags: [tech-debt, go, daemon, cli, docgen, security-hardening, classifier, dead-code]

# Dependency graph
requires:
  - phase: 95-readme-langsupport-lineage
    provides: v2.0 milestone audit surfacing the four non-blocking tech-debt items (TD-01..TD-04)
provides:
  - Hardened validateAdminAddr (empty-host/wildcard bind refusal + !ip.IsUnspecified guard)
  - mergeJSONConfig dead-code removal (helper + four tests)
  - classifyBashTarget grep-family leading-pattern skip
  - CLI-first get_tool_help Descriptions + regenerated README row
affects: [v2.0-milestone-archive, daemon-admin-listener, nudge-hook, docgen]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Loopback-bind validators refuse empty-host wildcards via an explicit '' switch case + !ip.IsUnspecified() guard (mirrors validateGRPCAddr CR-01)"
    - "Generated artifacts (README, golden files) are regenerated through their tooling (make docs / -update flag), never hand-edited"

key-files:
  created: []
  modified:
    - internal/daemon/telemetry.go
    - internal/daemon/telemetry_test.go
    - internal/cli/setup_clients.go
    - internal/cli/setup_test.go
    - internal/cli/nudge.go
    - internal/cli/nudge_test.go
    - internal/kernel/help/skill_adapter.go
    - internal/kernel/help/tools.go
    - README.md
    - test/bench/testdata/tool_descriptions.golden

key-decisions:
  - "TD-01: refuse empty-HOST (':9090') via an explicit switch case + !ip.IsUnspecified() guard; preserved the empty-ADDR ('') contract (no addr=='' short-circuit) so net.SplitHostPort still errors on '' and the existing test row stays green"
  - "TD-03: gate the leading-pattern skip to the grep family (grep/rg/ag/egrep/fgrep) only; cat/sed/find lead with a file/expr and were left unchanged"
  - "TD-04: reworded the four 'MCP tool' Description/BriefDescription metadata literals to 'Helix tool'; left the help-body text at tools.go:85 and code comments untouched (out of scope)"

patterns-established:
  - "Pattern 1: admin/grpc loopback validators share the same empty-host-refusal + IsUnspecified-guard shape"
  - "Pattern 2: a source-side string change that feeds a golden snapshot requires regenerating the golden via its documented -update flag in the same plan"

requirements-completed: [TD-01, TD-02, TD-03, TD-04]

# Metrics
duration: 6min
completed: 2026-06-22
status: complete
---

# Phase 96 Plan 01: Address v2.0 Tech Debt Summary

**Closed the four non-blocking v2.0 audit items: hardened the admin listener against wildcard binds, removed the dead mergeJSONConfig helper, stopped the Bash classifier from treating a grep PATTERN as a file, and reworded get_tool_help docs CLI-first with a regenerated README.**

## Performance

- **Duration:** 6 min
- **Started:** 2026-06-22T09:08:52Z
- **Completed:** 2026-06-22T09:15:00Z
- **Tasks:** 4
- **Files modified:** 10

## Accomplishments
- **TD-01 (security hardening):** `validateAdminAddr` now refuses empty-host/wildcard binds (`:9090`, `:0`, `[::]:9090`) and gains the `!ip.IsUnspecified()` defense-in-depth guard, mirroring `validateGRPCAddr`'s CR-01 fix — while preserving the empty-ADDR (`""`) contract handled upstream in `listenAdmin`.
- **TD-02 (dead-code removal):** removed the orphaned `mergeJSONConfig` helper and its four tests; the live sibling `removeFromJSONConfig` (teardown path) and the rest of `setup_clients.go` are untouched.
- **TD-03 (classifier edge case):** `classifyBashTarget` now skips the leading search PATTERN for the grep family only, so `grep foo.go` no longer mis-classifies `foo.go` as a file operand; fail-open / exit-0 / DATA-only tokenization preserved.
- **TD-04 (generated-doc correction):** reworded the four off-message "MCP tool" `get_tool_help` Description/BriefDescription literals to "Helix tool" and regenerated `README.md` through the docgen drift gate (single-row diff, no hand-edit).

## Task Commits

Each task was committed atomically (TDD tasks RED+GREEN folded into one commit each):

1. **Task 1: TD-01 harden validateAdminAddr** - `ae1f75bb` (fix)
2. **Task 2: TD-02 remove dead mergeJSONConfig** - `48745144` (refactor)
3. **Task 3: TD-03 grep-family leading-pattern skip** - `79e07bca` (fix)
4. **Task 4: TD-04 reword get_tool_help Descriptions + regen README** - `277f1533` (docs)
5. **Task 4 follow-on: regenerate tool-descriptions golden** - `6d8bbfca` (test, Rule 1 deviation)

## Files Created/Modified
- `internal/daemon/telemetry.go` - Hardened `validateAdminAddr` (empty-host refusal + `!ip.IsUnspecified()`)
- `internal/daemon/telemetry_test.go` - +3 `TestValidateAdminAddr` rows (wildcard_empty_host/_zero/_v6)
- `internal/cli/setup_clients.go` - Removed dead `mergeJSONConfig`; `removeFromJSONConfig` retained
- `internal/cli/setup_test.go` - Removed the four `mergeJSONConfig` tests + section header
- `internal/cli/nudge.go` - `classifyBashTarget` grep-family leading-pattern skip
- `internal/cli/nudge_test.go` - New `TestClassifyBashTarget_GrepPatternNotFile`
- `internal/kernel/help/skill_adapter.go` - CLI-first Description + BriefDescription (README-driving)
- `internal/kernel/help/tools.go` - CLI-first Description at the two registration sites (lines 27, 81)
- `README.md` - Regenerated `get-tool-help` row (docgen, not hand-edited)
- `test/bench/testdata/tool_descriptions.golden` - Regenerated for the reworded BriefDescription

## Decisions Made
- **TD-01:** Closed the empty-HOST gap with an explicit `""` refusal case rather than an `addr == ""` early-return — the empty-ADDR no-op is the caller's job (`listenAdmin` telemetry.go:42), and the validator must keep erroring on `""` so `{empty → wantErr:true}` stays green. Kept the admin error's `v1.3` auth-roadmap reference (admin path wording, not the gRPC `REMOTE-01` wording).
- **TD-03:** Implemented minimally — for the grep family the first non-flag token is the pattern and is skipped once; no attempt to consume `-e`/`-f` flag values (the existing doc comment already deems that acceptable). `cat`/`sed`/`find` left on prior behavior.
- **TD-04:** Reworded only the four user/agent-visible metadata literals; the help-body text at `tools.go:85` ("any registered MCP tool") and code comments are intentionally out of scope. Did not touch the docgen blank-import list or `verbs_gen.go`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Regenerated tool-descriptions golden file after TD-04 reword**
- **Found during:** Task 4 (TD-04), surfaced by the phase-gate `go test ./...`
- **Issue:** `TestToolDescriptionsGoldenFile` (`test/bench`) is a golden snapshot of every tool's BriefDescription; the TD-04 `get_tool_help` BriefDescription reword ("an MCP tool" → "a Helix tool") made the committed golden stale, failing the test.
- **Fix:** Regenerated the golden via its documented `-update` flag (`go test ./test/bench/... -run TestToolDescriptionsGoldenFile -update`) — an intentional description change, exactly the flag's purpose. Single-line golden diff.
- **Files modified:** `test/bench/testdata/tool_descriptions.golden`
- **Verification:** `go test ./test/bench/` green; full `go test ./...` green.
- **Committed in:** `6d8bbfca`

---

**Total deviations:** 1 auto-fixed (1 bug — stale generated artifact)
**Impact on plan:** The golden regeneration was the correct, documented response to an intentional source-side description change. No scope creep; no behavior change beyond the four planned literals.

## Issues Encountered
- None beyond the golden-file regeneration documented above. The `[::]:9090` TD-01 case already errored as non-loopback before the fix; the empty-host cases (`:9090`, `:0`) were the real RED failures, confirming the gap the fix closes.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All four v2.0 audit tech-debt items (TD-01..TD-04) are closed; the v2.0 milestone can archive with zero open items.
- Phase gate fully green: `go build ./...`, `go vet ./...`, `go test ./...`, `make verify-docs` all pass; `api/proto/` zero diff; `internal/cli/verbs_gen.go` unchanged (frozen 50-verb surface intact).
- The documented env-gated `cmd/helix-bench` `TestRunSubcommandWires*` tests self-skip without `HELIX_BIN` (out of scope, not a regression).

## Self-Check: PASSED

---
*Phase: 96-address-v2-0-tech-debt*
*Completed: 2026-06-22*
