# ARCHITECTURE — Helix v1.10 Live Semantic Index integration

**Confidence:** HIGH for integration points and middleware ordering (verified against `internal/daemon/daemon.go` and SPEC §5/6/24/36/39); MEDIUM for build-order edge cases (depends on whether DuckDB driver is CGO-required — see open question 1).

---

## 1. Where `internal/semantic/` sits — Layer 1.5

**Decision:** New layer **between kernel (Layer 1) and skills (Layer 2)**, not part of the kernel.

**Rationale & dependency direction:**

```
Layer 0  internal/mcp, internal/daemon, internal/forwarder
Layer 1  internal/kernel/{lspool, symbols, edit, fileops, diag, jsonrpc, health, help}
         internal/repomap, internal/fuzzy, internal/treesitter, internal/workspace
Layer 1.5 internal/semantic/{store, indexer, live, extract, resolve, lspenrich,
            graph, rank, cluster, retrieve, typeresolve, tools, testutil}
         internal/phasegraph
         internal/guardrails
Layer 2  internal/skill/{memory, repomap, workflow, semantic (NEW)}
Layer 3  internal/profile, internal/config, internal/cli
```

**Allowed import edges (kept acyclic):**

| From | To | Why |
|---|---|---|
| `internal/semantic/lspenrich` | `internal/kernel/lspool` (lease API only) | Reuses worker pool — see §4 |
| `internal/semantic/extract` | `internal/treesitter` (GrammarRegistry) | Single canonical registry per BUG-04 |
| `internal/semantic/resolve` | `protocol/gen` (LSP types) | Same as repomap enrichment today |
| `internal/semantic/tools` | `internal/mcp` (tool registration types) | Tool definitions |
| `internal/skill/semantic` | `internal/semantic` (SemanticService interface) | Skill is the thin MCP adapter |
| `internal/guardrails` | `internal/semantic` (read freshness, scores) | Policy needs graph state |
| `internal/eval` | `internal/cli` subprocess + `internal/semantic` (modes config only) | See §7 |

**Forbidden edges (would create cycles):**
- `internal/kernel/edit → internal/semantic` — broken via callback (see §3)
- `internal/repomap → internal/semantic` — broken via lookup function injected from daemon (see §8)
- `internal/skill/repomap → internal/semantic` — same; daemon wires `SetSemanticLookupFn` analogous to existing `SetEnrichFn`

**New components vs modified:**
- **NEW:** all of `internal/semantic/*`, `internal/phasegraph/`, `internal/guardrails/`, `internal/eval/`, `internal/skill/semantic/`
- **MODIFIED:** `internal/daemon/daemon.go` (add bootstrap steps 2.5, 12e–12i, 14d), `internal/kernel/edit/{rename,replace,delete,insert}.go` (post-success hook), `internal/skill/repomap/skill.go` (semantic lookup callback), `internal/repomap` package (no source change — daemon wires alternate enrich path), `internal/kernel/health` (add semantic section to report), `cmd/helix/main.go` (no source change; CLI subcommands added under `internal/cli/eval/`).

---

## 2. DuckDB lifecycle

**Bootstrap phase (in current imperative `daemon.New`):** new step **2.5** — between language registry (step 1/2) and worker pool/kernel creation (steps 4/5). DuckDB open is fail-fast (per SPEC §29.1, store corruption recovers via quarantine + auto-reindex, but driver open itself must succeed); kernel can refer to a `*semantic.Service` handle for `get_health` aggregation.

**Owner:** `internal/semantic.Service` owns the `*sql.DB` (or driver-native `duckdb.Connector`). Daemon holds `service *semantic.Service` as a struct field next to `kernel`.

**Open path:**
```go
// daemon.go step 2.5 (new):
semSvc, err := semantic.New(semantic.Config{
    Path: filepath.Join(globalDir, "semantic.duckdb"),
    Cfg:  cfg.SemanticIndex,
}, logger, observability.Metrics(), observability.Tracer())
if err != nil { return nil, fmt.Errorf("opening semantic store: %w", err) }
```

**Disable path:** if `cfg.SemanticIndex.Enabled == false`, `semantic.New` returns a `nil` service and all callsites guard with `if d.semantic != nil`. Preserves the **single-binary + CGO=0 fallback policy**: when `treesitter.Available == false` (existing CGO=0 guard at daemon.go:199-203), set `cfg.SemanticIndex.Enabled = false` automatically with a warn log — semantic index is undefined without tree-sitter extraction. No second hard-fail point.

