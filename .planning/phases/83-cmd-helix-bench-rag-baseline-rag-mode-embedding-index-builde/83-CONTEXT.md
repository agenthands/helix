# Phase 83: `cmd/helix-bench-rag` + baseline_rag Mode + Embedding-Index Builder - Context

**Gathered:** 2026-06-21
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

The RAG baseline is a competent grep + embedding-RAG control arm, not a strawman — implemented as a standalone `cmd/helix-bench-rag` MCP server with exactly 4 fixed tools (`rag_search`, `rag_read_chunk`, `grep`, `read_file`), backed by `chromem-go` and OpenAI `text-embedding-3-small` (with Ollama `nomic-embed-text` offline fallback). Self-contained; benefits from soaking before public benchmarks land.

**Requirements:** ABLATE-04

**Success Criteria (what must be TRUE):**

1. `cmd/helix-bench-rag --help` works; tool-list returns exactly 4 tools; a vet test asserts no import from `internal/kernel/` or `internal/semantic/` (`baseline_rag` is provably NOT a Helix profile and shares no code with the daemon's tool surface).
2. Per-corpus embedding index is built once per `(corpus, embedder_model)` and cached at `$HELIX_CACHE_DIR/bench-rag-index/<corpus_sha>/`; `bench/runners/baseline_rag_agent/EMBED-CHOICE.md` documents the model pin (OpenAI `text-embedding-3-small` primary, Ollama `nomic-embed-text` fallback) and chunking strategy.
3. A `baseline_rag` ToolBench run on Go produces schema-valid `result.v2.json` rows; the embedder ID is recorded in every row so reviewer pushback on "weak embedder" can be addressed factually.
4. The same-model-same-budget invariant holds: `baseline_rag` agent uses the identical fairness-contract model snapshot and budget as `your_agent_full`; embedding-API calls are NOT charged against the agent's per-task budget (documented in `BENCH.md`).

**Depends on:** Phase 82 (aggregator reports `baseline_rag` rows in the leaderboard), Phase 80 (scaffolding for `baseline_rag` runner exists)

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — discuss phase was skipped per user setting. Use the ROADMAP phase goal, success criteria, and codebase conventions to guide decisions. Honor the fairness contract from Phase 75 and the `baseline_rag` runner scaffolding from Phase 80.

</decisions>

<code_context>
## Existing Code Insights

Codebase context will be gathered during plan-phase research. Key anchors:
- Phase 80 landed `baseline_rag` runner scaffolding (stub) — this phase implements it for real.
- Phase 75 fairness contract (`fairness_contract.go`) supplies the model snapshot + budget invariant.
- Phase 82 aggregator already reports `baseline_rag` rows in the leaderboard.
- `result.v2.json` schema (Phase 75) is the output contract.

</code_context>

<specifics>
## Specific Ideas

No additional requirements beyond ABLATE-04 and the four success criteria above — discuss phase skipped. Refer to the ROADMAP phase description and success criteria.

</specifics>

<deferred>
## Deferred Ideas

None — discuss phase skipped.

</deferred>
