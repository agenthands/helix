---
status: complete
phase: 53-obs-metrics-gaps
source:
  - 53-01-SUMMARY.md
  - 53-02-SUMMARY.md
  - 53-03-SUMMARY.md
  - 53-04-SUMMARY.md
  - 53-05-SUMMARY.md
  - 53-06-SUMMARY.md
started: 2026-05-01T07:18:21Z
updated: 2026-05-01T07:32:00Z
verified_by: orchestrator (live daemon scrape, 2026-05-01T07:26:49Z to 07:30:00Z)
---

## Current Test

[testing complete]

## Tests

### 1. admin /metrics endpoint comes up clean
expected: `helix --serve --admin-addr 127.0.0.1:9090`. Daemon log shows `metrics endpoint enabled`. `curl http://127.0.0.1:9090/metrics` returns 200 OK with Go runtime metrics. No 5xx. helix_* families emit only after first observation — verified in Tests 2-7.
result: pass
verified_by: orchestrator — daemon log `msg="metrics endpoint enabled" addr=127.0.0.1:9090 path=/metrics`; curl returned `HTTP/1.1 200 OK Content-Type: text/plain; version=0.0.4`; body contained `go_gc_duration_seconds`, `process_resident_memory_bytes`. No 5xx, no panic.

### 2. lspool hit/miss counters increment on tool reuse
expected: Two `go_to_definition` calls on same Go file. First → `result="miss"`, second → `result="hit"`.
result: pass
verified_by: orchestrator — two go_to_definition calls on `internal/obs/metrics.go:83:15`. Result: `helix_lspool_lookups_total{language="go",result="miss"} 1` AND `helix_lspool_lookups_total{language="go",result="hit"} 1`.

### 3. repomap lookup + per-extractor latency emitted
expected: `get_repo_map` produces `helix_repomap_lookups_total{language=*,result=hit|miss}` AND `helix_repomap_extract_duration_seconds_bucket{language=*,extractor="treesitter|lsp|fallback"}` with histogram count == miss count per language.
result: pass
verified_by: orchestrator — get_repo_map call produced 25+ language rows. Sample: `helix_repomap_lookups_total{language="go",result="miss"} 395` AND `helix_repomap_extract_duration_seconds_count{extractor="treesitter",language="go"} 395`. Counts match per language. extractor="treesitter" labels live; lsp/fallback also valid label values.

### 4. edit_outcome=success on a clean replace
expected: `replace_in_file` with existing string → `helix_edit_outcome_total{tool_name="replace_in_file",outcome="success",strategy="exact"}` ≥1.
result: pass
verified_by: orchestrator — `replace_in_file path=graphify-out/uat-edit.txt pattern=hello`. Tool returned "1 replacement(s) made". Metric: `helix_edit_outcome_total{outcome="success",strategy="exact",tool_name="replace_in_file"} 1`.

### 5. edit_outcome failure path classifies correctly
expected: `replace_in_file` with non-existent string → `helix_edit_outcome_total{outcome="no_match",strategy="none"}` ≥1.
result: pass
verified_by: orchestrator — `replace_in_file pattern=NONEXISTENT_STRING_ZQX`. Tool returned `invalid_args: no fuzzy match found`. Metric: `helix_edit_outcome_total{outcome="no_match",strategy="none",tool_name="replace_in_file"} 1`. Note: outcome enum value is `no_match` (NOT `not_found`); strategy is `none` because no fuzzy strategy ran on the failure path. Both correct per Plan 04 SUMMARY's documented enum values.

### 5b. edit_outcome path-outside-workspace classifies as internal (Q-3 reclassification)
expected: `invalid_args` (e.g. path outside workspace) → `outcome="internal"` per Q-3 (preserves D-10 6-value enum; v1.3 reclassification deferred).
result: pass
verified_by: orchestrator — Two replace_in_file calls with `path=/tmp/uat-edit.txt` (outside workspace root). Tool returned `invalid_args: path outside workspace root`. Metric: `helix_edit_outcome_total{outcome="internal",strategy="none",tool_name="replace_in_file"} 2`. Q-3 classifier mapping `serr.InvalidArgs → outcome=internal` is live.

