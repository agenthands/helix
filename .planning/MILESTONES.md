# Milestones

## v2.14 Codebase-Map Re-mapping (Go tree) (Shipped 2026-07-01)

**Phases:** 3 (141 STRUCTURE + STACK · 142 ARCHITECTURE + INTEGRATIONS + CONVENTIONS · 143 CONCERNS + TESTING + integrity gate) — ALL COMPLETE. MILESTONE-AUDIT PASSED (`.planning/milestones/v2.14-MILESTONE-AUDIT.md`). Assess-and-document milestone (documentation-only; no source change).

**Goal: make the codebase map honest.** The v2.13 audit flagged (HIGH) that `.planning/codebase/` still described the REMOVED pre-Go Python `serena` tree (`src/serena/`, `SerenaAgent`, `*_tools.py`) as of its sole 2026-04-07 commit — ~13 milestones stale. Re-mapped all 7 files to the current Go tree at HEAD (85,472 non-test LOC; 24 `internal/` packages; largest subsystem `internal/semantic/` at 30.8k LOC; 16 `cmd/` binaries; 51 frozen verbs), every claim grounded in a citable Go path.

**Key accomplishments:**
- **Phase 141 (STRUCTURE + STACK):** Go directory layout (16 `cmd/` binaries incl. 7 `vet-*` gates, 24 `internal/` packages with per-package purpose + LOC, `api/proto`, `protocol/{gen,patch}`, `bench/`, `tools/dspy-tune/`) mapped onto the 4 layers; real dep stack read verbatim from `go.mod` (Go 1.25.1, CGO=1 split-runner, MCP go-sdk v1.5.0, cobra v1.10.2, koanf v2, tree-sitter + 23 grammars, modernc sqlite, duckdb, chromem-go, prometheus/otel, sigstore, arrow; anthropic/openai SDKs dev-time bench/tools only; no runtime Python).
- **Phase 142 (ARCHITECTURE + INTEGRATIONS + CONVENTIONS):** 4 layers + daemon bootstrap + the true **6-deep** MCP middleware LIFO chain (LazyInit→ProfileEnforce→Guardrail→Suggestion→ProfileFilter→Telemetry); `internal/semantic/` surveyed FRESH as an extract→store→enrich→emit pipeline with the edge-kind ledger; LSP three-tier installer / tree-sitter / gRPC StreamMCP / sigstore / otel / container auto-detect integrations; Go conventions + gofmt/vet + 7 custom `cmd/vet-*` architectural gates + generated-code drift gates.
- **Phase 143 (CONCERNS + TESTING + integrity gate):** grounded tech-debt / fragile-seam / security-posture map (vet-enforced layering invariants, CGO=1 constraint, retained-lineage naming, sourced deferred items; NO invented debt — 0 TODO/FIXME confirmed); Go test architecture (`go test` + testify, `testdata/` fixtures, vet-gate suite, real-binary E2E, `bench/` harness + 6 CI workflows). Integrity gate: all staleness banners removed, every `Analysis Date` → 2026-07-01, residue-grep clean (only labeled retained-lineage `SerenaMCPServer` + `api/proto/serena/v1/` remain).

**Grounding discipline held — the writers corrected 3 stale source-of-record facts** by reading real code, not parroting: `cmd/` = 16 (not 17); MCP middleware = 6-deep (not CLAUDE.md's 4 — `ProfileEnforce`+`Guardrail` omitted there); `helix setup` = 8 clients (not 7); cobra v1.10.2 (not CLAUDE.md's v1.9.1). The map is now more accurate than CLAUDE.md.

**Also bundled — planning-integrity fix:** the stale `ROADMAP.md` `## Phases` detail section (still describing v2.5-era phases 115-126) was regenerated. That stale section was the root cause of `anvil-cc roadmap analyze` manufacturing phantom incomplete phases 121-123 — which is how this milestone surfaced. `roadmap analyze` now reports 141/142/143 cleanly.

**Invariants held:** documentation-only — zero `.go`/`go.mod`/`go.sum` changes (so `go build`/`go vet`/`go test` are provably unaffected); no formatters/tests run by writers; every sampled claim independently re-verified against HEAD at audit time.

**Deferred / follow-ons (non-blocking):** (1) **CLAUDE.md drift (LOW)** — its hand-maintained Architecture/Stack sections carry the same 3 stale facts the map corrected; recommend a cheap reconcile pass (CLAUDE.md was out of scope for v2.14). (2) `docs/` contributor docs current (v2.13), untouched.