**Shutdown ordering** (current is *kernel-first* via `errgroup` — kernel.Run is the first `g.Go` at daemon.go:481-483; sockets/HTTP shut down on `ctx.Done()` simultaneously):

The new ordering: **(1) accept no new work → (2) drain live update queue + commit overlay flush → (3) close DuckDB → (4) kernel shuts down LS workers**.

- Add `g.Go(func() error { return d.semantic.Run(gctx) })` running the live-update + compaction goroutines.
- The live queue's `Run` method on context cancel: stops accepting new events, drains pending overlay writes within `live_updates.compact_after_idle_ms`-sized timeout, then `store.Close()` (DuckDB checkpoint).
- Kernel shutdown happens *after* `g.Wait()` returns in `d.shutdown()` (daemon.go:524). DuckDB close must happen before kernel's `Run` exits to ensure overlay flush isn't racing LSP enrichment lease releases.

**Compaction timing on shutdown:** flush only — do not run compaction. Compaction during shutdown risks aborting mid-snapshot and is unnecessary because compaction is not durability (overlay alone is durable).

---

## 3. Live overlay + edit tool hook (SPEC §24.4)

**Decision:** **Per-tool hook in `internal/kernel/edit`, called via injected callback** — not middleware, not daemon interceptor.

**Why not middleware:** middleware operates on MCP request/response envelopes; it does not know which files an edit tool actually wrote. Brittle to re-parse responses for `ChangedFiles`.

**Why not daemon interceptor:** daemon doesn't see individual tool result structs.

**Implementation:** Add a `PostEditHook` field to `edit.RegisterTools`:

```go
// CHANGE in internal/kernel/edit/tools.go:131
func RegisterTools(server *mcp.SerenaMCPServer, k *kernel.Kernel,
    extractor *BodyExtractor, diagStore *diag.DiagnosticStore,
    wsKeyFn func() workspace.WorkspaceKey,
    postEditHook func(ctx context.Context, changedFiles []string, source string)) // NEW

// Inside each tool handler, after a successful write, just before returning OK:
if postEditHook != nil {
    postEditHook(ctx, result.ChangedFiles, "helix_edit")
}
```

**Daemon wiring:**
```go
var editHook func(context.Context, []string, string)
if d.semantic != nil {
    editHook = d.semantic.LiveQueue().EnqueueChangeEventsFunc()
}
edit.RegisterTools(mcpServer, k, bodyExtractor, diagStore, wsKeyFn, editHook)
```

**Coupling property:** kernel/edit gains exactly one new function-typed parameter. It does not import `internal/semantic`. Hook is `nil`-safe. Mirrors `RepoMapSkill.SetEnrichFn` (daemon.go:298) — clean inversion-of-control.

**Bonus seam — replace_in_file / fuzzy_edit:** thread the same hook through `fileops.RegisterTools` (currently daemon.go:243).

---

## 4. Graph cache + worker pool sharing (no parallel pool)

**Decision:** Reuse `kernel.Pool().AcquireLease(...)` exactly as `enrichRepoMapFromLSP` does today (daemon.go:765-825). No second pool.

**Interface (semantic takes a small interface, not the kernel struct):**

```go
// internal/semantic/lspenrich/budget.go
type LeaseAcquirer interface {
    AcquireLease(ctx context.Context, sessionID string,
        key workspace.WorkspaceKey, dirty bool) (*lspool.WorkerLease, error)
    ReleaseLease(sessionID string)
}
```

Daemon passes `k.Pool()` into `semantic.Service` at construction. Semantic never imports `internal/kernel` — only `internal/kernel/lspool` (lease types) and `internal/workspace`.

**Backpressure:** LSP enrichment is a passive background reader; never claims `dirty=true`; uses session IDs prefixed with `lspenrich-` (matching `enrich-repomap` and `diag-` conventions at daemon.go:251, 766).

**Priority ordering (per SPEC §14.3):**
1. Foreground MCP tool calls — implicit priority via `LazyInitMiddleware` + `TelemetryMiddleware` per-tool budgets.
2. High-priority revalidation (file just edited via `ChangeHelixEdit`) — head-of-line, 5s budget.
3. Background enrichment (initial index, idle revalidation) — tail of queue, yields on context cancel.

