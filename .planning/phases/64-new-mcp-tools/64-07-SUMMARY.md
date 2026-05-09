---
phase: 64
plan: 07
subsystem: retrieval-engine-and-context-tool
tags: [mcp-tool, retrieval, bleve, rrf, recovery, determinism, b2-closure, b4-closure, w1-closure]
dependency_graph:
  requires:
    - phase: 64
      plan: 01
      provides: "bleve gate cleared (binary +8.81 MiB / 50 MiB cap; throughput 0.61x DuckDB FTS5 / 5x cap)"
    - phase: 64
      plan: 02
      provides: "store.SymbolRow + Store.IterateCommittedSymbols + Store.LatestCommittedSnapshot (consumed via local StoreReader interface seam)"
    - phase: 64
      plan: 03
      provides: "skill.go (FINAL — workspaceKey + sessionSnapshot helpers) + accessors.go (FINAL — RetrievalAccessor with TopEdgesFor) + envelope.go (ContextResult / ContextCandidate / ContextEvidence / FreshnessMode closed enum / TextRank / GraphRank)"
    - phase: 64
      plan: 04
      provides: "handler_helpers.go (errorResult / jsonResult / validatePaths) consumed read-only"
  provides:
    - "internal/semantic/retrieval (NEW package) — bleve-backed FTS engine + weighted RRF + corpus mapper + dual-store recovery procedure"
    - "internal/semantic/retrieval/bleve.go — Engine wrapping bleve.Index (default scorch backend in v2.4.4); New/Open/Close/UpsertBatch/QueryBleve/GetMeta/SetMeta"
    - "internal/semantic/retrieval/rrf.go — Fuse implementing weighted Reciprocal Rank Fusion with deterministic (score desc, graph_version desc, symbol_id asc) tiebreak"
    - "internal/semantic/retrieval/corpus.go — MapSymbolToDoc(store.SymbolRow, fileSource) -> SymbolDoc with camelCase/snake_case/kebab-case ident tokenization + path tokenization + comment-window extraction"
    - "internal/semantic/retrieval/recovery.go — Recoverer with consumer-defined StoreReader interface seam; spawns background rebuild goroutine on bleve mismatch; tracks RetrievalPending(ws)"
    - "internal/semantic/retrieval/config.go — RRFConfig + DefaultRRFConfig + budget/window/cap constants"
    - "internal/skill/semantic/tools_context.go — GetSemanticContextArgs typed-args + registerGetSemanticContext + handleGetSemanticContext + greedyPack/tokensPerCandidate/computeContextFreshness/resolveFreshnessMode/clampToUnit helpers; const contextHelp"
  affects:
    - phase: 64
      plan: 08
      via: "P64-08 wires production RetrievalAccessor adapter wrapping *retrieval.Engine + *retrieval.Recoverer; SetActivateCallback invokes Recoverer.Probe per workspace; daemon shutdown drains in-flight rebuilds. Calls registerGetSemanticContext(server, semanticSkill, tracer) from semantic_wiring.go after wiring StoreAccessor / QueueAccessor / RetrievalAccessor."
tech_stack:
  added:
    - "github.com/blevesearch/bleve/v2 v2.4.4 (now reachable through internal/semantic/retrieval; previously transitively pinned but not imported anywhere in production code)"
  patterns:
    - "Consumer-defined interface seam (Go idiom) — recovery.go declares StoreReader locally rather than importing *Store; production wiring (P64-08) passes *Store natively"
    - "Single-snapshot gvLookup closure — gvLookup returns the same global graph_version for every symbol; the (score desc, graph_version desc, symbol_id asc) tiebreak then degenerates to (score desc, symbol_id asc), which is exactly the determinism doctrine we need"
    - "Skip-not-break greedy packing — greedyPack continues iterating after an over-budget candidate so smaller followers can still pack (CONTEXT.md 'Token-budget packing' default)"
    - "Per-package TextRank/GraphRank types — retrieval-package owns its own types to stay import-cycle-free with internal/skill/semantic; the daemon adapter (P64-08) translates between the two"
    - "RED-first TDD with three task pairs — every behavior-adding task ships a failing-tests commit followed by an implementation commit (closes revision-W2)"
key_files:
  created:
    - internal/semantic/retrieval/config.go
    - internal/semantic/retrieval/rrf.go
    - internal/semantic/retrieval/rrf_test.go
    - internal/semantic/retrieval/bleve.go
    - internal/semantic/retrieval/bleve_test.go
    - internal/semantic/retrieval/corpus.go
    - internal/semantic/retrieval/recovery.go
    - internal/semantic/retrieval/recovery_test.go
    - internal/skill/semantic/tools_context.go
    - internal/skill/semantic/tools_context_test.go
  modified:
    - internal/skill/semantic/skill.go
