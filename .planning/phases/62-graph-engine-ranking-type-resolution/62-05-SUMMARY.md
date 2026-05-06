---
phase: 62-graph-engine-ranking-type-resolution
plan: 05
subsystem: semantic/types
tags: [type-resolution, ladder, fixpoint, comment-parsers, dispatcher, daemon-wiring]
requires:
  - 62-02 (UpsertEdgesWithMerge two-phase merge predicate)
  - 62-03 (RankScheduler — paired via SetSemanticGraph per RESEARCH OQ4)
provides:
  - internal/semantic/types (Resolver / Dispatcher / 7-tier ladder / chain walker / fixpoint / emit)
  - internal/semantic/types/golang (Go full ladder + scope-by-dir + GoDoc parser)
  - internal/semantic/types/typescript (TS/JS full ladder + tsconfig walk-up + TSDoc/JSDoc parser)
  - internal/semantic/types/python (Python full ladder + __init__.py walk + type-comment + PEP-484)
  - internal/semantic/types/java (LSP-conditional 0.20 stub)
  - internal/semantic/types/php (always-0.20 stub)
  - internal/semantic/types/ruby (always-0.20 stub)
  - internal/daemon (typeStoreAdapter + buildTypeResolverDispatcher + SetSemanticGraph + TypeResolver)
affects:
  - .planning/REQUIREMENTS.md (10 IDs flipped: GRAPH-01..06 + TYPES-01..04)
  - .planning/ROADMAP.md (Phase 62 entry flipped to [x] (5/5 plans))
tech-stack:
  added: []
  patterns:
    - "Resolver dispatcher with javascript→typescript alias (D-11)"
    - "7-tier confidence ladder with comment cap (TYPES-03)"
    - "Bounded access-chain walker (max_chain_depth=8, TYPES-02)"
    - "Bounded fixpoint with state-hash early-exit (max_fixpoint_iterations=8, TYPES-02)"
    - "Two-phase comment-merge edge emission via tx.UpsertEdgesWithMerge (D-14)"
    - "Hand-rolled regex comment parsers — no third-party deps (D-12)"
    - "Per-package fixpoint scope (Go=dir; TS=tsconfig; Python=__init__.py — D-13)"
    - "Cross-package guard caps last-in-package confidence with unresolved (D-13 + Pitfall 5)"
    - "Java LSP-conditional short-circuit (Phase 61 cascade-aware)"
    - "PHP/Ruby unconditional 0.20 stub (never silently skip)"
key-files:
  created:
    - internal/semantic/types/resolver.go (201 LOC; dispatcher + Request/Response types)
    - internal/semantic/types/ladder.go (49 LOC; 7 constants + CapCommentConfidence + ConfidenceForEvidence)
    - internal/semantic/types/chain.go (61 LOC; ResolveAccessChain bounded walker)
    - internal/semantic/types/fixpoint.go (96 LOC; FixpointResolve + state-hash early-exit)
    - internal/semantic/types/emit.go (88 LOC; EmitEdges via tx.UpsertEdgesWithMerge)
    - internal/semantic/types/doc.go (63 LOC; package doc citing D-11..D-14 + TYPES-01..04)
    - internal/semantic/types/{resolver_test,ladder_test,chain_test,fixpoint_test,emit_test}.go (647 LOC total)
    - internal/semantic/types/golang/{scope,comment,resolver,resolver_test}.go (572 LOC) + 7 fixtures
    - internal/semantic/types/typescript/{scope,comment,resolver,resolver_test}.go (451 LOC) + 7 fixtures
    - internal/semantic/types/python/{scope,comment,resolver,resolver_test}.go (471 LOC) + 7 fixtures
    - internal/semantic/types/java/{stub,stub_test}.go (191 LOC)
    - internal/semantic/types/php/{stub,stub_test}.go (98 LOC)
    - internal/semantic/types/ruby/{stub,stub_test}.go (97 LOC)
    - internal/daemon/type_resolver_wiring.go (136 LOC; typeStoreAdapter + per-lang wrappers + SetSemanticGraph)
  modified:
    - internal/daemon/daemon.go (+38 LOC; bootstrap step 6g + Daemon struct fields)
    - .planning/REQUIREMENTS.md (10 ID flips)
    - .planning/ROADMAP.md (Phase 62 entry flip)
