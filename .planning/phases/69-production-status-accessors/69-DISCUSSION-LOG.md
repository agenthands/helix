# Phase 69 — Discussion Log

**Date:** 2026-05-14
**Mode:** default (interactive, AskUserQuestion)

## Gray areas presented

1. Cluster status data source
2. Retrieval status envelope shape
3. Counters + provenance
4. ClusterStatus additive fields + E2E fixture

User selected: ALL FOUR.

## Q1 — Cluster status data source

**Options:**
- Persisted ClusterSummary rows via new *Store accessor (Recommended)
- In-memory cluster engine snapshot
- Hybrid: engine for "building", store for everything else

**Selected:** Persisted ClusterSummary rows via new *Store accessor.
**Captured as:** D1.

## Q2 — Retrieval status envelope shape

**Options:**
- Nested retrieval_status struct mirroring cluster_status (Recommended)
- Flat fields hoisted onto StatusResult
- Nested struct + retire RetrievalPending into it

**Selected:** Nested retrieval_status struct mirroring cluster_status.
**Captured as:** D2.

## Q3 — Counters + provenance

**Options:**
- bleve as source-of-truth for counts; corpus_version + last_compact_at via bleve meta keys (Recommended)
- *Store as source-of-truth; bleve is derived
- Split: counts from bleve, versions/timestamps from obs metrics

**Selected:** bleve as source-of-truth; meta keys.
**Captured as:** D3.

## Q4 — ClusterStatus additive fields + E2E fixture

**Options:**
- Additive fields + extend Phase 64 P07 fixture (Recommended)
- Additive fields + fresh dedicated fixture
- Aggregate MemberCount only at status time; no fixture extension

**Selected:** Additive fields + extend Phase 64 P07 fixture.
**Captured as:** D4.

## Deferred ideas raised

- Per-projection cluster status block
- Historical cluster status (last-N versions)
- Streaming retrieval-status delta over MCP

All recorded in CONTEXT.md > Deferred Ideas.

## Claude's discretion (not asked)

- Phase 69 is mostly implementation choices grounded in existing seams; no questions about WHAT to build (locked by ROADMAP success criteria + REQUIREMENTS) or scope.
- Did NOT ask about test framework, file layout, or commit structure — planner's call.
- Did NOT ask about "building" state signal mechanism — flagged in CONTEXT.md as a researcher question with two candidate approaches.