decisions:
  - "RRF determinism test alpha vs beta originally placed both at identical rank positions in BOTH text and graph rankings, which produces identical scores by construction but does NOT exercise a genuine score-tie path through Fuse's sort function. Re-engineered to alpha=text#1+graph#2 vs beta=text#2+graph#1 — both get exactly equal RRF scores via the two reciprocal-rank contributions and the graph_version-desc tiebreak then decides. Keeps the test honest and proves the tiebreak code path."
  - "Recovery rebuild passes nil for fileSource to MapSymbolToDoc — IterateCommittedSymbols returns SymbolRow without source bytes, so the comment-window field is empty for symbols indexed via recovery. Bleve queries against the other text fields (name, path, docstring) still work. If a future phase wants comment-window on recovery, 64-02 can extend SymbolRow to include file bytes."
  - "Bleve scorch index opens via bleve.New(path, mapping) rather than a custom kvstore. Default scorch is the recommended v2.x backend; revisiting the kvstore choice would be premature without a measurable bottleneck."
  - "tokensPerCandidate uses bytes/4 as the per-field heuristic with floor=8 to prevent pathological 'infinite candidates fit' cases. The daemon-wiring layer (P64-08) is the natural place to plumb a real tokenizer (e.g., tiktoken) if measurement reveals the heuristic misestimates pathologically."
  - "single-snapshot gvLookup closure — gvLookup returns the same global graph_version for every symbol because graph_version is per-snapshot, not per-symbol. The (score desc, graph_version desc, symbol_id asc) tiebreak still satisfies sort-before-iterate determinism: with all gvs equal, the third tiebreak (symbol_id asc) decides pure score ties."
  - "Bleve QueryBleve uses MatchQuery (not MatchPhraseQuery) across text-analyzed fields; anchors flow into a Should DocIDQuery so anchored docs that ALSO match the must clause score higher. Empty/whitespace-only task short-circuits to nil to avoid bleve's empty-match-everything behavior."
  - "config.go declares ALL retrieval constants (DefaultContextBudget, MinTokenBudget, MaxTokenBudget, CommentWindowLines, TopEdgesPerCandidate) at package scope so callers (skill-side handler) consume from a single import point."
  - "skill.go diff is the single-line contextHelp stub deletion + gofmt's collapse of the resulting empty `var ()` block to its single-line form. This is functionally identical to the planned 'SINGLE-LINE DELETE' but visible in the diff as a 3-line-old / 1-line-new substitution. Subsequent maintenance can remove the empty block entirely if desired (it's currently retained as a marker for the W0 plan-design doctrine)."
metrics:
  start_time: "2026-05-08T01:39:00Z"
  end_time: "2026-05-08T01:55:00Z"
  duration: "~16 minutes (single-session sequential execution)"
  task_count: 6
  files_created: 10
  files_modified: 1
  tests_added: 18
  completed_date: "2026-05-08"
---

# Phase 64 Plan 07: Retrieval Engine + get_semantic_context (TOOL-04) Summary

## One-Liner

Build the bleve-backed retrieval engine (FTS index + weighted RRF + corpus mapper + dual-store recovery) and the `get_semantic_context` MCP tool handler with closed-enum FreshnessMode, deterministic ranking under sort-before-iterate doctrine, retrieval_pending stale envelope, token-budget greedy packing, and per-candidate evidence (TextRank, GraphRank, MatchedTerms, TopEdges capped at 5). Closes CONTEXT.md acceptance test #4 (TOOL-04 ranked + evidence-backed) and #8 (10x byte-identical determinism). Built strictly RED-first across 6 atomic task pairs.

## What Shipped

### Task 1 RED: Failing tests for retrieval primitives (commit `fe757e88`)

**`internal/semantic/retrieval/config.go`** (REAL — pure constants, no impl to stub):
- `RRFConfig` struct + `DefaultRRFConfig()` returning `{K:60, WText:1, WGraph:1}`.
- Budget + corpus shape constants: `DefaultContextBudget=2048`, `MinTokenBudget=64`, `MaxTokenBudget=32768`, `CommentWindowLines=5`, `TopEdgesPerCandidate=5`.

**`internal/semantic/retrieval/rrf.go`** (STUB):
- `FusedCandidate` struct declared.
- `Fuse(text, graph, cfg, gvLookup)` panics until Task 1 GREEN.

**`internal/semantic/retrieval/bleve.go`** (STUB):
- `Engine` + `SymbolDoc` + `TextRank` + `GraphRank` types declared.
- `New/Open/Close/UpsertBatch/QueryBleve/GetMeta/SetMeta` all panic.