Single goroutine `Service.runEnrichmentLoop` pulls from a priority queue (`container/heap`). **Lease acquisition itself is the natural backpressure** — when foreground tools hold leases, enrichment waits. No semaphore needed at the semantic layer.

**Cache:** `internal/semantic/graph/cache.go` (in-memory, per repo) is separate from the LS worker pool. Loaded from DuckDB on first query, repaired by `internal/semantic/live/update.go` events.

---

## 5. Pipeline DAG migration (SPEC §39)

**Decision:** **Strangler fig**, not big-bang. v1.10 ships `internal/phasegraph/` library + uses it for the **new graphs** (semantic-index, live-update, eval) immediately. Daemon bootstrap stays imperative in v1.10; migrated opportunistically later.

**Reasoning:**
- v1.9 closed a clean `daemon.New` flow with 16 numbered steps + git-blame continuity (daemon.go:206 comment). Rewriting it touches every test that constructs a daemon.
- DAG validation value (cycle/missing-dep detection) accrues mostly to the *new* graphs.
- Bootstrap order today is in one file, reviewable; the risk isn't ordering bugs, it's coupling.

**Boundary:**
- `internal/phasegraph` is a generic library: PhaseSpec, PhaseGraph, ValidatePhaseGraph, RunPhaseGraph (per SPEC §39.2/3/8).
- New consumers: `internal/semantic/indexer/planner.go`, `internal/semantic/live/update.go`, `internal/eval/runner.go`.
- Bootstrap migration deferred. TODO at top of daemon.go: `// TODO(v1.11): migrate to phasegraph.RunPhaseGraph(BootstrapPhases)`.

---

## 6. MCP middleware ordering (with guardrail middleware)

**Current LIFO install order** (daemon.go:349-380):
1. `InstallMiddleware` → `TelemetryMiddleware` then `ProfileFilterMiddleware` (step 14)
2. `InstallSuggestionMiddleware` (step 14b)
3. `InstallLazyInitMiddleware` (step 14c, must be LAST so it runs FIRST)

**Execution order on incoming request (LIFO reverses):** `LazyInit → Suggestion → ProfileFilter → Telemetry → handler`

**Decision:** install guardrail middleware **between SuggestionMiddleware and LazyInitMiddleware** — new step **14b.5**.

**New install order:** Telemetry+ProfileFilter → Suggestion → **Guardrail (NEW)** → LazyInit (last)

**New execution order:** `LazyInit → Guardrail → Suggestion → ProfileFilter → Telemetry → handler`

**Why this position:**
- **After LazyInit:** guardrail policy may need to read semantic graph state (freshness, score status) — workspace must be activated first.
- **Before ProfileFilter:** guardrail must run for every tool call regardless of profile description overrides.
- **Before Suggestion:** guardrail decisions are not parameter-typo errors.

**Invariant preserved:** `LazyInit must remain installed last (executes first)`. CLAUDE.md > Middleware Execution Order unchanged. Guardrail does NOT need to run before LazyInit — guardrail evaluation requires workspace state.

**Telemetry classification extension:** add new outcome classes `guardrail_blocked` and `guardrail_warned` to `TelemetryMiddleware`.

**Mode gating (SPEC §30.2):** orthogonal to middleware order. Implemented inside `GuardrailPolicy.Evaluate` by reading active mode from `SessionInfo`.

---

## 7. Eval harness placement & process model

**Decision:** **Out-of-process by default** (subprocess MCP forwarders), with **in-process baseline mode** for fast local CI.

**Why:**
- `baseline` mode (no Helix) must NOT have Helix tools available — cleanest if the agent talks to a different process or none at all.
- `native` / `semantic` / `semantic_guarded` need real production-equivalent profile/mode gating, middleware stack, and tool registration. Spawning a real `helix daemon` subprocess per task matches a real Claude Code session.
- Avoids circular deps: `internal/eval` does not import the daemon's tool registration code; only `internal/cli` (consumer-facing) to invoke `helix setup` / `helix daemon`, plus `internal/semantic.Config` types for mode-specific configuration.

