---
phase: 100-polyglot-edit-benchmark-committed-baseline
verified: 2026-06-23T00:00:00Z
status: passed
score: 4/4 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: none
  previous_score: none
---

# Phase 100: Polyglot Edit Benchmark + Committed Baseline Verification Report

**Phase Goal:** The model's edit is routed through helix EDIT verbs against the warm daemon via the reused verb-agnostic `RunExercise` loader, surfaced as a new filesystem-table bench mode with an additive `edit_format_applied` result key, and a byte-reproducible committed polyglot-edit baseline is captured `HELIX_BIN`-gated, fail-not-skip.
**Verified:** 2026-06-23
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
| --- | ----- | ------ | -------- |
| 1 | EDIT-verb `AgentFn` in `bench/runtime` (daemon-dialing) routes edits through `replace_in_file`, plugged into the existing `RunExercise` seam, loader untouched, WR-01 pristine-test restore preserved (EDITBENCH-01) | ✓ VERIFIED | `bench/runtime/aider_edit_agent.go:85-140` dials `forwarder.OpenSession` in-process (no helix shelling) + `replace_in_file` with real keys path/pattern/replacement/is_regex. `loader.go` diff (bedacf92..fff4dbf8) is ONLY the two exported wrappers; `RunExercise`+`restorePristineTests` intact (loader.go:249,263). `go list -deps ./bench/datasets/aider-polyglot/` returns 0 forbidden imports. Live `TestAiderEditCellLiveRan` RAN+PASS under HELIX_BIN. |
| 2 | `bench/runners/aider_edit/MODE.md` adds the mode via filesystem-table with zero mode-resolver Go change; additive `edit_format_applied` (`*bool`, omitempty) open key on result.v2 with no v3 bump (EDITBENCH-02 + EDITBENCH-03) | ✓ VERIFIED | MODE.md has strict two-key frontmatter `mode: aider_edit`/`profile: bench-full`. `TestResolveAiderEdit` passes; `mode_resolver.go` UNCHANGED in phase diff. `result.go:152,249,305` carries `EditFormatApplied *bool` json `edit_format_applied,omitempty`; `resultSchemaVersion = "v2"` (result.go:24); all 7 `v3` matches are "no v3 bump" doc disclaimers. `TestEditFormatApplied` 4/4 subtests pass (false-preserved, true-marshals, nil-drops, schema-v2-valid). |
| 3 | Committed polyglot-edit baseline captured HELIX_BIN-gated, fail-not-skip; byte-reproducible deterministic metrics only — no live latency/tokens (BASELINE-01) | ✓ VERIFIED | `bench/reports/aider-edit-baseline/{result.v2.json,BENCH-RESULTS.md}` git-tracked (`git ls-files`) despite `bench/reports/*` gitignore; negated allowlist `.gitignore:276 !/bench/reports/aider-edit-baseline/**` proven via `git check-ignore -v --no-index`. **`make bench-aider-edit` independently regenerated → `git diff bench/reports/aider-edit-baseline/` EMPTY (byte-reproducible, exit 0).** result.v2.json: `grep -cE 'duration_ms\|latency\|started_at\|/tmp/\|/home/'` = 0; patch_validator metrics NULL with metric_errors annotations; `edit_format_applied: true`. |
| 4 | Anti-vacuity: hermetic golden sibling (no binary/network) is sole authoritative proof; "did it RUN" sentinel proves live leg ran under HELIX_BIN; AgentFn respects vet-ablation-leakage leaf boundary | ✓ VERIFIED | `TestAiderEditCellHermetic` runs full path in-process (no HELIX_BIN/network), PASS. `TestAiderEditCellLiveRan` SKIPs when HELIX_BIN unset, RUNs+PASS when set — non-vacuous: reads `res.ResultPath` with fail-closed `require.NoError`, asserts non-empty + schema-valid + `present` signal on edit_format_applied. `TestAiderEditCellAntiTamper` + `TestAiderEditBaselineAntiVacuity` PASS (wrong body → non-success). `bench/runtime` imports forwarder; leaf does NOT (`go list -deps` clean). |