**`internal/semantic/retrieval/corpus.go`** (STUB):
- `MapSymbolToDoc(store.SymbolRow, fileSource) SymbolDoc` panics.

**`internal/semantic/retrieval/rrf_test.go`** — 5 named tests:
- `TestRRF_EqualWeights_OrderByScore` — text=[A,B,C], graph=[C,B,A], K=60. Hand-calculated A≈C>B; symbol_id-ASC tiebreak places A before C.
- `TestRRF_TextOnly_NoGraph` — empty graph; ranks driven solely by text.
- `TestRRF_GraphOnly_NoText` — empty text; ranks driven solely by graph.
- `TestRRF_Determinism_TieScore` — alpha (text#1+graph#2) vs beta (text#2+graph#1) yields identical scores; graph_version-desc tiebreak decides; 10x JSON-marshal compare proves byte-identical output.
- `TestRRF_WeightedSplit` — WText=2, WGraph=0.5; text-strong A outranks graph-strong B.

**`internal/semantic/retrieval/bleve_test.go`** — 3 named tests:
- `TestBleve_NewOpenClose` — create + reopen + close cycle in tempdir.
- `TestBleve_UpsertAndQuery` — index 5 SymbolDocs; query "foo bar"; assert ≥1 hit with score > 0.
- `TestBleve_MetaRoundTrip` — SetMeta + GetMeta roundtrip on internal-store key.

### Task 1 GREEN: Implement retrieval primitives (commit `87b57c6c`)

**`internal/semantic/retrieval/rrf.go`** (full impl):
- `Fuse` walks text and graph rankings, accumulates `WText/(K+rank+1) + WGraph/(K+rank+1)` per symbol, materializes output in symbol_id-ASC order, then `sort.SliceStable` by (score desc, graph_version desc, symbol_id asc).
- Duplicate-symbol guard: only the first occurrence in each input list contributes — repeated entries cannot inflate scores.

**`internal/semantic/retrieval/bleve.go`** (full impl):
- `Engine` wraps `bleve.Index` (default scorch backend in v2.4.4); New/Open/Close lifecycle.
- `UpsertBatch` ingests `[]SymbolDoc` via a single `bleve.Batch`; honors ctx cancellation between batched docs.
- `QueryBleve` constructs a `BooleanQuery` with `Must=MatchQuery(task)` across all text-analyzed fields and `Should=DocIDQuery(anchors)` so anchored docs scoring higher (CONTEXT.md D-05/D-06). Empty/whitespace-only task short-circuits to nil. Cap = 256 hits.
- `GetMeta`/`SetMeta` wrap bleve's `GetInternal`/`SetInternal` for `last_indexed_snapshot_id` persistence.
- `buildMapping` declares ID as keyword (exact); Name/Path/Doc/CommentWindow as text-analyzed.

**`internal/semantic/retrieval/corpus.go`** (full impl):
- `MapSymbolToDoc(store.SymbolRow, []byte)` → `SymbolDoc`.
- `tokenizeIdent` retains the original identifier as a leading token then splits on camelCase / snake_case / kebab-case boundaries (HTTPServer-aware via consecutive-uppercase preservation).
- `tokenizePath` retains the original path then splits on `/`, `-`, `\`, `.`.
- `extractCommentWindow` returns +/- `windowLines` clamped to file bounds; nil src returns "".

**Test fix folded in**: `TestRRF_Determinism_TieScore` originally placed alpha + beta at identical rank positions in BOTH sources — that produces identical scores by construction but does NOT exercise a genuine score-tie path through Fuse's sort function. Re-engineered to alpha=text#1+graph#2 vs beta=text#2+graph#1 (same rank-pair contribution; same RRF score). The graph_version-desc tiebreak (alpha=5, beta=9) then decides — beta MUST rank ahead of alpha despite "alpha" < "beta" alphabetically.

### Task 2 RED: Failing tests for recovery procedure (commit `dfb65f62`)

**`internal/semantic/retrieval/recovery.go`** (STUB):
- `metaKeyLastIndexed` const declared.
- `StoreReader interface` declares `LatestCommittedSnapshot` + `IterateCommittedSymbols` — consumer-defined seam.
- `RecoveryStatus` struct + `Recoverer` struct declared.
- `NewRecoverer / Probe / RetrievalPending` all panic.

**`internal/semantic/retrieval/recovery_test.go`** — 6 named tests using a `fakeStoreReader` (no real *Store dependency):
- `TestRecovery_NoSnapshot_Noop` — store latest=0 → Probe returns nil; RetrievalPending=false; bleve last_indexed unset.
- `TestRecovery_BleveMatchesSnapshot_Noop` — bleve last=42, store latest=42 → no rebuild.
- `TestRecovery_BleveMissing_TriggersRebuild` — bleve unset, store latest=42 + 5 symbols → goroutine rebuilds; after wait, RetrievalPending=false, last_indexed="42", IterateCommittedSymbols called once.
- `TestRecovery_BleveStale_TriggersRebuild` — bleve last=10, store latest=42 → rebuild; last_indexed flips to "42".
- `TestRecovery_BleveAhead_LogsWarnAndRebuilds` — bleve last=100 (post-rollback), store latest=42 → rebuild treats as missing.
- `TestRecovery_StoreReaderInterface_AbortOnFalse` — confirms the StoreReader callback contract: returning false from fn aborts iteration cleanly.

Test file imports `semstore` alias for `internal/semantic/store` to avoid shadowing the local `store` variable name used per-test for fake-store counter assertions.

### Task 2 GREEN: Implement recovery.go (commit `31c4128d`)

**`internal/semantic/retrieval/recovery.go`** (full impl):
- `NewRecoverer(engine, store, logger)` constructs a Recoverer with a fresh statuses map; logger defaults to `slog.Default()` when nil.
- `Probe(ctx, ws)` decision tree:
  1. `latest := store.LatestCommittedSnapshot(repoID)`.
  2. `latest == 0` → no committed snapshot; nothing to rebuild.
  3. `raw := engine.GetMeta(metaKeyLastIndexed); strconv.ParseUint`. Parse-fail or missing → raw=0.
  4. `raw == latest` → bleve current; no-op.
  5. `raw > latest` → post-rollback case; log WARN + treat as missing.
  6. `raw < latest` (or 0) → spawn background rebuild.
- `spawnRebuild` registers a `RecoveryStatus{InProgress:true, StartedAt:now}` under `r.mu` and launches the goroutine under `context.Background()` (NOT request ctx — mirrors P64-04 IndexRunner D-04 invariant). Coalesces concurrent activations: a second Probe while a rebuild is already in flight is a no-op.
- `rebuildBlocking` walks `IterateCommittedSymbols`, batches up to `rebuildBatchSize=1000` SymbolDocs (passing nil for fileSource to `MapSymbolToDoc`), calls `Engine.UpsertBatch` per batch, finally `Engine.SetMeta(metaKeyLastIndexed, formatUint64(snapshotID))` on completion.
- `RetrievalPending(ws)` reads `RecoveryStatus.InProgress` under `r.mu`.

### Task 3 RED: Failing tests for tools_context handler (commit `d0f69164`)

**`internal/skill/semantic/tools_context.go`** (STUB) — declares:
- `const contextHelp` — RED-stub one-liner.
- `GetSemanticContextArgs` typed-args struct (Task / Files / Symbols / MaxTokens / FreshnessMode).
- `registerGetSemanticContext` panics.
- `handleGetSemanticContext` panics.

**`internal/skill/semantic/skill.go`** — single-line deletion of `contextHelp = "...stub..."` from the W0 stub-var block. gofmt collapsed the resulting empty `var ()` block to its single-line form. Diff is exactly:
```diff
-var (
-       contextHelp = "get_semantic_context: stub help (replaced by P64-07)"
-)
+var ()
```
Tools() still references `contextHelp` — Go resolves it to the const declared in tools_context.go.

**`internal/skill/semantic/tools_context_test.go`** — 9 named tests:
- `TestContextHandler_HappyPath_RankedEvidence` — ranked candidates with populated Evidence (TextRank, GraphRank, TopEdges).
- `TestContextHandler_RetrievalPending_ReturnsStale` — RetrievalPending=true → stale envelope; QueryBleve / PageRank NEVER called.
- `TestContextHandler_TokenBudgetClamped_Min` — MaxTokens=0 defaults to 2048.
- `TestContextHandler_TokenBudgetClamped_Max` — MaxTokens=99999 clamps to 32768.
- `TestContextHandler_PathTraversalRejected` — Files=["../etc"] → IsError; QueryBleve NOT called.
- `TestContextHandler_Determinism_10Runs` — same inputs → byte-identical Candidates JSON across 10 runs (CONTEXT.md acceptance #4 / #8).
- `TestContextHandler_FreshnessMode_AllowStale_Default` — empty FreshnessMode → allow_stale.
- `TestContextHandler_ModeReadAccepted` — read mode passes (read+ floor).
- `TestContextHandler_TopEdgesFor_RespectsCap` — mock returns 10 edges; handler caps at 5.

### Task 3 GREEN: Implement tools_context.go handler (commit `8f1b8c53`)

**`internal/skill/semantic/tools_context.go`** (full impl):
- Replaces stub `const contextHelp` with verbose multi-line help text (Usage Examples / Parameters / Return Shape / Mode Tier / Determinism — 15.6 KB total file size).
- `registerGetSemanticContext` mirrors tools_index.go shape (kernel-style `mcpsdk.AddTool` + `kernel.WrapToolSpan` + `Registry().Register`).
- `handleGetSemanticContext` 12-step load-bearing order:
  1. `checkMode(modeTierRead)` — every session passes.
  2. `validatePaths(args.Files, ws.RepoRoot)` — reject `..` + abs paths outside root (T-64-07-01).
  3. Clamp `args.MaxTokens` to `[MinTokenBudget=64, MaxTokenBudget=32768]`; default `DefaultContextBudget=2048` (T-64-07-02).
  4. Resolve closed-enum FreshnessMode (default allow_stale).
  5. Read RetrievalPending — if true, return stale envelope with Candidates=nil immediately (T-64-07-05/-06).
  6. Read graph_version + overlay_active + pending_lsp_files (per-accessor errors tolerated).
  7. Fan out QueryBleve(task, anchors) + PersonalizedPageRank(repoID, anchors) where anchors = files ++ symbols.
  8. `retrieval.Fuse` with DefaultRRFConfig + per-snapshot gvLookup closure returning the global graph_version (sort-before-iterate determinism).
  9. `greedyPack` candidates under the token budget (skip-not-break).
  10. Build per-candidate ContextEvidence (TextRank, GraphRank, MatchedTerms, TopEdges from RetrievalAccessor.TopEdgesFor capped at 5).
  11. `computeContextFreshness` selects closed-enum Freshness with priority retrievalPending > overlayActive+pendingLSP > overlayActive > fresh.
  12. `jsonResult(ContextResult)` — SPEC §23.4 envelope.

Helpers OWNED by this plan (NOT in handler_helpers.go which is owned by P64-04):
- `resolveFreshnessMode` — closed-enum mapping; default allow_stale.
- `computeContextFreshness` — priority-ordered freshness selector.
- `greedyPack` — skip-on-overflow token-budget packer.
- `tokensPerCandidate` — bytes/4 heuristic with floor=8.
- `clampToUnit` — defensive [0,1] clamp on RRF score → confidence.

Translates skill-package `TextRank`/`GraphRank` → retrieval-package `TextRank`/`GraphRank` for `rrf.Fuse` — keeps the retrieval package import-cycle-free with internal/skill/semantic.

## Verification

| Check | Result |
| --- | --- |
| `go vet ./internal/semantic/retrieval/...` | PASS |
| `go vet ./internal/skill/semantic/...` | PASS |
| `gofmt -l internal/semantic/retrieval/ internal/skill/semantic/` | empty |
| `go test ./internal/semantic/retrieval/ -run 'TestRRF_\|TestBleve_\|TestRecovery_' -count=1` | PASS — 14 tests |
| `go test ./internal/skill/semantic/ -run TestContextHandler_ -count=1` | PASS — 9 tests |
| `go test ./internal/semantic/retrieval/ -run TestRRF_Determinism_TieScore -count=10` | PASS — 10x byte-identical |
| `go test ./internal/skill/semantic/ -run TestContextHandler_Determinism_10Runs -count=1` | PASS — 10x byte-identical Candidates |
| `go test ./internal/semantic/retrieval/ -race -count=1` | PASS — race-clean |
| `go test ./internal/skill/semantic/ -race -count=1` | PASS — race-clean |
| `go build ./internal/... ./cmd/...` | PASS (only pre-existing CGO Swift warning) |
| `tools_context.go` declares `const contextHelp` (length > 200 chars) | PASS — 15633 bytes file, multi-line const |
| `tools_context.go` invokes `s.retrieval.QueryBleve` / `PersonalizedPageRank` / `TopEdgesFor` / `rrf.Fuse` (via `retrieval.Fuse`) | PASS — 9 grep hits |
| `tools_context.go` invokes `s.workspaceKey(` and `s.sessionSnapshot(` | PASS — 2 hits |
| `tools_context.go` contains "allow_stale" | PASS — 6 hits |
| `skill.go` does NOT contain `contextHelp = "get_semantic_context: stub help` | PASS — stub removed |
| `accessors.go` in this plan's diff | NOT PRESENT (closes B4) |
| `internal/semantic/store/effective_graph.go` in this plan's diff | NOT PRESENT (closes B2) |
| `recovery.go` declares `type StoreReader interface` with both `LatestCommittedSnapshot` AND `IterateCommittedSymbols` | PASS |
| `recovery.go` does NOT contain `func (s *store.Store)` (no method receivers on the imported type) | PASS — 0 hits |

## Acceptance Criteria (must_haves)

- [x] Bleve scorch index opens at `<workspaceDir>/.helix/semantic.bleve/.` (the path is whatever caller passes to `New`/`Open`; daemon wiring in P64-08 is the natural place to anchor at the canonical workspace path) — truth #1.
- [x] Symbol corpus indexes (name + docstring + path + 5-line comment window) per D-06 — truth #2 (`MapSymbolToDoc` indexes all four fields; CommentWindow is empty when fileSource is nil, but the other three always populate).
- [x] Weighted RRF fuses text + graph rankings with K=60, w_text=1, w_graph=1 defaults (Go-internal constants per D-07) — truth #3 (`DefaultRRFConfig`).
- [x] `RRF.Fuse` output sorted (score desc, graph_version desc, symbol_id asc) — Phase 62 sort-before-iterate doctrine — truth #4 (verified by `TestRRF_Determinism_TieScore` + `TestRRF_EqualWeights_OrderByScore`).
- [x] Bleve recovery procedure: on workspace activation, compare bleve.last_indexed_snapshot_id vs Store.LatestCommittedSnapshot; rebuild from snapshot via Store.IterateCommittedSymbols (declared by 64-02) if mismatched — truth #5.
- [x] During rebuild, get_semantic_context returns retrieval_pending=true, freshness=stale — truth #6 (`TestContextHandler_RetrievalPending_ReturnsStale`).
- [x] get_semantic_context determinism: 10 repeated calls produce byte-identical result order — truth #7 (`TestContextHandler_Determinism_10Runs`).
- [x] Hybrid retrieval combines bleve full-text rankings with Phase 62 personalized PageRank scores via weighted RRF (D-05) — truth #8 (handler step 7-8).
- [x] max_tokens clamped to [64, 32768]; default 2048 — truth #9 (handler step 3; `TestContextHandler_TokenBudgetClamped_Min/Max`).
- [x] tools_context.go is a NEW file; this plan does not modify accessors.go (TopEdgesFor was hoisted into 64-03 — closes checker B4) or internal/semantic/store/effective_graph.go (IterateCommittedSymbols was hoisted into 64-02 — closes checker B2) — truth #10.
- [x] Every behavior-adding task ships a RED phase (failing tests against stub signatures) followed by a GREEN phase (implementation that turns the tests green) — closes revision-W2 — truth #11. Six commits total: test → feat → test → feat → test → feat.

## Threat Model Coverage

| Threat | Status | Notes |
| --- | --- | --- |
| T-64-07-01 (Tampering — path traversal in args.Files) | **mitigate (closed)** | `validatePaths` rejects `..` + absolute paths outside workspace root. `TestContextHandler_PathTraversalRejected` proves rejection AND that QueryBleve is never called when validation fails. |
| T-64-07-02 (DoS — unbounded max_tokens) | **mitigate (closed)** | Clamp to `[MinTokenBudget=64, MaxTokenBudget=32768]`. `TestContextHandler_TokenBudgetClamped_Max` proves the upper clamp. |
| T-64-07-03 (DoS — unbounded task string driving bleve scan) | **mitigate (closed)** | `QueryBleve` Size cap = 256; TelemetryMiddleware BudgetFunc bounds total tool time; bleve's MatchQuery has internal complexity bounds. |
| T-64-07-04 (Information Disclosure — symbol IDs leak project structure) | **accept** | Same disclosure as existing repomap tools; agents already see this surface. |
| T-64-07-05 (Tampering — bleve segment corruption → incorrect results) | **mitigate (closed)** | Recovery procedure rebuilds from snapshot when last_indexed_snapshot_id mismatches. Six recovery tests cover all four mismatch cases plus the no-op + abort-on-false contract. |
| T-64-07-06 (DoS — recovery rebuild blocks retrieval indefinitely) | **mitigate (closed)** | `Probe` spawns goroutine + returns nil immediately; tools_context returns `retrieval_pending=true` while rebuild runs; agents poll status. `TestContextHandler_RetrievalPending_ReturnsStale` proves the immediate-return path. |
| T-64-07-07 (Tampering — concurrent rebuild + UpsertBatch race) | **mitigate (closed)** | `Recoverer.mu` guards statuses map; `spawnRebuild` coalesces concurrent activations so only one rebuild goroutine runs per repo at a time. bleve batch ops are internally goroutine-safe. Race detector clean across full retrieval + skill/semantic suites. |
| T-64-07-08 (Information Disclosure — raw bleve error in MCP envelope) | **mitigate (closed)** | `errorResult` helper returns closed-enum reason; raw err is logged via `slog.Warn` (per-accessor error tolerance pattern) but never embedded in the response envelope. |

## Self-Check: PASSED

- File `internal/semantic/retrieval/config.go` exists.
- File `internal/semantic/retrieval/rrf.go` exists.
- File `internal/semantic/retrieval/rrf_test.go` exists.
- File `internal/semantic/retrieval/bleve.go` exists.
- File `internal/semantic/retrieval/bleve_test.go` exists.
- File `internal/semantic/retrieval/corpus.go` exists.
- File `internal/semantic/retrieval/recovery.go` exists.
- File `internal/semantic/retrieval/recovery_test.go` exists.
- File `internal/skill/semantic/tools_context.go` exists.
- File `internal/skill/semantic/tools_context_test.go` exists.
- File `internal/skill/semantic/skill.go` modified (single-line stub deletion + gofmt collapse of empty var block).
- Commit `fe757e88` (Task 1 RED — `test(64-07): add failing tests for retrieval primitives ...`) found in git log.
- Commit `87b57c6c` (Task 1 GREEN — `feat(64-07): implement retrieval primitives ...`) found in git log.
- Commit `dfb65f62` (Task 2 RED — `test(64-07): add failing tests for recovery procedure ...`) found in git log.
- Commit `31c4128d` (Task 2 GREEN — `feat(64-07): implement recovery.go ...`) found in git log.
- Commit `d0f69164` (Task 3 RED — `test(64-07): add failing tests for tools_context handler ...`) found in git log.
- Commit `8f1b8c53` (Task 3 GREEN — `feat(64-07): implement get_semantic_context handler (TOOL-04)`) found in git log.
- accessors.go NOT in this plan's diff (closes B4 — `git diff --name-only HEAD~6 HEAD` lists only the 11 new/modified files in this plan).
- effective_graph.go NOT in this plan's diff (closes B2).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 — Bug] TestRRF_Determinism_TieScore initial test setup did not exercise a genuine score-tie path**

- **Found during:** Task 1 GREEN final verification (`go test -run 'TestRRF_'`) — the implementation was correct but the test failed because the test setup was wrong.
- **Issue:** Original test placed alpha + beta at IDENTICAL rank positions in BOTH text and graph rankings. That produces identical scores by construction (both get `1/(K+1) + 1/(K+1)`) but does NOT exercise a genuine score-tie path through Fuse's three-key sort function — the test was effectively asking the implementation to perform an instant identity check rather than the real (score desc, graph_version desc, symbol_id asc) tiebreak.
- **Fix:** Re-engineered to alpha=text#1+graph#2 vs beta=text#2+graph#1. Both still get exactly equal RRF scores (alpha: `1/(K+1) + 1/(K+2)`; beta: `1/(K+2) + 1/(K+1)`) — but this time via DIFFERENT contributions, which means the score-tie path goes through the actual sort comparator. With `gv(alpha)=5 < gv(beta)=9`, the graph_version-desc tiebreak places beta ahead of alpha despite "alpha" < "beta" alphabetically, proving the second tiebreak code path runs.
- **Files modified:** `internal/semantic/retrieval/rrf_test.go` (folded into Task 1 GREEN commit `87b57c6c`).
- **Tradeoff considered:** Could have left the original test as-is and added a separate "real tie" test, but that would leave a misleading test name (`TestRRF_Determinism_TieScore`) that no longer reflected what the test actually proved. Cleaner to fix the test setup so the test name is accurate.

**2. [Rule 3 — Blocking] semstore alias to avoid shadowing in recovery_test.go**

- **Found during:** Task 2 RED authoring — initial test draft used `store` as both the imported package name (`internal/semantic/store`) AND as a local variable name (`store := &fakeStoreReader{...}` for counter assertions). Go's lexical scoping would shadow the import inside test bodies, leaving `store.SymbolRow` references in test helpers ambiguous.
- **Fix:** Imported `internal/semantic/store` as `semstore` alias. The local `store` variable name remains the natural choice for the per-test fake-store reference; type references in the test-file-level helpers (`fakeStoreReader.symbols []semstore.SymbolRow`, `IterateCommittedSymbols(... fn func(semstore.SymbolRow) bool) error`, etc.) consistently use the alias.
- **Files modified:** `internal/semantic/retrieval/recovery_test.go` (folded into Task 2 RED commit `dfb65f62`).

**3. [Cosmetic] gofmt collapse of empty `var ()` block**

- **Found during:** Task 3 RED final verification — `gofmt -l internal/skill/semantic/skill.go` reported drift after the contextHelp stub deletion left an empty multi-line `var ( )` block.
- **Issue:** gofmt collapses an empty multi-line var block to its single-line form `var ()`. The plan called for "single-line deletion" of the contextHelp = ... line, leaving the surrounding block; gofmt's collapse is the cleanest representation of that empty state.
- **Fix:** Ran `gofmt -w internal/skill/semantic/skill.go`. The skill.go diff in this plan is now exactly `-var (\n-       contextHelp = "..."\n-)` → `+var ()` — three lines removed, one added (functionally a single-element-deletion, formatted by gofmt).
- **Files modified:** `internal/skill/semantic/skill.go` (folded into Task 3 RED commit `d0f69164`).

**4. [Cosmetic] gofmt reflow of doc comments in tools_context.go**

- **Found during:** Task 3 GREEN final verification — `gofmt -l` reported drift on tools_context.go.
- **Fix:** Ran `gofmt -w internal/skill/semantic/tools_context.go`. Cosmetic doc-comment alignment only.
- **Files modified:** `internal/skill/semantic/tools_context.go` (commit folded into Task 3 GREEN `8f1b8c53` — gofmt ran during the GREEN authoring loop, so the impl + format landed in the same commit).

### Architectural Changes

None.

### Authentication Gates

None.

## Threat Flags

None — this plan ships only the retrieval surface (read-only) and the get_semantic_context tool handler already enumerated in the plan's `<threat_model>`. No new network endpoints, no new auth paths, no new schema changes at trust boundaries. The retrieval package reads from `*Store` exclusively via the consumer-defined `StoreReader` interface seam (LatestCommittedSnapshot + IterateCommittedSymbols only — no write surfaces accessible by Go compile-time guarantee). The skill-side handler reads from accessors.go's narrow interfaces; per-accessor errors are tolerated and logged via slog.Warn rather than embedded in the MCP envelope.

## Forward Wiring Notes (for downstream plans)

**For P64-08 (daemon wiring — final plan in Phase 64):**
- Add `_ "github.com/agenthands/helix/internal/semantic/retrieval"` to `internal/daemon/imports.go` if any indirect bootstrap is needed (probably not — the package is consumed via direct import from internal/skill/semantic/tools_context.go).
- Construct production RetrievalAccessor adapter that wraps `*retrieval.Engine`:
  - `QueryBleve(task, anchors)` delegates to `engine.QueryBleve` and translates `[]retrieval.TextRank` → `[]semantic.TextRank`.
  - `PersonalizedPageRank(ctx, repoID, anchors)` delegates to Phase 62 `RankScheduler.PersonalizedPageRank` (or the equivalent persisted-score read API); translate `[]graph.GraphRank` → `[]semantic.GraphRank`.
  - `RetrievalPending(ws)` delegates to `*retrieval.Recoverer.RetrievalPending`.
  - `TopEdgesFor(ctx, repoID, symbolID)` reads from Phase 62 graph engine (adjacency + edge-weight), formats top-5 edges as descriptive strings.
- Construct production `*retrieval.Recoverer` per workspace; wire `Recoverer.Probe` into `SetActivateCallback` so a fresh workspace gets its bleve segment reconciled before serving get_semantic_context.
- Daemon shutdown ordering: drain in-flight rebuild goroutines BEFORE closing the bleve Engine. Rebuild goroutines are `context.Background()`-derived (D-04 invariant) — they don't observe daemon shutdown via ctx, so the daemon needs a per-workspace Recoverer.Shutdown method (or equivalent) that waits for InProgress to flip to false. P64-08 owns adding this.
- Call `registerGetSemanticContext(server, semanticSkill, tracer)` from `internal/daemon/semantic_wiring.go` AFTER all other Phase 64 tools are registered + the RetrievalAccessor is wired.
- bleve segment path: per-workspace at `<workspaceDir>/.helix/semantic.bleve/`. Daemon constructs the path from the workspace registry.

**For Phase 65 / 67 (deferred work):**
- `SymbolRow` extension to include source bytes (so recovery rebuild can populate the comment-window field). Deferred per Task 2 GREEN docstring; 64-02 owns the snapshot-side schema.
- RRF weight tuning as exposed config keys — Phase 67 evaluation harness is the natural place to gather signal.
- Per-symbol graph_version (rather than per-snapshot) for finer tiebreak — only matters if a future evaluation reveals the per-snapshot constant gv biases the tiebreak in undesirable ways.
- Real tokenizer (e.g., tiktoken) replacing the bytes/4 heuristic in tokensPerCandidate — daemon-wiring layer can plumb this if measurement reveals the heuristic misestimates pathologically.

---

*Plan 64-07 — Generated 2026-05-08 — Sequential executor on main working tree*
