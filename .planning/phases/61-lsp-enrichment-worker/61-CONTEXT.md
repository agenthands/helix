# Phase 61: LSP Enrichment Worker - Context

**Gathered:** 2026-05-05
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 61 ships the **async LSP enrichment worker** that drains Phase 60's
`internal/semantic/live/lspqueue` and promotes tree-sitter facts to
LSP-validated evidence — without burying foreground tool calls or regressing
v1.9 LS readiness invariants.

**Phase 61 ships:**

1. **`LeaseAcquirer` interface** in `internal/semantic/lspenrich/` (or a
   sibling under `internal/semantic/`). The single seam between semantic and
   the existing `internal/kernel/lspool` — semantic does NOT import
   `internal/kernel`, only `internal/kernel/lspool` types and
   `internal/workspace`. Mechanical enforcement via the existing
   `internal/lint/nokernel2semantic` analyzer.

2. **2-lane priority queue.** Producer maps `helix_edit` + agent-requested →
   high lane; watcher / manifest-scan → background lane. Consumer drains
   high before background via `select` over two typed channels. The current
   single-channel `lspqueue.Queue` (Phase 60) becomes one of the two
   channels (or grows a sibling); the producer-side wiring in
   `live/handler/handler.go:144-146` is updated to choose lane by
   `SourceChangeKind`.

3. **Configurable worker concurrency cap, default 1.** New config key
   `semantic_index.lsp_enrichment.max_concurrent_workers` (default `1`)
   added to `internal/config/defaults.go` and the existing
   `lsp_enrichment.*` block. One worker goroutine per cap slot, all
   draining the same 2-lane queue.

4. **Voluntary preemption between cascade steps.** Worker calls
   `LeaseAcquirer.ForegroundBusy(wsKey) bool` between LSP calls in the
   §14.4 cascade. On `true`, abandons the remainder of the cascade for the
   current file and marks it `partial:true, partial_reason:"preempted"` so
   the next edit / manual refresh re-triggers it. New config key
   `semantic_index.lsp_enrichment.yield_check_window_ms` (default `200`)
   bounds how recent a foreground lease has to be to count as "busy".

5. **`ForegroundBusy(wsKey) bool` on `LeaseAcquirer`**, implemented inside
   `internal/kernel/lspool` by tracking timestamps of non-enrichment
   `AcquireLease` calls per `wsKey` under the existing pool mutex.
   "Non-enrichment" is identified by `sessionID` prefix (`lsp-enrichment:*`
   sessions are excluded from the busy signal — see Claude's Discretion
   below).

6. **Bulk-update suppression at producer.** `live/handler/handler.go`
   conditionalizes `LSPQueue.Enqueue` on `event.Kind != ChangeBulkUpdate`.
   On `ChangeBulkUpdate`, the handler instead marks every affected file
   `semantic_pending` in the overlay (new
   `tx.MarkFileSemanticPending(repoID, path, reason="bulk_update_pending")`
   API on the overlay tx, persisting via the existing `partial_reason`
   columns on `semantic_files`). No fanout into the queue.

7. **No auto-recovery for `semantic_pending` files.** Phase 61 ships zero
   background drainer. Pending files unblock only via:
   - The next per-file edit (helix_edit / fsnotify / manifest scan)
     enqueues that single file normally into the high or background lane.
   - Phase 64's `refresh_semantic_graph(paths=...)` MCP tool (out of
     scope here) drives manual catch-up.

8. **Full SPEC §14.4 cascade.** All 6 LSP capabilities ship in v1:
   - `documentSymbol` — symbol shape + container hierarchy.
   - `hover` — symbol type info on prioritized symbols.
   - `callHierarchy` — depth=`max_call_hierarchy_depth` (default 2),
     produces `CALLS` edges.
   - `typeHierarchy` — depth=`max_type_hierarchy_depth` (default 2),
     produces `EXTENDS` / `IMPLEMENTS` edges.
   - `implementations` — produces `IMPLEMENTS` edges.
   - `references` + `go-to-definition` (on references only — never on
     definitions per SPEC line 1261) — produces `RESOLVES_TO` edges.
   All edges land in `semantic_live_overlay_edges` with
   `validation_state="validated"`, `confidence=1.0`,
   `source="lsp.<call>"`. Phase 62 consumes them as the high-confidence
   tier of the §38.2 ladder.

9. **All-language enrichment surface, gated only by LS readiness.** Worker
   honors:
   - `JdtlsAdapter.WaitUntilJavaReady(ctx)` for Java
     (`internal/kernel/lspool/quirks.go:427`).
   - `experimental/serverStatus.quiescent=true` for rust-analyzer
     (`quirks.go:165`).
   - `AcquireLease` success for every other language (the lease-acquire
     itself is the gate — circuit breaker open or LS not installed
     surfaces as job-drop).
   Effective enrichment surface = intersection of (Phase 59 first-class
   extraction tier × LS readiness × LS installed). Files for ungated
   languages drop with `outcome=dropped`, no error logged per file (one
   summary log per worker startup).

10. **Per-file budget enforcement.** Existing `lsp_enrichment.*` config
    keys (timeout_per_file=5s, timeout_total=120s,
    max_symbols_per_file=200, max_references_per_symbol=1000,
    max_references_per_file=5000, max_call_hierarchy_depth=2,
    max_type_hierarchy_depth=2) are honored by a small `Budget` value
    type the worker passes through the cascade. On exhaustion: file
    marked `partial:true, partial_reason:"budget exhausted"`, remaining
    cascade steps skipped, file remains queryable with the partial
    facts already written.

