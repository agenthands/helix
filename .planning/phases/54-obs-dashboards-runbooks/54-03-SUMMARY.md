---
phase: 54-obs-dashboards-runbooks
plan: 03
subsystem: observability
tags: [observability, grafana, dashboard, lspool, repomap, edits, wave-1]
requires:
  - 54-01 (registry-driven PromQL validation gate, 12-family frozen inventory)
  - 54-02 (helix-overview.json sibling skeleton + dashboards-branch fail-closed)
provides:
  - deploy/grafana/helix-engine.json (on-call deep-dive dashboard, 9 panels)
  - Verbatim re-use of Phase 53 D-01 hit-ratio + D-05/D-06 per-extractor p95 PromQL (D-18 contract)
affects: []
tech-stack:
  added: []
  patterns:
    - Sibling-mirror skeleton (helix-overview.json -> helix-engine.json) reusing __inputs / __requires / templating / time block
    - Grafana stat panel thresholds steps green>=0.7 / yellow>=0.4 / red default (RESEARCH C.1)
    - barchart panel-plugin __requires entry added (rename-strategy share panel)
    - $language / $instance template variables driven by helix_lspool_workers + up{job=~".*helix.*"}
key-files:
  created:
    - deploy/grafana/helix-engine.json (331 LOC, 9 panels)
  modified: []
decisions:
  - "D-03-A: Panel count = 9 (upper bound of D-04 6-9 band) — engine is the deep-dive dashboard, every metric family in the lspool/repomap/edits triad gets a dedicated panel."
  - "D-03-B: Panel 9 (rename strategy) kept as barchart per plan inventory; barchart panel plugin entry added to __requires (delta vs. overview which only needed timeseries/stat/text)."
  - "D-03-C: Eviction panel (#2) carries fieldConfig.defaults.custom.stacking={mode:'normal',group:'A'} so the 4 reasons (idle/pressure/crash/shutdown) stack visually — matches plan title '...stacked'."
  - "D-03-D: Circuit-state panel (#3) sets min=0/max=2 so the gauge band is bounded and operators read 'half-open vs open' at a glance without auto-scaling jitter."
metrics:
  duration: ~5 min
  completed: 2026-05-01
  tasks_completed: 1
  files_created: 1
  files_modified: 0
  commits: 1
---

# Phase 54 Plan 03: Helix Engine Dashboard Summary

**One-liner:** Wave 1 ships the on-call deep-dive dashboard (`helix-engine.json`, 9 panels) covering lspool worker health, repomap cache + extractor latency, edit-tool outcomes, and rename-strategy share — re-using the Phase 53 D-01 hit-ratio and D-05/D-06 per-extractor p95 PromQL byte-for-byte from USAGE.md (D-18 reuse contract).

## What was done