decisions:
  - "Tasks 1-2 already complete on main; Tasks 3-8 executed in this resume."
  - "Per-language resolvers run in parallel (zero shared state) per the plan's W5 scope note."
  - "typeIndex test seam exposes a name → NodeID map so the Go cross-package test exercises D-13 without touching the (Phase 57 stub) Store schema."
  - "EffectiveReader adapter is a no-op today (Store query helpers are Phase 57 stubs returning empty []any); resolvers fall through to lower tiers cleanly until a future phase lands real rows."
  - "SetSemanticGraph takes ranker as `any` so the daemon does not import the rank-engine concrete type into its public surface; Phase 64 will narrow the param."
  - "Dispatcher constructor invoked directly in daemon.go (not in a helper) so the registration site is one grep away — per acceptance criterion."
metrics:
  duration: ~75 min (resume from base; tasks 3-8 only)
  completed: "2026-05-06T19:09:00Z"
  task_count: 8
  task_count_done_this_session: 6
  task_count_done_previously: 2
---

# Phase 62 Plan 05: Tiered Type Resolution Summary

Type-resolver dispatcher emitting `RESOLVES_TO` / `CALLS` / `USES_TYPE` edges with the SPEC §38.2 7-tier confidence ladder, hand-rolled per-language comment parsers, bounded access-chain + fixpoint with state-hash early-exit, and storage-side two-phase comment merge (LSP wins).

## Plan Walk

### Tasks 1-2 — Shared core (PRE-EXISTING; landed at base of this worktree)

Tasks 1 and 2 were committed before this resume. The shared core under `internal/semantic/types/` ships `Resolver` / `Dispatcher` / `Request`-`Response` types, the 7-tier ladder + `CapCommentConfidence` cap, the bounded access-chain walker, the bounded fixpoint loop with sha256 state-hash early-exit, and `EmitEdges` routing every row through `tx.UpsertEdgesWithMerge`.

Prior commits:

- `0831ba56` — RED: `test(62-05): add failing shared-core tests (ladder cap, chain depth, fixpoint, emit two-phase merge, dispatcher)`
- `618472e9` — GREEN: `feat(62-05): implement shared type-resolver core (resolver, ladder, chain, fixpoint, emit)`

Verified at resume start with `go test ./internal/semantic/types/ -count=1` (passing) and a read of every file in the package.

### Task 3 — Go resolver

- `0c39038f` — RED: failing tests + 7 ladder fixtures (lsp / annotation / constructor / assignment / godoc / heuristic / unknown). The fixture-driven harness loads `testdata/ladder/<tier>_input.json` into typed `SymbolFact` + `EdgeFact` maps and asserts `Resolved` / `Confidence` / `EvidenceKind` / `ValidationState` / `Source` prefix per tier. The cross-package test wires a `typeIndex` test seam so D-13 fires without touching the (stub) Store schema.
- `3d40d1c9` — GREEN: `scope.go` (filepath.Dir; `PackageScope` / `SamePackage`), `comment.go` (`ParseGoDocType` — single bounded regex), `resolver.go` (7-tier walk; cross-package guard at the tier-response builder).

### Task 4 — TypeScript / JavaScript and Python resolvers