11. **Overlay write via existing `BeginOverlayTx`.** Worker writes
    enriched symbols, references, edges, diagnostics, and invalidations
    through the Phase 60 `OverlayTx` API. Each enrichment commits one tx
    per file; the per-tx `overlay_epoch` (Phase 60 D-04) advances per
    write. Invalidations are computed but `WriteInvalidations` remains
    a typed no-op stub until Phase 62 wires the consumer.

12. **Stress-test fixture for ENRICH-05.** A `git checkout` storm + 100-
    file edit burst with interactive `get_symbols` calls every 200ms for
    30s — interactive p95 must stay under the foreground-tool budget
    while enrichment runs. Languages: Go (canonical) + Java (worst
    case) on a fixture repo.

**Out of scope (deferred):**

- **Graph cache repair / score invalidation consumer** — Phase 62.
  Phase 61 calls `WriteInvalidations` and `MarkAffectedScoresAndClusters`
  with the diff already computed; Phase 62 fills the consumer.
- **PageRank-bias / public-symbols-first re-prioritization** — Phase 62.
  Phase 61's high lane is for helix_edit + agent-requested only;
  rank-driven prioritization needs the graph engine to exist.
- **`refresh_semantic_graph(paths=...)` MCP tool** — Phase 64. Phase 61
  ships the underlying enrichment path but no MCP wrapper.
- **`get_health` watcher/enrichment status integration** — Phase 65
  strangler-fig. Phase 61 ships a `Status()` accessor on the worker
  manager (queue depth per lane, last error per language, files
  enriched/dropped/preempted counters); Phase 65 wires it into
  `get_health`.
- **Background drainer for `semantic_pending` files left by
  bulk_update** — explicitly out of scope (see D-06). PITFALLS C8
  prescription.
- **Per-language fan-out / multiple in-flight enrichments per
  language** — `max_concurrent_workers` is global (across all
  languages). Per-language fan-out is a benchmark-driven follow-up.
- **Adaptive priority promotion** (e.g. background → high after N
  read-tool hits on a `semantic_pending` file) — Phase 62+ once the
  tools that read pending files exist.
- **Receipts / guardrails on enrichment-derived edges** — Phase 66.

</domain>

<decisions>
## Implementation Decisions

### Queue topology + concurrency

- **D-01: 2 priority lanes (high / background).** Producer side
  (`live/handler/handler.go`) maps `Kind=ChangeHelixEdit` and any future
  agent-requested signal → high lane; `Kind=ChangeFile{Created,Modified,
  Deleted,Renamed}` from watcher / manifest-scan sources → background
  lane. Consumer drains via:

  ```go
  select {
  case job := <-q.high:
      run(job)
  default:
      select {
      case job := <-q.high:
          run(job)
      case job := <-q.background:
          run(job)
      case <-ctx.Done():
          return ctx.Err()
      }
  }
  ```

  The closed lane enum (`high | background`) is additive — Phase 62 may
  add lanes without breaking existing producers.

  **Hard invariants:**
  - Lane is owned by the producer (handler), not derived inside the
    worker. The handler reads `event.Kind` once and chooses the channel.
  - Background lane is drained ONLY when high lane has no pending job.
    Strict priority — no weighted fairness, no aging.
  - Both lanes are bounded buffered channels (default capacity 1024
    each); a full lane drops the new job and increments
    `helix_semantic_lsp_enrichment_dropped_total{lane}`.

- **D-02: Worker concurrency cap default 1, configurable.** New config
  key:

  ```
  semantic_index.lsp_enrichment.max_concurrent_workers: 1   # default
  ```

  Added to `internal/config/defaults.go` adjacent to the existing
  `lsp_enrichment.*` keys (lines 99-106) and to
  `SerenaConfig.SemanticIndex.LSPEnrichment` (Phase 57 reserved this
  struct field). One worker goroutine per cap slot. Cap is **global**
  across all languages — no per-language fan-out in v1.

  **Hard invariants:**
  - All worker goroutines drain the same shared 2-lane queue. No
    per-language queues.
  - Cap is read once at startup; runtime reload not in scope.
  - Cap value is recorded in startup slog at `INFO` level so ops can
    confirm it was honored.

### Foreground preemption

- **D-03: Voluntary yield between cascade steps; mark file `partial:true,
  partial_reason:"preempted"`.** The §14.4 cascade is a sequence of
  ~6 LSP calls per file. Between each call the worker invokes
  `acquirer.ForegroundBusy(wsKey)`. If `true`:
  - Abort the remainder of the cascade for the current file.
  - Commit whatever facts have been gathered so far via the open
    `OverlayTx`.
  - Stamp the file row with `partial=true, partial_reason="preempted"`.
  - Increment `helix_semantic_lsp_enrichment_total{outcome="preempted",
    language=...}`.
  - The file becomes a candidate for re-enrichment via the standard
    re-trigger paths (next per-file edit or Phase 64 manual refresh).

  **Hard invariants:**
  - The yield check is between LSP calls only, never mid-call. We do
    NOT cancel an LSP request once issued — that would touch the
    `lspool` RPC layer Phase 56 just stabilized.
  - A file partially-enriched with `partial_reason="preempted"` is
    queryable; consumers see whatever validated edges/symbols were
    written before the yield.
  - Yield is best-effort; a single LSP call that hangs past
    `timeout_per_file` is killed by the per-call deadline, not by
    yield.

