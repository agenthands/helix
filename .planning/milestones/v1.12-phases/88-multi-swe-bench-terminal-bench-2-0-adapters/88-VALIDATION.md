---
phase: 88
slug: multi-swe-bench-terminal-bench-2-0-adapters
status: planned
nyquist_compliant: true
wave_0_complete: false
created: 2026-06-21
---

# Phase 88 — Validation Strategy

> Seeded from the "Validation Architecture" section of 88-RESEARCH.md. Governing rule (MEMORY
> false-green + Phase 81 vacuous-gate): the config.json producer, harness/tb-JSON → result.v2 ingestion,
> the per-language slicing, and the long-wall checkpoint/resume state machine are all pure logic — their
> HERMETIC tests against committed fixtures (sample multi_swe_bench config + report, sample tb/harbor
> results.json, a checkpoint-resume case with an injected clock) are the SOLE authoritative proof. The
> live Multi-SWE-bench Mini-set run and the Terminal-Bench `tb run` smoke SKIP cleanly (Docker + the
> multi_swe_bench package + the tb/harbor CLI all absent here). No live test may be the sole proof.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (committed multi_swe_bench/tb JSON fixtures + checkpoint fixtures) |
| **Config file** | none — standard Go toolchain |
| **Quick run command** | `go vet ./bench/... && go test ./bench/evaluators/... ./bench/longwall/... ./bench/datasets/... ./bench/aggregator/...` |
| **Full suite command** | `go test ./...` (live Docker + multi_swe_bench + tb/harbor smoke tests SKIP unless present) |
| **Estimated runtime** | ~60–120 seconds (hermetic); live smoke gated/skipped |

---

## Sampling Rate

- **After every task commit:** Run the quick run command for touched packages
- **After every plan wave:** Run the full suite command
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~120 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| (planner fills from RESEARCH Validation Architecture) | | | ADAPTER-MULTI-01 / ADAPTER-TERM-01 | unit(fixture) | | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Multi-SWE-bench `config.json` producer hermetic test (the documented config fields; deterministic)
- [ ] Multi-SWE-bench harness argv builder (`python -m multi_swe_bench.harness.run_evaluation --config <config.json>`) golden-argv test; fixed-argv + strict env (clone swebench harness.go)
- [ ] Multi-SWE-bench report → result.v2 ingestion hermetic test (per-instance resolved; directory→`Cell.Language` mapping so per-language slicing works through the existing `reduceLanguageRows`)
- [ ] Terminal-Bench `tb run`/`harbor run` argv builder (binary-name seam) + tb `results.json` → result.v2 ingestion hermetic test (`accuracy`/`n_resolved`/`is_resolved`)
- [ ] **Long-wall checkpoint/resume state machine (`bench/longwall/`)**: 3 hermetic invariants — checkpoint round-trip (atomic write/read), resume-skips-done cells, idempotent re-entry; injected clock; NO real 24h run
- [ ] Per-language slicing verified through the existing aggregator `reduceLanguageRows`/`ByLanguage` (zero new aggregator code) over a multi-language fixture
- [ ] `bench/LICENSES.md` rows: Multi-SWE-bench (CC0, Mini set) + Terminal-Bench 2.0 (Apache-2.0); full-set license status documented (defer to v1.13 if unresolved)

---

## Manual-Only / Docker-Gated Verifications

| Behavior | Requirement | Why Gated | Test Instructions |
|----------|-------------|-----------|-------------------|
| Multi-SWE-bench Mini-set live run via subprocess to multi_swe_bench harness | ADAPTER-MULTI-01 | Needs Docker + the multi_swe_bench Python package + images | On a Docker host with multi_swe_bench installed, run the Mini set; confirm per-language result.v2 rows |
| Terminal-Bench 2.0 ≥5-task smoke via `tb run`/`harbor run` + container-isolation | ADAPTER-TERM-01 | Needs the tb/harbor CLI + Docker | Run ≥5 tb tasks; confirm per-task fresh container (no cross-task fs leakage) + results.json ingested |
| Exact upstream config field spellings + Mini-set HF repo id + tb-vs-harbor binary | ADAPTER-MULTI-01/TERM-01 | A1-A7 [ASSUMED] upstream details | checkpoint:human-verify on a Docker host — confirm contracts against upstream |

*All producer/ingestion/slicing/checkpoint-resume decision logic has hermetic automated proof; the >24h long-wall is proven via an injected-clock checkpoint test, not a real 24h run.*

---

## Validation Sign-Off

- [ ] config producer, both ingestions, per-language slicing, and the checkpoint state machine each have a hermetic fixture test (sole proof)
- [ ] Checkpoint-resume proven via injected clock (no real 24h run); resume-skips-done + idempotent re-entry asserted
- [ ] No live/Docker/tb test is the sole proof of a success criterion
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** planned