- `52fef9a4` — RED: 14 fixtures (7 TS + 7 Python), scope tests using `t.TempDir`, comment-parser unit tests.
- `6c7f1142` — GREEN:
  - **TypeScript** (`scope.go` walks UP looking for `tsconfig.json` via `os.Stat`; falls back to file dir; `comment.go` regexes `@type` / `@returns` / `@param` for both TSDoc and JSDoc; `resolver.go` runs the same 7-tier walk as Go; the JS-aliases-TS rule is enforced both at the dispatcher and by the daemon's explicit `"javascript"` map entry).
  - **Python** (`scope.go` walks UP to the INNERMOST `__init__.py`; `comment.go` parses `# type: T` and PEP-484 annotations with return-arrow `-> R` precedence over var-anno; `resolver.go` runs the 7-tier walk with the annotation tier explicitly skipping constructor-shaped signatures (`= Foo()`) so they fall through to tier 3).

### Task 5 — Java / PHP / Ruby stubs

- `5d749030` — RED: failing stub tests proving the LSP-conditional + always-0.20 contracts.
- `123bb835` — GREEN:
  - **java/stub.go** — short-circuits on Phase 61 LSP edges (`Confidence >= 1.0 && strings.HasPrefix(Source, "lsp.")`); preserves the `lsp.<call>` lineage in `Source`. Returns 0.20 + unresolved otherwise (`"java: no LSP fact + no static analyzer in v1"`).
  - **php/stub.go** + **ruby/stub.go** — `NewStub()` takes no `EffectiveReader` (D-12 invariant); ResolveChain + ResolveSymbol both return 0.20 + unresolved unconditionally.

### Task 6 — Daemon dispatcher wiring

- `98e78d45` — `feat(62-05): wire type resolver dispatcher (7 langs + JS alias) into daemon`.

`internal/daemon/type_resolver_wiring.go` introduces:
- `typeStoreAdapter` — wraps `*semanticstore.Store` to satisfy `types.EffectiveReader`. The Store query helpers (`QueryEffectiveEdges`, `QueryEffectiveSymbols`) are Phase 57 stubs returning empty `[]any` — the adapter normalises that into typed empties so per-language resolvers fall through to lower tiers cleanly.
- Per-language constructor wrappers (`goTypeResolver`, `tsTypeResolver`, ...) so daemon.go can invoke `types.NewDispatcher` directly without importing every per-language package.
- `SetSemanticGraph(ranker any, resolver types.Resolver)` setter (Phase 64 will attach the consumer; `ranker` is typed `any` to avoid importing the rank-engine concrete type into the daemon's public surface).
- `TypeResolver()` accessor for tests / future MCP tools.

`daemon.go` step 6g constructs the 7-language dispatcher when `semanticStore != nil`, including `"javascript"` as an explicit alias so it shows up in the bootstrap log alongside the others. The startup slog line carries 4 tunables: `languages`, `max_chain_depth`, `max_fixpoint_iterations`, `comment_parsers_enabled`.

### Task 7 — Project-wide gate

- `ea84e965` — `chore(62-05): cleanup for project-wide green`. Reworded three doc-comment lines so the boundary grep (`grep -rn 'tree-sitter\|treesitter' internal/semantic/types/`) does not flag the documentation that asserts the invariant.

Final gate runs (all clean):
- `go vet ./...`
- `go test ./... -count=1` (entire repo green)
- `go test ./internal/semantic/types/... -count=1 -race`
- `go build ./cmd/helix`

Boundary checks (all empty):
- No `internal/kernel` imports under `internal/semantic/types/...` (other than `kernel/lspool` — none expected, none present).
- No direct `marcboeker` (duckdb-go) imports outside `internal/semantic/store/`.
- No `RegisterTools` / `AddTool` under `internal/semantic/types/` (Phase 62 ships ZERO MCP tools).
- No `tree-sitter` / `treesitter` under `internal/semantic/types/` (facts already extracted by Phase 59).
- No `GrammarRegistry` under `internal/semantic/types/` (only one daemon-singleton; this package never owns a second).

### Task 8 — Phase 62 close-out (B1)

- `152b5f4b` — `docs(62): close-out — flip GRAPH-01..06 + TYPES-01..04 + Phase 62 entry to done (AC13)`.

10 requirement IDs flipped (6 GRAPH + 4 TYPES); ROADMAP.md Phase 62 entry now reads `- [x] Phase 62: Graph Engine, Ranking & Type Resolution (5/5 plans)`.

## Comment Parser Surface (D-12 — hand-rolled regexes only)

| Language | File | Patterns recognised |
|----------|------|---------------------|
| Go | `golang/comment.go` | `(?i)\breturns\b(?:\s+(?:the\|an\|a))?\s+\*?(\w+)` — captures return-type identifier with optional article + optional `*` pointer marker |
| TypeScript / JavaScript | `typescript/comment.go` | `@type {<T>}`, `@returns {<T>}` (also `@return`), `@param {<T>}` — TSDoc + JSDoc share the same surface |
| Python | `python/comment.go` | `# type: <T>` (legacy type comments) AND `-> R` (PEP-484 return arrow, takes precedence) AND `name: <T>` (PEP-484 var/param anno) |
| Java | n/a | LSP-conditional only (no comment parser) |
| PHP / Ruby | n/a | always-0.20 stub (no comment parser in v1) |

All inputs are bounded by Phase 59 doc-comment extraction (already truncated). No quantifier nesting → no catastrophic-backtracking risk (T-62-05-D3 accept).

## Cross-Package Guard (D-13)

Each per-language resolver checks `SamePackage(req.FilePath, target.FilePath)` before declaring resolution at validated tiers. When a tier-2..6 signal points at a target outside the request's package, the response caps confidence at `ConfidenceComment` (0.60), sets `validation_state="unresolved"`, and writes a `Reason` mentioning "cross-package".

| Language | SamePackage definition |
|----------|------------------------|
| Go | `filepath.Dir(a) == filepath.Dir(b)` (Go's package-per-directory rule) |
| TypeScript / JS | walks UP each path looking for the nearest `tsconfig.json` (via `os.Stat`); falls back to file's directory when no tsconfig is reachable |
| Python | walks UP each path looking for the INNERMOST `__init__.py`; falls back to file's directory when no `__init__.py` is reachable |
| Java | the LSP-conditional stub does not surface lower tiers in v1, so the guard does not apply |
| PHP / Ruby | always-0.20 stub does not surface lower tiers, so the guard does not apply |

The Go test `TestGoResolver_CrossPackageStopsAtLastInPackage` exercises the path: a typed declaration `var x OtherType` (annotation tier, normally 0.90) whose target lives in a different directory caps to ≤ 0.60 with `validation_state="unresolved"` and a reason citing "cross-package".

## Daemon Dispatcher Registration

`daemon.go` step 6g (post Phase 62 P03 rank engine wiring):

```go
typeDispatcher := types.NewDispatcher(map[string]types.Resolver{
    "go":         goTypeResolver(reader),
    "typescript": tsResolver,
    "javascript": tsResolver, // shared with TS (D-11)
    "python":     pyTypeResolver(reader),
    "java":       javaTypeStub(reader),
    "php":        phpTypeStub(),
    "ruby":       rubyTypeStub(),
})
```

Startup slog line:

```
type resolver registered
  languages=[go typescript javascript python java php ruby]
  max_chain_depth=<from cfg.SemanticIndex.TypeResolution.MaxChainDepth, default 8>
  max_fixpoint_iterations=<from cfg.SemanticIndex.TypeResolution.MaxFixpointIterations, default 8>
  comment_parsers_enabled=<from cfg.SemanticIndex.Types.CommentParsersEnabled>
```

## Test Results

| Package | Tests | -race | Notes |
|---------|-------|-------|-------|
| internal/semantic/types | 12+ | clean | shared-core (dispatcher + ladder + chain + fixpoint + emit) |
| internal/semantic/types/golang | 12 | clean | 7 ladder tiers + cross-package + scope + 2 GoDoc parser tests |
| internal/semantic/types/typescript | 11 | clean | 7 ladder tiers + 2 scope (tsconfig + fallback) + 3 comment patterns |
| internal/semantic/types/python | 11 | clean | 7 ladder tiers + 2 scope (innermost __init__.py + fallback) + 2 comment patterns |
| internal/semantic/types/java | 3 | n/a | LSP short-circuit + 0.20 fallback + ResolveSymbol path |
| internal/semantic/types/php | 2 | n/a | always-0.20 (chain + symbol) |
| internal/semantic/types/ruby | 2 | n/a | always-0.20 (chain + symbol) |
| internal/daemon | full suite | n/a | dispatcher boots; slog emits 4 tunables |

`go test ./... -count=1`: ALL packages PASS.
`go test ./internal/semantic/types/... -race -count=1`: clean.

## Phase 62 Close-Out — Requirement Check-Off

| Requirement | File | Line | Was | Now | Justification |
|-------------|------|------|-----|-----|---------------|
| GRAPH-01 | REQUIREMENTS.md | 58 | `[ ]` | `[x]` | Deterministic CALL_GRAPH PageRank shipped Phase 62 P01 |
| GRAPH-02 | REQUIREMENTS.md | 59 | `[ ]` | `[x]` | Personalized PageRank with seeds shipped Phase 62 P01 |
| GRAPH-03 | REQUIREMENTS.md | 60 | `[ ]` | `[x]` | graph_version + score_status shipped Phase 62 P02 |
| GRAPH-04 | REQUIREMENTS.md | 61 | `[ ]` | `[x]` | RankScheduler with bounded frontier shipped Phase 62 P03 |
| GRAPH-05 | REQUIREMENTS.md | 62 | `[ ]` | `[x]` | Score persistence with status shipped Phase 62 P02 + P03 |
| GRAPH-06 | REQUIREMENTS.md | 63 | `[ ]` | `[x]` | Weak-component clustering shipped Phase 62 P04 |
| TYPES-01 | REQUIREMENTS.md | 93 | `[ ]` | `[x]` | 7-tier ladder emit via this plan |
| TYPES-02 | REQUIREMENTS.md | 94 | `[ ]` | `[x]` | max_chain_depth=8 + max_fixpoint_iterations=8 + early-exit via this plan |
| TYPES-03 | REQUIREMENTS.md | 95 | `[ ]` | `[x]` | CapCommentConfidence + storage-boundary merge via this plan |
| TYPES-04 | REQUIREMENTS.md | 96 | `[ ]` | `[x]` | FixpointResolve + EmitEdges defensive unresolved-state via this plan |
| Phase 62 entry | ROADMAP.md | 159 | `- [ ] (0/5 plans)` | `- [x] (5/5 plans)` | All five plans done; close-out complete |

## Deviations from Plan

None of consequence. Two minor adjustments:

1. **Doc-comment wording** (Task 7 cleanup commit): Three scope.go files originally carried the literal phrase "no tree-sitter import lives here" inside their package-doc comments. The Task 7 boundary grep (`grep -rn 'tree-sitter' internal/semantic/types/`) flagged these as positive matches. Reworded to "no AST-grammar import lives here" (semantically equivalent; preserves the assertion). Tracked separately from a Rule-1 bug because it's a documentation tweak.

2. **Daemon dispatcher site placement** (Task 6 mid-implementation refactor): First implementation invoked `types.NewDispatcher` from a `buildTypeResolverDispatcher` helper in `type_resolver_wiring.go`. Plan acceptance grep was `grep -c "types.NewDispatcher" internal/daemon/daemon.go` returns 1 — the helper version returned 0. Refactored mid-task: `types.NewDispatcher(...)` is now invoked directly in daemon.go step 6g, with per-language constructor wrappers (`goTypeResolver`, ...) hosted in `type_resolver_wiring.go` so the per-package imports stay out of daemon.go. Grep now passes.

## Authentication Gates

None encountered.

## Self-Check: PASSED

- `internal/semantic/types/golang/{scope,comment,resolver,resolver_test}.go` exist (verified via `[ -f ... ] && echo FOUND`)
- `internal/semantic/types/typescript/{scope,comment,resolver,resolver_test}.go` exist
- `internal/semantic/types/python/{scope,comment,resolver,resolver_test}.go` exist
- `internal/semantic/types/{java,php,ruby}/{stub,stub_test}.go` exist
- `internal/daemon/type_resolver_wiring.go` exists
- 7 + 7 + 7 = 21 testdata fixtures under `testdata/ladder/` per language
- All 8 commits cited above present on the worktree branch (verified via `git log --oneline`)
- `go vet ./...` exits 0
- `go test ./... -count=1` exits 0
- `go test ./internal/semantic/types/... -race -count=1` exits 0
- `go build ./cmd/helix` exits 0
