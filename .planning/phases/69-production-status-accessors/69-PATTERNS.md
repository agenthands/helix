# Phase 69: Production Status Accessors — Pattern Map

**Mapped:** 2026-05-14
**Files analyzed:** 9 (3 new, 6 modified)
**Analogs found:** 9 / 9 (all in-repo, all VERIFIED via Read)

## File Classification

| File | New/Mod | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|---|
| `internal/semantic/store/effective_graph.go` (extension) | MOD (add method) | read accessor (`*Store`) | request-response (DB SELECT) | `CountStaleScoreRows` @ effective_graph.go:155-176 | exact |
| `internal/semantic/store/effective_graph_test.go` (extension) | MOD (add tests) | unit test (race-stress) | concurrent read | `TestCountStaleScoreRows_LockFree_RaceSafe` @ effective_graph_test.go:242-293 | exact |
| `internal/semantic/retrieval/recovery.go` | MOD (extend SetMeta block) | write site (bleve single-writer) | batch-then-commit | existing `SetMeta(metaKeyLastIndexed,...)` @ recovery.go:258-260 | exact (same site) |
| `internal/semantic/compact/accessors.go` | MOD (add interface) | dep-interface declaration | type-system seam | existing `CoalescerAccessor` / `OverlayRowAccessor` @ accessors.go:28-54 | exact |
| `internal/semantic/compact/compactor.go` | MOD (add SetMeta call) | write site (post-commit) | event-driven success path | nil-safe `c.deps.Metrics` guards @ compactor.go:223,233 | role+flow match |
| `internal/daemon/compact_wiring.go` | MOD (thread dep) | adapter wiring | factory injection | `ensureCompactor` @ compact_wiring.go:142-187 | exact |
| `internal/daemon/semantic_wiring.go` | MOD (replace 3 placeholders) | adapter (Scheduler+Retrieval) | request-response | `semSchedulerAdapter` @ semantic_wiring.go:410-462; `semRetrievalAdapter` @ 581-639 | exact |
| `internal/skill/semantic/envelope.go` | MOD (extend structs) | envelope DTO | shape | existing `ClusterStatus` / `StatusResult` @ envelope.go:67-112 | exact (additive) |
| `internal/skill/semantic/accessors.go` | MOD (extend interface) | accessor seam | interface | existing `RetrievalAccessor` / `SchedulerAccessor` @ accessors.go:35-111 | exact (additive) |
| `internal/skill/semantic/integration_test.go` (new test fn) | MOD (add E2E test) | integration test | end-to-end | `TestE2E_IndexThenContext_SymbolCount` @ integration_test.go:445-470 + `e2eSchedAcc` @ 241-247 | exact |

---

## Pattern Assignments

### Read Accessor on `*Store` — `ClusterStatusForGraphVersion`

**File:** `internal/semantic/store/effective_graph.go` (extend)
**Analog:** `CountStaleScoreRows` at `internal/semantic/store/effective_graph.go:155-176`

**Pattern (copy verbatim shape):**
```go
// effective_graph.go:155-176
func (s *Store) CountStaleScoreRows(ctx context.Context, repoID, projection string) (
    stale, total int, err error,
) {
    if s == nil || s.db == nil {
        return 0, 0, errors.New("CountStaleScoreRows: nil store")
    }
    const q = `
        SELECT
          COALESCE(SUM(CASE WHEN status = 'stale' THEN 1 ELSE 0 END), 0) AS stale,
          COUNT(*)                                                       AS total
        FROM semantic_graph_scores
        WHERE repo_id    = ?
          AND score_name = ?
    `
    if err := s.db.QueryRowContext(ctx, q, repoID, projection).Scan(&stale, &total); err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return 0, 0, nil
        }
        return 0, 0, fmt.Errorf("CountStaleScoreRows(%q, %q): %w", repoID, projection, err)
    }
    return stale, total, nil
}
```

**Apply to Phase 69:**
- Method must hang on `*Store` (not `OverlayTx`) — D-09 compile-time guard.
- Pure `s.db.QueryRowContext` — no `s.mu`, no `BeginOverlayTx`, no overlay lock.
- Return zero-value struct when no rows (mirror the `sql.ErrNoRows → return zeros, nil` arm).
- SELECT must JOIN/COUNT both `semantic_clusters` and `semantic_cluster_members` (see CONTEXT.md D4 + research Q8 — `score` column abused as `MemberCount` is unreliable; use `COUNT(*)` on members).