**Process layout per task:**
```
internal/eval/runner.go
  └── spawns: helix daemon (subprocess, isolated config dir under .helix/eval/<task>/)
       └── baseline: --profile=baseline (new minimal profile, no Helix tools)
       └── native: --profile=full + semantic_index.enabled=false
       └── semantic: --profile=full + semantic_index.enabled=true + guardrails.enforcement=off
       └── semantic_guarded: --profile=full + semantic_index.enabled=true + guardrails.enforcement=warn
  └── spawns: agent process (Anthropic/DeepSeek client, similar to test/oracle/llm/)
       └── connects to daemon via stdio forwarder (helix forwarder ...)
```

**Reuse existing oracle infrastructure:** `test/oracle/llm/` (multi-provider LLM tests, judge scoring) provides Anthropic + DeepSeek client wrappers, judge prompts, recorder patterns. Move/refactor to `internal/eval/agents/` and `internal/eval/scoring/`.

**In-process variant:** `internal/eval/runner_inproc.go` uses `daemon.New` + `mcpServer.SDK().Connect(ctx, transport, nil)` (same pattern as `test/harness/Runner`). Useful for `make eval-quick`, not for cost/latency measurements.

---

## 8. Compatibility with `get_repo_map` / `get_context` (strangler fig)

**Decision:** **Lookup-callback inversion**, mirroring `repomapSkill.SetEnrichFn` (daemon.go:296-307).

```go
// internal/skill/repomap/skill.go (new method)
type SemanticLookup interface {
    GraphScores(ctx context.Context, repoID string) (*ScoresResult, error)
    Available() bool
}

func (s *RepoMapSkill) SetSemanticLookup(lookup SemanticLookup) { ... }
```

**Daemon wiring (new step 12e):**
```go
if rs := repomapSkill.GetRepoMapSkill(); rs != nil && d.semantic != nil {
    rs.SetSemanticLookup(d.semantic.RepoMapAdapter())
}
```

**Inside `get_repo_map` handler:**
```go
if s.semanticLookup != nil && s.semanticLookup.Available() {
    if scores, err := s.semanticLookup.GraphScores(ctx, repoID); err == nil {
        // SPEC §24.1: use persisted graph scores, clusters, effective overlay
        return renderFromSemantic(scores, ...)
    }
    // Fall through on error — never block on semantic
}
return s.renderTreeSitterTags(ctx, ...) // existing path unchanged
```

**Property:** zero source change to `internal/repomap` (the engine). Skill-level decision. Fallback automatic when `semantic == nil`, `Available()` returns false, or lookup errors.

`get_context`: same pattern. `analyze_blast_radius` (`internal/kernel/symbols`): same callback through `symbols.RegisterTools` (daemon.go:241). `get_health` (daemon.go:254): nullable `semanticStatus func() *SemanticHealthSection`, populated from `d.semantic.HealthSnapshot()`. Output structure matches SPEC §24.5.

---

## Suggested build order (honoring dependency edges)

| # | Phase (SPEC §32 numbering) | Files touched | Rationale |
|---|---|---|---|
| 1 | Phase 0 — Schema & Store | `internal/semantic/{store,types,config}` | Foundation; no other deps |
| 2 | Phase 12 (early) — phasegraph library | `internal/phasegraph/*` | Used by Phases 1, 2, 10 |
| 3 | Phase 1 — Tree-sitter Extraction | `internal/semantic/{extract,resolve}` | Depends on `internal/treesitter.GrammarRegistry` |
| 4 | Phase 2 — Live Overlay Updates | `internal/semantic/live/*` + edit-tool hook | First daemon integration (§3) |
| 5 | Phase 3 — LSP Enrichment | `internal/semantic/lspenrich/*` | Depends on lspool lease API (§4) |
| 6 | Phase 4 — Graph Scores | `internal/semantic/{graph,rank}/*` | In-process; pure Go |
| 7 | Phase 5 — Clustering | `internal/semantic/cluster/*` | Builds on graph |
| 8 | Phase 6 — 10 new MCP Tools | `internal/semantic/tools/*` + `internal/skill/semantic/*` | Existing skill registration pattern |
| 9 | Phase 7 — Existing Tool Integration | repomap callback, symbols callback, health | Strangler fig (§8) |
| 10 | Phase 8 — Compaction & Retention | `internal/semantic/indexer/compaction.go` | Background worker; reuses store |
| 11 | Phase 11 — Type Resolution | `internal/semantic/typeresolve/*` | Improves edge precision; doesn't gate other phases |
| 12 | Phase 9 — Guardrails + Middleware | `internal/guardrails/*`, `internal/mcp/guardrail_middleware.go`, daemon step 14b.5 | §6 |
| 13 | Phase 10 — Eval Harness | `internal/eval/*`, `internal/cli/eval/*` | §7; depends on `--profile=baseline` |

