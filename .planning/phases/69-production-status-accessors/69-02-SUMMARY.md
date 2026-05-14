---
phase: 69
plan: 02
subsystem: semantic-retrieval
tags: [semantic-graph, status, retrieval, bleve, single-writer, tdd]
dependency_graph:
  requires: []
  provides:
    - "retrieval.MetaKeyCorpusVersion (exported string const) — readable from internal/semantic/compact (Plan 69-03) and internal/daemon (Plan 69-05)"
    - "retrieval.MetaKeyIndexedFiles (exported string const)"
    - "retrieval.MetaKeyLastCompactAt (exported string const) — written by Plan 69-03 compactor"
    - "(*retrieval.Engine).DocCount() (uint64, error) — Plan 69-05 indexed_symbols source"
    - "Recoverer.rebuildBlocking single-writer commit point now writes corpus_version + indexed_files alongside last_indexed_snapshot_id"
  affects:
    - "internal/semantic/retrieval/bleve.go"
    - "internal/semantic/retrieval/recovery.go"
    - "internal/semantic/retrieval/recovery_test.go"
tech_stack:
  added: []
  patterns:
    - "Narrow consumer-defined interface seam (bleveMetaWriter) for SetMeta failure-injection in tests"
    - "Non-fatal Warn policy on best-effort meta writes (corpus_version, indexed_files)"
    - "Fatal-on-error preserved for last_indexed_snapshot_id (single-writer invariant)"
key_files:
  created:
    - "internal/semantic/retrieval/exported_keys_external_test.go"
  modified:
    - "internal/semantic/retrieval/bleve.go"
    - "internal/semantic/retrieval/recovery.go"
    - "internal/semantic/retrieval/recovery_test.go"
decisions:
  - "Exposed three EXPORTED meta-key constants (MetaKeyCorpusVersion/MetaKeyIndexedFiles/MetaKeyLastCompactAt) beside the existing unexported metaKeyLastIndexed; downstream plans (69-03, 69-05) consume them directly with no re-export shim"
  - "Added bleveMetaWriter narrow seam (UpsertBatch/GetMeta/SetMeta) on Recoverer to allow test-time injection of a failing-SetMeta shim — keeps the non-fatal Warn assertion honest without build tags"
  - "Introduced test-only constructor newRecovererWithMetaWriter(engine, writer, reader, logger); production NewRecoverer keeps a 3-arg signature and points writer at the same *Engine value"
  - "StoreReader interface extended with CurrentGraphVersion(ctx, repoID) (uint64, error); *Store already implements it natively at internal/semantic/store/overlay.go:983 (no producer-side work needed)"
  - "Non-fatal Warn format: r.logger.Warn(\"retrieval.rebuild: <step> failed (non-fatal)\", \"repo_id\", repoID, \"err\", err) — \"(non-fatal)\" suffix included in message text so operators triaging logs can distinguish it from the fatal last_indexed_snapshot_id failure"
  - "corpus_version is skipped (not written) when CurrentGraphVersion returns gv == 0 — avoids stamping a misleading zero into the bleve meta when the overlay has never been initialized"
  - "Distinct-file accumulator uses map[string]struct{} keyed on SymbolRow.FileID (string in the current store schema, decimal-encoded uint64 today); treats the field as opaque so a future numeric migration does not break this code"
  - "Replaced bytes.Buffer with mutex-guarded syncBuffer in the Warn-capture tests — the rebuild goroutine writes log records concurrently with the test assertion read; -race flagged this on first run, fixed pre-commit"
metrics:
  duration: "~15 minutes"
  completed: "2026-05-14"
  tests_added: 6   # 1 DocCount + 4 subtests in TestRebuild_WritesCorpusVersionAndFileCount + 1 external-test compile guard
---

# Phase 69 Plan 02: Bleve Corpus-State Meta + DocCount Accessor Summary

**One-liner:** Single-writer extension of `Recoverer.rebuildBlocking` to stamp `corpus_version` (from `store.CurrentGraphVersion`) and `indexed_files` (distinct-FileID count) into bleve internal-meta after a successful rebuild, plus `(*Engine).DocCount()` wrapping `bleve.Index.DocCount()`; three exported MetaKey constants land for cross-package readers in Plans 69-03 and 69-05.

