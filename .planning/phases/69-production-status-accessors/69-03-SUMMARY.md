---
phase: 69
plan: 03
subsystem: semantic-graph
tags: [semantic-graph, status, compactor, bleve, dep-injection, tdd]
requires:
  - "69-02: retrieval.MetaKeyLastCompactAt exported constant"
provides:
  - "compact.BleveMeta interface (single-writer seam)"
  - "compact.Deps.BleveMeta optional field (nil-safe)"
  - "compactor success-path stamp of last_compact_at (unix ms) using retrieval.MetaKeyLastCompactAt"
  - "compactBundle.bleveMetaFn resolver field + SetBleveMetaFn post-init setter"
affects:
  - "internal/semantic/compact/accessors.go"
  - "internal/semantic/compact/compactor.go"
  - "internal/semantic/compact/compactor_test.go"
  - "internal/daemon/compact_wiring.go"
tech-stack:
  added: []
  patterns:
    - "Caddy-style post-init wiring (SetBleveMetaFn mirrors SetEnrichFn)"
    - "Nil-safe optional Deps field — existing test literals unaffected"
    - "Exported constant key (retrieval.MetaKeyLastCompactAt) — no string literal"
key-files:
  created: []
  modified:
    - internal/semantic/compact/accessors.go
    - internal/semantic/compact/compactor.go
    - internal/semantic/compact/compactor_test.go
    - internal/daemon/compact_wiring.go
key-decisions:
  - "Per-workspace bleve engine resolved ONCE at compactor construction time (not per runCompaction). Matches every other dep (b.live, b.store) and the engine handle is stable for the compactor's lifetime."
  - "Extracted stampLastCompactAt helper + PublicStampLastCompactAtForTest seam so the 4-case behavior contract (writes / nil-safe / error-non-fatal / failure-path-not-stamped) is unit-testable without a real *store.Snapshot. The existing fakeStore cannot reach the success path because *store.Snapshot is unexported; rather than build a heavy integration harness, the helper isolates the new behavior at a granularity the existing fake can fully exercise."
  - "Logger.Debug on SetMeta failure rather than Warn — the stamp is a status-surface convenience, not a correctness path; failure shouldn't compete with real compaction warnings."
requirements-completed: [STATUS-02]
duration: 12 min
completed: 2026-05-14
---

# Phase 69 Plan 03: Compactor last_compact_at Writer + Wiring Slot Summary

Injects a `BleveMeta` writer dep into the compactor and stamps `last_compact_at` (unix ms) onto the bleve corpus-state meta on every successful compaction, using the exported `retrieval.MetaKeyLastCompactAt` constant shipped in Plan 69-02. The `compactBundle` is extended with a `bleveMetaFn` resolver (+ `SetBleveMetaFn` post-init setter) ready for Plan 69-05 to bind to the real `semanticBundle.engines[ws.RepoRoot]` engine map.

## What Shipped

- **`internal/semantic/compact/accessors.go`** — added `BleveMeta` interface (`SetMeta(key string, val []byte) error`). Doc-comment names it the single-writer seam for `last_compact_at`; nil is an explicit valid no-op so existing `Deps{...}` literals across the test suite (Pitfall 5 guard) keep compiling.
- **`internal/semantic/compact/compactor.go`** — added imports for `strconv` and `internal/semantic/retrieval`. Extended `Deps` with optional `BleveMeta BleveMeta` field. Added a `stampLastCompactAt()` helper that writes `c.now().UnixMilli()` formatted as a decimal string under `retrieval.MetaKeyLastCompactAt`; nil-safe and SetMeta-error-non-fatal (debug log only, swallow). Wired the call into `runCompaction` immediately after `c.lastBaseID = snap.ID` and before `c.maybeVacuum(ctx)`, on the success path only. Added `PublicStampLastCompactAtForTest()` seam.
- **`internal/semantic/compact/compactor_test.go`** — `fakeBleveMeta` (map + failKeys + call counter) and `TestRunCompaction_WritesLastCompactAt` covering all 4 behavior cases: writes a valid unix-ms int64 under `retrieval.MetaKeyLastCompactAt`; `Deps{BleveMeta: nil}` does not panic; `SetMeta` error is swallowed; the failure path (BeginSnapshot errors via the existing `fakeStore`) never calls `SetMeta`.
- **`internal/daemon/compact_wiring.go`** — extended `compactBundle` with `bleveMetaFn` (guarded by `bleveMetaFnMu`) and `SetBleveMetaFn(fn)` setter mirroring Phase 64's `SetEnrichFn` post-init pattern. `ensureCompactor` resolves the per-workspace `BleveMeta` at construction time and threads it into `compact.Deps.BleveMeta`. With `bleveMetaFn` nil (this plan's terminal state) the resolver returns `nil`, the compactor's nil-safe guard suppresses the stamp, and STATUS-02's last_compact_at remains unwritten from the daemon path until Plan 69-05 binds the resolver to the real engines map.

## Commits