- **D-04: `LeaseAcquirer.ForegroundBusy(wsKey) bool` method.** The
  interface (which Phase 61 introduces anyway per ENRICH-01) gets two
  methods:

  ```go
  // internal/semantic/lspenrich/acquirer.go (or sibling)
  type LeaseAcquirer interface {
      AcquireLease(ctx context.Context, sessionID string,
                   wsKey workspace.WorkspaceKey, dirty bool) (*lspool.WorkerLease, error)
      ForegroundBusy(wsKey workspace.WorkspaceKey) bool
  }
  ```

  `internal/kernel/lspool` provides the implementation (a small
  `*Pool` adapter or a method on `*Pool` directly — planner picks):

  ```go
  func (p *Pool) ForegroundBusy(wsKey workspace.WorkspaceKey) bool {
      p.mu.RLock()
      defer p.mu.RUnlock()
      last, ok := p.lastForegroundLease[wsKey]
      if !ok { return false }
      return time.Since(last) < p.cfg.YieldCheckWindow
  }
  ```

  `lastForegroundLease` is a `map[workspace.WorkspaceKey]time.Time`
  updated under the existing pool mutex inside `AcquireLease` whenever
  the `sessionID` does NOT start with `"lsp-enrichment:"`.

  New config key:

  ```
  semantic_index.lsp_enrichment.yield_check_window_ms: 200   # default
  ```

  **Hard invariants:**
  - Enrichment session IDs MUST start with the `lsp-enrichment:`
    prefix; the pool relies on this prefix to distinguish enrichment
    from foreground traffic.
  - `ForegroundBusy` is non-blocking (read-lock only). Worker calls it
    on every cascade-step boundary; a spinning lock would itself be a
    foreground-busy event.
  - Window is per-`wsKey`, not global. Foreground busy on a different
    workspace doesn't preempt enrichment on this one.

### Bulk-update / git-checkout backoff

- **D-05: Producer suppression on `Kind=ChangeBulkUpdate`; no
  enqueue.** The Phase 60 handler currently calls `LSPQueue.Enqueue`
  unconditionally after each successful overlay commit
  (`internal/semantic/live/handler/handler.go:144-146`). Phase 61
  changes the handler to:

  ```go
  // Phase 61 modification — handler.go after overlay commit succeeds
  switch event.Kind {
  case live.ChangeBulkUpdate:
      // Mark all affected files semantic_pending; do NOT enqueue.
      _ = h.markBulkPending(ctx, repoID, event.Paths,
                            "bulk_update_pending")
      // Single counter bump:
      h.metrics.LSPEnrichmentBulkSuppressed(len(event.Paths))
  default:
      lane := selectLane(event.Kind)
      _ = h.LSPQueue.EnqueueLane(lane, lspqueue.RevalidateFileJob{
          RepoID: repoID, Path: path,
      })
  }
  ```

  `markBulkPending` is a new helper that opens an overlay tx and calls
  the new `tx.MarkFileSemanticPending(repoID, path, reason)` API on
  every affected path. The API stamps the existing `partial_reason`
  column with the closed-enum value `"bulk_update_pending"` and sets
  `semantic_status="pending"`.

  **Hard invariants:**
  - The producer is the single decision point for bulk suppression.
    The worker NEVER sees a `ChangeBulkUpdate` job — there's no such
    `RevalidateFileJob.Kind` field; the lane choice happens before
    enqueue.
  - `bulk_update_pending` is added to the `partial_reason` closed enum
    alongside the existing `"budget exhausted"` and `"preempted"`
    values. Update `internal/semantic/store/migrations.go` doc
    comment, no schema change required (`partial_reason TEXT` is
    permissive).
  - `markBulkPending` runs inside the same coalescer dispatch goroutine
    that produced the bulk_update event; no fanout, no parallelism.

- **D-06: No auto-recovery for `bulk_update_pending` files in Phase 61.**
  Recovery happens via:
  - **Per-file edit re-trigger.** A subsequent helix_edit / fsnotify /
    manifest_scan event for a single file flows through the standard
    classifier → coalescer → handler path with `Kind != ChangeBulkUpdate`
    and is enqueued normally. The pending state is overwritten on
    successful enrichment.
  - **Phase 64 `refresh_semantic_graph(paths=...)` MCP tool.** Out of
    scope for Phase 61. The tool will scan `semantic_files WHERE
    partial_reason='bulk_update_pending'` and enqueue them in
    user-controlled batches.

  **Hard invariants:**
  - Phase 61 ships NO background goroutine that drains pending files.
    PITFALLS C8 prescription is verbatim: "do **NOT** flood the queue —
    mark all affected files `pending` and let lazy-on-tool-call drive
    enrichment".
  - The "untouched-file in checked-out branch stays pending forever"
    trade-off is accepted. Read tools that surface pending state
    (Phase 64+) will make this visible to the user.
  - The `partial_reason="bulk_update_pending"` value is observable via
    the worker `Status()` accessor (D-09 below) so ops can see how many
    files are in this state per workspace.

### Enrichment scope + languages