**Deviations for planner:**
- Sister signatures use multiple named returns. Phase 69 prefers a typed struct (`ClusterStatusRow`) per research target shape — diverges from `CountStaleScoreRows`'s multi-return for clarity given 3+ fields.
- `computed_at TIMESTAMP` in DuckDB needs `EXTRACT(EPOCH FROM ...) * 1000`-style cast to unix-ms `int64`.

---

### Race-Clean Read-Path Test

**File:** `internal/semantic/store/effective_graph_test.go` (extend)
**Analog:** `TestCountStaleScoreRows_LockFree_RaceSafe` at `internal/semantic/store/effective_graph_test.go:242-293`

**Pattern (mirror verbatim):**
```go
// effective_graph_test.go:242-293
func TestCountStaleScoreRows_LockFree_RaceSafe(t *testing.T) {
    if testing.Short() {
        t.Skip("race-safety stress; skipped under -short")
    }
    s, ctx, _ := openStoreForOverlayTest(t)
    repoID := "r-race"
    for i := 0; i < 10; i++ {
        seedScoreRow(t, ctx, s, repoID, 1, uint64(i+1), "call_graph", "exact")
    }

    // Background overlay tx writer to create lock contention.
    var stop atomic.Bool
    var wg sync.WaitGroup
    wg.Add(1)
    go func() {
        defer wg.Done()
        for !stop.Load() {
            tx, err := s.BeginOverlayTx(ctx, repoID)
            if err != nil {
                return
            }
            _ = tx.Commit()
        }
    }()

    // 100 concurrent reader probes — must NOT deadlock against the
    // overlay-tx writer since CountStaleScoreRows reads with no overlay
    // lock per the SchedulerStore contract.
    var readers sync.WaitGroup
    for i := 0; i < 100; i++ {
        readers.Add(1)
        go func() {
            defer readers.Done()
            done := make(chan error, 1)
            go func() {
                _, _, e := s.CountStaleScoreRows(ctx, repoID, "call_graph")
                done <- e
            }()
            select {
            case e := <-done:
                if e != nil {
                    t.Errorf("CountStaleScoreRows under contention: %v", e)
                }
            case <-time.After(5 * time.Second):
                t.Errorf("CountStaleScoreRows deadlocked (>5s under contention)")
            }
        }()
    }
    readers.Wait()
    stop.Store(true)
    wg.Wait()
}
```

**Apply to Phase 69:**
- Spawn a new `TestClusterStatusForGraphVersion_LockFree_RaceSafe` with the same skeleton.
- Replace `seedScoreRow` with a new helper `seedClusterRows(t, ctx, s, repoID, gv, n)` (research Wave 0 helper checklist).
- Background writer can stay as `BeginOverlayTx → Commit` (contention surface is the same overlay mutex). Alternative for stronger guarantee: have the background writer call `UpsertClusters` / `UpsertClusterMembers` via the same overlay tx pattern.
- 5-second timeout, 100 readers — preserve constants.

---

### Bleve Single-Writer Meta — Recoverer Extension

**File:** `internal/semantic/retrieval/recovery.go` (extend success-commit block)
**Analog:** existing `SetMeta(metaKeyLastIndexed, ...)` at `internal/semantic/retrieval/recovery.go:255-261`

**Pattern (extend the same site):**
```go
// recovery.go:251-261 — existing single-writer commit point
if err := flush(); err != nil {
    return err
}

// Persist the new last_indexed marker. This is the recovery's commit
// point: subsequent Probe calls will see raw == latest and short-
// circuit until the next snapshot commit.
if err := r.engine.SetMeta(metaKeyLastIndexed, []byte(strconv.FormatUint(snapshotID, 10))); err != nil {
    return fmt.Errorf("SetMeta(last_indexed=%d): %w", snapshotID, err)
}
return nil
```