**Score:** 4/4 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `bench/runtime/aider_edit_agent.go` | daemon-dialing AgentFn + native TestFn + aiderEditMode const | ✓ VERIFIED | 158 lines; OpenSession + activate_project + replace_in_file; `const aiderEditMode = "aider_edit"` |
| `bench/runtime/aider_edit_cell.go` | runAiderEditCell mode-name branch driving RunExercise verbatim | ✓ VERIFIED | 295 lines; StartDaemon → RunExercise verbatim → assembleAiderEditResult → Validate → writeDurable |
| `bench/runtime/result.go` | EditFormatApplied *bool wired through BuildResult | ✓ VERIFIED | result.go:152,249,305; schema stays v2 |
| `bench/runners/aider_edit/MODE.md` | two-key filesystem-table mode entry | ✓ VERIFIED | 35 lines; strict `mode:`/`profile:` frontmatter |
| `bench/datasets/aider-polyglot/loader.go` | exported LoadExercise/NativeTestCommand only | ✓ VERIFIED | diff = 2 wrappers; RunExercise/WR-01 untouched; leaf clean |
| `bench/reports/aider-edit-baseline/result.v2.json` | committed byte-reproducible row, deterministic only | ✓ VERIFIED | git-tracked; edit_format_applied:true; latency/tokens null |
| `bench/reports/aider-edit-baseline/BENCH-RESULTS.md` | deterministic baseline summary | ✓ VERIFIED | git-tracked; no latency/timestamp/path |
| `bench/aggregator/aider_edit_baseline.go` | RenderAiderEditBaseline sort-before-emit | ✓ VERIFIED | 101 lines |
| `bench/aggregator/aider_edit_baseline_test.go` | double-render golden + committed assertion | ✓ VERIFIED | 3 tests pass |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | --- | --- | ------ | ------- |
| `bench/runtime/aider_edit_agent.go` | `internal/forwarder` | `forwarder.OpenSession` in-process gRPC dial | ✓ WIRED | agent.go:90 |
| `bench/runtime/cell.go` | `bench/runtime/aider_edit_cell.go` | `if cfg.Mode == aiderEditMode { return runAiderEditCell(...) }` after baselineRagMode branch | ✓ WIRED | cell.go:454-456 |
| `bench/runtime/aider_edit_cell.go` | `bench/datasets/aider-polyglot/loader.go` | `aiderpolyglot.RunExercise(...)` verbatim | ✓ WIRED | cell.go:218 |
| `bench/reports/aider-edit-baseline/` | `.gitignore` | negated allowlist force-tracks under ignored `bench/reports/*` | ✓ WIRED | .gitignore:276; git ls-files confirms tracked |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Hermetic suite (HELIX_BIN unset) | `go test ./bench/runtime/ ./bench/aggregator/ ./bench/datasets/aider-polyglot/ ./bench/runners/ -run 'TestEditFormatApplied\|TestResolveAiderEdit\|TestLoadExercise\|TestNativeTestCommand\|TestAiderEdit'` | all ok | ✓ PASS |
| Live "did it RUN" sentinel (HELIX_BIN set) | `HELIX_BIN=$(pwd)/helix go test ./bench/runtime/ -run TestAiderEditCellLiveRan -v` | RUN → PASS (0.34s) | ✓ PASS |
| Sentinel gating | `go test ... -run TestAiderEditCellLiveRan` (HELIX_BIN unset) | SKIP | ✓ PASS |
| Byte-reproducibility (independent regen) | `make bench-aider-edit && git diff bench/reports/aider-edit-baseline/` | diff EMPTY (exit 0) | ✓ PASS |
| EDITBENCH-03 round-trip | `go test ./bench/runtime/ -run TestEditFormatApplied -v` | 4/4 subtests PASS | ✓ PASS |
| Anti-vacuity | `go test ... -run 'TestAiderEditCellAntiTamper\|TestAiderEditBaselineAntiVacuity'` | PASS | ✓ PASS |
| go vet tree-wide | `go vet ./...` | clean | ✓ PASS |
| go.mod unchanged | `git diff go.mod` | empty (zero new deps) | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ----------- | ----------- | ------ | -------- |
| EDITBENCH-01 | 100-01, 100-02 | EDIT-verb AgentFn drives RunExercise against warm daemon, loader untouched, WR-01 preserved | ✓ SATISFIED | Truth 1 |
| EDITBENCH-02 | 100-01 | aider_edit MODE.md via filesystem-table, zero mode-resolver change | ✓ SATISFIED | Truth 2 |
| EDITBENCH-03 | 100-01 | additive edit_format_applied *bool omitempty, no v3 bump | ✓ SATISFIED | Truth 2 |
| BASELINE-01 | 100-02 | committed byte-reproducible baseline, HELIX_BIN-gated fail-not-skip, deterministic metrics only | ✓ SATISFIED | Truth 3 |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| (none) | — | No TBD/FIXME/XXX/PLACEHOLDER debt markers in any Phase 100 modified file | ℹ️ Info | — |

### Tree-Wide Test Status

The ONLY tree-wide `go test ./...` failure is `TestRunSubcommandWiresDeltaPass` in `cmd/helix-bench` (run_cmd_test.go:144 — "delta pass not wired into runBench"). This was introduced in commit `05db8792` (`test(80-05)`, Phase 80-05) — a KNOWN pre-existing failure that predates Phase 100. Confirmed Phase 100 commits (bedacf92..fff4dbf8) did NOT touch `cmd/helix-bench`. Matches the documented expectation exactly. NOT a Phase 100 regression.

### Gaps Summary

No gaps. All 4 ROADMAP Success Criteria verified against the codebase with real commands. The headline BASELINE-01 byte-reproducibility was independently proven: `make bench-aider-edit` regenerated the committed baseline against the warm daemon and `git diff` returned empty. The live HELIX_BIN sentinel RAN (not skipped) and is non-vacuous (fail-closed present-signal reads). The hermetic sibling is the sole authoritative proof and passes with no binary/network. Leaf-import boundary clean (`go list -deps` = 0 forbidden imports). Schema stays v2. go vet clean tree-wide; go.mod unchanged.

---

_Verified: 2026-06-23_
_Verifier: Claude (gsd-verifier)_