## Commits

| Commit   | Type              | Message                                                                                  |
| -------- | ----------------- | ---------------------------------------------------------------------------------------- |
| 50c72d90 | test(69-02) RED   | RED bleve meta writes + DocCount + exported MetaKey constants                            |
| 94219248 | feat(69-02) GREEN | write corpus_version/indexed_files meta + DocCount wrapper + export MetaKey constants    |

## What Changed

### `internal/semantic/retrieval/bleve.go`
- **Added exported constants** at top of file (beside unexported `metaKeyLastIndexed` in `recovery.go`):
  - `MetaKeyCorpusVersion = "corpus_version"`
  - `MetaKeyIndexedFiles = "indexed_files"`
  - `MetaKeyLastCompactAt = "last_compact_at"`
- **Added `(*Engine).DocCount() (uint64, error)`** — wraps `e.idx.DocCount()`, mirrors GetMeta nil-guards, error wrapping format `idx.DocCount: %w`.

### `internal/semantic/retrieval/recovery.go`
- **Extended `StoreReader` interface** with `CurrentGraphVersion(ctx, repoID) (uint64, error)`. `*Store` already implements this (overlay.go:983) — no producer changes needed.
- **Added `bleveMetaWriter` interface** covering `UpsertBatch`/`GetMeta`/`SetMeta` so tests can substitute a failing-SetMeta shim. `*Engine` satisfies it natively.
- **`Recoverer` struct** now has both `engine *Engine` and `writer bleveMetaWriter`. Production `NewRecoverer` sets `writer = engine`; test-only `newRecovererWithMetaWriter` lets callers split the two.
- **`Probe` and `rebuildBlocking`** now route meta reads/writes through `r.writer` instead of `r.engine` (semantically identical in production).
- **`rebuildBlocking` post-flush block**:
  - Accumulates `distinctFiles := map[string]struct{}{}` over the walk loop (keyed on `row.FileID`).
  - Calls `r.store.CurrentGraphVersion(ctx, repoID)`. On error → Warn (non-fatal). On `gv > 0` → `r.writer.SetMeta(MetaKeyCorpusVersion, …)`; on error → Warn (non-fatal).
  - Always writes `MetaKeyIndexedFiles` with `strconv.FormatInt(int64(len(distinctFiles)), 10)`; on error → Warn (non-fatal).
  - Finally writes `metaKeyLastIndexed` — FATAL on error (single-writer invariant preserved).

### `internal/semantic/retrieval/recovery_test.go`
- Extended `fakeStoreReader` with `CurrentGraphVersion` injection (and `graphVersionCalls` counter for invariant assertions).
- Added `syncBuffer` (mutex-guarded log sink) — replaces `bytes.Buffer` to avoid the -race detector tripping on the goroutine→test handoff.
- Added `engineFailingSetMeta` test shim that returns a configured error from `SetMeta(failKey, …)` and passthroughs the rest.
- Added `TestEngine_DocCount` (empty index → 0; after `UpsertBatch` of 5 distinct docs → 5).
- Added `TestRebuild_WritesCorpusVersionAndFileCount` with four subtests:
  1. `writes_corpus_version_and_indexed_files_when_gv_nonzero` (gv=7, 9 symbols across 3 files → `corpus_version=7`, `indexed_files=3`)
  2. `skips_corpus_version_when_gv_zero` (gv=0 → no `corpus_version` key written; `indexed_files=2` still written)
  3. `setmeta_corpus_version_failure_is_non_fatal` (`engineFailingSetMeta` rejects `corpus_version` → rebuild completes; `metaKeyLastIndexed` + `indexed_files` still written; Warn observed)
  4. `metaKeyLastIndexed_failure_remains_fatal` (`engineFailingSetMeta` rejects `last_indexed_snapshot_id` → goroutine logs Error; key not present)

### `internal/semantic/retrieval/exported_keys_external_test.go` (NEW)
- `package retrieval_test` compile-time guard that all three `retrieval.MetaKey*` constants resolve from outside the package — protects against a future refactor downcasing the leading letter and breaking Plans 69-03/69-05.