**Apply to Phase 69:**
- Add two new `SetMeta` calls IMMEDIATELY after the `metaKeyLastIndexed` write (same single-writer site, same tx-boundary).
- New meta keys (declare as `const metaKey*` next to the existing one): `"corpus_version"`, `"indexed_files"`.
- `corpus_version`: requires reading `store.CurrentGraphVersion(ctx, repoID)` (research A4 flags a possible seam-extension on `StoreReader` — planner must verify whether `r.store` already exposes this; if not, extend the local interface).
- `indexed_files`: accumulate `distinctFileCount` during the rebuild walk loop (`map[uint64]struct{}{}` keyed on `row.FileID`), write once at the end.
- **Error policy diverges:** the existing `metaKeyLastIndexed` write is hard-fatal (returns an error). The Phase 69 additions should be `r.logger.Warn(...)` non-fatal per research §Bleve Meta Write: a SetMeta lapse must not undo the rebuild.

---

### Bleve `GetMeta` / `SetMeta` Surface

**File:** `internal/semantic/retrieval/bleve.go` (no edit required for read; possibly one new method `DocCount`)
**Analog:** existing `GetMeta` / `SetMeta` at `internal/semantic/retrieval/bleve.go:181-202`

**Pattern (existing — DO NOT edit, use as-is):**
```go
// bleve.go:181-202
func (e *Engine) GetMeta(key string) ([]byte, error) {
    if e == nil || e.idx == nil {
        return nil, fmt.Errorf("retrieval.GetMeta: engine closed")
    }
    v, err := e.idx.GetInternal([]byte(key))
    if err != nil {
        return nil, fmt.Errorf("idx.GetInternal(%q): %w", key, err)
    }
    return v, nil
}

func (e *Engine) SetMeta(key string, val []byte) error {
    if e == nil || e.idx == nil {
        return fmt.Errorf("retrieval.SetMeta: engine closed")
    }
    if err := e.idx.SetInternal([]byte(key), val); err != nil {
        return fmt.Errorf("idx.SetInternal(%q): %w", key, err)
    }
    return nil
}
```

