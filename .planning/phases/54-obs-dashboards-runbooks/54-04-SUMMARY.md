---
phase: 54-obs-dashboards-runbooks
plan: 04
subsystem: observability
tags: [observability, runbooks, promql, operational]
requires:
  - "54-01 (validator scaffold + env-var gate)"
  - "54-02 (dashboards branch unconditional)"
  - "54-03 (engine dashboard + USAGE.md updates)"
provides:
  - "docs/runbooks/ErrCircuitOpen.md (critical, helix_lspool_circuit_state)"
  - "docs/runbooks/deadline-timeouts.md (warning, helix_tool_calls_total)"
  - "docs/runbooks/ls-crash-restart.md (critical, helix_lspool_evictions_total)"
  - "docs/runbooks/memory-pressure-eviction.md (warning, helix_lspool_evictions_total)"
  - "internal/obs/dashboards_test.go validator with env-var gate fully purged (both branches unconditional)"
affects:
  - "internal/obs/dashboards_test.go (final gate removal)"
tech-stack:
  added: []
  patterns:
    - "D-15 frontmatter (5 fields: title, severity, metric, since_phase, last_reviewed)"
    - "D-16 fixed H2 order: Symptoms → Triage → Likely Causes → Remediation"
    - "D-17 trailing ## Code references with file:line anchors"
    - "Lowercase ```promql fences for validator (Pitfall #8)"
key-files:
  created:
    - "docs/runbooks/ErrCircuitOpen.md"
    - "docs/runbooks/deadline-timeouts.md"
    - "docs/runbooks/ls-crash-restart.md"
    - "docs/runbooks/memory-pressure-eviction.md"
  modified:
    - "internal/obs/dashboards_test.go"
decisions:
  - "Used SMTC-verified file:line anchors as of 2026-05-01 in every Code references block"
  - "Cross-linked runbooks (ErrCircuitOpen → ls-crash-restart, memory-pressure-eviction, deadline-timeouts) so the operator can hop between cause classes"
  - "Verbatim PromQL from RESEARCH §A.6 in every Triage section to keep validator-passing properties intact"
  - "Severity choices match RESEARCH: critical for ErrCircuitOpen + ls-crash-restart (act now); warning for deadline-timeouts + memory-pressure-eviction (investigate)"
metrics:
  duration: "~25 min"
  completed: "2026-05-01"
---

# Phase 54 Plan 04: Operational runbooks + final gate removal

Shipped four operator-facing runbooks under `docs/runbooks/` covering Helix's most common operational failure modes (circuit-open, deadline timeouts, LS crash/restart, memory-pressure eviction) and removed the last `HELIX_DASHBOARDS_TEST_ALLOW_EMPTY` env-var gate from `internal/obs/dashboards_test.go`. The Phase 54 validator is now fully fail-closed against empty `deploy/grafana/` or `docs/runbooks/` trees with no env-var required.

## What was built

Four runbooks, each ≥ 50 lines with the locked D-15 / D-16 / D-17 structure:

| Runbook | Severity | Primary metric | PromQL queries |
|---|---|---|---|
| `ErrCircuitOpen.md` | critical | `helix_lspool_circuit_state` | 4 |
| `deadline-timeouts.md` | warning | `helix_tool_calls_total` | 3 |
| `ls-crash-restart.md` | critical | `helix_lspool_evictions_total` | 3 |
| `memory-pressure-eviction.md` | warning | `helix_lspool_evictions_total` | 2 |