### 6. session_lifecycle stdio: started + ended counted
expected: stdio session via `helix forwarder` produces `helix_session_lifecycle_total{phase="started",transport="stdio"}` ≥1 and `phase="ended"` ≥1.
result: blocked
blocked_by: infrastructure
reason: Forwarder stdio launch would interrupt the orchestrator's running daemon session. Code path is covered by `internal/daemon/forwarder_test.go` — TDD RED→GREEN gate verified the 3 emission sites (started, ended, error). All unit tests green via `go test -race ./internal/daemon/...`.

### 7. session_lifecycle http: started + ended on DELETE /mcp
expected: `Mcp-Session-Id` first-seen → started; `DELETE /mcp` → ended.
result: pass
verified_by: orchestrator — POST /mcp initialize created session `LCKPB5L6TTOS3ZO35PQPP3FQWH`; follow-up POST with `Mcp-Session-Id` header → `helix_session_lifecycle_total{phase="started",transport="http"} 1`; `DELETE /mcp` with that session id → `phase="ended"` 1. After a second session: started=2, ended=1 (correct: second session not yet deleted).

### 8. USAGE.md PromQL hit-ratio query runs against Prometheus
expected: Paste lspool hit-ratio PromQL into a running Prometheus → returns numeric or empty (no syntax/eval error).
result: blocked
blocked_by: infrastructure
reason: No Prometheus scrape instance running. PromQL syntax verified by inspection — the queries `sum(rate(helix_lspool_lookups_total{result="hit"}[5m])) / sum(rate(helix_lspool_lookups_total[5m]))` and `histogram_quantile(0.95, sum by (le, extractor) (rate(helix_repomap_extract_duration_seconds_bucket[5m])))` use only standard PromQL and reference metric names + label names confirmed live in Tests 2 and 3. USAGE.md fix commit `1c50942e` already corrected the WR-03 typo (`outcome="error"` → real enum values).

### 9. USAGE.md table renders cleanly
expected: USAGE.md "Prometheus Metrics" 5 new rows render correctly; HTTP best-effort caveat visible.
result: pass
verified_by: orchestrator — read USAGE.md directly. The 5 new rows preserve the 3-column structure (Metric | Type | Description). All `{...}` enums and `helix_*` names are inside backticks (no pipe-escape issues). Two PromQL examples present (`promql` syntax tag). Best-effort blockquote starts with `> **HTTP session ended is best-effort.**` and is clearly worded.

## Summary

total: 9
passed: 7
issues: 0
pending: 0
skipped: 0
blocked: 2
extras: 1 (Test 5b)

## Gaps

[none]

## Blocked Items (not implementation gaps)

| Test | Reason | Mitigation |
|------|--------|------------|
| 6 | Forwarder stdio launch would interrupt orchestrator's running daemon | Code covered by unit test `internal/daemon/forwarder_test.go` (TDD RED→GREEN gate); 3 emission sites verified |
| 8 | No Prometheus scrape instance available | PromQL syntax verified by inspection; metric names + label sets used confirmed live in Tests 2-3 |

## Notes from this UAT cycle

- **Test 1 instructions error (mine):** Initial UAT said "`./helix daemon`" — the actual CLI uses `helix --serve` (or `helix --mode http`). Updated.
- **404 misdirection (mine):** Claimed `/metrics` lives on the admin listener, gave correct port, but didn't note that **CounterVec/HistogramVec families with no observed labelsets do NOT export HELP/TYPE comments** — only emit after first observation. This is documented Prometheus client_golang behavior, not a bug. The existing `internal/daemon/telemetry_metrics_test.go:42-51` primes every vector before scraping for exactly this reason. Updated Test 1 to reflect "endpoint comes up + Go runtime metrics visible" as the success signal, with helix_* family checks moved to Tests 2-7.
- **Q-3 reclassification confirmed live (Test 5b):** invalid_args → outcome=internal correctly preserves the locked D-10 6-value enum. The v1.3 follow-up TODO at `internal/kernel/edit/tools.go` (commit `31af9d8c` from WR-06 fix) tracks the future cardinality bound bump from 168 to 196 when invalid_args becomes its own bucket.
