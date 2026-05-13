# Phase 68 Discussion Log

**Date:** 2026-05-13
**Phase:** 68 — Precise FileFactDiff Populator
**Mode:** discuss (default)

## Gray Areas Selected

User selected all four presented gray areas:
1. Pre-edit FileFact seam (Store accessor vs handler cache)
2. Extractor invocation — sync in-tx vs async scheduler-driven
3. Tier dispatch policy & outcome metric shape
4. E2E test scope — Go-only vs Go+TS, plus race-clean strategy

## Decisions

### 1. Pre-edit FileFact seam

- **Options presented:**
  - (A) Store accessor `GetLatestFileFact(repoID,path)` reading
    `semantic_overlay_facts` + snapshot fallback **(Recommended)**
  - (B) Handler-local LRU cache populated post-commit
  - (C) Hybrid: LRU in front of Store accessor
- **User selected:** (A) Store accessor
- **Rationale:** Survives daemon restart; single source of truth;
  matches existing Store API surface; cache can be layered later
  under same signature.

### 2. Extractor invocation

- **Options presented:**
  - (A) Synchronous: handler calls `extractor.ExtractFile` inside
    live tx **(Recommended)**
  - (B) Async: read what scheduler eventually writes
  - (C) Hybrid: sync on HelixEdit lane, async on background lane
- **User selected:** (A) Synchronous in-tx
- **Rationale:** Tier-1 needs post-edit FileFact synchronously to
  diff against the pre-edit fact; deterministic test seam; bounded
  tx-span widening.

### 3. Tier dispatch + outcome metric

- **Options presented:**
  - (A) Proposed mapping + single counter `helix_live_filefactdiff_
    total{tier,repo}` **(Recommended)**
  - (B) Same mapping, three separate counters
  - (C) Different mapping — user specifies
- **User selected:** (A) Single counter, proposed mapping confirmed
- **Mapping locked:**
  - Tier-1 (full): prior FileFact found AND `ExtractionStatusReady`
  - Tier-2 (added-only): `ExtractionStatusPartial`
  - Tier-3 (synthetic): prior missing (cold-start) OR `Failed` OR
    `Unsupported`
- **Additional decision:** Tier-3 path also emits a bounded-label
  warn metric distinguishing `cold_start` / `extract_failed` /
  `extract_unsupported`.

### 4. E2E test scope

- **Options presented:**
  - (A) Go-only fixture, `-race` clean, single-symbol-edit assertion
    **(Recommended)**
  - (B) Go + TypeScript fixtures, `-race` clean
  - (C) Go-only + concurrent-edit subtest
- **User selected:** (A) Go-only fixture
- **Rationale:** Smallest viable proof of contract; race-clean by
  construction via 62-09's single-goroutine tx ownership invariant;
  TS coverage deferred unless regression risk emerges.

## Deferred Ideas

- Handler-local LRU cache (defer until profiling justifies it)
- TypeScript E2E fixture (defer; covered by unit recorder tests)
- Hybrid lane sync/async policy (rejected)
- Concurrent same-file edit subtest (covered by existing contract)

## Claude's Discretion

- Exact return shape of `GetLatestFileFact` (raw `FileFact` vs
  store-side `FileFactRow`)
- Diff algorithm internals (`stable_key` / `(src,dst,kind)` matching)
- Metric registration site (alongside `apply_repair` vs new block in
  `difffacts.go`)
- Per-language `ExtractFile` shim implementation (wrap batch
  extractor vs parallel path)
