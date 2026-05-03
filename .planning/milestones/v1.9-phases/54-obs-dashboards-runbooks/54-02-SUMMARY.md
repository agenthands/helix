---
phase: 54-obs-dashboards-runbooks
plan: 02
subsystem: observability
tags: [observability, grafana, dashboard, red-metrics, sessions, wave-1]
requires:
  - 54-01 (registry-driven PromQL validation gate, prometheus/prometheus@v0.311.3 dep)
provides:
  - deploy/grafana/helix-overview.json (RED + sessions operator dashboard, 7 panels)
  - dashboards-branch of TestDashboardsAndRunbooksReferenceRegisteredMetrics now unconditionally fail-closed
affects:
  - internal/obs/dashboards_test.go (env-var gate scoped to runbooks branch only)
tech-stack:
  added: []
  patterns:
    - Grafana 10 schemaVersion 38 import-template (${DS_PROMETHEUS}, no hardcoded UIDs)
    - $language template var driven by `label_values(helix_lspool_workers, language)`
    - $instance template var driven by `label_values(up{job=~".*helix.*"}, instance)`
    - stat panel reduceOptions.calcs=["lastNotNull"] (Pitfall #2)
    - text panel for HTTP `ended` best-effort caveat (D-09 cross-doc consistency)
key-files:
  created:
    - deploy/grafana/helix-overview.json (249 LOC, 7 panels)
  modified:
    - internal/obs/dashboards_test.go (env-var gate removed from dashboards branch; comments rescoped)
decisions:
  - "D-02-A: schemaVersion 38 baseline kept (no bump to 39) — RESEARCH §A.3 baseline is sufficient for the panel types in use (timeseries, stat, text); no schemaVersion-39-only feature needed."
  - "D-02-B: Panel count = 7 (within 6–9 D-04 band). Layout: 2 RED panels top row (12+12 cols), p50/p95/p99 latency full-width second row (24 cols), session triplet third row (12 + 6 + 6 cols), text caveat full-width fourth row."
  - "D-02-C: Both $language and $instance template vars include `All` with allValue=`.*` — operators get a single-language drill-down without breaking the default 'show me everything' view."
  - "D-02-D: The text caveat panel (id=99) keeps datasource=null per Grafana convention for non-query panels — JSON validator and registry-validator both ignore it because it has no targets[]."
metrics:
  duration: ~10 min
  completed: 2026-05-01
  tasks_completed: 2
  files_created: 1
  files_modified: 1
  commits: 2
---

# Phase 54 Plan 02: Helix Overview Dashboard Summary

**One-liner:** Wave 1 ships the screenshot-target operator dashboard — 7 panels covering MCP tool RED metrics + workspace session lifecycle + the HTTP-ended best-effort caveat — and unconditionally enforces the dashboards-branch of the registry validator.

## What was done

- **Task 1 (commit `0d48e377`)**: authored `deploy/grafana/helix-overview.json` (249 LOC, 7 panels). Every PromQL `targets[].expr` references the frozen 12-family Phase 53 metric inventory (`helix_tool_calls_total`, `helix_tool_duration_seconds_bucket`, `helix_session_lifecycle_total`); template-variable queries reference `helix_lspool_workers` (for `$language`) and `up{job=~".*helix.*"}` (for `$instance`). Stat panel #5 carries the mandatory `reduceOptions.calcs=["lastNotNull"]`. Text panel #99 reproduces the Phase 53 D-09 best-effort caveat verbatim ("`DELETE /mcp`", "best-effort", "lower bound"). No `alert` blocks (D-07).
- **Task 2 (commit `902b5c64`)**: removed the `os.Getenv("HELIX_DASHBOARDS_TEST_ALLOW_EMPTY") != "1"` gate from the dashboards-empty fatal in `internal/obs/dashboards_test.go::TestDashboardsAndRunbooksReferenceRegisteredMetrics`. The runbooks-empty fatal is still gated until Plan 54-04 ships content. Updated the file-level comment and the per-test comment to scope the remaining Wave-0 escape strictly to the runbooks branch.

## Panel inventory (7 panels — within 6–9 D-04 band)

| id | type       | title                                      | gridPos (h,w,x,y) | metric family                       |
| -- | ---------- | ------------------------------------------ | ----------------- | ----------------------------------- |
| 1  | timeseries | Tool call rate (5m)                        | (8, 12, 0, 0)     | helix_tool_calls_total              |
| 2  | timeseries | Tool error rate by outcome (5m)            | (8, 12, 12, 0)    | helix_tool_calls_total              |
| 3  | timeseries | Tool duration p50/p95/p99                  | (8, 24, 0, 8)     | helix_tool_duration_seconds_bucket  |
| 4  | timeseries | Session start rate by transport (5m)       | (8, 12, 0, 16)    | helix_session_lifecycle_total       |
| 5  | stat       | Session error rate (5m)                    | (8, 6, 12, 16)    | helix_session_lifecycle_total       |
| 6  | timeseries | Session ended rate by transport (5m)       | (8, 6, 18, 16)    | helix_session_lifecycle_total       |
| 99 | text       | Note: HTTP session `ended` is best-effort  | (4, 24, 0, 24)    | (no targets — markdown caveat)      |

## Verification results

| Gate | Result |
|------|--------|
| `python3 -m json.tool deploy/grafana/helix-overview.json` | ✅ valid JSON |
| `grep -c '"uid": "helix-overview"'` | ✅ 1 |
| `grep -c '"schemaVersion": 38'` | ✅ 1 |
| `grep -c "DS_PROMETHEUS"` (any context) | ✅ 17 (well above ≥ 8 intent) |
| `grep -c '"alert"'` | ✅ 0 (D-07) |
| `grep -c '"calcs": ["lastNotNull"]'` | ✅ 1 (stat panel #5) |
| `grep -c 'label_values(helix_lspool_workers, language)'` | ✅ 2 (definition + query) |
| `grep -c 'label_values(up{job=~".*helix.*"}, instance)'` | ✅ 2 (definition + query) |
| Text panel contains "best-effort" + "DELETE /mcp" | ✅ both present |
| Panel count | ✅ 7 (within [6, 9]) |
| `HELIX_DASHBOARDS_TEST_ALLOW_EMPTY=1 go test ./internal/obs/... -count=1` | ✅ green (all tests pass) |
| `go test ./internal/obs/... -count=1 -run "^TestDashboardsAndRunbooksReferenceRegisteredMetrics$"` (no env var) | ✅ FAILS exclusively on `no runbooks found at`; dashboards branch silent (proves Task 2) |
| `go test ./internal/obs/... -count=1 -run TestDashboardsAndRunbooksValidatorFailsClosedOnEmpty` | ✅ green (fail-closed contract preserved) |
| `go vet ./...` | ✅ green |

## Env-var gate state (post-plan)

```
$ grep -n "HELIX_DASHBOARDS_TEST_ALLOW_EMPTY" internal/obs/dashboards_test.go
20:// HELIX_DASHBOARDS_TEST_ALLOW_EMPTY=1 because Plan 54-04 has not yet shipped
26:// REMOVE the `os.Getenv("HELIX_DASHBOARDS_TEST_ALLOW_EMPTY") != "1"` clause
274:// runbooks branch is still gated behind HELIX_DASHBOARDS_TEST_ALLOW_EMPTY=1
300:	if len(rbs) == 0 && os.Getenv("HELIX_DASHBOARDS_TEST_ALLOW_EMPTY") != "1" {
302:			"Set HELIX_DASHBOARDS_TEST_ALLOW_EMPTY=1 only during Phase 54 Wave 0.", rbGlob)
333:// fail-closed precondition INDEPENDENTLY of the HELIX_DASHBOARDS_TEST_ALLOW_EMPTY
```

The remaining mentions are: (1) two file-level comment lines pointing Plan 54-04 at the runbook gate, (2) one per-test comment line scoping the gate to the runbooks branch, (3) the runbooks-empty fatal condition + its message (lines 300/302 — the only LIVE gate), (4) one mention in the `TestDashboardsAndRunbooksValidatorFailsClosedOnEmpty` doc comment. The dashboards branch (lines 282-285 prior to edit) is fully cleansed of the env-var.

> **Plan 54-04 instruction (carried forward from 54-01):** the FIRST action when shipping any runbook content is to delete the `if len(rbs) == 0 && os.Getenv("HELIX_DASHBOARDS_TEST_ALLOW_EMPTY") != "1"` clause at line 300 and replace with the unconditional fail-closed (mirroring this plan's dashboards-branch edit), then update the package-level and per-test comments to drop the remaining "still pending Plan 54-04" language.

## Deviations from Plan

### Plan vs. acceptance-criterion grep precision

The plan acceptance criterion read `grep -c '"DS_PROMETHEUS"' deploy/grafana/helix-overview.json` returns ≥ 8. As written this grep matches only the literal `"DS_PROMETHEUS"` string (i.e., the `__inputs[0].name` field) and would NOT match the `${DS_PROMETHEUS}` template expansions used throughout the panel and templating bodies. The dashboard file actually contains 1 `"DS_PROMETHEUS"` literal (the inputs entry) and 16 `${DS_PROMETHEUS}` expansions — 17 total references, well above the ≥ 8 intent. No file change required; the criterion as written is too strict but the dashboard's actual structure satisfies the intent. Not tracked as a Rule deviation because no code/data changes were needed.

### Auto-fixed issues

None. The plan was executed exactly as written for both tasks.

### Auth gates

None.

## Self-Check: PASSED

- ✅ `deploy/grafana/helix-overview.json` exists (249 lines, 7 panels)
- ✅ commit `0d48e377` exists (`feat(54-02): add deploy/grafana/helix-overview.json (RED + sessions)`)
- ✅ commit `902b5c64` exists (`test(54-02): drop env-var gate from dashboards branch of validator`)
- ✅ `HELIX_DASHBOARDS_TEST_ALLOW_EMPTY=1 go test ./internal/obs/... -count=1` exits 0
- ✅ `go test ./internal/obs/... -count=1 -run "^TestDashboardsAndRunbooksReferenceRegisteredMetrics$"` (no env var) exits non-zero with the failure scoped to "no runbooks found at" (NOT "no dashboards found at")
- ✅ `go test ./internal/obs/... -count=1 -run TestDashboardsAndRunbooksValidatorFailsClosedOnEmpty` exits 0 (fail-closed contract preserved)
- ✅ `go vet ./...` exits 0
- ✅ `python3 -m json.tool deploy/grafana/helix-overview.json` exits 0
- ✅ Every PromQL in the dashboard references one of the 12 registered helix_* families with valid label names (validated by registry test)
