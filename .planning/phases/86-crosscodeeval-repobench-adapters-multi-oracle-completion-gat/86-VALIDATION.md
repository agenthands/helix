---
phase: 86
slug: crosscodeeval-repobench-adapters-multi-oracle-completion-gate
status: planned
nyquist_compliant: true
wave_0_complete: false
created: 2026-06-21
---

# Phase 86 — Validation Strategy

> Seeded from the "Validation Architecture" section of 86-RESEARCH.md. Governing rule (MEMORY
> false-green + Phase 81 vacuous-gate): the EM/edit-similarity/identifier scorers, the multi-oracle
> gate (with abstain), and the canary probe are PURE logic — their HERMETIC unit tests against
> committed CCE/RepoBench paper-example fixtures are the SOLE authoritative proof. The live HF fetch +
> "metrics match published reference values" smoke run SKIPs cleanly offline. No live/network test may
> be the sole proof of any success criterion.
> NOTE: the Phase 75 HF fetcher does NOT exist in-tree — this phase BUILDS it (stdlib net/http + arrow-go parquet).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (committed CCE/RepoBench paper-example fixtures) |
| **Config file** | none — standard Go toolchain |
| **Quick run command** | `go vet ./bench/... && go test ./bench/evaluators/... ./bench/datasets/... ./bench/aggregator/...` |
| **Full suite command** | `go test ./...` (live HF-fetch + reference-match smoke tests SKIP unless network present) |
| **Estimated runtime** | ~60–120 seconds (hermetic); live HF-fetch gated/skipped |

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
| (planner fills from RESEARCH Validation Architecture) | | | ADAPTER-CCE-01 / ADAPTER-REPO-01 / VERIFIED-03 | unit(fixture) | | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] EM (exact-match) scorer hermetic test against CCE paper examples (SOLE proof of EM)
- [ ] Edit-similarity scorer hermetic test (normalized Levenshtein, NOT git-numstat EditDistancePatch) against CCE/RepoBench reference values
- [ ] Identifier-match scorer hermetic test against CCE paper examples
- [ ] Multi-oracle gate hermetic test: all-three-required → pass; any-fail → not-pass; per-oracle configurable threshold; abstain → `verified_correctness=false` (NEVER a false `true`)
- [ ] CrossCodeEval loader hermetic test over a committed CCE-shaped fixture (Python/Java/TS/C#)
- [ ] RepoBench loader hermetic test over committed fixtures for RepoBench-R (acc@k) / -C (EM/ES) / -P (Python+Java)
- [ ] HF parquet fetcher (net/http + arrow-go) — unit test of the cache layout + parquet parse over a committed small parquet fixture; live HF fetch network-gated
- [ ] Canary-emission probe hermetic test (emit + contaminated-task flag) + aggregator canary-pass-rate column
- [ ] `bench/evaluators/VERIFIED.md` documents the multi-oracle gate (SC#3)

---

## Manual-Only / Network-Gated Verifications

| Behavior | Requirement | Why Gated | Test Instructions |
|----------|-------------|-----------|-------------------|
| CrossCodeEval live smoke (≥1 task/lang) via HF parquet fetch | ADAPTER-CCE-01 | Needs network + the HF dataset rev | Run the CCE adapter online; confirm ≥1 scored task per Python/Java/TS/C# |
| RepoBench live smoke (-R/-C/-P) metrics match published reference on a sampled subset | ADAPTER-REPO-01 | Needs network + HF datasets | Run RepoBench adapter online; compare sampled metrics to published reference within tolerance |
| CCE HF mirror rev pin / dataset-viewer cast-error workaround | ADAPTER-CCE-01 | Known upstream cast-error; rev must be pinned + fixtures sourced from the paper | checkpoint:human-verify — confirm the pinned CCE rev loads per-language |

*All scorers, the multi-oracle gate, the loaders' parse logic, the parquet cache layout, and the canary probe have hermetic automated proof via committed fixtures.*

---

## Validation Sign-Off

- [ ] EM/ES/identifier scorers + multi-oracle gate + canary each have a hermetic fixture test (sole proof)
- [ ] No live/network test is the sole proof of a success criterion
- [ ] Abstain mode proven to emit `verified_correctness=false`, never a false `true`
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** planned