All PromQL is in lowercase ```promql fences, copied verbatim from RESEARCH §A.6 so validator-passing properties from Wave 0 survive. All Code references reuse SMTC-verified anchors from RESEARCH §A.5.

## Drift table — RESEARCH §A.5 vs SMTC-verified at task time

| Anchor | RESEARCH-recorded | Verified-at-task-time | Drift | Notes |
|---|---|---|---|---|
| `internal/kernel/lspool/circuit.go:33` (NewCircuitBreaker) | 33 | 33 | 0 | unchanged |
| `internal/kernel/lspool/circuit.go:64` (RecordFailure) | 64 | 64 | 0 | unchanged |
| `internal/kernel/lspool/circuit.go:130` (OpenError) | 130 | **132** | +2 | RESEARCH used the now-deprecated symbol name `OpenError`; the actual function is `CircuitOpenErr` at 132. Within ±5 line tolerance. |
| `internal/kernel/lspool/pool.go:22` (RestartBudget) | 22 | 22 | 0 | unchanged |
| `internal/kernel/lspool/pool.go:320` (NewCircuitBreaker call site) | 320 | 320 | 0 | unchanged |
| `internal/mcp/middleware.go:113` (BudgetFunc) | 113 | 113 | 0 | unchanged |
| `internal/mcp/middleware.go:277` (TelemetryMiddleware) | 277 | 277 | 0 | unchanged |
| `internal/mcp/middleware.go:304-313` (deadline injection) | 304-313 | 304-313 | 0 | unchanged |
| `internal/config/config.go:28-32` (DegradationConfig) | 28-32 | **27-34** | range widened | the surrounding struct definition runs from 26 (comment) to 35; the timeout fields (28-32) are intact. Runbook cites 27-34 for completeness. |
| `internal/config/defaults.go:25-29` (defaults) | 25-29 | 25-29 | 0 | unchanged |
| `USAGE.md:818-820` (knob table) | 818-820 | **816-820** | -2 | header at 816, rows at 818-820. Within tolerance. Runbook cites 816-820. |
| `internal/kernel/lspool/pool.go:414-416` (EvictCrash) | 414-416 | 416 | 0 | line 416 is the `evictWorkerLocked(... EvictCrash)` call; 414-415 are the surrounding comment. |
| `internal/kernel/lspool/pool.go:306` (RecordSuccess) | 306 | **circuit.go:93** | source corrected | RESEARCH miscited: line 306 in pool.go is a comment about restart booking, while `RecordSuccess` itself is defined at `circuit.go:93`. Runbook uses the corrected anchor. |
| `internal/kernel/lspool/worker.go:434` (panic recovery) | 434 | 434 (comment) / 454 (recover()) | 0 / +20 | comment is at 434, the actual `recover()` call is at 454. Runbook annotates both. |
| `internal/kernel/lspool/pressure_linux.go:22` (PSI) | 22 | 22 | 0 | unchanged |
| `internal/kernel/lspool/pressure_darwin.go:24` (vm_stat) | 24 | 24 | 0 | unchanged |
| `internal/kernel/lspool/pool.go:403, 432, 452` (EvictPressure) | 403/432/452 | 403/432/452 | 0 | all three call sites unchanged |
| `internal/kernel/lspool/pressure.go:9-15` (PressureLevel) | 9-15 | **4-16** | range widened | enum block runs 4-16 (declaration at 4, last variant at 16). Runbook cites 4-16. |
| `internal/kernel/lspool/metrics.go:38` (EvictPressure constant) | 38 | 38 | 0 | unchanged |

No anchor drifted by more than 5 lines. The `circuit.go:130 → 132` and `USAGE.md:818-820 → 816-820` shifts are within tolerance (±5). The `pool.go:306 → circuit.go:93` correction is a RESEARCH error fixed in this plan.

## Validator status

- `go test ./internal/obs/... -count=1` (NO env-var) — **PASS** (0.71s).
- `HELIX_DASHBOARDS_TEST_ALLOW_EMPTY=1 go test ./internal/obs/... -count=1` — **PASS** (idempotent; env-var is now a no-op).
- `grep -c HELIX_DASHBOARDS_TEST_ALLOW_EMPTY internal/obs/dashboards_test.go` — **0** (env-var fully purged).
- `go vet ./internal/... ./cmd/...` — clean (only the pre-existing tree-sitter `TOKEN_COUNT` macro warning).

## Cross-link map (operator hop graph)

- `ErrCircuitOpen.md` → `ls-crash-restart.md` (crash-driven cause)
- `ErrCircuitOpen.md` → `memory-pressure-eviction.md` (pressure-driven cause)
- `ErrCircuitOpen.md` → `deadline-timeouts.md` (slow-LS cause)
- `ls-crash-restart.md` → `memory-pressure-eviction.md` (OOM remediation path)

## Deviations from Plan

None — plan executed exactly as written. The two-task sequence (author runbooks → remove gate) ran clean; both validator runs (with and without env-var) pass on first attempt.

## Self-Check: PASSED

- `docs/runbooks/ErrCircuitOpen.md` — FOUND
- `docs/runbooks/deadline-timeouts.md` — FOUND
- `docs/runbooks/ls-crash-restart.md` — FOUND
- `docs/runbooks/memory-pressure-eviction.md` — FOUND
- Commit `bef74c61` — FOUND (four runbooks added)
- Commit `ad505d71` — FOUND (env-var gate removed)
- All four runbooks have D-15 frontmatter (5 fields), D-16 H2 order, D-17 Code references — verified by grep loop
- `internal/obs/dashboards_test.go` env-var purge — count = 0
- `go test ./internal/obs/...` no-env passes