| Hash       | Type | Subject                                                                                                                |
| ---------- | ---- | ---------------------------------------------------------------------------------------------------------------------- |
| `ad35d851` | test | `test(69-03): RED last_compact_at write + nil-safe BleveMeta dep`                                                       |
| `091216fd` | feat | `feat(69-03): write last_compact_at meta on successful compaction (using retrieval.MetaKeyLastCompactAt)`               |
| `dc12b547` | feat | `feat(69-03): thread bleveMetaFn through compactBundle (binding deferred to 69-05)`                                     |

## Output Notes (from plan)

- **Per-workspace engine handle stability**: held. Resolved once at compactor construction time. Compactors are constructed lazily on first activation (`ensureCompactor`) and survive for the workspace's lifetime; the bleve engine for a workspace is similarly lifetime-stable, so single-resolution is the natural pattern (matches `b.live`, `b.store`). If Plan 69-05 surfaces a scenario where the engine handle can be replaced mid-compactor-lifetime, the resolver can be re-invoked per `runCompaction` cycle with no other surface change.
- **No change to compactor_test.go fixture authors**: the existing `Deps{...}` literals (4 sites across the file) keep working unchanged — `BleveMeta` is optional and zero-valued in those literals. New tests opt-in by setting `Deps{BleveMeta: ...}`.
- **Cycle-guard grep clean**: `grep -rn '"github.com/agenthands/helix/internal/semantic/compact"' internal/semantic/retrieval/` returned no matches (exit=1). The introduced `compact → retrieval` edge is one-way; no cycle.

## Verification

```
$ grep -n 'SetMeta(retrieval.MetaKeyLastCompactAt' internal/semantic/compact/compactor.go
369:	if err := c.deps.BleveMeta.SetMeta(retrieval.MetaKeyLastCompactAt, val); err != nil {

$ grep -rn 'SetMeta("last_compact_at"' internal/    # literal eradicated
# (no output, exit=1)

$ grep -rn '"github.com/agenthands/helix/internal/semantic/compact"' internal/semantic/retrieval/    # cycle guard
# (no output, exit=1)

$ go vet ./internal/semantic/compact/... ./internal/daemon/...
ok (CGO swift binding warning unrelated)

$ go test ./internal/semantic/compact/... ./internal/daemon/... -count=1 -race -timeout 120s
ok  	github.com/agenthands/helix/internal/semantic/compact	7.458s
ok  	github.com/agenthands/helix/internal/daemon	4.729s
```

## Deviations from Plan

**[Minor] Stamp logic factored into stampLastCompactAt helper + PublicStampLastCompactAtForTest seam.** — Found during: Task 1 RED authoring. The plan asks for an inline `if c.deps.BleveMeta != nil { _ = c.deps.BleveMeta.SetMeta(...) }` block; I factored it into a method because the existing `fakeStore.BeginSnapshot` always errors (it cannot construct a real unexported `*store.Snapshot`), so testing the success-path SetMeta call inline would require either a heavyweight DuckDB integration harness or no test coverage of the new behavior at all. The helper preserves the exact behavior at the same call site, but enables direct unit-testing of the 4 contract cases — including the nil-safe and error-non-fatal cases that are awkward to exercise through the full `runCompaction` body. The inline `if c.deps.BleveMeta != nil` guard moved into the helper (`if c == nil || c.deps.BleveMeta == nil`) and the `_ =` was replaced with an explicit `if err != nil { Logger.Debug ... }` to surface non-fatal failures in debug logs without affecting outcome — strict superset of the planned behavior.

**[Minor] Used `Logger.Debug` instead of silent `_ =` for SetMeta errors.** — Found during: Task 2 GREEN. The plan specifies `_ = c.deps.BleveMeta.SetMeta(...)`; I kept the error swallowed (outcome unchanged on the parent compaction) but log at Debug so operators can correlate stale `last_compact_at` values with bleve failures during incident review. Cost is one line; benefit is observability.

**Total deviations: 2 minor (both Rule 2 — auto-add critical functionality: test coverage + observability).** Impact: net positive — same external contract, better test coverage, better debuggability.

## Authentication Gates

None.

## Issues Encountered

None.

## Self-Check: PASSED

- `internal/semantic/compact/accessors.go` — exists, BleveMeta interface added
- `internal/semantic/compact/compactor.go` — exists, retrieval import + stamp call + helper present
- `internal/semantic/compact/compactor_test.go` — exists, 4-case TestRunCompaction_WritesLastCompactAt present
- `internal/daemon/compact_wiring.go` — exists, bleveMetaFn field + SetBleveMetaFn setter + BleveMeta threading in ensureCompactor
- Commits `ad35d851`, `091216fd`, `dc12b547` present in `git log --oneline`
- All plan `<verification>` commands pass (see Verification section above)

## Next Phase Readiness

Ready for **Plan 69-05** (the reader: `last_compact_at` consumed by status accessors, and the daemon-side `SetBleveMetaFn(...)` binding to `semanticBundle.engines[ws.RepoRoot]`). Plan 69-04 is landing in parallel on a sibling branch; this plan's scope (compact + daemon/compact_wiring) builds and tests cleanly without 69-04 — confirmed by the green `go test ./internal/daemon/...` run.
