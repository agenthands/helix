# Phase 9: Benchmark Harness & v1.1 Baseline - Discussion Log

> **Audit trail only.** Decisions captured in CONTEXT.md.

**Date:** 2026-04-09
**Areas discussed:** Benchmark runner strategy, Benchmark scope per tool, Memory profile granularity

---

## Benchmark Runner Strategy

| Option | Selected |
|--------|----------|
| GitHub-hosted with wider gate (15%/25%) | |
| Self-hosted dedicated runner (tight 10%/20%) | |
| Both: dev wide, release tight | ✓ |
| You decide | |

**User's choice:** Both — GitHub-hosted on PRs (relaxed gate, fast feedback), self-hosted on release tags (tight gate, deterministic).

---

## Benchmark Scope

| Option | Selected |
|--------|----------|
| All 38 tools fully | ✓ |
| Critical-path subset | |
| Representative per category | |
| Tiered: quick vs full | |

**User's choice:** All 38 tools — comprehensive baseline worth the CI time.

---

## Memory Profile Granularity

| Option | Selected |
|--------|----------|
| b.ReportAllocs() only | |
| ReportAllocs + RSS snapshots | |
| Full pprof heap profiles | ✓ |
| Tiered: PR fast, release deep | |

**User's choice:** Full pprof heap profiles per major scenario, committed as CI artifacts.

---

## Claude's Discretion

- Histogram bucket configuration deferred to Phase 11
- benchstat invocation method (direct vs Make wrapper)
- p99 confidence interval handling (use as warning per PITFALLS guidance)
- Self-hosted runner provisioning details

## Deferred Ideas

None