- **D-07: Full SPEC §14.4 cascade in v1.** All six LSP capabilities
  ship in Phase 61, in this fixed order per file:

  1. `textDocument/documentSymbol` — populates `result.Symbols` shape.
  2. `textDocument/publishDiagnostics` (already-pushed) drained into
     `result.Diagnostics` for the file.
  3. `PrioritizeSymbols(symbols, refs, budget)` — for each prioritized
     symbol up to `max_symbols_per_file`:
     a. `textDocument/hover` — type info, into `EnrichSymbolType`.
     b. `callHierarchy/{prepare,incomingCalls,outgoingCalls}` — depth
        `max_call_hierarchy_depth` (default 2), into `BuildCallEdges`.
     c. `typeHierarchy/{prepare,supertypes,subtypes}` — depth
        `max_type_hierarchy_depth` (default 2), into `BuildTypeEdges`.
     d. `textDocument/implementation` — into
        `BuildImplementationEdges`.
  4. `PrioritizeReferences(refs, budget)` — for each prioritized
     reference up to `max_references_per_file`:
     a. `textDocument/definition` — into a `RESOLVES_TO` edge with
        `source="lsp.definition"`, `validation_state="validated"`,
        `confidence=1.0`. NEVER called on definitions themselves
        (SPEC line 1261).

  Edges land in `semantic_live_overlay_edges`. Symbols update
  `semantic_live_overlay_symbols` with `lsp_validated=true`. References
  update `semantic_live_overlay_references` with `validation_state=
  "validated"`. Diagnostics land in `semantic_live_overlay_diagnostics`
  via `tx.UpsertDiagnostics(...)`.

  **Hard invariants:**
  - Cascade order is fixed in v1; no per-language opt-out. If a
    capability is unsupported by a given LS, the worker observes the
    `MethodNotFound` JSON-RPC error, logs once at `slog.Debug`, and
    continues to the next step. Per-language capability cache lives in
    the worker (small `map[language]capabilitySet`).
  - Edges from this cascade ALL get `confidence=1.0`. Lower-confidence
    edges (heuristic, comment-derived) are Phase 62 territory and use
    a different `source` value.
  - Budget is checked before EACH cascade step; exhaustion mid-cascade
    is the `partial_reason="budget exhausted"` outcome (existing
    requirement).