## Plan Truths — Status

| Truth | Status |
| ----- | ------ |
| Recoverer.rebuildBlocking writes corpus_version meta (uint64 string-encoded) after UpsertBatch | ✅ done (recovery.go:315) |
| Recoverer.rebuildBlocking writes indexed_files meta (int64 string-encoded, distinct FileID count) | ✅ done (recovery.go:325) |
| Engine exposes DocCount() (uint64, error) wrapping idx.DocCount | ✅ done (bleve.go) |
| corpus_version / indexed_files SetMeta failures are logged via r.logger.Warn (non-fatal) | ✅ done — Warn messages tagged "(non-fatal)" |
| metaKeyLastIndexed write retains fatal-on-error policy (single-writer site preserved) | ✅ done (recovery.go:333) |
| Exported MetaKey* constants reachable from package retrieval_test | ✅ done (external test compiles) |

## Verification Output

```
$ go test ./internal/semantic/retrieval/ -count=1 -race -timeout 60s
ok  	github.com/agenthands/helix/internal/semantic/retrieval	3.686s

$ go vet ./internal/semantic/retrieval/...
(clean)

$ grep -rn 'SetMeta(MetaKeyCorpusVersion' internal/
internal/semantic/retrieval/recovery.go:315  (exactly one match — single-writer invariant)

$ grep -rn 'SetMeta(MetaKeyIndexedFiles' internal/
internal/semantic/retrieval/recovery.go:325  (exactly one match — single-writer invariant)

$ go test ./internal/semantic/... -count=1 -timeout 120s
(all 18 semantic packages PASS — no upstream regression)
```

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Replaced `bytes.Buffer` with `syncBuffer` (mutex-guarded) in Warn-capture tests**
- **Found during:** GREEN verification (`-race` mode)
- **Issue:** `TestRebuild_WritesCorpusVersionAndFileCount` subtests `setmeta_corpus_version_failure_is_non_fatal` and `metaKeyLastIndexed_failure_remains_fatal` failed under `-race`: the rebuild goroutine writes log records concurrently with the test reading `logBuf.String()`.
- **Fix:** Introduced a small `syncBuffer{ mu sync.Mutex; buf bytes.Buffer }` adapter with `Write`/`String` taking the lock; switched `captureLogger` to consume it.
- **Files modified:** `internal/semantic/retrieval/recovery_test.go`
- **Commit:** 94219248 (folded into GREEN)

No other deviations. StoreReader extension was already anticipated in the plan body; bleveMetaWriter seam was an offered option in `<action>` (Task 1) and we chose the explicit interface variant over a build-tag workaround.

## Downstream Consumer Contracts

For Plan 69-03 (compactor) and Plan 69-05 (daemon adapter):

- Reference constants as `retrieval.MetaKeyCorpusVersion`, `retrieval.MetaKeyIndexedFiles`, `retrieval.MetaKeyLastCompactAt` — never string literals.
- `corpus_version` may be absent (key missing) when the overlay has never reached `gv > 0`. Callers MUST treat `(nil, nil)` from `GetMeta` as "uninitialized" rather than "zero".
- `indexed_files` is always written on a successful rebuild, even when zero distinct files were seen (string `"0"` on the key).
- `(*Engine).DocCount()` is safe to call concurrently with `UpsertBatch`/`QueryBleve` per bleve's documented concurrency model; returns `(0, "engine closed")` after `Close`.

## Self-Check: PASSED

- `internal/semantic/retrieval/bleve.go` — modified, MetaKeyCorpusVersion/DocCount present (`grep`)
- `internal/semantic/retrieval/recovery.go` — modified, two new `SetMeta(MetaKey…)` sites present
- `internal/semantic/retrieval/recovery_test.go` — modified, `TestEngine_DocCount` + `TestRebuild_WritesCorpusVersionAndFileCount` present
- `internal/semantic/retrieval/exported_keys_external_test.go` — created (package retrieval_test, compiles)
- Commit `50c72d90` (RED test) — present in `git log`
- Commit `94219248` (GREEN feat) — present in `git log`