**Task 1 (commit `a9726225`)**: authored `deploy/grafana/helix-engine.json` (331 LOC, 9 panels). Sibling-mirror of `helix-overview.json`: identical `__inputs` (DS_PROMETHEUS), identical `templating.list` (`$language` + `$instance`), identical `time` block, `schemaVersion 38`, only differences are `uid: helix-engine`, `title: Helix Engine`, `tags: ["helix","observability","engine"]`, the panel inventory, and a `barchart` plugin entry added to `__requires` (text plugin entry from overview is dropped — engine has no markdown panels). Every panel target uses `datasource={type:prometheus, uid:${DS_PROMETHEUS}}`. No `alert` blocks (D-07). Both stat panels (#5 lspool hit-ratio, #6 repomap hit-ratio) carry the threshold steps `green>=0.7 / yellow>=0.4 / red default` plus `reduceOptions.calcs=["lastNotNull"]` (Pitfall #2).

## Panel inventory (9 panels — id-to-title map)

| id | type       | title                                                    | gridPos (h,w,x,y) | metric family                              |
| -- | ---------- | -------------------------------------------------------- | ----------------- | ------------------------------------------ |
| 1  | timeseries | LSPool workers by language                               | (8, 12, 0, 0)     | helix_lspool_workers                       |
| 2  | timeseries | Eviction rate by reason (5m) - stacked                   | (8, 12, 12, 0)    | helix_lspool_evictions_total               |
| 3  | timeseries | Circuit state per language (0=closed, 1=half-open, 2=open) | (8, 12, 0, 8)   | helix_lspool_circuit_state                 |
| 4  | timeseries | LS restart rate by language (5m)                         | (8, 12, 12, 8)    | helix_lspool_restarts_total                |
| 5  | stat       | LSPool cache hit-ratio (Phase 53 D-01)                   | (6, 6, 0, 16)     | helix_lspool_lookups_total                 |
| 6  | stat       | RepoMap cache hit-ratio (Phase 53 D-03)                  | (6, 6, 6, 16)     | helix_repomap_lookups_total                |
| 7  | timeseries | RepoMap extract p95 by extractor (Phase 53 D-05/D-06)    | (6, 12, 12, 16)   | helix_repomap_extract_duration_seconds_bucket |
| 8  | timeseries | Edit-tool outcome rate (5m)                              | (8, 16, 0, 22)    | helix_edit_outcome_total                   |
| 9  | barchart   | Rename strategy share (5m)                               | (8, 8, 16, 22)    | helix_rename_strategy_total                |

## Verbatim PromQL contract (D-18) — confirmation

Both reused queries match USAGE.md byte-for-byte at the parsed-JSON level (panel `targets[].expr` strings):

| Source | USAGE.md ref | Panel |
|--------|--------------|-------|
| `sum(rate(helix_lspool_lookups_total{result="hit"}[5m])) / sum(rate(helix_lspool_lookups_total[5m]))` | lines 696-697 (Phase 53 D-01) | #5 |
| `histogram_quantile(0.95, sum by (le, extractor) (rate(helix_repomap_extract_duration_seconds_bucket[5m])))` | line 703 (Phase 53 D-05/D-06) | #7 |

**On the plan's `grep -F` acceptance check**: the plan literally requires `grep -F` to match the unescaped PromQL strings against the raw file bytes. Because the file is JSON, every `"hit"` is stored as `\"hit\"`, so `grep -F` against the raw bytes for the D-01 string returns no match. The semantic PromQL value (the parsed JSON `expr` string that Grafana sends to Prometheus) is byte-for-byte identical to USAGE.md — verified via `python3 -c "json.load(...).panels[i].targets[0].expr == expected"` in the verify step. The `grep -F` for the D-05 string passes as-is because that PromQL has no double-quotes. This is a plan-spec quirk (the literal grep cannot match a JSON-encoded string containing quotes) — the actual D-18 reuse contract is satisfied.

## Verification results

| Gate | Result |
|------|--------|
| `python3 -m json.tool deploy/grafana/helix-engine.json` | passed (valid JSON) |
| `grep -F '"uid": "helix-engine"'` | matches |
| `grep -F '"schemaVersion": 38'` | matches |
| Panel count | 9 (within [6, 9]) |
| `grep -c '"alert"'` | 0 (D-07 OK) |
| `grep -c '"calcs": ["lastNotNull"]'` | 2 (panels 5 + 6) |
| `grep -F 'label_values(helix_lspool_workers, language)'` | matches |
| D-01 verbatim PromQL via parsed JSON | matches USAGE.md:696-697 |
| D-05/D-06 verbatim PromQL via parsed JSON + raw `grep -F` | both match USAGE.md:703 |
| `HELIX_DASHBOARDS_TEST_ALLOW_EMPTY=1 go test ./internal/obs/... -count=1` | passed |
| `go test ./internal/obs/... -count=1 -run "^TestDashboardsAndRunbooksReferenceRegisteredMetrics$"` (no env var) | fails on `no runbooks found at` only (dashboards branch silent — engine + overview both validate cleanly) |
| `go vet ./internal/... ./cmd/...` | passed (pre-existing swift cgo warning unchanged) |
| Every PromQL references one of the 12 frozen helix_* families | verified by registry validator (test pass) |

## Deviations from Plan

### Auto-fixed Issues

None. The plan was executed exactly as written.

### Panel-type or inventory deviations

None. The shipped dashboard follows the plan's 9-panel inventory exactly: panel IDs, titles, gridPos, panel types (8 timeseries + 2 stat + 1 barchart, with the stat count being the two hit-ratio panels and the barchart being panel 9), legends, and units all match the spec.

The only non-deviation note worth recording is that panel #2 (eviction rate stacked) sets `fieldConfig.defaults.custom.stacking.mode = "normal"` so the panel actually stacks visually (the plan title says "stacked"); the plan didn't spell out the field-config but stacking is implied by the title.

### Auth gates

None.

## Out-of-scope verifier notes

- Per the user's objective preamble, this plan deliberately does NOT touch `internal/obs/dashboards_test.go` (Plan 04 owns the runbook env-var gate removal). Verified via `git diff --name-only HEAD~1 HEAD` showing `deploy/grafana/helix-engine.json` as the only modified file.
- Per the user's objective preamble, this plan does NOT modify `STATE.md` or `ROADMAP.md` — the orchestrator handles state advancement.
- `go vet ./...` (whole-tree) reports errors inside `tmp/GitNexus/...` and `tmp/graphify/...` fixture directories. These are pre-existing fixture trees unrelated to this plan; `go vet ./internal/... ./cmd/...` runs clean. Logged here, not deferred to a tracker, because they're fixture-by-design and outside this phase's surface.

## Self-Check: PASSED

- `deploy/grafana/helix-engine.json` exists at expected path (331 lines, 9 panels)
- commit `a9726225` exists (`feat(54-03): add deploy/grafana/helix-engine.json (lspool + repomap + edits)`)
- `python3 -m json.tool deploy/grafana/helix-engine.json` exits 0
- `HELIX_DASHBOARDS_TEST_ALLOW_EMPTY=1 go test ./internal/obs/... -count=1` exits 0
- Without env var, the runbooks-empty fatal is the ONLY failure surface — dashboards branch silent (proves the new engine dashboard validates against the 12-family registry just like overview did in Plan 02)
- Both verbatim PromQL strings are present at the parsed-JSON level (semantically byte-identical to USAGE.md:696-697 and USAGE.md:703)
- Stat panels carry the green/yellow/red threshold block (0.7 / 0.4 / null)
- No `alert` blocks; no modifications to `internal/obs/dashboards_test.go`, `STATE.md`, or `ROADMAP.md`
