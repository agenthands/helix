---
phase: 64
slug: new-mcp-tools
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-05-07
---

# Phase 64 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (Go 1.22+) |
| **Config file** | none — uses repo-root `go.mod` and existing `_test.go` infrastructure |
| **Quick run command** | `go test ./internal/skill/semantic/... ./internal/semantic/... -count=1 -timeout=120s` |
| **Full suite command** | `go test ./... -count=1 -timeout=600s && go vet ./... && gofmt -l .` |
| **Estimated runtime** | ~90s quick / ~600s full |

---

## Sampling Rate

- **After every task commit:** Run quick command (skill + semantic packages)
- **After every plan wave:** Run full suite command
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~120s for quick, ~600s for full

---

## Per-Task Verification Map

> Populated by gsd-planner during plan generation. Every plan task with `<automated>` block contributes one row referencing the test file + command. Rows below are seeded from RESEARCH.md §"Validation Architecture" (23 acceptance tests across unit/integration/property/determinism/benchmark) and will be expanded by the planner.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD     | TBD  | TBD  | TOOL-01     | —          | N/A             | unit      | TBD               | ❌ W0       | ⬜ pending |
| TBD     | TBD  | TBD  | TOOL-02     | —          | N/A             | unit      | TBD               | ❌ W0       | ⬜ pending |
| TBD     | TBD  | TBD  | TOOL-03     | —          | N/A             | unit      | TBD               | ❌ W0       | ⬜ pending |
| TBD     | TBD  | TBD  | TOOL-04     | —          | profile/mode gating | integration | TBD          | ❌ W0       | ⬜ pending |
| TBD     | TBD  | TBD  | TOOL-05     | —          | N/A             | unit      | TBD               | ❌ W0       | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/skill/semantic/*_test.go` — handler unit-test stubs for the 4 tools (RESEARCH.md §V.1)
- [ ] `internal/semantic/*_test.go` — `QueryEffectiveAdjacency` / `CountStaleScoreRows` / `MarkAllScoreRowsStale` table tests (RESEARCH.md §V.1)
- [ ] `internal/mcp/profile_filter_semantic_test.go` — `tools/list` filtering for the four new tools across profile matrix (RESEARCH.md §V.2)
- [ ] `internal/skill/semantic/integration_test.go` — overlay→edit→`get_semantic_context` foreground-budget integration (RESEARCH.md §V.3)
- [ ] `internal/skill/semantic/dispatch_property_test.go` — singleflight property test for D-02 dispatch (RESEARCH.md §V.4)
- [ ] `internal/skill/semantic/determinism_test.go` — stable-key tiebreak determinism for `get_semantic_context` selection (RESEARCH.md §V.5)
- [ ] `bench/semantic_bench_test.go` — bleve binary-size + 50k-symbol-fixture throughput benchmarks (D-08; threshold gate before locking bleve)

*Wave 0 must complete before Wave 1 task work begins. Bleve benchmark is BLOCKING per D-08 — fallback to DuckDB FTS5 if thresholds bust.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `get_tool_help` returns parameter docs for the 4 new tools | TOOL-05 | Output is human-readable markdown; cheaper to eyeball than to fixture | `helix daemon` then call `get_tool_help index_semantic_graph` etc. through the forwarder; verify schema, modes, freshness fields present. Drive via Bash, never ask user (per memory: feedback_uat_no_manual_mcp). |

*Phase 64 has no UI surface — all behavior reachable through MCP tool calls.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references (incl. bleve benchmark gate D-08)
- [ ] No watch-mode flags
- [ ] Feedback latency < 600s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