**Apply to Phase 69:**
- Reader side in `semRetrievalAdapter` calls `engine.GetMeta(...)` and parses with `strconv.ParseUint` / `ParseInt`; missing key returns nil bytes (nil, nil) which planner must distinguish from a zero count → emit `Reason="corpus_version-uninitialized"`.
- Add a thin `DocCount()` wrapper on `*Engine` to expose `e.idx.DocCount() (uint64, error)` for `indexed_symbols` (research §Don't-Hand-Roll table).

---

### Compactor Dep Injection (`BleveMeta` interface)

**File 1:** `internal/semantic/compact/accessors.go` (add interface)
**Analog:** existing dep-interfaces at `internal/semantic/compact/accessors.go:28-74`

**Pattern (mirror the in-file convention):**
```go
// accessors.go:28-30 — minimal interface declaration shape
type CoalescerAccessor interface {
    LastFlushAt() time.Time
}
```

**Apply:** add
```go
// BleveMeta is the single-writer seam used by runCompaction to stamp
// last_compact_at after a successful commit. Compactor MUST treat a nil
// BleveMeta as a no-op (test deps may omit it).
type BleveMeta interface {
    SetMeta(key string, val []byte) error
}
```

**File 2:** `internal/semantic/compact/compactor.go` (call site)
**Analog:** nil-safe metrics guards at `internal/semantic/compact/compactor.go:222-225`:
```go
defer func() {
    if c.deps.Metrics != nil {
        c.deps.Metrics.SemanticCompactionObserve(outcome, c.now().Sub(start).Seconds())
    }
}()
```

**Apply to Phase 69:**
- Add a new `BleveMeta BleveMeta` field on `compact.Deps`.
- Insert call IMMEDIATELY after `c.lastBaseID = snap.ID` at compactor.go:332 and BEFORE `c.maybeVacuum(ctx)`. Wrap in `if c.deps.BleveMeta != nil { ... }` per pitfall 5 (compactor_test.go literals would otherwise break).
- Use `c.now().UnixMilli()` for the timestamp (`c.now` is the canonical clock source — research A3).
- Result is non-fatal: `_ = c.deps.BleveMeta.SetMeta(...)`.

**File 3:** `internal/daemon/compact_wiring.go` (thread dep into factory)
**Analog:** `ensureCompactor` at `compact_wiring.go:172-178`:
```go
c := compact.NewCompactor(ws, repoID, b.cfg, compact.Deps{
    Gate:       gate,
    Store:      b.store,
    OverlayOps: b.store,
    Metrics:    b.metrics,
    Logger:     b.logger,
})
```

**Apply:** add `BleveMeta: <handle>,` to the literal. The handle must resolve the per-workspace `*retrieval.Engine` (lives in `semanticBundle.engines[ws.RepoRoot]` per semantic_wiring.go:594). Planner must thread a `BleveMetaResolver` (or a closure `func(workspace.WorkspaceKey) BleveMeta`) into `compactBundle` at construction time — analog pattern to how `b.live`, `b.rankBundle`, `b.lspQueue` are already threaded through.

---

### Adapter Replacement — `semSchedulerAdapter.ClusterStatus`

**File:** `internal/daemon/semantic_wiring.go` (lines 408, 441-462)
**Analog (placeholder to replace):**
```go
// semantic_wiring.go:450-462 — current placeholder
func (a *semSchedulerAdapter) ClusterStatus(repoID string) semantic.ClusterStatus {
    _ = repoID
    return semantic.ClusterStatus{
        State:  "unknown",
        Reason: "phase-62-clustering-no-status-accessor",
    }
}
```

**Apply to Phase 69 (research §Code-Examples §State Derivation):**
- Inject `*Store` access via the same `a.bundle.store` field shape used by `semStoreAdapter` (semantic_wiring.go:398-405 region).
- Call `a.bundle.store.CurrentGraphVersion(ctx, repoID)`; if 0 → `{State:"unknown", Reason:"no-graph-version"}`.
- Call `a.bundle.store.ClusterStatusForGraphVersion(ctx, repoID, gv)`; if rows missing → `{State:"unknown", Reason:"no-cluster-rows"}`.
- If rows present and gv matches → `{State:"current", ComputedAt:..., MemberCount:...}`.
- Per research Q1: omit `"building"` state — no daemon-resident cluster scheduler exists.
- Per research §Open-Q3 anti-pattern: do NOT trust `semantic_clusters.score`; rely on the `COUNT(*) FROM semantic_cluster_members` derived inside the accessor.

**Deviation:** placeholder uses `_ = repoID`. New impl actually consumes the arg.

---

### Adapter Addition — `semRetrievalAdapter.RetrievalStatus` (NEW method)

**File:** `internal/daemon/semantic_wiring.go` (extend `semRetrievalAdapter` block at 581-639)
**Analog (engine lookup pattern):**
```go
// semantic_wiring.go:588-595
func (a *semRetrievalAdapter) engineFor(ws workspace.WorkspaceKey) *retrieval.Engine {
    if a == nil || a.bundle == nil {
        return nil
    }
    a.bundle.mu.Lock()
    defer a.bundle.mu.Unlock()
    return a.bundle.engines[ws.RepoRoot]
}
```

**Apply to Phase 69:**
- Add `RetrievalStatus(ws workspace.WorkspaceKey) semantic.RetrievalStatus` method on `semRetrievalAdapter`.
- Use the existing `engineFor(ws)` helper to grab the per-workspace `*retrieval.Engine`.
- If `engine == nil` → `{Reason:"bleve-unavailable"}` (closed enum per research Q6).
- For each meta key: `engine.GetMeta("corpus_version")` → parse uint64 (nil bytes → `Reason:"corpus_version-uninitialized"`).
- `engine.DocCount()` (new wrapper) → `IndexedSymbols int64`.
- Compare `corpus_version < store.CurrentGraphVersion` → `Reason:"corpus_version-lag"` (priority order from research Open-Q2: `bleve-unavailable` > `corpus_version-uninitialized` > `corpus_version-lag` > `compactor-never-ran`).

---

### Envelope Extension (additive, JSON-stable)

**File:** `internal/skill/semantic/envelope.go` (lines 67-112)
**Analog:** existing `ClusterStatus` struct at `envelope.go:67-74`:
```go
type ClusterStatus struct {
    State  string `json:"state"`
    Reason string `json:"reason,omitempty"`
}
```

**Apply to Phase 69 (CONTEXT.md D4):**
- Add two `omitempty` fields to `ClusterStatus`: `ComputedAt int64 json:"computed_at,omitempty"`, `MemberCount int json:"member_count,omitempty"`. PRESERVE existing field names + JSON tags (closed-enum invariant per CONVENTIONS.md cited in CONTEXT.md L27).
- Declare new `RetrievalStatus` struct (CONTEXT.md D2 exact shape) — CorpusVersion uint64, IndexedFiles int64, IndexedSymbols int64, LastCompactAt int64, Reason string `omitempty`.
- Extend `StatusResult` at envelope.go:104-112 with `RetrievalStatus RetrievalStatus json:"retrieval_status"`. KEEP `RetrievalPending bool` at top level (CONTEXT.md D2 — Phase 64 consumers depend on it).
- Per pitfall 4: tests should assert non-zero rather than presence (omitempty drops zero-value `int64`).

---

### Accessor Interface Extension

**File:** `internal/skill/semantic/accessors.go` (lines 41-53, 96-111)
**Analog:** existing `SchedulerAccessor` / `RetrievalAccessor` (already shown above).

**Apply:**
- `SchedulerAccessor.ClusterStatus(repoID string) ClusterStatus` signature unchanged — return-value struct grows additively (covered by envelope extension).
- Add new method to `RetrievalAccessor`:
  ```go
  // RetrievalStatus returns the bleve-corpus state for the given workspace
  // ({corpus_version, indexed_files, indexed_symbols, last_compact_at}).
  // Closed-enum Reason set: bleve-unavailable > corpus_version-uninitialized
  // > corpus_version-lag > compactor-never-ran.
  RetrievalStatus(ws workspace.WorkspaceKey) RetrievalStatus
  ```
- All existing test fakes (e.g., `e2eRetrievalAcc` at integration_test.go:275-289) MUST grow a `RetrievalStatus(ws) RetrievalStatus` stub method — flag for planner: this is a build-breaking interface change. Audit & extend every implementer.

---

### E2E Fixture Extension

**File:** `internal/skill/semantic/integration_test.go` (new test fn near line 470)
**Analog 1 (test skeleton):** `TestE2E_IndexThenContext_SymbolCount` at `integration_test.go:445-470`:
```go
func TestE2E_IndexThenContext_SymbolCount(t *testing.T) {
    const expectedSymbolCount = 15
    h := newE2EHarness(t, "review", makeFixtureFacts(expectedSymbolCount))

    res := h.skill.handleIndexSemanticGraph(context.Background(), IndexSemanticGraphArgs{Mode: "full"})
    if res.IsError {
        t.Fatalf("index returned error: %s", extractText(t, res))
    }
    ir := decodeIndexResult(t, res)
    if ir.SnapshotID == 0 {
        t.Fatalf("SnapshotID: got 0, want non-zero (build did not commit)")
    }
    // ...
}
```

**Analog 2 (placeholder adapter to swap out):** `e2eSchedAcc` at `integration_test.go:241-247`:
```go
type e2eSchedAcc struct{}

func (a *e2eSchedAcc) IsQuiescent(_ string) bool                 { return true }
func (a *e2eSchedAcc) ScoreStatus(_, _ string) graph.ScoreStatus { return graph.ScoreStatusMissing }
func (a *e2eSchedAcc) ClusterStatus(_ string) ClusterStatus {
    return ClusterStatus{State: "unknown", Reason: "phase-62-clustering-no-status-accessor"}
}
```

**Apply to Phase 69 (research §Q5 + §Code-Examples):**
- New test `TestE2E_IndexThenStatus_NonPlaceholderClusterAndRetrieval`.
- Reuse `newE2EHarness("review", makeFixtureFacts(15))` (Phase 64 P07 harness).
- After indexing, call `primeBleveCorpus(...)` (already exists in `internal/daemon/integ_lookup_e2e_helpers.go:185-206`) to ensure bleve docs + meta are populated.
- Seed clusters via `h.store.BeginOverlayTx → UpsertClusters → UpsertClusterMembers → Commit` (existing overlay APIs at `internal/semantic/store/overlay.go:814,850`).
- Swap `e2eSchedAcc` for a real `*Store`-backed adapter (or a new harness helper that wraps the real `semSchedulerAdapter` shape from semantic_wiring.go).
- Swap `e2eRetrievalAcc` to include the new `RetrievalStatus` method returning real bleve-derived values.
- Assertions: `sr.ClusterStatus.State == "current"`, `sr.ClusterStatus.MemberCount > 0`, `sr.ClusterStatus.ComputedAt > 0`, `sr.RetrievalStatus.CorpusVersion == gv`, `sr.RetrievalStatus.IndexedSymbols > 0`.

**Deviation:** existing E2E test asserts on `SnapshotID`/`IterateCommittedSymbols`; new test asserts on the full status envelope shape. Decoder for `StatusResult` may need to be added (mirror `decodeIndexResult`).

---

## Shared Patterns

### Pattern: D-09 Read-Path Invariant (apply to all new accessors)

**Source:** `internal/semantic/store/effective_graph.go:155-176` (`CountStaleScoreRows` doctrine)
**Apply to:** `ClusterStatusForGraphVersion` and any companion read accessor in this phase.

**Rules (compile-time enforced via vet-nokernel2semantic + grep audit per CONTEXT.md L94):**
- Method receiver is `*Store`, NOT `*OverlayTx`.
- Body uses `s.db.QueryRowContext` / `s.db.QueryContext`, never `BeginOverlayTx` / `LockOverlayWorkspace`.
- No `Begin*` / `Commit*` / `Abort*` / `Write*` on the read path.

### Pattern: Nil-Safe Optional Dep (apply to compactor extension)

**Source:** `internal/semantic/compact/compactor.go:222-225` (Metrics nil-guard)
**Apply to:** new `c.deps.BleveMeta` write call.

```go
if c.deps.Metrics != nil {
    c.deps.Metrics.SemanticCompactionObserve(...)
}
```

Existing compactor tests construct `compact.Deps` literals (research pitfall 5). Every new field MUST be nil-safe at the dereference site.

### Pattern: Single-Writer Bleve Meta (apply to recovery.go + compactor.go)

**Source:** `internal/semantic/retrieval/recovery.go:258` (`metaKeyLastIndexed` writer)
**Apply to:** new `corpus_version`, `indexed_files` (writer = `Recoverer.rebuildBlocking`); new `last_compact_at` (writer = `Compactor.runCompaction`).

**Rule:** each meta key has EXACTLY ONE writer site in the codebase. No fallback writes, no double-writes. Grep audit: every new `SetMeta("corpus_version", ...)` call MUST appear in `recovery.go` only; every new `SetMeta("last_compact_at", ...)` MUST appear in `compactor.go` only.

### Pattern: Closed-Enum Reason Strings (apply to envelope + adapters)

**Source:** existing `ClusterStatus.Reason` values like `"phase-62-clustering-no-status-accessor"` (envelope.go:67-74).
**Apply to:** new `RetrievalStatus.Reason` values.

Per research Q6 the closed set is: `""`, `"bleve-unavailable"`, `"corpus_version-uninitialized"`, `"corpus_version-lag"`, `"compactor-never-ran"`. Priority order documented in tools_status.go composition.

### Pattern: Placeholder Comment Removal (success criterion #5)

**Source:** comment blocks at `internal/daemon/semantic_wiring.go:407-409, 441-443, 450-455`.
**Apply:** all three placeholder doc-comments MUST disappear after Phase 69 lands (grep-audit: `grep -n "phase-62-clustering-no-status-accessor\|W1 placeholder" internal/daemon/semantic_wiring.go` returns no matches).

---

## No Analog Found

None — every Phase 69 file extends an existing site or mirrors an existing pattern. This is a closure phase by construction.

---

## Metadata

**Analog search scope:**
- `internal/semantic/store/` (read accessors)
- `internal/semantic/retrieval/` (bleve meta + recovery)
- `internal/semantic/compact/` (compactor deps + write site)
- `internal/skill/semantic/` (envelope + integration tests + accessors)
- `internal/daemon/` (adapters + compact/semantic wiring)

**Files scanned:** 11 (all in-repo, all read via `Read` tool)
**Pattern extraction date:** 2026-05-14
