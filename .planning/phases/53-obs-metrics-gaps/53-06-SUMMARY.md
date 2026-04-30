---
phase: 53-obs-metrics-gaps
plan: 06
subsystem: observability-docs
tags: [observability, docs, roadmap, helix, usage]
requires:
  - 53-01..53-05 (all 5 metric families must be live in production code before they are documented)
provides:
  - USAGE.md §"Prometheus Metrics" extended with 5 new rows + 2 PromQL examples + http best-effort caveat
  - ROADMAP.md Phase 53 success-criterion-1 corrected from serena_*_cache_hits_total to helix_*_lookups_total{result} (and the 4 other helix_* names with full label sets)
affects:
  - Phase verification can now check criterion-1 against actual shipped metric names instead of stale serena_* strings
  - Operators have a complete map from /metrics output to meaning + 2 PromQL queries to start with
tech-stack:
  patterns:
    - markdown table extension preserving existing column structure
    - PromQL single-line form chosen so plan-verify grep matches (semantically equivalent to multi-line)
    - blockquote caveat for operator-facing limitations (no Caveats subsection existed; paragraph form fallback)
key-files:
  modified:
    - USAGE.md (+20 lines: 5 table rows + 2 PromQL examples + 1 blockquote caveat)
    - .planning/ROADMAP.md (1 line rewritten: Phase 53 success-criterion-1)
decisions:
  - "USAGE.md PromQL examples use single-line histogram_quantile form (PromQL is whitespace-insensitive). Reason: plan-verify grep is line-based; multi-line form would not match. Semantic identical."
  - "HTTP best-effort caveat rendered as a blockquote paragraph rather than a Caveats subsection — the §Observability section has no existing Caveats / Known limitations subsection, paragraph form was the plan's documented fallback."
  - "ROADMAP.md success-criterion-1 was rewritten in full (not just s/serena/helix/) because the original used a category name (*_cache_hits_total) that does not match the actual carve-out: Plan 01 D-01 chose a single counter per family with result={hit,miss}. Shipped names are *_lookups_total."
  - "One remaining serena_v* reference at ROADMAP.md:165 is a goreleaser archive negative-assertion (Phase 52 verifying NO leftover serena_v* archives) and is correct as-is — left untouched."
  - "Human verify gate: docs and PromQL examples reviewed and approved by user 2026-05-01."
metrics:
  duration: "~5 minutes (plus user verify time)"
  completed: "2026-05-01"
  tasks: 3
  files_modified: 2
  lines_added: 21
---

# Phase 53 Plan 06: USAGE.md + ROADMAP correction Summary

**One-liner:** Closed the documentation half of OBS-03. USAGE.md §"Prometheus Metrics" now documents the 5 new `helix_*` metric families landed in plans 53-01..53-05, with 2 PromQL examples (hit-ratio + per-extractor p95 latency) and a blockquote caveat for the http `ended` best-effort semantic. ROADMAP.md Phase 53 success-criterion-1 corrected from stale `serena_*_cache_hits_total` strings to the actual `helix_*` shipped names with full label sets.

## What Landed

### Task 6.1 — USAGE.md (commit `fd04a61d`)

Five rows added to the existing Prometheus Metrics table at lines 649-654, preserving the table's `Metric | Type | Description` column structure:

| Metric | Description summary |
|---|---|
| `helix_lspool_lookups_total{language, result}` | hit (warm worker reuse) vs miss (spawn) at AcquireLease share/spawn boundary |
| `helix_repomap_lookups_total{language, result}` | hit (mtime match) vs miss (extractor invoked) in TagCache.GetOrExtract |
| `helix_repomap_extract_duration_seconds{language, extractor}` | extractor latency, cache-miss path only — `extractor` ∈ `{treesitter, lsp, fallback}`, custom buckets 1ms→2.5s |
| `helix_session_lifecycle_total{phase, transport}` | session phase transitions — `phase` ∈ `{started, ended, error}`, `transport` ∈ `{stdio, http}`; http `ended` is best-effort |
| `helix_edit_outcome_total{tool_name, outcome, strategy}` | 7 edit/fileops handler outcomes — `outcome` ∈ 6-value enum, `strategy` ∈ 4-value enum (`failed` is NOT emitted as strategy) |

Two PromQL examples appended after the existing `Eviction rate by reason` block:

1. **lspool hit-ratio** (Phase 53 D-01): `sum(rate(helix_lspool_lookups_total{result="hit"}[5m])) / sum(rate(helix_lspool_lookups_total[5m]))`
2. **per-extractor p95 RepoMap latency** (Phase 53 D-05/D-06): `histogram_quantile(0.95, sum by (le, extractor) (rate(helix_repomap_extract_duration_seconds_bucket[5m])))`

HTTP best-effort caveat appended as a blockquote: clarifies that `helix_session_lifecycle_total{transport="http", phase="ended"}` only fires on a client-issued `DELETE /mcp`; SDK-internal timeouts are NOT counted. Recommends `started − error` as a fallback for HTTP, notes stdio is reliable.

### Task 6.2 — ROADMAP.md success-criterion-1 (commit `2d0a12a9`)

Single-line edit at `.planning/ROADMAP.md:189`. Old text referenced 5 stale `serena_*_cache_hits_total` etc. strings without label sets. New text lists the 5 actual shipped names with full label sets:

- `helix_lspool_lookups_total{language, result}` (counter; `result` ∈ {hit, miss})
- `helix_repomap_lookups_total{language, result}` (counter)
- `helix_repomap_extract_duration_seconds{language, extractor}` (histogram)
- `helix_session_lifecycle_total{phase, transport}` (counter)
- `helix_edit_outcome_total{tool_name, outcome, strategy}` (counter)

Goal line, Requirements, Sizing, Plans, and criteria 2/3/4 were untouched. Verification: `grep serena_ .planning/ROADMAP.md` returns exactly one remaining match at line 165, which is a goreleaser archive negative-assertion (Phase 52 verifying NO leftover `serena_v*` archives) — correct as-is.

### Task 6.3 — Human verify gate

User approved 2026-05-01 after reviewing diffs of both commits inline. No issues raised against rendering, PromQL formatting, or ROADMAP wording.

## Verification

- `grep -c 'helix_' USAGE.md` shows the 5 new families present
- `grep -c 'helix_' .planning/ROADMAP.md` shows criterion-1 lists all 5
- `grep -c 'serena_' .planning/ROADMAP.md` returns 1 (the negative-assertion at line 165, intentionally retained)
- Both PromQL examples are single-line and grep-detectable per the plan's verify check

## Self-Check: PASSED

All 3 tasks complete (2 autonomous + 1 human-verify gate). Both edits are committed atomically and behind by precisely one merge commit when this SUMMARY's commit lands. ROADMAP.md tracking-row edits (plan-progress checkbox flips, percent updates) remain owned by the orchestrator and will be reconciled after merge — they are NOT part of this plan's commits.

## Cross-Phase Effects

- **Phase 53 verification** (gsd-verifier) can now check criterion-1 by grepping for the 5 helix_* names directly
- **Future Phase 54** (OBS-04 dashboards/runbooks) will reference the metric names documented here — USAGE.md is the authoritative single source
- **Phase 52 brand rename** is now consistent end-to-end: code, docs, and roadmap all use `helix_*`. The single remaining `serena_v*` archive name in ROADMAP.md is a Phase 52 verification artifact, not a brand drift