**Order rationale:** store first (durability anchor), extract → live update (write path), enrichment (LS reuse), ranking + clustering (in-process compute), tools (user-visible), strangler integration last (lowest risk), guardrails + eval (depend on a working semantic stack).

---

## Data-flow change: live update path

```
agent calls replace_symbol_body
  → middleware chain: LazyInit → Guardrail → Suggestion → ProfileFilter → Telemetry
  → kernel/edit/replace.go writes file via fileops
  → on success: postEditHook(ctx, []string{"path/to/file.go"}, "helix_edit")
       └── semantic.LiveQueue.Enqueue(SourceChangeEvent{Kind: ChangeHelixEdit, Path: ...})
  → tool returns success response to agent

[asynchronously, semantic.Service goroutine pool]
LiveQueue
  → coalesce (debounce 250ms per SPEC §16.2)
  → parse changed file with tree-sitter (semantic/extract)
  → diff old effective facts vs new (semantic/store: LoadEffectiveFileFacts)
  → BeginOverlayTx → UpsertSymbols/References/Edges + Tombstones → Commit
  → repair graph cache (semantic/graph/repair.go) — incremental PageRank repair
  → mark affected scores/clusters as approximate (SPEC §18.4)
  → enqueue HIGH-priority LSP revalidation (head-of-line for ChangeHelixEdit)

[asynchronously, lspenrich worker]
  → AcquireLease (shared with foreground; backpressure via pool)
  → fetch hover/references/diagnostics
  → write LSP-confirmed edges with confidence=1.0 to overlay
  → mark validation_state=validated

[asynchronously, idle compaction goroutine]
  → after compact_after_idle_ms (5s default)
  → BeginSnapshot → merge overlay into new committed snapshot
  → ClearOverlay on commit success
```

**Subsequent `get_repo_map` call:** reads through `semanticLookup.GraphScores()` → DuckDB effective query (snapshot ⊕ overlay) → up-to-date ranking *without* re-extracting tree-sitter tags. Fallback to existing tree-sitter path is automatic if semantic is disabled or scores are missing.

---

## Files referenced

- `internal/daemon/daemon.go` — bootstrap; lines 199-203 (CGO=0 guard), 241-255 (kernel tool registration), 296-336 (repomap skill wiring), 349-380 (middleware install order), 481-525 (Run/errgroup/shutdown), 765-825 (existing LSP enrichment pattern)
- `internal/kernel/edit/tools.go:131` — `RegisterTools` signature to extend
- `internal/kernel/edit/{rename,replace,delete,insert}.go` — per-tool hook callsites
- `internal/kernel/lspool/lease.go` — lease API consumed by semantic enrichment
- `internal/skill/repomap/skill.go` — pattern for `SetSemanticLookup`
- `internal/mcp/lazy_init.go:106-108` — middleware-order invariant
- `SPEC-DRAFT.md` §5, §6, §24.4, §32, §36, §37, §39

---

## Open questions / risks

1. **DuckDB driver CGO requirement.** `marcboeker/go-duckdb` (and the official `duckdb/duckdb-go` v2 successor) require CGO. Consistent with Helix's CGO=1 norm (tree-sitter), but the CGO=0 placeholder binary must hard-disable `semantic_index.enabled` automatically. No pure-Go DuckDB exists.
2. **Test harness impact.** `test/harness/Runner` constructs daemons in-process. With semantic enabled, every test creates a DuckDB file. Use `:memory:` for harness tests, real file only for end-to-end eval and integration tests that exercise compaction.
3. **Bootstrap step renumbering.** daemon.go preserves step numbers in comments for git-blame continuity. Decimal additions (2.5 / 12e–12i / 14b.5) follow existing convention.
4. **Forwarder gRPC propagation.** Live update events from edit tools are in-process — no gRPC boundary. Eval harness subprocess setup needs programmatic profile + semantic_index config (likely via `HELIX_CONFIG_PATH` per existing 4-layer precedence; confirm).