- **D-08: All languages run, gated only by LS readiness.** Per-language
  enrichment proceeds when:

  ```text
  language is installed (Pool can spawn worker)
    AND language readiness gate satisfied:
        - Java: JdtlsAdapter.WaitUntilJavaReady(ctx) returns nil
        - Rust: experimental/serverStatus.quiescent=true observed
        - All others: AcquireLease succeeds (the spawn itself is the gate)
    AND Phase 59 emitted facts for the file
        (extraction_status != "unsupported")
  ```

  Files for ungated languages drop with `outcome=dropped` and a
  summary log line at worker startup ("language X: not installed,
  enrichment disabled"). No per-file error log — that would be noise.

  **Hard invariants:**
  - The readiness gate is checked BEFORE `AcquireLease` for Java +
    Rust (per ENRICH-03 "never bypassed"). For Java this is
    `JdtlsAdapter.WaitUntilJavaReady` with a `ctx` derived from the
    job's `timeout_per_file` deadline. For Rust the worker waits on
    the existing `serverStatus.quiescent` channel.
  - On readiness-gate timeout (jdtls cold start exceeds
    `timeout_per_file`), the file is marked `semantic_pending` with
    `partial_reason="lsp_not_ready"` and re-queued for the next per-
    file edit. Same recovery path as bulk_update.
  - No special-casing for "best-effort" languages. The
    `extraction_status="unsupported"` row from Phase 59 is the SINGLE
    point of language gating; the enrichment worker simply doesn't
    receive jobs for those files (handler skips enqueue when the
    file's facts have `extraction_status="unsupported"`).

### Acceptance criteria (must hold at end of phase)

1. `internal/semantic/lspenrich/` (or wherever the worker lives) does
   NOT import `internal/kernel`. Verified by the existing
   `internal/lint/nokernel2semantic` analyzer (run via `make vet`).
2. `LeaseAcquirer` interface has exactly two methods (`AcquireLease`,
   `ForegroundBusy`). The pool implementation lives in
   `internal/kernel/lspool/`.
3. The 2-lane queue: a unit test enqueues 100 background jobs followed
   by 1 high job and asserts the high job is drained before any
   background job after the first in-flight one completes.
4. `max_concurrent_workers=1` default + `max_concurrent_workers=4`
   override both honored — verified by an integration test counting
   concurrent in-flight LSP calls observed by a mock LS.
5. `ForegroundBusy` returns `true` within `yield_check_window_ms`
   after a non-enrichment `AcquireLease` and `false` after the window
   elapses. Unit test asserts the timing (with fake clock).
6. Voluntary yield: an integration test with a slow mock LS that runs
   a 5-call cascade injects a foreground lease after call 2; asserts
   the worker commits 2 calls' worth of facts and stamps
   `partial_reason="preempted"`.
7. Bulk-update suppression: an integration test fires a coalescer
   bulk_update event covering 250 files; asserts ZERO
   `RevalidateFileJob` enqueued and ALL 250 files have
   `partial_reason="bulk_update_pending"` in
   `semantic_files`.
8. Per-file budget: an integration test with a cascade that returns
   200ms-per-call uses `timeout_per_file=500ms`; asserts the file is
   marked `partial_reason="budget exhausted"` after ~2 calls and
   remains queryable.
9. Full §14.4 cascade for Go and Java: integration tests against real
   gopls + jdtls fixtures verify each of the 6 capabilities produces
   the expected edge/symbol shape with `confidence=1.0`,
   `validation_state="validated"`.
10. Java readiness gate: a daemon-restart integration test asserts the
    Phase 61 worker waits for `WaitUntilJavaReady` before issuing the
    first enrichment call; the test injects a slow jdtls and asserts
    no LSP requests fire until ServiceReady + ProjectStatus=OK.
11. ENRICH-05 stress test: 100-file edit burst (per-file
    `helix_edit` events, NOT bulk_update) + interactive
    `get_symbols` calls every 200ms for 30s on a Go+Java fixture
    repo. Assertion: interactive `get_symbols` p95 < 5s
    (foreground-tool budget). Recorded in
    `internal/semantic/lspenrich/stress_test.go` and gated by `-stress`
    build tag.
12. ENRICH-01..ENRICH-05 all marked `Done` in
    `.planning/REQUIREMENTS.md` after Phase 61 close.

### Claude's Discretion (no user input needed)

- **Package layout:** `internal/semantic/lspenrich/` matches SPEC §3
  (line 344). The 2-lane queue extension may live as
  `internal/semantic/live/lspqueue/` (extending the existing package
  with a typed `LaneQueue` wrapping two `*Queue` instances) or as a
  new `internal/semantic/lspenrich/queue.go` — planner picks.
- **Session-ID scheme:** `"lsp-enrichment:<workspace-key>:<language>"`
  per long-lived clean lease. One lease per (workspace, language)
  pair held for the lifetime of the worker; releases on workspace
  deactivation. Long-lived leases preserve share-until-dirty cache
  reuse with foreground sessions.
- **Failure handling per LSP call:** `MethodNotFound` →
  `slog.Debug` once per (language, method); continue cascade.
  Generic JSON-RPC error → `slog.Warn`; commit partial facts;
  continue cascade. Whole-job failure (LS crash mid-cascade,
  `ErrCircuitOpen` on initial AcquireLease, readiness-gate timeout)
  → mark file `semantic_pending`,
  `partial_reason="lsp_unavailable"`, increment
  `helix_semantic_lsp_enrichment_errors_total{language, outcome}`,
  do NOT re-enqueue. Recovery via the standard re-trigger path
  (next per-file edit / Phase 64 refresh).
- **Lease release timing:** long-lived per (workspace, language)
  lease released on `workspace.OnDeactivate` callback (Phase 60
  bootstrap point). No per-job acquire/release.
- **Metrics (closed-enum bounded labels):**
  - `helix_semantic_lsp_enrichment_total{language, outcome}` —
    `outcome ∈ {applied, partial_budget, partial_preempted,
    partial_lsp_unavailable, dropped}`.
  - `helix_semantic_lsp_enrichment_duration_seconds{language}` —
    histogram per file (existing in SPEC §28.1).
  - `helix_semantic_lsp_enrichment_errors_total{language, outcome}` —
    `outcome ∈ {timeout, ls_crash, circuit_open, readiness_timeout,
    other}` (existing in SPEC §28.1).
  - `helix_semantic_lsp_enrichment_lane_depth{lane}` — gauge
    `lane ∈ {high, background}`.
  - `helix_semantic_lsp_enrichment_bulk_suppressed_total` — counter
    bumped per bulk_update event (not per-file; the event count is
    what matters).
  All labels closed-enum, registered via the same `internal/obs/`
  bounded-label discipline Phase 60 used.
- **Trace spans:** `semantic.lsp_enrich_file` (per-file root span),
  child spans per cascade step (`semantic.lsp_enrich.documentSymbol`,
  `semantic.lsp_enrich.hover`, etc.). SPEC §28.2 alignment.
- **D-09: Worker `Status()` accessor.** Read-only struct exposing
  `{lane_depths, in_flight, files_enriched, files_dropped,
  files_preempted, files_pending, last_error_per_language}` for
  Phase 65 `get_health` integration. Phase 61 ships the accessor;
  Phase 65 wires it.
- **Stress-test fixture:** small Go module + Java Maven project
  under `internal/semantic/lspenrich/testdata/stress/` (5-10 files
  each is enough — the test loops edits, not file count).
- **Plan layout:** the planner decides wave structure. Suggested
  4 plans:
  P01 LeaseAcquirer interface + ForegroundBusy + lane-typed queue
      + handler producer-side rewiring + bulk suppression API.
  P02 Worker goroutine + cascade engine + budget enforcement +
      readiness gate honoring + overlay commit per file.
  P03 Metrics + trace spans + Status() accessor + per-language
      capability cache + Daemon bootstrap (`SetEnrichmentWorker`).
  P04 ENRICH-05 stress test + Go/Java integration tests + REQ
      check-off.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and requirements (load-bearing)

- `.planning/REQUIREMENTS.md` §ENRICH (lines 48–53) — ENRICH-01 through
  ENRICH-05. Each requirement maps directly to acceptance tests above.
- `.planning/milestones/v1.10-ROADMAP.md` Phase 61 block (lines
  116–125) — Goal, Depends on (Phase 60), Requirements, 4 Success
  Criteria.

### Specification (source of truth for shapes and rules)

- `SPEC-DRAFT.md` §14 (lines 1244–1357) — LSP Enrichment Layer:
  - §14.1 — capabilities to use; explicit warning against
    go-to-definition on definitions (line 1261).
  - §14.2 — budget config (lines 1265–1278). Phase 61 honors verbatim.
  - §14.3 — enrichment priority ladder (lines 1282–1291). Phase 61
    implements the simplified ENRICH-02 version (helix_edit + agent-
    requested = high, rest = background); the full §14.3 ladder is
    Phase 62 territory.
  - §14.4 — cascade pseudocode (lines 1295–1357). Phase 61 implements
    line-for-line.
- `SPEC-DRAFT.md` §21 (lines 2124–2191) — LSP Revalidation Queue.
  - `RevalidateFileJob` struct (line 2127).
  - `LSPRevalidationQueue` interface (line 2135). Phase 61's lane queue
    implements (extended) this contract.
  - `RevalidateFile(ctx, job)` flow (line 2153). Phase 61's worker is
    a goroutine that loops on this function per drained job.
- `SPEC-DRAFT.md` §27.2 — Watcher misses fallbacks; Phase 61
  inherits manifest-scan recovery via the standard producer.
- `SPEC-DRAFT.md` §28.1 (line 2722–2723) — `helix_semantic_lsp_
  enrichment_*` metrics. Phase 61 emits exactly these.
- `SPEC-DRAFT.md` §28.2 (line 2761) — `semantic.lsp_enrich_file`
  trace span.
- `SPEC-DRAFT.md` §29.5 — Rapid edit storm. Phase 61 implements the
  PITFALLS C8 prescription that §29.5 omits ("do NOT flood LSP
  queue").
- `SPEC-DRAFT.md` §32 Phase 3 — "LSP Enrichment Worker" deliverables
  list.
- `SPEC-DRAFT.md` §38.2 — confidence ladder. Phase 61's edges land at
  the top of the ladder (validated, confidence=1.0).

### Pitfall catalog

- `.planning/research/PITFALLS.md` C8 (lines 167–185) — death-spiral
  prevention: strict priority, do-not-flood after bulk_update, reuse
  v1.9 readiness gates. Phase 61 D-01, D-03, D-05, D-06, D-08
  implement this verbatim.

### Phase 60 lock-down (must not regress)

- `.planning/phases/60-live-update-pipeline/60-CONTEXT.md`:
  - D-03 (EditNotifier) — the producer side of the LSP queue lives in
    the same package boundary; Phase 61 reuses the exact wiring.
  - D-04 (overlay_epoch) — Phase 61's `BeginOverlayTx` per file
    advances the epoch. CAS contract for Phase 63 is preserved.
  - D-05 (manifest scan) — Phase 61 inherits the
    `WorkspaceChangeSignal` → classifier → coalescer pipeline; no
    new sources.
  - D-07 (`ScheduleIncremental`) — Phase 61 does NOT call this.
    Enrichment is a separate path that consumes `RevalidateFileJob`,
    not `FileChange`. The two paths share the overlay tx but not the
    scheduler.
- `internal/semantic/live/lspqueue/queue.go` — Phase 61 extends
  this package (or wraps it) with a 2-lane variant. The
  `RevalidateFileJob` struct is unchanged.
- `internal/semantic/live/handler/handler.go:71-145` — Phase 61
  modifies the handler to choose the lane based on event kind and
  to suppress enqueue on `Kind=ChangeBulkUpdate`. The `LSPQueue`
  field stays nil-safe per Phase 60 CR-04.
- `internal/daemon/live_wiring.go:151-159` — Phase 61 extends this
  bootstrap to construct the worker, attach it to the queue, and
  pass the `LeaseAcquirer` (kernel pool wrapper).

### Phase 59 lock-down (must not regress)

- `.planning/phases/59-tree-sitter-extraction-stable-symbol-ids/`
  CONTEXT — D-01 (no `internal/repomap` import inside
  `internal/semantic/extract/`); Phase 61 inherits the same
  invariant. D-05 (partial extraction model with
  `extraction_status`); enrichment uses
  `extraction_status != "unsupported"` as the per-file gate.
- `internal/semantic/types.go` — typed identifiers Phase 61 may extend
  (e.g. add `LSPEnrichmentLane`, `LSPEnrichmentOutcome`).

### v1.9 LS readiness gates (must honor, never bypass)

- `internal/kernel/lspool/quirks.go` — `RustAnalyzerAdapter` (line
  28+), `experimental/serverStatus` notification handler (line 123+),
  `ExperimentalCapabilities` (line 165), `JdtlsAdapter` (line 322+),
  `WaitUntilJavaReady(ctx)` (line 423+, line 427).
- `internal/kernel/lspool/pool.go` — `AcquireLease(ctx, sessionID,
  wsKey, dirty)` (line 119+). Phase 61 calls through `LeaseAcquirer`.
- `internal/kernel/lspool/metrics.go` — bounded-label metric
  registration pattern. Phase 61's new metrics follow the same shape.
- `internal/kernel/lspool/worker.go` — `Worker` lifecycle. Phase 61
  does not construct workers; the pool does.

### Schema + storage

- `internal/semantic/store/migrations.go` — `partial_reason TEXT`
  columns on `semantic_files`, `semantic_symbols`, `semantic_references`
  (Phase 59 D-05). Phase 61 extends the closed-enum doc comment with
  three new values: `"preempted"`, `"bulk_update_pending"`,
  `"lsp_unavailable"`. NO schema change required.
- `internal/semantic/store/overlay.go` — Phase 60's `BeginOverlayTx`
  API. Phase 61 extends with `tx.MarkFileSemanticPending(repoID, path,
  reason)` (a thin wrapper over the existing partial_reason update).
- `internal/semantic/store/duckdb.go` — fact-store open path. Phase 61
  does not change open semantics.

### Architectural invariants

- `CLAUDE.md` "Middleware Execution Order (LIFO)" — Phase 61 does NOT
  touch middleware; enrichment plumbing lives below the MCP layer.
- `internal/lint/nokernel2semantic/` — vet analyzer (Phase 60)
  enforcing `internal/semantic/...` does not import
  `internal/kernel/...`. Phase 61 introduces `LeaseAcquirer` to comply.
- `cmd/vet-noduckdb/` — Phase 57 vet-tool. Phase 61's
  `internal/semantic/lspenrich/...` packages MUST NOT import
  `duckdb-go` directly — they go through `internal/semantic/store/`.
- `internal/treesitter/registry_cgo.go` — Phase 61 does NOT use
  tree-sitter directly; enrichment reads facts already extracted by
  Phase 59. The grammar registry is irrelevant to this phase.

### Pattern templates (must mirror)

- `internal/daemon/daemon.go` `SetEnrichFn` / `SetActivateCallback` /
  `kernel.SetEditNotifier` — Phase 61's
  `daemon.SetEnrichmentWorker(worker)` follows the same setter pattern
  for daemon-bootstrap-time wiring.
- `internal/semantic/live/coalescer/` — closed-channel goroutine
  lifecycle, debounce timer, structured slog. Phase 61's worker
  goroutine mirrors the start/stop discipline.
- `internal/kernel/lspool/circuit.go` — circuit-breaker pattern.
  Phase 61 honors circuit state via `AcquireLease`'s existing
  `ErrCircuitOpen` return; no new circuit logic.
- Phase 60 `nil-safe LSPQueue` pattern (CR-04) — Phase 61 worker
  similarly nil-safe when bootstrap can't construct it.

### Configuration

- `internal/config/defaults.go` lines 99–106 — existing
  `lsp_enrichment.*` block. Phase 61 adds:
  - `semantic_index.lsp_enrichment.max_concurrent_workers: 1`
  - `semantic_index.lsp_enrichment.yield_check_window_ms: 200`
- `SerenaConfig.SemanticIndex.LSPEnrichment` — struct field reserved
  by Phase 57 P02/P03; Phase 61 populates the two new fields.
- Phase 61 adds a `TestLoad_LSPEnrichmentDefaults` per-feature test
  asserting the two new keys round-trip through the koanf precedence
  chain. Mirrors Phase 60's `TestLoad_LiveUpdatesDefaults`.

### Test fixtures (must seed)

- New: `internal/semantic/lspenrich/testdata/stress/` — small Go
  module + Java Maven project for ENRICH-05.
- New: `internal/semantic/lspenrich/testdata/cascade/` — Go and Java
  fixtures with known-shape symbols/references for the §14.4 cascade
  integration tests (acceptance #9).
- Reuse: `internal/kernel/lspool/testdata/` Java/Rust/Go fixtures
  Phase 56 + Phase 47 already maintain. Phase 61 readiness-gate tests
  use the same fixtures.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- **`internal/semantic/live/lspqueue/queue.go`** — typed buffered
  channel + non-blocking `Enqueue`. Phase 61 extends with a 2-lane
  variant; the single-channel form remains for tests.
- **`internal/semantic/live/handler/handler.go:144-146`** — current
  unconditional `LSPQueue.Enqueue` site. Phase 61's bulk suppression
  edits this site (and only this site) on the producer side.
- **`internal/kernel/lspool/pool.go`** — `Pool` with
  `AcquireLease(ctx, sessionID, wsKey, dirty)` (line 119) and the
  existing per-language circuit breaker. Phase 61 wraps this in
  `LeaseAcquirer` and adds `ForegroundBusy(wsKey)` on `*Pool`.
- **`internal/kernel/lspool/quirks.go`** — `JdtlsAdapter.
  WaitUntilJavaReady(ctx)` (line 427), `RustAnalyzerAdapter`
  `experimental/serverStatus.quiescent` channel (line 165). Phase 61
  honors both before issuing per-language enrichment requests.
- **`internal/kernel/lspool/metrics.go`** — `LSPoolLookup` bounded-
  label metric. Phase 61's new
  `helix_semantic_lsp_enrichment_lane_depth` etc. follow the same
  registration shape.
- **`internal/semantic/store/overlay.go`** — Phase 60 `BeginOverlayTx`
  API. Phase 61 calls `tx.UpsertSymbols`, `tx.UpsertReferences`,
  `tx.UpsertEdges`, `tx.UpsertDiagnostics`, `tx.WriteInvalidations`
  (already typed; Phase 60 left `WriteInvalidations` as a no-op stub
  Phase 62 fills).
- **`internal/semantic/store/migrations.go`** — `partial_reason TEXT`
  columns ready for Phase 61's three new closed-enum values.
- **`internal/config/defaults.go`** lines 99-106 — existing
  `lsp_enrichment.*` config keys. Phase 61 adds two more in-place.
- **`internal/obs/`** — bounded-label metric registration. All Phase 61
  metrics route through here.
- **Phase 60 `live_wiring.go`** — bootstrap pattern for live-update
  components. Phase 61 extends `buildLiveBundle` with a worker
  constructor + `kernel.SetEnrichmentWorker(worker)` call.

### Established Patterns

- **`LeaseAcquirer`-style narrow interfaces** between `internal/
  semantic` and `internal/kernel/lspool` (Phase 60 D-03 EditNotifier
  is the template). Single-method or two-method interfaces; no
  `internal/kernel` import.
- **Setter-style cross-package wiring** (`SetEnrichFn`,
  `SetActivateCallback`, `SetEditNotifier`). Phase 61 adds
  `kernel.SetForegroundLeaseObserver(...)` if needed for the
  `lastForegroundLease` map (or the pool tracks it internally —
  planner picks).
- **Bounded-label metrics with closed enum.** Every Phase 61 metric
  has a documented closed-enum label set; no string interpolation
  into label values.
- **Per-feature defaults test.** Phase 61 adds
  `TestLoad_LSPEnrichmentDefaults` for the two new keys.
- **Long-lived clean lease + share-until-dirty.** Phase 61's
  enrichment session is one of many sessions sharing the same warm
  worker. The `lsp-enrichment:` session-ID prefix is the discriminator
  for the foreground-busy filter.
- **Nil-safe queue/worker fields** (Phase 60 CR-04). Phase 61
  bootstrap path stays nil-safe.

### Integration Points

- **Daemon bootstrap (`internal/daemon/daemon.go`)** — after the
  Phase 60 `buildLiveBundle` call, Phase 61 adds:
  - Construct `*lspenrich.Worker` with the `LeaseAcquirer` (a small
    adapter around `kernel.Pool()`), the lane queue, the overlay
    writer, and the budget config.
  - Start `worker.Run(ctx)` in the existing errgroup.
  - Call `kernel.SetEnrichmentWorker(worker)` (or equivalent) only if
    it needs cross-call coordination — likely not required.
- **`SerenaConfig.SemanticIndex.LSPEnrichment`** — Phase 61 populates
  two new koanf keys via `internal/config/defaults.go` and the
  per-feature defaults test.
- **`internal/semantic/live/handler/handler.go`** — single-site
  edit at the post-overlay-commit Enqueue point. The handler grows
  a small `markBulkPending` helper.
- **`internal/kernel/lspool/pool.go`** — additive: `ForegroundBusy`
  method, `lastForegroundLease` field, session-prefix filter inside
  `AcquireLease`. No removal or rename.

### Constraints

- Phase 61 must NOT import `internal/kernel` from
  `internal/semantic/...` — only `internal/kernel/lspool` types and
  `internal/workspace`. Enforced by `internal/lint/nokernel2semantic`.
- Phase 61 must NOT bypass `JdtlsAdapter.WaitUntilJavaReady` or
  `experimental/serverStatus.quiescent` for "best-effort" reasons
  (ENRICH-03).
- Phase 61 must NOT regress BUG-02 (rust-analyzer
  `serverStatusNotification`) or Phase 56 (jdtls notification
  dispatch + readiness) fixes.
- Phase 61 must NOT touch middleware (CLAUDE.md "Middleware Execution
  Order (LIFO)").
- Phase 61 must NOT introduce a second `GrammarRegistry` (BUG-04 /
  EXTRACT-05) — enrichment doesn't touch tree-sitter at all.
- Phase 61 must NOT import `duckdb-go` outside
  `internal/semantic/store/` — `cmd/vet-noduckdb/` analyzer enforces.
- Phase 61's worker MUST mark partial files queryable — never delete
  or hide a partial-enriched file from query paths.
- Phase 61's worker MUST advance `overlay_epoch` correctly per
  enrichment commit (Phase 60 D-04 contract). Phase 63 compaction
  CAS depends on this.
- Phase 61 ships ZERO background drainer for `bulk_update_pending`
  files (D-06 invariant). PITFALLS C8.
- Phase 61's queue lanes are strict-priority: no aging, no fairness
  budget. Trade-off accepted for v1; revisit if benchmarks demand.

</code_context>

<specifics>
## Specific Ideas

- **The user's "filesystem state is truth" cascade from Phase 60 carries
  forward.** Enrichment is layered ON TOP of that truth: tree-sitter
  produces structural facts, enrichment validates and adds edges. The
  overlay is the merge point.
- **Strict priority over fairness in v1.** The user accepted the
  "untouched files in a checked-out branch stay pending forever"
  trade-off explicitly (D-06). The framing: enrichment is best-effort
  freshness; correctness is in the structural facts already.
- **Voluntary yield over hard cancellation.** The user chose the path
  that does NOT touch the kernel/lspool RPC layer Phase 56 just
  stabilized. Preemption is between calls, not mid-call. Honest about
  the worst-case 5s blocking window (the per-call deadline).
- **C8 prescription is non-negotiable.** Bulk-update suppression at
  the producer is the literal PITFALLS prescription; deviating from
  it would re-introduce the death-spiral risk.
- **Full §14.4 cascade in v1.** The user prioritized "no awkward
  data-but-no-consumer gap" with Phase 62 over a smaller v1. Phase 62
  consumes the edges immediately when it lands.
- **All-language readiness-gated enrichment.** The user explicitly
  rejected hardcoding a Phase-59-tier allowlist. Java + Rust ride the
  same readiness-gate machinery v1.9 already proved.

</specifics>

<deferred>
## Deferred Ideas

- **Per-language fan-out** — `max_concurrent_workers` is global in
  v1. If benchmarks show a slow jdtls re-validate blocking gopls
  enrichment, revisit with per-language sub-pools. Not now.
- **PageRank-bias / public-symbols-first prioritization** —
  Phase 62. The §14.3 8-class ladder needs the graph engine to exist.
  Phase 61's 2-lane is the minimum to satisfy ENRICH-02.
- **Background drainer for `bulk_update_pending` files** — explicitly
  out of scope (D-06). PITFALLS C8 prescription. Could revisit in a
  follow-up phase if user-reported "untouched-file pending" surfaces
  become a UX issue.
- **`refresh_semantic_graph(paths=...)` MCP tool** — Phase 64.
  Phase 61 ships the underlying worker; Phase 64 wraps it as MCP.
- **`get_semantic_graph_status` MCP tool** — Phase 64. Phase 61
  ships the `Status()` accessor (D-09); Phase 64 wraps.
- **`get_health` enrichment-status integration** — Phase 65
  strangler-fig. Phase 61 ships the data accessor; Phase 65 wires
  it into the existing `get_health` tool.
- **Hard ctx-cancel preemption (lspool RPC layer)** — Phase 61 took
  the voluntary-yield path. If voluntary yield proves insufficient
  under a future workload, hard cancel is a separate phase.
- **Subscription-channel foreground-busy signal** — Phase 61 took
  the bool-method path. If polling overhead surfaces as a
  bottleneck, switching to a channel is additive.
- **Single sortable heap queue** — rejected for v1 in favor of 2
  typed channels. If the priority surface grows past 4 lanes,
  revisit.
- **Receipts / guardrails on enrichment-derived edges** — Phase 66.
- **Adaptive priority promotion for `semantic_pending` files** —
  Phase 62+ once the read-tool surface that drives "should I
  promote" exists.
- **Cross-workspace enrichment coordination** — workspaces are
  independent. Phase 60 cascade applies; no cross-workspace shared
  worker.

</deferred>

---

*Phase: 61-lsp-enrichment-worker*
*Context gathered: 2026-05-05*