**Prior:** v2.13 remains uncommitted (co-driver's call); v2.14's commit is separate and additive.

## v2.13 True intraprocedural DATA_FLOWS (in-body origins, variable-level) (Shipped 2026-07-01)

**Phases:** 3 (138 origin-space engine + 11-lang unions + all-11 unit matrix · 139 in-body + return-bridge emission + read surface + Go multi-hop · 140 all-11 real-binary E2E + function-seeded multi-hop + docs) — ALL COMPLETE + independently verified. MILESTONE-AUDIT PASSED (`.planning/milestones/v2.13-MILESTONE-AUDIT.md`). Independence: start-of-milestone red-team (`agent://RedTeamV213`, PROCEED-WITH-FIXES → all folded); each phase carried an independent qa VERIFICATION.md with its own revert-and-fail (138: m2 guards RED; 139: return-bridge gate inverted RED; 140: langFromExt arm removed + binary rebuilt → subtest RED), all restored byte-identical; audit-time consolidated fresh-binary re-run.

**Goal: model flow originating MID-BODY** — the return of an in-body call (`y := producer(); sink(y)`), not just a parameter — and make it queryable across ALL 11 languages, using the no-schema-change cut (edges anchor on existing function + parameter symbol nodes, never persistent variable-level nodes). v2.9 gave DATA_FLOWS a case-1 param→param producer; v2.13 deepens it to in-body origins.

**Key accomplishments:**
- **Phase 138 (engine + breadth):** widened the `dataflow` leaf engine's taint origin to `Origin{Param, Callee}` (param ∪ callReturn); records `InBodyFlow{Producer, Consumer, ArgPos}` via a D-COMPOSE strict-whitelist RHS-unwrap (m2 guards: selector/multi-call RHS refuse). Extended the tree-sitter kind unions to all 11 grammars (method_parameters, assignment, init_declarator, value_arguments, property_declaration). All-11 pure-unit matrix: 11/11, 0 skip. Fixed the latent **Ruby zero-DATA_FLOWS-since-v2.9** bug (its params parsed as `method_parameters ∉ paramListKinds` → engine bailed).
- **Phase 139 (emission + Go proof):** two new DATA_FLOWS passes — `inBodyDataFlowEdges` (producer.function→consumer.param, `def_use_inbody`) + `returnBridgeEdges` (param→enclosing-function, `def_use_return`, the reclaimed dead `ParamFlow.Returns` as the multi-hop connector). Wired STRICTLY after `dataFlowEdges` (M1, EdgeIDs unshifted); map-LOOKUP-only output path (M4 determinism). Distinctness + anti-mis-bind + def_use-SET non-regression guards (mutation-RED); in-process Go multi-hop `src→transform→sink` proven; 3 now-false M3 doc-comments corrected + the function-seed reachability test repurposed.
- **Phase 140 (all-11 real-binary E2E + docs):** `TestCLI_E2E_InBodyDataFlow` = 11/11 PASS through the real `helix` binary (explain-symbol-deep surfaces the in-body data_flows edge per language); function-seeded multi-hop via `trace-data-flow` (broken-hop → unreachable); determinism guard. `docs/edge-types.md` updated (3 Source markers + all-11 coverage ledger + function-seed note).

**The one scope call (co-driver-approved, Path B):** Phase 140 found the daemon file-walk gate `langFromExt` did NOT map .rs/.kt/.php/.rb — so those 4 langs, proven at engine+emission+unit level, could not index through the real binary (an M5-class gap at the daemon layer the bare-parse-tree unit matrix never exercised). Escalated as a genuine fork; co-driver chose **Path B** — wire the 4 into `langFromExt`, achieving 11/11 E2E. Verified: adds ZERO new `has_type` edges (their type resolvers are nil-stubs, double-guarded); an independent regression pass confirmed no pre-existing daemon/skill test broke.

**Invariants held:** zero new Go deps (`go.mod`/`go.sum` byte-unchanged); no schema migration (`EdgeFact` + `internal/semantic/store/` byte-unchanged); deterministic (re-index → identical edge set); the v2.9 `def_use` param→param SET byte-unchanged for the 6 already-working langs (go/ts/java/csharp/php/rust); `make vet` clean; affected-pkg `go test` + fresh-binary E2E green.

**Security review:** PASSED, 0 findings (5/5 checks clean — langFromExt is a pure ext→id map; emission bounded+deduped; `EdgeFact.Reason` write-only/opaque via parameterized SQL, no injection sink; no new deps; nil-resolver double-guarded). Harness self-audit CLEAN (4 probes, 0 findings).

**Honest scope limits:** in-body origin's source identity is the producer FUNCTION node (a return value has no distinct node — by design, no variable-level nodes); the `trace_data_flow` verb's documented seed stays a parameter (the function-seed multi-hop uses the mechanically-accepted path; full verb support is a fast-follow); callReturn-reaches-return not recorded (no v2.13 edge maps to it).

**Deferred / follow-ons:** (1) **Codebase-map re-mapping pass (HIGH doc-integrity)** — `.planning/codebase/` still describes the REMOVED pre-Go Python serena tree (2026-04-07), never updated through the Go rewrite; contained for v2.13 (STRUCTURE.md addendum + staleness banner), a dedicated re-mapping milestone recommended. (2) Full function-seed support in the `trace_data_flow` verb contract. (3) Variable-level graph nodes, field/heap flow, source/sink taint.

**NOT committed** — the working tree carries v2.13 (138-140) + the planning artifacts; commit is the co-driver's call.

## v2.12 Production wiring for the type resolvers (C-family E2E) (Shipped 2026-07-01)

**Phases:** 3 (135 extraction foundation + 136 resolver ChainTokens/producer/emit + 137 real-binary E2E + docs) — ALL COMPLETE + independently verified. MILESTONE-AUDIT PASSED (`.planning/milestones/v2.12-MILESTONE-AUDIT.md`). Independence: start-of-milestone red-team (`agent://RedTeamV212`, verdict REWORK → all folded); each phase re-verified by the architect via uncached test runs + code re-read; headline E2E re-run against a freshly-built binary.

**Goal: make the ORPHANED type-resolver subsystem run in production** so a C-family type reference becomes a real, queryable `RESOLVES_TO`/`has_type` edge — proven end-to-end through the `helix` CLI. v2.11 shipped unit-tested C-family resolvers but the whole subsystem was orphaned (dispatcher built at bootstrap, never consumed; `EmitEdges` zero callers; no working `RESOLVES_TO` producer at all). This milestone delivered exactly the "production E2E for the C-family" that v2.11 deferred.

**The red-team's decisive finding:** the milestone rested on a LATENT v2.11 contract break — resolvers parse the type from the referencing symbol's `Signature`, but extractors strip it to a bare name (`"x"`, not `"Foo x"`), so no type ever reached the resolver in production; the v2.11 "tiers 2-6 work" proof used a FABRICATED signature. Fork resolved (co-driver: **Option A**): the producer links each var→type and feeds the type name via `ChainTokens`; the resolver's annotation tier consumes it.

**Key accomplishments:**
- **Phase 135 (extraction foundation):** extended `langFromExt` so the production index extracts C-family files (was a 4-lang go/ts/js/py allowlist); added an in-memory-only `extract.SymbolFact.DeclaredType` populated for C var/field/param via tree-sitter co-capture; a pure deterministic `linkVarTypes` helper yielding `(var → type-name)` tuples. Fixed a PRE-EXISTING latent crash B1 exposed: C `signatureHash` truncates at `{`, so a struct definition + type-use collided on SymbolID → PK violation once C hit the committed path → a deterministic snapshot-level SymbolID dedup (prefer-richer definition).
- **Phase 136 (resolver wiring, in-process proof):** all 7 full resolvers' annotation tier consumes `ChainTokens` (Option A) + `NewResolverWithIndex`; a per-batch producer/driver in `factsFromExtracted` (`resolveTypeEdges`) builds `typeIndex` from `nameToNode` (skipping `nameCount>1` anti-mis-bind), runs `FixpointResolve` over sorted groups (determinism), converts to `RESOLVES_TO` EdgeFacts (gating dst=0/low-confidence). Proven in-process against the REAL C extractor: exactly one committed `RESOLVES_TO` p→Foo. Clean cutover: the orphaned bootstrap dispatcher + dead `TypeResolver()`/`SetSemanticGraph`/`typeStoreAdapter` seams REMOVED (no shim).
- **Phase 137 (real-binary E2E + docs):** `TestCLI_E2E_CTypeResolution` drives a REAL `helix` binary — `explain-symbol-deep` returns a `has_type` edge (RESOLVES_TO→has_type) for the C fixture; differential anti-vacuity (primitive param → zero) + determinism (re-index identical). `docs/type-resolution.md` reconciled to the production path.

**Invariants held:** zero new Go deps (`go.mod`/`go.sum` byte-unchanged), no schema migration, deterministic (byte-identical snapshot), extractor goldens byte-identical; `make vet` (8 vettools) clean; 12 pkg `go test` ok; D-12 always-emit preserved. Leaf boundary relaxed (Option A: extractors + `internal/semantic/types/*` + daemon).

**Honest scope limits (stated in the audit):** C is the proven language — C++/C#/Java share the identical batch wiring + ChainTokens path but only C has co-capture linkage + a dedicated real-binary E2E (fast-follow). The `explain-symbol-deep` seed `file_path` must be absolute (paths stored verbatim from the full-walk).

**Deferred:** Tier-1 LSP short-circuit (`QueryEffectiveEdges` stub); the `type_chain` response field + its Schema-v6 migration (also unblocks v2.11 Tier-5 comment); cross-package/cross-TU resolution (intra-package/TU only); C++/C#/Java var→type co-capture + E2E fixtures; the deeper C fix (extractor emits struct type-uses as `definition.struct` symbols — would churn the `with_fields` golden).

**NOT committed** — the working tree carries v2.12 (135-137) + the planning artifacts; commit is the user's call.

## v2.11 Type-resolution depth (C-family) (Shipped 2026-07-01)

**Phases:** 5 (130 plumbing + 131 C + 132 C++ + 133 C# + 134 Java) — ALL COMPLETE + committed (`d43d7ce7`..`97fc4bc9`). MILESTONE-AUDIT PASSED (`.planning/milestones/v2.11-MILESTONE-AUDIT.md`). Independence: start-of-milestone red-team (`RedTeamV211`, PROCEED-WITH-FIXES, folded) + a close red-team (oracle).

**Goal: replace the 4 C-family stub type-resolvers (Java/C#/C/C++) with full tiered resolvers**, raising their RESOLVES_TO/USES_TYPE/CALLS edges from a flat `0.20 unresolved` to real tiered confidence. The engine (7-tier ladder, fixpoint, dispatcher) already existed (Phase 62); v2.11 was per-language resolver depth + the data-plumbing prerequisite the red-team surfaced.

**Key accomplishments:**
- **Phase 130 (plumbing) — the load-bearing unblock.** The daemon adapter's `QueryEffectiveSymbol` returned an empty `SymbolFact`, so EVERY resolver's tiers 2–6 collapsed to Tier 7 in production (the existing Go/Py/TS resolvers were production-inert too). New `Store.QueryEffectiveSymbolFact` (snapshot JOIN `semantic_symbols`⋈`semantic_files`) + adapter re-route; proven end-to-end (real go resolver → real store → Tier-2 edge).
- **Phases 131–134 (C-family resolvers):** each stub → a full resolver walking the ladder with language-specific parsing. C: tiers 2/4/6 (no ctor). C++: 2/3/4/6 + template/qualifier normalization. C#: 2/3/4/6 + attribute-strip + namespace scope. Java: 2/3/4/6 + `@`-strip + `*Impl`/`Abstract*`/`get*` heuristics + package scope. Docs: `docs/type-resolution.md`.

**Red-team folds (RedTeamV211, all applied):** B1 (starved adapter → Phase 130); B2 (no `doc_comment` column → Tier 5/comment DROPPED, no migration); B3 (c_sharp/java `stub_test.go` orphans → replaced with `resolver_test.go`); M1 (real per-language scope: Java/C# intra-package/namespace, C/C++ flat program-wide); M2 (C++ stack/brace + heap-new construction); M3 (Java/C# T2 = typed declaration, annotations auxiliary); m5 (risk-first build C→C++→C#→Java).

**Invariants held:** zero new Go deps (`go.mod`/`go.sum` byte-unchanged), no schema migration, deterministic, RE2 (no ReDoS); `make vet` (8 vettools) clean; 13 pkg `go test` ok; D-12 always-emit preserved. Leaf boundary (stdlib + `internal/semantic/{graph,types}`).

**Honest scope limits (stated in the audit):** E2E composition (extraction→snapshot→resolver→emit→graph) inferred not tested for the 4 new languages; production cross-package cap is test-seam-gated (`typeIndex` nil in production — pre-existing Phase-62 characteristic); T6 heuristic is best-effort 0.45.

**Deferred:** T5/comment tier (needs `doc_comment` schema migration + extraction backfill); Rust/Kotlin/PHP/Ruby resolvers (v2.12); production E2E for the C-family.

## v2.10 trace_data_flow verb — DATA_FLOWS read surface (Shipped 2026-06-30, `12a9ec3c`)

**Phases:** 3 (127 ReachableFrom primitive + 128 handler + 129 atomic frozen-51 surface bump) — ALL COMPLETE + verified. MILESTONE-AUDIT PASSED (`.planning/milestones/v2.10-MILESTONE-AUDIT.md`).

**Goal: make v2.9's DATA_FLOWS substrate consumable by an agent.** A dedicated `trace_data_flow` verb answers "from this seed PARAMETER, what's reachable via interprocedural data flow?" via a new read-only `DataFlowReachabilityAccessor` (bulk `QueryAllDataFlowEdges` → in-memory BFS → `[]ReachableSymbol{symbol_id, hops}`). The verb is the **51st frozen verb** (the frozen-50 anchor bumped deliberately, the sanctioned mechanism).

**Red-team** (`agent://RedTeamV210`, PROCEED-WITH-FIXES): its headline finding (frozen-50 anchor absent) was REFUTED by direct read — the anchor IS real; its grep regex failed. Its real wins were the 4 count-sensitive gates (render-class, querySet, graphReaderVerbs, D-09 gatedHandlerFiles) + the bulk-query + 3-phase-split folds. The function-seed trap (B3) confirmed: param-only seed is the only HEAD-viable design (no CONTAINS function→param emission).

**Invariants held:** zero new Go deps, no schema migration, deterministic, bounded; `make vet` (8 vettools) clean; cligen/refgen/docgen --check green at 51.

**Deferred:** function-seed support (needs OwnerSymbolID population); field-access DATA_FLOWS extension; `get-change-impact-graph` multi-projection rework.

---

## v2.9 Interprocedural DATA_FLOWS — param-flow reachability substrate (Complete + verified 2026-06-30; pending commit)

**Phases:** 2 (125 intraprocedural flow-summary engine + 126 arg→param emission / read surface / reachability) — BOTH COMPLETE + verified. MILESTONE-AUDIT PASSED (`.planning/milestones/v2.9-MILESTONE-AUDIT.md`).

**Goal: give the `DATA_FLOWS` edge kind a real producer.** v2.8 reserved it as producerless (after renaming the structural-shape edge to `STRUCTURAL_TWIN`). v2.9 emits **case-1-only param→param interprocedural flow** (caller.param → callee.param through a resolved in-repo call) — the classical function-summary taint-composition primitive. **Honest framing:** a syntactic pass-through *reachability substrate*, NOT full taint analysis (no in-body origins / field-flow / sinks / sanitization — those need variable-level nodes, deferred).

**Roadmap red-teamed** (`agent://RedTeamV29`, PROCEED-WITH-FIXES, all findings folded). Two blocking findings caught pre-build: (1) `DEFINES` is a flat file-container heuristic (`semantic_wiring.go:2218`, comment "First symbol in file is typically the outer class/module") — callee params resolve by **emit-order adjacency**, not DEFINES; (2) reference nodes are a separate namespace — `SrcNodeID` must be a symbol (caller.param), never a call-reference node. Plus 6 majors folded (no tree-sitter CALLS edge — substrate = reference.call refs + name index; anti-mis-bind on >1 callee candidate; non-positional handling; directed dedup; total-edge-budget bound; Phase 127 collapses into 126).

**Invariants (planned):** zero new Go deps, no schema migration (binding = node identity), deterministic, bounded; `explain-symbol-deep` read surface already wired (`MapInternalKind` maps DATA_FLOWS).

**Deferred (by design):** dedicated `trace-data-flow` verb (milestone-sized); variable-level nodes / in-body origins; queryable arg/param-position column.

---

## v2.8 Graph Intelligence Depth — cbm-mcp parity (Shipped: 2026-06-30)

**Phases completed:** 4 phases (121–124), MILESTONE-AUDIT PASSED (goal-backward, code-grounded). Run inline (native standing-team/SMTC substrate absent); independence gate delegated to an `oracle` red-team (PROCEED-WITH-FIXES, all 6 findings folded).

**Outcome: the one declared-but-unwired cbm-mcp edge kind, SEMANTICALLY_RELATED, now emits — honestly distinct, selective, deterministic, and readable.** Closed the depth gap (not just the schema gap) for the relatedness edge, via Random Indexing (cbm-mcp's own mechanism) rather than the unported neural embedder — keeping the single-binary / zero-runtime-dep invariant.

**Key accomplishments:**
- **Workstream A (121–123):** new pure-Go RI engine `internal/semantic/relatedidx/` (256-dim int32 context vectors over identifier/comment vocabulary; deterministic by integer accumulation, SimHash LSH for bounded emission, curated stop-list against boilerplate saturation); `SymbolFact.ContextVec` plumbed through the shared `FingerprintBody` seam → all 11 providers; emitted in `semanticallyRelatedEdges`, readable via `helix explain-symbol-deep`.
- **Distinctness measured + mutation-confirmed:** same-struct/disjoint-vocab → Jaccard 1.0 / cosine 0.0; diff-struct/shared-vocab → Jaccard 0.0 / cosine 0.80; removing the stop-list links boilerplate-saturated unrelated bodies (cosine 0.69 > 0.55) → guard goes RED.
- **B0-a (124):** renamed the structural-shape edge (mis-named `DATA_FLOWS`) to `STRUCTURAL_TWIN`; reserved `DATA_FLOWS`/`data_flows` (declared + validatable, producerless) for the true interprocedural arg-to-param flow of a future Workstream B.
- **Invariants held:** zero new Go deps (`go.mod`/`go.sum` byte-unchanged), no schema migration, generated-file `--check` gates pass; 45 pkg `go test` ok, `make vet` (8 vettools) clean.

**Audit:** PASSED — see `.planning/milestones/v2.8-MILESTONE-AUDIT.md`. `debt query: raw_debt=0`; `ledger validate: 0 errors`. One process finding: verify gates were authored as prose VERIFICATION.md, not §5 typed `gate_result` records, so `measure refusal-rate` reads 0 gates (observability gap, not correctness — the tests are real + mutation-confirmed).

**Deferred (by design):** Workstream B feature build (true interprocedural DATA_FLOWS: def-use → arg→param → propagation) — own roadmap + red-team cycle, future milestone.

---

## v2.4 Corpus Growth & Real Optimization Verdict (Shipped: 2026-06-24)

**Phases completed:** 4 phases (111–114), 4 plans, 10/10 requirements (CORPUS-01/02, SCALE-01/02/03, RUN-01/02/03, REPORT-01, ADOPT-05)

**Outcome: a REAL, gate-cleared verdict (SHIP-by-rule, marginal — adoption NOT recommended on this margin).** v2.3 no-shipped because the corpus was below the `val_size>50` gate; v2.4 grew the corpus past the gate and ran the task-success pipeline FOR REAL, turning the no-ship into a measured one.

**Key accomplishments:**

- Materialized a 103-task Aider-polyglot reward corpus (go 39 / python 34 / rust 30) with a 3-way disjoint split (train 26 / val 26 / **sequestered held-out 51 > 50**, planted-leak RED test) — the literal v2.2/v2.3 no-ship axis, now cleared (CORPUS-01/02).
- Hardened the optimizer for the real run: DeepSeek-`v4-flash` pin (program + reflection LM, alias-deprecation-safe), cost caps (rollout / bounded concurrency / 429 backoff / agent turn cap), and a re-verified SWE-bench "0-tests⇒resolved" footgun refusal (SCALE-01/02/03).
- **Ran the pipeline for real, cost-aware ($0.23 total):** GEPA `optimize.py` (val_size=51), an honest ON-vs-OFF attribution on the sequestered split — **delta +0.0392 (ON 3/51 vs OFF 1/51) → SHIP-by-rule** — and a SWE-bench Verified gold-patch confirm on Podman (**2/2 resolved**, fail-not-skip proven) (RUN-01/02/03).
- **Fixed three integration gaps the v2.3 hermetic fakes had hidden** (the v2.3 pipeline was never functional end-to-end): agent verb argv (positional → real `--flags`); a **real product bug** — `helix activate` now also calls `activate_project` so the CLI file/edit verbs get a workspace (E2E regression-tested); and the GEPA candidate→agent steering thread.
- REPORT-only ship/no-ship REPORT with honest caveats (thin/noise margin, GEPA non-evolving, ON candidate = existing SKILL.md, K=2) + ADOPT-04 boundary re-verified (zero new Go deps since v2.0; adoption gate desync → `--check` exit 1) (REPORT-01, ADOPT-05).

**Audit:** PASSED — 10/10 reqs, 4/4 phases verified, E2E real run, build/`make vet`/tests green. Executed inline (gsd-* subagents absent). Follow-ons (non-blocking): TUNE-FUT-06 (rebuild GEPA-as-agent-program so reflection evolves), TUNE-FUT-05 (larger SWE-bench K), a significance test in `decide_ship`, and TUNE-FUT-03 (actual human-gated SKILL.md adoption). See `.planning/milestones/v2.4-ROADMAP.md` + `v2.4-MILESTONE-AUDIT.md`.

---

## v2.3 Task-Success-Driven Skill Optimization (Shipped: 2026-06-24)

**Phases completed:** 4 phases (107–110), 10/10 requirements (AGENT-01/02/03, ORACLE-01/02, TUNE-02/03/04, ADOPT-03/04)

**Outcome: NO-SHIP by design** — the full task-success pipeline is built, gated, and green, but no optimized steering text was adopted: the corpus is still below the strict `val_size>50` gate (the same v2.2 root cause, now reached through real task-success machinery rather than the gameable `choice_rate` proxy). TUNE-FUT-01 (grow the corpus) is the recorded precondition for a real ship decision.

**Key accomplishments:**

- Shipped a dev-time, importable Python ReAct agent (`tools/dspy-tune/agent/`) that drives the real `helix` CLI in a bounded loop with DeepSeek-primary/OpenAI-fallback provider selection, loud-fail-on-missing-key, and a first-class steering ON/OFF switch — the foundation the task-success metric imports in-process.
- Replaced the gameable `choice_rate` reward with an honest Aider-polyglot task-success oracle (parity-pinned native test commands, 0-tests=hard-ERROR, anti-tamper gold-test restore) wired as the GEPA metric behind a sequestered held-out split + strict `val_size>50` adoption gate — the direct fix for the v2.2 no-ship root cause.
- Added the heaviest grader: SWE-bench task-success via the upstream `swebench==4.1.0` harness on Podman (`grade_swebench.py`, a parity mirror of `bench/evaluators/swebench/harness.go` pinned by a shared golden + asserted on both sides), enforcing FAIL_TO_PASS + PASS_TO_PASS with a dataset-org pin and 0-tests refusal, plus an honest ON-vs-OFF attribution delta (`attribution.py`) recording per-arm cost.
- Gated the pipeline output behind human-reviewed `helix-refgen --check` adoption (optimizer writes only git-ignored output), re-verified the single-binary / no-runtime-Python invariant end-to-end (zero new Go deps — go.mod untouched since v2.0 Phase 90), and recorded the ship/no-ship REPORT.

**Audit:** PASSED — 10/10 requirements, 4/4 phases verified, 5/5 cross-phase integration seams wired, E2E flow confirmed. Every new gate ships a break-the-invariant → assert-RED test (3 guards mutation-confirmed RED in Phase 109; the `helix-refgen --check` desync exit-1 proven live in Phase 110). Executed inline with `uv` (the gsd-* executor/verifier subagents are not installed in this roster).

**Known deferred items at close:** 3 pre-existing quick-tasks (`260414-e5n`, `260617-j29`, `260617-t7x` — already-completed stale tracking entries) + 1 pre-existing out-of-scope `cmd/helix-bench` network/HELIX_BIN test failure; plus TUNE-FUT-01 (grow corpus past `val_size>50`). The Phase-109 CONTEXT "open questions" audit flag was a false positive (the section records resolved answers). None originate from v2.3 phases. See STATE.md Deferred Items.

---

## v2.2 Agent-Facing Skill Quality & Prompt Tuning (Shipped: 2026-06-24)

**Phases completed:** 4 phases, 5 plans, 5 tasks

**Key accomplishments:**

- Rewrote the hand-authored `## Decision matrix` to route one agent intent to one tool — QUERY/ACTION rows split, every `Not this` cell named, 8 indexed-graph readers marked † — gated by 3 revert-and-fail anti-vacuity guards keyed to the 50 frozen verbs.
- Shared golden parity corpus pinned to the test/oracle/adopt Go classifier, plus an inverted `toolsquarantine` go/analysis import-boundary analyzer (with RED/GREEN/lookalike fixtures) wired into `make vet` to keep the dev-time `tools/` tree out of the shipped binary, `go.mod`, and `go test ./...`.
- Per-verb `cmd/helix-refgen` override map fixed the generated `reference.md` group-collapse (10 verbs corrected); closed-set skill bundle ships only `{SKILL.md, reference.md}`; non-vacuous exact-count (==50) reference contract. The DSPy spike concluded a documented **no-ship** (MinTasks=5 too small) — the single binary stays 100% Python-free.

**Audit:** PASSED — 7/7 requirements (BUNDLE-01/02, REFGEN-01, SKILL-01/02/03, TUNE-01), 4/4 phases verified, cross-phase integration wired, 3/3 E2E flows, Nyquist 4/4. Code review caught + fixed a vacuous Guard B (104) and a real Python↔Go parity bug (106).

**Known deferred items at close:** 3 pre-existing quick-tasks (`260414-e5n`, `260617-j29`, `260617-t7x` — already-completed stale tracking entries) + 1 pre-existing out-of-scope `cmd/helix-bench` network/HELIX_BIN test failure. None originate from v2.2 phases. See STATE.md Deferred Items.

---

## v2.1 Agent Adoption & Aider-Derived Validation (Shipped: 2026-06-23)

**Phases completed:** 6 phases, 13 plans, 14 tasks

**Key accomplishments:**

- `runAiderEditCell` (an `aider_edit` mode-name branch off `RunCell`) drives the Plan 01 deterministic EDIT-verb AgentFn + live native TestFn through `RunExercise` VERBATIM against the warm daemon, stamps `edit_format_applied`, and produces a committed, byte-reproducible polyglot-edit baseline (deterministic metrics only) — proven hermetically with no HELIX_BIN/network and fail-closed under HELIX_BIN.
- [plan-checker fix — detector lift made REAL]
- 1. [Rule 1 - Bug] Discriminator margin exceeded the initial observed spread
- 1. [Rule 1 - Bug] Ambiguous case construction (whole-function duplicate matched only one site)
- cmd/helix-refgen generates a 50-verb reference.md from the live tool registry (args from the new VerbSpecsForDocs accessor, not InputSchema), shipped via an embed.FS skill bundle and gated by a `--check` drift gate wired into make + CI.
- Two merge-gating, non-vacuous contracts: (a) reference.md covers every frozen verb sourced from VerbToolNames() as authority, and (b) each standard-tool shape steers to the SPECIFIC emitted `helix <verb>` keyed on the command — both with mandatory revert-and-fail proofs and an empty-bucket floor, replacing the weak `Contains(...,"helix")` assertion.
- 1. [Rule 1 — Test breakage] Pre-existing generic-registrar tests leaked an `AGENTS.md` into the package dir

---

## v2.0 v2.0 (Shipped: 2026-06-22)

**Phases completed:** 7 phases, 20 plans, 41 tasks

**Key accomplishments:**

- Per-socket gofrs/flock startup lock with double-checked tryConnect in `ConnectOrStartDaemon`, proven by a synctest fan-out test to spawn exactly one daemon under N concurrent cold callers (CLI-03).
- Task 1 (TDD): No-arg helix → grouped help, exit 0, no stdio session (CLI-04)
- `helix call <verb> --flag=val` issues a single MCP `tools/call` through the warm daemon over the EXISTING gRPC `StreamMCP` wire via a client-side transport mirror + the MCP SDK client (initialize handshake, not hand-framed JSON-RPC), with race-safe cold auto-start (90-01), ordered clean teardown, and zero proto changes (CLI-01/CLI-02).
- 1. [Rule 1 - Bug] `search` verb mapped to a non-existent tool
- A go/packages+AST generator (`cmd/helix-cligen`) that emits a committed `internal/cli/verbs_gen.go` of 50 capability-grouped `helix <verb>` subcommands — one per live-registry tool — drift-gated by `helix-cligen --check`, plus the exported `VerbToolNames()` seam for 91-03.
- Server-side `tools/call` authz (ProfileEnforcementMiddleware) that refuses any tool outside the session's resolved AllowedTools with a typed `serr.PermissionDenied`, installed between Guardrail and LazyInit so LazyInit-first LIFO is preserved.
- 1. [Rule 3 - Blocking] Flat verb name and flag differed from the plan's assumptions
- Three pure, dependency-free foundation units for the Phase 92 terse renderer: a 50-verb render-class map, a locus parse + path-normalization + sort/dedup core, and the 9-kind serr.Kind → frozen exit-code + stderr-prefix mapper — all built TDD with no cobra/network dependency.
- Wired the 92-01 foundation (render class, locus core, exit-code mapper) into the live verb path: `renderResultFor` now emits sorted+deduped `relpath:line:col<TAB>payload` for locus-list verbs with a workspace-clamped CLI-side snippet for bare nav loci, passes tree/opaque through verbatim, gates color up front, adds persistent `--color`/`--abs`/`--json` flags every generated verb inherits, preserves the daemon's typed `<kind>:` error so `main.go` exits with the per-kind code, and adds color/abs to the cligen denylist — the output shape FREEZES here.
- Re-pointed the TEST-02 contract oracle from MCP `TextContent` goldens to REAL `helix <verb> --flags` subprocess stdout — per-verb goldens now freeze the terse `relpath:line:col<TAB>payload` shape (plus `--abs` absolute and `--color=never` zero-ANSI variants) against the binary; the MCP schema meta-validation is replaced by an untagged default-suite typed-args→cobra-flags parity test; and a behavioral chain proves a nav locus feeds a downstream verb verbatim (OUT-04) with a self-contained snippet (OUT-03).
- 1. [Rule 3 - Blocking] Repaired a corrupt local Go module cache (environmental, not code)
- Repurposed the PreToolUse nudge from a generic 5-call "use find_symbol" tip into a per-call advisory that steers grep/sed/cat/find over positively-identified CODE targets toward the frozen Phase 92 `helix` verbs via `hookSpecificOutput.additionalContext` JSON — fail-open and exit-0 on prose/log/config, unparseable, and no-operand commands.
- Task 1 — SKILL-04 dependency-free idle-cost bound + filled token-note
- Landed the CLI-is-sole-sufficient-surface proof (TestCLI_DualRunParity) and the loopback-gated gRPC-TCP opt-in that replaces the to-be-deleted HTTP /mcp head's network reach — both green with BOTH agent-facing heads still alive, zero new deps, zero proto change.
- Deleted both agent-facing MCP heads — the stdio forwarder (RunForwarder) and the HTTP /mcp listener (listenHTTP/HTTPHandler/httpSessionMiddleware) — plus the dead RunStdio and the entire --http-addr thread, leaving the CLI's per-verb gRPC dial path as the sole agent surface; the retained E2E + RETIRE-03 parity tests stay GREEN post-deletion, with zero proto change and zero new deps.
- Re-keyed the auto-generated README tool table to `helix <verb>` names and closed the v1.12 docgen-drift hole by wiring `go run ./cmd/docgen --check` into both `make verify-docs` and CI.
- Rewrote Helix's identity from MCP-primary to CLI-first across README/CLAUDE.md/PROJECT.md and added a Helix-CLI tool-routing decision matrix to CLAUDE.md citing the 50 real frozen verbs, with the external SMTC matrix left byte-for-byte intact.
- Closed the four non-blocking v2.0 audit items: hardened the admin listener against wildcard binds, removed the dead mergeJSONConfig helper, stopped the Bash classifier from treating a grep PATTERN as a file, and reworded get_tool_help docs CLI-first with a regenerated README.

---

## v1.11 Semantic Index Completion & P1 MCP Tools (Shipped: 2026-06-07)

**Phases completed:** 7 phases, 38 plans, 19 tasks

**Key accomplishments:**

- 1. [Rule 1 — Cycle] PriorFileFact.Symbols cannot be []extract.SymbolFact
- Go
- [Rule 2 - Missing critical functionality] Added label-allowlist carve-outs + vector priming.
- Duplicated
- Test:
- None significant.
- One-liner:
- [Minor] Stamp logic factored into stampLastCompactAt helper + PublicStampLastCompactAtForTest seam.
- 1. [Rule 2] Wire RetrievalStatus into the get_semantic_graph_status envelope
- Migration (v5 → v6)
- `QuerySymbolByName(ctx, repoID, path, name) ([]string, error)`
- Files:
- Issue:
- Production wiring of `RetrievalAccessor` + `ClusterMembershipAccessor` for `find_related_symbols`:
- Branch B
- cluster_id.go
- Types:
- 1. [Rule 2 - Design clarification] MemberCount uses all-rows pre-cap
- TestConn_Call race condition in `internal/kernel/jsonrpc/codec_test.go`
- Task 1:
- Task 1:
- Task 1 — Expand getChangeImpactGraphHelp
- Task 1 — SC#3 Static Wrapper-Consistency Gate (wrapper_consistency_test.go)
- Added WiredAccessorsBoolMap struct and WiredAccessorsForTest function to export_p1_test.go, providing a reflection-free test seam for all 10 P1 accessor nil-checks callable from daemon-package bootstrap tests.
- semP1SymbolEdgesAdapter (SymbolEdgesAccessor) and semP1ClusterMembershipAdapter (ClusterMembershipAccessor) added to semantic_wiring.go with supporting Store helper methods in effective_graph.go, closing BLOCKER-1 for the D-01a FOLD accessors.
- One-liner:
- One-liner:

---

## v1.9 Polish & Infra (Shipped: 2026-05-03)

**Phases completed:** 12 phases (46–56, including emergent 51.1), 51 plans
**Files changed:** 517 files, +57,759 / -3,120 lines (326 commits)
**Timeline:** 7 days (2026-04-24 → 2026-05-01; close 2026-05-03)
**Tag:** v1.9

**Key accomplishments:**

- Closed all 4 known LSP/tooling bugs (BUG-01..BUG-04): repomap PageRank starvation on polyglot workspaces, rust-analyzer rename via experimental/serverStatus readiness + RenameOverride QuirkAdapter, jdtls warm cache for Java integration tests in default `go test ./...`, single canonical `GrammarRegistry` shared across all consumers
- Resolved Go 1.25 + gopls linux/amd64 incompatibility on `ubuntu-latest` and converted the benchmark harness to local-only (per the project's local-only bench rule) — removed `bench.yml`, `capture-baseline.yml`, `*-github-hosted.txt` baselines
- Shipped reproducible multi-arch signed release pipeline via goreleaser (6 archives × darwin/linux/windows × amd64/arm64 with minisign signing); CGO=0 build path preserved via Phase 51.1 `//go:build cgo` stubs across treesitter/repomap/edit
- **Renamed product `serena → helix` as a hard-cut breaking change at v1.9** — binary, module path (`github.com/agenthands/helix`), env vars (`SERENA_* → HELIX_*`), config dir (`~/.serena/ → ~/.helix/`), MCP server registration name. Shipped in-binary self-upgrade (`helix update` / `helix upgrade`) with minisign verify + atomic swap + downgrade refusal + daemon-aware re-launch. EMBED-AUDIT.md manifest classifies every runtime asset
- Closed v1.2 observability gaps: 5 new Prometheus metric families (cache hit-rate lspool+repomap, repomap extract latency histogram, session lifecycle, edit outcomes — all bounded labels), 2 Grafana dashboards (`helix-overview.json`, `helix-engine.json`) with registry-driven PromQL validator, 4 runbooks (ErrCircuitOpen, deadline-timeouts, ls-crash-restart, memory-pressure-eviction), full per-MCP-tool + per-LS-call trace coverage with TRACE-AUDIT.md hygiene review and real Jaeger smoke capture
- **Phase 56 (emergent)** — surfaced and fixed that `jsonrpc.Conn.OnNotification` was never wired in production; QuirkAdapter notification handlers were silently dropped. Wired dispatch in `Worker.Start` with regression assertion + shipped `JdtlsAdapter.WaitUntilJavaReady(ctx)` deterministic gate

**Audit result:** `tech_debt` — 14/14 in-scope requirements satisfied (PKG-01 SC-3 deployment-gated on maintainer keypair + first v* tag); 25/25 cross-phase integration wires verified; 4/4 E2E flows wired.

**Deferred to v1.10 / future milestones:**

- PKG-DEFER-03/04/05: Homebrew tap, Scoop bucket, native Linux package (deb/rpm/AUR) — Phase 52 rescoped from these to self-contained-binary + in-binary self-upgrade
- Phase 51 reproducibility gate scope (snapshot-vs-snapshot, not real-release-vs-Pass-3)
- Phase 55 forwarder.tools.call span via Noop tracer (pre-v1.2 architectural limitation; application chain daemon → kernel → lspool fully verified)

**Known deferred items at close:** 5 (see STATE.md Deferred Items)

---

## v1.8 Documentation Overhaul (Shipped: 2026-04-23)

**Phases completed:** 8 phases, 19 plans, 11 tasks

**Key accomplishments:**

- 1. [Rule 3 - Blocking] Created skill adapters for health and help packages
- Commit:
- 1. [Rule 3 - Blocking] rust-analyzer entry not found
- Commit:
- INSTALL.md
- 1. [Rule 1 — Plan acceptance criterion off-by-one] `serena setup opencode` count is 1, not ≥2
- One-liner:
- 1. [Rule 3 - Blocking] Sentence insertion position adjusted to preserve rust-analyzer block byte-position

---

## v1.7 Developer Experience & Auto-Setup (Shipped: 2026-04-22)

**Phases completed:** 5 phases (34-38), 11 plans, 61 files changed, ~7,000 LOC
**Timeline:** 2 days (2026-04-21 → 2026-04-22)

**Key accomplishments:**

- One-command MCP registration (`serena setup <client>`) for 6 clients (Claude Code, VS Code, JetBrains, Claude Desktop, Gemini CLI, generic) with automatic language detection and LS pre-installation
- `get_health` MCP tool and `serena status` CLI for workspace health inspection with error-only defaults and verbose mode
- Claude Code hook auto-installation (SessionStart activation, PreToolUse nudge toward symbolic tools, Stop cleanup) with `--no-hooks` opt-out
- Smart error responses: MCP middleware enriches parameter typos and enum value errors with "Did you mean" suggestions using Levenshtein distance matching
- Progressive tool descriptions: tiered brief/detailed descriptions, `get_tool_help` for comprehensive on-demand docs, golden-file regression gating
- Lazy workspace initialization: sync.Once per workspace path, transparent activation on first MCP tool call

**Known deferred items at close:** 6 (see STATE.md Deferred Items) — all require live external environments (Claude CLI, VS Code, JetBrains) not available in CI

---

## v1.5 Typed Errors & Hardening (Shipped: 2026-04-15)

**Phases completed:** 3 phases (22-24), 12 plans
**Files changed:** 95 files, +7,199 / -534 lines
**Timeline:** 1 day (2026-04-14 → 2026-04-15)

**Key accomplishments:**

- Created `internal/errors/` package with 7 error kinds (NotFound, InvalidArgs, NoWorkspace, Unsupported, Internal, CircuitOpen, Timeout), builder pattern, JSON serialization, and cause-chain wrapping via errors.Is/As
- Migrated existing sentinels (ErrCircuitOpen, ErrSessionExpired, ErrLSCrashed) into unified error taxonomy with backward-compatible re-exports, then removed all deprecated bridge aliases
- Migrated all 38+ MCP tools across 10 packages (symbols, edit, fileops, diag, memory, workflow, profile, MCP core) from raw `fmt.Errorf` strings to typed `serr.New`/`serr.Wrap` errors
- Added inline input validation to all 24 kernel tool handlers — empty-string checks on required fields before any workspace or LS work begins
- Upgraded three-band error tests with `extractKind` helper and `expectedKind` struct field for Kind-level assertions, plus 4 typed error golden files (invalid_args, no_workspace, not_found, unsupported)

**Tech debt accepted:**

- 3 golden files deferred (circuit_open, timeout, internal runtime) — cannot trigger deterministically without live LS
- unsupported.golden captures raw error (lspool not yet using serr.Unsupported)
- 2 internal flow-control fmt.Errorf in fileops (errLimitReached, never reaches MCP)

---

## v1.4 Integration Testing v2 (Shipped: 2026-04-14)

**Phases completed:** 4 phases (18-21), 11 plans
**Lines changed:** +5,615 / -51 across 128 files
**Timeline:** 4 days (2026-04-11 → 2026-04-14)

**Key accomplishments:**

- Extracted importable test harness (`test/harness/`) with Runner, golden store, fixture helpers, and build tag taxonomy (`integration`, `llm`, `llmjudge`)
- Protocol oracle tests — MCP handshake, tools/list validation, session isolation, reconnect resilience
- Contract oracle tests — JSON Schema Draft 2020-12 validation, 23 golden output files, 6 error category contracts, tool selectability heuristics
- Scenario oracle matrix — 14+ full-cycle agent workflow tests across Go, Python, TypeScript, C++, Swift, Zig, JavaScript, PHP, SQL, Markdown, polyglot, unsupported, collision, and degraded fixtures
- LLM behavioral tests — tool selection (33 tools), disambiguation (11 pairs), output interpretation with multi-provider support (Anthropic + DeepSeek)
- Judge scoring infrastructure — structured rubrics (5 dimensions), transcript pipeline, aggregate reporting, never blocks merge

---

## v1.3 Documentation Catchup (Shipped: 2026-04-11)

**Phases completed:** 2 phases, 4 plans, 7 tasks

**Key accomplishments:**

- 1. [Rule 1 - Bug] Fixed inaccurate tool count claim
- Commit:
- 1. [Rule 1 - Bug] Corrected serena_tool_duration_seconds label documentation
- Go binary install guide with per-agent MCP config sections for Claude Code, Codex, OpenCode, Cursor, Gemini CLI, Antigravity, and HTTP mode

---

## v1.2 Performance & Production Hardening (Shipped: 2026-04-10)

**Phases completed:** 15 phases, 56 plans, 78 tasks

**Key accomplishments:**

- Python code migrated to legacy/, Go module initialized with cobra CLI skeleton producing single serena binary
- 1. [Rule 1 - Bug] Fixed Unix socket path length on macOS
- MCP server with official SDK, dummy tools (ping/echo/activate_project), gRPC forwarder-daemon IPC, stdio forwarder with auto-start, and Streamable HTTP endpoint
- Full LSP 3.17 type generation from metaModel.json (324 structs, 216 union types) plus Content-Length framed JSON-RPC 2.0 codec with session-prefixed ID routing
- 6 pure Go file operation tools (read, write, list, find, search, replace) with symlink-aware path security and MCP registration
- Multi-LS worker pool with share-until-dirty policy, adaptive TTL, circuit breaking, pressure eviction, and workspace-scoped language detection
- 1. [Rule 2] Added LeaseProvider abstraction
- Tree-sitter body extraction for 4 languages with 6 symbol editing MCP tools and automatic post-edit diagnostic verification
- 52-language embedded registry with YAML deep-merge overlay and three-tier LS installer (PATH/download/error)
- Markdown-based memory CRUD with SQLite FTS5 search, project/global scoping, and fsnotify auto-reindex
- Go skill/plugin interfaces with init()-based registry and YAML-driven context/mode composition for tool filtering
- QuirkAdapter interface replacing hardcoded LanguageQuirks with per-language behavioral hooks, wired to language registry for LS resolution
- Memory skill wrapping MemoryStore as 7 MCP tools and workflow skill with onboarding project analysis and session handoff via Caddy-style init() registration
- Profile/Mode types extending skill specs, 5 agent profile YAMLs and 4 mode YAMLs embedded via go:embed, loader with override merging and 6 tests
- switch_mode and get_token_budget MCP tools via profile skill with per-session mode tracking and transition validation
- Profile selection wired through 4-layer koanf config precedence with ProfileFilterMiddleware applying tool filtering and description overrides on MCP tools/list responses
- 1. [Rule 3 - Blocking] daemon.New signature change required caller updates
- 10 integration tests proving daemon bootstrap registers 38 tools, initializes 7 skills, resolves profiles, and all 6 E2E flows work end-to-end
- In-process daemon test harness with MCP InMemoryTransports, Go fixture project (8 symbols), and build-tag-gated self-tests
- Commit:
- 1. [Rule 1 - Bug] Corrected tool parameter names
- 1. [Rule 1 - Bug] Worker process killed when request context completes
- 1. [Rule 3 - Blocking] Edit tools did not resolve relative paths against workspace root
- 1. [Rule 3 - Blocking] Added requireLS helper inline
- 1. [Rule 2 - Missing] Added requireLS to helpers.go instead of inline
- 1. [Rule 3 — Blocker] switch_mode arg name mismatch in harness (08-01 inherited)
- 1. [Rule 1 - Bug / T-08-08 mitigation] Data race on SessionInfo under concurrent switch_mode + tool invocation
- File:
- 1. [Rule 3 - Blocking] Parity assertion source: session.ListTools → Registry().Names()
- 1. [Rule 3 - Blocking] Bumped worker pool cap in `bench_helpers_test.go`
- Task 1 — Cold + warm LSP indexing benchmarks (`test/bench/lsp_index_bench_test.go`)
- BenchmarkMemory walks 4 D-06 scenarios with dual Go/kernel RSS reporting and per-scenario pprof heap snapshots, locking the v1.1 memory baseline for BENCH-04.
- Commit:
- Stdlib-only obs package with noop Provider, zero-alloc trace-aware slog ContextHandler, ObservabilityConfig schema, and --admin-addr CLI flag wired through runDaemon and runForwarder.
- OBS-03 (loopback admin listener)
- OBS-06 measured and locked: obs.ContextHandler adds 0 alloc/op over a bare slog.JSONHandler on the no-span fast path, with a committed v1.2 baseline for Phase 11 to measure against.
- `github.com/prometheus/client_golang v1.23.2`
- 1. [Rule 3 - Blocking] Added go.sum entry for prometheus testutil dependency
- 1. [Rule 3 - Blocking] daemon.go observability construction order
- 1. otelgrpc not pinned in go.mod
- RED:
- 1. [Rule 3 - Blocking] fileops/diag tracer parameter approach
- BenchmarkTracingOffPath
- 1. [Rule 3 - Blocking] DegradationConfig struct created in Task 1 instead of Task 2
- 1. [Rule 3 - Blocking] Bench tests missing BudgetFunc parameter
- Go-only CHANGELOG.md with v1.0 MVP, v1.1 Integration Testing, and v1.2 Performance & Production Hardening entries dated from milestone data
- Commit:

---

## v1.1 Integration Testing (Shipped: 2026-04-09)

**Phases completed:** 3 phases (6-8), 11 plans, 64 commits
**Lines changed:** 12,615 insertions, 1,149 deletions across 116 files
**Timeline:** 2 days (2026-04-08 → 2026-04-09)

**Key accomplishments:**

- Integration test harness with MCP `NewInMemoryTransports()` and HTTP `StreamableClientTransport` — in-process daemon spawning, LS readiness polling, build-tag-gated separation (`//go:build integration`)
- Go dogfooding suite exercising all 38 MCP tools against Serena's own 25,779 LOC codebase with strict structural assertions
- Edit round-trip testing for all 6 symbol editing tools (replace body, insert before/after, rename, safe delete, verify edit) with tree-sitter body surgery validated end-to-end
- Multi-language fixture projects for Python, TypeScript, Java, and Rust with known symbols, cross-file reference chains, and per-language LS skip logic via `requireLS`
- Profile/mode contract tests using golden file pattern (19 `.tools.golden` files) with `-update` flag and diff-reviewable tool visibility assertions
- Three-tier concurrency testing: `t.Parallel()` scenario stress + `errgroup` fan-out on worker pool + `testing/synctest` deterministic unit tests for `WorkerMetrics` decay
- Three-band error path coverage: category matrix (shared harness) + exhaustive per-tool for 9 destructive tools + thin smoke for read-only tools (30 cases total)
- **10 production bugs found and fixed during dogfooding** — including `ProfileFilterMiddleware` unfiltered initial sessions (access control), `SessionInfo` data race on concurrent mode switches (fixed with `sync.RWMutex` + Snapshot pattern), Language field routing causing circuit breaker failures, and 5 edit tool bugs (path resolution, DocumentSymbol ranges, rename DocumentChanges, SafeDelete SelectionRange, two-phase lease pattern)
- `make test-stress` target for elevated-count concurrency fan-out in CI

---

## v1.0 MVP (Shipped: 2026-04-08)

**Phases completed:** 5 phases, 20 plans, 91 commits
**Lines of Go:** 25,779 across 21 packages
**Timeline:** 2 days (2026-04-07 - 2026-04-08)

**Key accomplishments:**

- Go rewrite with daemon skeleton, MCP runtime, gRPC forwarder, and Streamable HTTP transport — single binary distribution
- Full LSP 3.17 type generation from metaModel.json (324 structs, 216 union types) plus JSON-RPC 2.0 codec with session-prefixed ID routing
- 24 MCP tools for symbol retrieval (9), symbol editing (6), file operations (6), and diagnostics (3) — backed by live language servers with tree-sitter body surgery
- Multi-LS worker pool with share-until-dirty policy, adaptive TTL, circuit breaking, and platform-aware pressure eviction
- 52-language embedded registry with YAML deep-merge overlay and three-tier LS installer (PATH/download/error)
- Markdown-based memory system with SQLite FTS5 search, 7 MCP tools, and fsnotify auto-reindex
- Skill/plugin interfaces with Caddy-style init() registration, onboarding workflow, and session handoff
- 5 agent profiles (Claude Code, Codex, IDE assistant, CI bot, full), 4 modes (read/edit/review/admin), token budget awareness, profile filtering middleware
- Full daemon bootstrap wiring: 38+ callable MCP tools, centralized registration, fail-fast startup, kernel-first shutdown

---
