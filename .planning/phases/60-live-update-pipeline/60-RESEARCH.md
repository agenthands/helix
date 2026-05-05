# Phase 60: Live Update Pipeline — Research

**Researched:** 2026-05-05
**Domain:** Filesystem watch + change-coalescing pipeline; DuckDB schema migration; kernel→semantic decoupling via interface seam.
**Confidence:** HIGH (every load-bearing claim verified against in-tree source, fsnotify v1.9.0 GitHub source, official Vim/inotify man pages, or pkg.go.dev `time` docs).

## Summary

Phase 60 has an unusually mature `60-CONTEXT.md` — seven decisions (D-01..D-07) and 13 acceptance criteria are already locked. The remaining research surface is narrow but load-bearing:

1. **Library / OS behavior verification.** fsnotify v1.9.0 quirks (Rename event always carries the OLD path; ENOSPC propagates unwrapped from `inotify_add_watch`; the `Add()` doc explicitly tells you NOT to watch individual files because of atomic rename — directory watches are mandatory). Vim's `backupcopy=auto` defaults to **rename-and-create-new** when safe, which is the swap-rename pattern that drops fsnotify file-watches; JetBrains uses a documented two-rename `___jb_old___`/`___jb_tmp___` sequence; VS Code does NOT atomic-save by default — only when `files.atomicSave` is enabled.
2. **DuckDB ALTER TABLE constraint.** Phase 59 D-05 already documented in `migrations.go:380-396` that DuckDB rejects `ADD COLUMN ... NOT NULL DEFAULT <expr>`. Phase 60's v2→v3 migration MUST use `DEFAULT 0` alone for the five new `UBIGINT` columns; CONTEXT.md D-04's SQL block needs that adjustment before the planner converts it to a task.
3. **CONTEXT.md drift on `write_file`.** CONTEXT.md D-03 names eight kernel hook entry points: 5 in `internal/kernel/edit/` + `replace_in_file` + `fuzzy_edit` + `write_file`. The kernel ships `create_file`, NOT `write_file` (verified `internal/kernel/fileops/skill.go:32` and `tools.go:233`). Plus `OverwriteFile` is the helper used by `replace_in_file` / `fuzzy_edit` internally. The planner must reconcile this — either rewire D-03 to 7 + create_file, or add an OnEdit call inside the shared `OverwriteFile` helper (which fires from create_file, replace_in_file, and fuzzy_edit success paths in one place).
4. **Validation Architecture.** All 13 CONTEXT acceptance criteria map cleanly to automated checks; only the editor-fixture suite (#3) needs a small platform-aware skip on Windows for the JetBrains `___jb_old___` step where it's a no-op.

**Primary recommendation:** Adopt CONTEXT.md decisions verbatim with three localized adjustments — (a) the migration SQL DROPs `NOT NULL` per the Phase 59 D-05 DuckDB constraint, (b) the eighth tool wiring is `create_file` (or, better, fire `OnEdit` inside the shared `OverwriteFile`/`os.WriteFile` helper paths in `internal/kernel/fileops/`), (c) the `write_epoch` index choice is per-table on `(repo_id, write_epoch)` per CONTEXT.md and is correct for Phase 63's `WHERE write_epoch <= captured_epoch` scan.

## Architectural Responsibility Map

Phase 60 is below the MCP layer. The "tier" mapping reflects PROCESS layers inside the daemon, not network tiers.

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| fsnotify watcher (per-workspace) | Semantic / live (`internal/semantic/live/watcher`) | OS / kernel (inotify, FSEvents, ReadDirectoryChangesW via fsnotify) | Source of file-mutation events; isolates the OS-quirk surface from classifier. |
| Manifest scanner | Semantic / live (`internal/semantic/live/scanner`) | Filesystem walk + xxhash64 | Correctness floor; ENOSPC fallback; reuses Phase 59 hash function. |
| Helix-edit hook | Kernel-side seam (`internal/kernel/notifier.go`) → Semantic impl | — | Interface lives in kernel so kernel can call it without importing semantic (D-03). |
| Path classification (kind decision) | Semantic / live (`internal/semantic/live/classifier`) | Store reads (`semantic_files.content_hash`) | Single owner of `SourceChangeKind` per D-01. |
| Coalescer / dispatcher | Semantic / live (`internal/semantic/live/coalescer`) | — | Per-workspace goroutine; pure-function coalescing per SPEC §16.2. |
| Overlay write transactions | Semantic / store (`internal/semantic/store/overlay.go`) | DuckDB | Stays inside the `cmd/vet-noduckdb/` allowlisted prefix (`internal/semantic/store/...`); per-workspace epoch lock lives here. |
| Schema migration v2→v3 | Semantic / store (`migrations.go`) | DuckDB ALTER TABLE | Plugs into Phase 57 D-02 explicit registry. |
| Live-update phase wiring | `internal/phasegraph/pipelines/live.go` | — | Phase 57 D-04 left typed-ID stubs; Phase 60 fills `Run` bodies. |
| ENOSPC observability | `internal/obs/` bounded-label metric | slog one-shot warn + watcher `Status()` accessor | Closed-enum labels per Phase 57 D-07. |
| Editor-save fixture suite | `internal/semantic/live/testdata/editors/...` | Go test harness invoking small scripts/programs | Validates LIVE-02 acceptance directly. |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/fsnotify/fsnotify` | **v1.9.0** | Cross-platform filesystem watch | Already in `go.mod:10`. Sole cross-platform Go fsnotify implementation; `internal/memory/watcher.go` already uses it. [VERIFIED: `go.mod:10` + `go.sum`] |
| `github.com/cespare/xxhash/v2` | **v2.3.0** | xxhash64 for content-hash compare | Already in `go.mod`; Phase 59 uses `xxhash.Sum64String` for stable IDs (`internal/semantic/extract/stable_id.go:47`). Same hash for manifest scan symmetric equality. [VERIFIED: `internal/semantic/extract/stable_id.go:6,47`] |
| `github.com/duckdb/duckdb-go/v2` | (current pin) | Schema migration v2→v3 | Already imported by `internal/semantic/store/duckdb.go:21`; Phase 60 stays on the same import (the `cmd/vet-noduckdb/` analyzer enforces `internal/semantic/store/...` as the only allowed prefix). [VERIFIED: `internal/lint/noduckdb/analyzer.go:16-17`] |
| `log/slog` (stdlib) | Go 1.x | Structured logging including ENOSPC one-shot warn | Already used everywhere. [VERIFIED: `internal/memory/watcher.go:5`] |
| `time` stdlib `AfterFunc` | Go 1.x | Debounce timer + max-batch-delay cap | Pattern proven in `internal/memory/watcher.go:109` (`timer = time.AfterFunc(w.debounce, flush)`). [VERIFIED: `internal/memory/watcher.go:109`] |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `internal/obs` (`*obs.Metrics`) | in-tree | Bounded-label metric registration | Add `helix_semantic_live_updates_total{kind, outcome}` via the same pattern `EditOutcomeInc` uses (`internal/obs/metrics.go:339-351`) — switch over closed enums; drop unknowns. |
| `internal/phasegraph` | in-tree | Typed phase IDs + `Run`/`Validate`/`Shutdown` | Phase 57 D-04 left bodies as `noopRun`; Phase 60 fills them. |
| `database/sql` (stdlib) | Go 1.x | Transactional overlay writes | Use `db.BeginTx` for `BeginOverlayTx`; DuckDB driver supports `database/sql` semantics via `modernc.org/sqlite`-style adapter. Phase 60 reuses the existing `*sql.DB` handle on the `Store` struct. |
| `errors` (`errors.Is`) | Go 1.x | `syscall.ENOSPC` detection | fsnotify v1.9.0 returns ENOSPC unwrapped from `inotify_add_watch`. |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `fsnotify` directly | `notify` (`github.com/rjeczalik/notify`) | `notify` supports inotify recursive watches (saves the `addRecursive` walk), but adds a new dependency for marginal gain — `internal/memory/watcher.go` already proves the recursive-add pattern works. |
| `xxhash64` | `xxh3` | xxh3 is faster but Phase 59's stable IDs are already locked to xxhash64; using a different hash would break manifest-scan-vs-extract symmetry (D-05 invariant). |
| Per-tx allocation of `current_epoch` | UUID-style overlay write IDs | UUIDs are non-monotonic; Phase 63's CAS reasoning requires monotone integers (`WHERE write_epoch <= captured_epoch`). |
| Standalone `semantic_watcher_manifest` table | Reuse `semantic_files.content_hash` | A dedicated table avoids a join, but Phase 60 D-05 + CONTEXT `<deferred>` already debated this — single source of truth wins until benchmarks force the optimization. |

**Installation:** Zero new deps. Phase 60 uses only what's already in `go.mod`.

**Version verification:** Verified against the local `go.mod` and `go.sum`. fsnotify v1.10.x exists upstream but Helix is pinned at v1.9.0; no need to bump.

## Architecture Patterns

### System Architecture Diagram

```
                         ┌─────────────────────────────────────────────┐
                         │                                             │
                         │    Source 1: fsnotify Watcher (per-ws)      │
                         │    (Create/Write/Rename/Remove → []paths)   │
                         │                                             │
                         └────────────┬────────────────────────────────┘
                                      │ WorkspaceChangeSignal{paths,fsnotify}
                                      ▼
┌────────────────────────┐    ┌──────────────────────────┐    ┌────────────────────┐
│ Source 2: helix_edit   │───►│  liveService.signal()    │◄───│ Source 3: manifest │
│ kernel.OnEdit(ws,paths)│    │  (per-workspace channel) │    │ scanner periodic   │
│ (8 tool success paths) │    └──────────┬───────────────┘    │ (10s default)      │
└────────────────────────┘               │                    └────────────────────┘
                                         │ WorkspaceChangeSignal
                                         ▼
                              ┌──────────────────────────┐
                              │  ClassifyPathChange      │
                              │  (reads FS + content_hash│
                              │   → SourceChangeKind)    │
                              └──────────┬───────────────┘
                                         │ SourceChangeEvent
                                         ▼
                              ┌──────────────────────────┐
                              │  Per-workspace coalescer │
                              │  (single goroutine,      │
                              │   debounce timer +       │
                              │   max-batch-delay cap)   │
                              └──────────┬───────────────┘
                                         │ on flush:
                                         │  CoalesceEvents([]) → []
                                         │  if > threshold → bulk_update
                                         ▼
                              ┌──────────────────────────┐
                              │  Sequential dispatcher   │
                              │  per Kind:               │
                              │  - UpdateChangedFile     │
                              │  - HandleFileDeleted     │
                              │  - HandleFileRenamed     │
                              │  - HandleBulkUpdate      │
                              └──────────┬───────────────┘
                                         │ extracts via Phase 59 extract.Registry
                                         ▼
                              ┌──────────────────────────┐
                              │  BeginOverlayTx          │
                              │  (per-ws lock,           │
                              │   ++current_epoch,       │
                              │   stamp write_epoch)     │
                              └──────────┬───────────────┘
                                         ▼
                              ┌──────────────────────────┐
                              │  semantic_live_overlay_* │
                              │  (DuckDB, schema v3)     │
                              └──────────┬───────────────┘
                                         │ no-op stubs (P61/P62/P63 fill):
                                         ├──► LSPQueue.Enqueue (typed; no consumer P60)
                                         ├──► GraphCache.ApplyRepair (typed no-op)
                                         └──► MarkAffectedScoresAndClusters (typed no-op)
```

Reader-traceable use case: a Vim user saves `auth.go`. fsnotify emits `Rename(auth.go) + Create(auth.go) + Write(auth.go)` (per Vim default `backupcopy=auto`). All three normalize to `WorkspaceChangeSignal{Paths:[auth.go]}`. Classifier reads disk + hash → `ChangeFileModified`. Coalescer collapses three events to one. Dispatcher invokes `UpdateChangedFile`; Phase 59 `ExtractFile` runs; `BeginOverlayTx` stamps a fresh `write_epoch`; `tx.Commit` writes overlay rows; LSPQueue gets a typed `RevalidateFileJob` that Phase 61 will drain. ENRICH=stub for now.

### Recommended Project Structure

```
internal/semantic/live/
├── service.go              # liveService + EditNotifier impl + signal()
├── signal.go               # WorkspaceChangeSignal + ChangeSource consts (D-01)
├── classifier/
│   ├── classify.go         # ClassifyPathChange (FS + content_hash compare)
│   └── classify_test.go
├── watcher/
│   ├── watcher.go          # WatcherManager + per-workspace fsnotify loop
│   ├── reattach.go         # atomic-rename re-attach (Vim/JetBrains/VS Code)
│   ├── status.go           # WatcherStatus accessor (Phase 65 consumes)
│   └── watcher_test.go
├── scanner/
│   ├── scanner.go          # Manifest scanner per-workspace loop (10s default)
│   ├── walk.go             # File walk with Phase 59 ignore rules (no repomap import)
│   └── scanner_test.go
├── coalescer/
│   ├── coalescer.go        # Per-workspace single-goroutine coalescer
│   ├── coalesce.go         # CoalesceEvents (pure fn per SPEC §16.2)
│   └── coalesce_test.go    # all 6 merge rules + bulk_update threshold
├── handler/
│   ├── update.go           # UpdateChangedFile (SPEC §16.3)
│   ├── delete.go           # HandleFileDeleted (SPEC §16.4)
│   ├── rename.go           # HandleFileRenamed (SPEC §16.5)
│   └── bulk.go             # HandleBulkUpdate
├── lspqueue/
│   └── queue.go            # Typed buffered handoff; no consumer P60 (P61 fills)
└── testdata/
    └── editors/
        ├── vim/save.sh        # Vim swap-rename script (sed/perl-driven)
        ├── jetbrains/save.go  # JetBrains ___jb_tmp___ + ___jb_old___ Go program
        └── vscode/save.go     # VS Code atomic-rename Go program

internal/kernel/
└── notifier.go             # EditNotifier interface + kernel.SetEditNotifier (D-03)

internal/semantic/store/
└── overlay.go              # FILL: BeginOverlayTx, OverlayTx, FlushOverlay (Phase 57 stub)
                            #       NB: stays inside `internal/semantic/store/` per
                            #       cmd/vet-noduckdb allowlist
└── migrations.go           # APPEND: applyMigration003 (5 ADD COLUMN + 4 indexes)

internal/phasegraph/pipelines/
└── live.go                 # FILL: 9 noopRun → real Run bodies

internal/semantic/scheduler/
└── scheduler.go            # FILL: ScheduleIncremental body (was sentinel stub)
```

### Pattern 1: Setter-Style Cross-Package Wiring (D-03)

**What:** Daemon constructs the semantic impl AFTER kernel construction, then injects via setter. Kernel never imports semantic.
**When to use:** Any time downstream package needs upstream package to call into it without an upstream → downstream import.
**Example:**

```go
// internal/kernel/notifier.go (NEW)
package kernel

import (
    "context"
    "github.com/agenthands/helix/internal/workspace"
)

type EditNotifier interface {
    OnEdit(ctx context.Context, ws workspace.WorkspaceKey, paths []string) error
}

// On Kernel:
func (k *Kernel) SetEditNotifier(n EditNotifier) { k.editNotifier.Store(n) }
func (k *Kernel) EditNotifier() EditNotifier {
    if v := k.editNotifier.Load(); v != nil { return v.(EditNotifier) }
    return nil
}
```

```go
// internal/daemon/daemon.go — exact mirror of SetEnrichFn (line 376) /
// SetActivateCallback (line 463) pattern:
liveSvc := live.NewService(store, scheduler, ...)
k.SetEditNotifier(liveSvc) // semantic-side OnEdit impl
```

```go
// internal/kernel/edit/replace.go success path (after the verifier branch
// at line ~371-378):
if n := k.EditNotifier(); n != nil {
    _ = n.OnEdit(ctx, wsKey, []string{args.Path}) // fire-and-forget
}
```

[VERIFIED: `internal/daemon/daemon.go:376` `SetEnrichFn`, line 463 `SetActivateCallback` — confirmed pattern]

### Pattern 2: Debounced fsnotify Loop (D-02)

**What:** Single goroutine drains `Watcher.Events`, accumulates into a `pending` map, fires `time.AfterFunc(debounce)` to flush.
**When to use:** Any "burst-coalescing" filesystem watcher.
**Example:** `internal/memory/watcher.go:60-118` is the load-bearing analog. Phase 60 mirrors the structure but produces `WorkspaceChangeSignal` instead of calling `index.Upsert/Remove` directly.

Critical detail: `time.AfterFunc(d, f)` returns a `*Timer` whose `Reset(d)` resets-and-reuses without allocation. After firing, `Reset` is safe to call. [VERIFIED: pkg.go.dev `time.Timer.Reset` — "Reset stops a timer and resets its period to the specified duration"; AfterFunc is the recommended pattern for reusable timers per the same docs.]

### Pattern 3: Schema Migration via Registry (D-04)

**What:** Append a new `Migration{From: 2, To: 3, Kind: MigrationInPlace, Apply: applyMigration003}` entry; bump `CurrentSchemaVersion` to 3; write the body in `migrations.go`.
**When to use:** Any backward-compatible DuckDB schema change.
**Example:**

```go
// internal/semantic/store/migrations.go — APPEND below applyMigration002
// (line 404). NB: Phase 59 D-05 documented the DuckDB constraint at line
// 380-396: "DuckDB rejects ALTER TABLE ... ADD COLUMN ... NOT NULL DEFAULT
// <expr>". Use DEFAULT 0 alone — the application layer (Phase 60 overlay
// writer) explicitly sets every column on every write.
func applyMigration003(ctx context.Context, db *sql.DB) error {
    stmts := []string{
        // Per-workspace monotone epoch counter on the meta table.
        `ALTER TABLE semantic_live_overlay_meta ADD COLUMN current_epoch UBIGINT DEFAULT 0`,

        // write_epoch stamp on every overlay fact row.
        `ALTER TABLE semantic_live_overlay_files      ADD COLUMN write_epoch UBIGINT DEFAULT 0`,
        `ALTER TABLE semantic_live_overlay_symbols    ADD COLUMN write_epoch UBIGINT DEFAULT 0`,
        `ALTER TABLE semantic_live_overlay_references ADD COLUMN write_epoch UBIGINT DEFAULT 0`,
        `ALTER TABLE semantic_live_overlay_edges      ADD COLUMN write_epoch UBIGINT DEFAULT 0`,

        // Phase 63 CAS scan path: WHERE write_epoch <= captured_epoch.
        `CREATE INDEX idx_overlay_files_write_epoch
            ON semantic_live_overlay_files(repo_id, write_epoch)`,
        `CREATE INDEX idx_overlay_symbols_write_epoch
            ON semantic_live_overlay_symbols(repo_id, write_epoch)`,
        `CREATE INDEX idx_overlay_references_write_epoch
            ON semantic_live_overlay_references(repo_id, write_epoch)`,
        `CREATE INDEX idx_overlay_edges_write_epoch
            ON semantic_live_overlay_edges(repo_id, write_epoch)`,

        `INSERT INTO semantic_schema_version (version, applied_at) VALUES (3, now())`,
    }
    for i, s := range stmts {
        if _, err := db.ExecContext(ctx, s); err != nil {
            return fmt.Errorf("applyMigration003: stmt %d (%s): %w", i+1, firstLine(s), err)
        }
    }
    return nil
}
```

```go
// internal/semantic/store/migrations_registry.go:27-30 — APPEND:
{From: 2, To: 3, Kind: MigrationInPlace, Apply: applyMigration003},
```

```go
// internal/semantic/store/migrations_types.go:16:
const CurrentSchemaVersion = 3 // bump from 2
```

[VERIFIED: `internal/semantic/store/migrations.go:380-396` documents the NOT NULL constraint with prior Phase 59 testing.]

### Pattern 4: Bounded-Label Metric (D-07 cascade)

```go
// internal/obs/metrics.go — APPEND helper after EditOutcomeInc (line 339):

// SemanticLiveUpdatesInc increments helix_semantic_live_updates_total.
// kind ∈ {file_created, file_modified, file_deleted, file_renamed,
//        helix_edit, bulk_update}; outcome ∈ {applied, no_op, error, dropped}.
// Any other value is dropped (closed-enum drop-unknown discipline,
// T-53-01 mitigation).
func (m *Metrics) SemanticLiveUpdatesInc(kind, outcome string) {
    switch kind {
    case "file_created", "file_modified", "file_deleted", "file_renamed",
         "helix_edit", "bulk_update":
    default:
        return
    }
    switch outcome {
    case "applied", "no_op", "error", "dropped":
    default:
        return
    }
    m.SemanticLiveUpdates.WithLabelValues(kind, outcome).Inc()
}
```

The vector itself is constructed in `newMetrics()` and registered in the `reg.MustRegister(...)` block (`internal/obs/metrics.go:223-241`). Phase 60 also extends `internal/obs/metrics_labels_test.go` allowlist to include `kind` (carve-out, like `strategy` and `result`).

### Anti-Patterns to Avoid

- **Watching individual files instead of directories.** fsnotify's own docs explicitly forbid this for atomic-rename editors. [VERIFIED upstream: "Watching individual files (rather than directories) is generally not recommended as many programs (especially editors) update files atomically"]. CONTEXT.md D-01 already commits to directory-level watch.
- **Pre-classifying at the source.** D-01 invariant — sources MUST emit paths only; classifier owns the decision. Don't shortcut "we know this is a fsnotify Remove → ChangeFileDeleted" inside the watcher; it breaks the manifest-scan-vs-watcher symmetric truth.
- **Per-file goroutine fan-out for dispatch.** D-02 invariant — sequential per workspace. Cross-path coalescing only works if a single goroutine owns the per-workspace pending map.
- **Advancing `current_epoch` on no-op flushes.** D-04 invariant — empty coalesced batches DO NOT open a tx. Otherwise epoch sequence becomes useless for status reporting.
- **Empty `pending` map flushes opening a tx.** Subset of the above — if `pending` is empty but the timer fires anyway (e.g., spurious wake), the dispatcher must short-circuit before `BeginOverlayTx`.
- **Importing `internal/repomap` from `internal/semantic/...`.** Phase 59 D-01 invariant cascades. The manifest scanner walker is structurally similar to repomap's walker but is net-new code. The shared ignore-rule list (`.git/`, `node_modules/`, etc.) is the only thing copied — by re-declaration, not import.
- **Importing `duckdb-go` from `internal/semantic/live/...`.** Phase 57 D-12 + `cmd/vet-noduckdb/`. Verified analyzer at `internal/lint/noduckdb/analyzer.go:16` allows ONLY `internal/semantic/store` prefix.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cross-platform file watch | A custom inotify/FSEvents/RDCW wrapper | `github.com/fsnotify/fsnotify` v1.9.0 | Already in `go.mod`; battle-tested by `internal/memory/watcher.go`. |
| Debounce timer logic | A handcoded ticker + counter | `time.AfterFunc` + `Reset` | Stdlib doc-blessed pattern; avoids subtle "fired but not yet drained" race. |
| Atomic file rename | Custom temp-file-then-rename | `internal/kernel/fileops.OverwriteFile` (already does it) | Tool calls go through this; the watcher just observes the resulting fsnotify events. |
| Content hashing | A new hash impl | `xxhash.Sum64String` (already used Phase 59) | Symmetric equality is required for manifest-scan ↔ extract content_hash compare (D-05). |
| Per-package init() registration | Caddy-style `live.Register(&Service{})` | Constructor injection | Phase 59 D-02 invariant cascades — semantic stays init-free. |
| Schema migration framework | A new migration runner | `internal/semantic/store/migrations_registry.go` (existing) | One-liner registry append per Phase 57 D-02. |
| Bounded-label cardinality enforcement | Hand-rolled label sanity | `internal/obs/metrics_labels_test.go` allowlist + `Inc` switch-drop | Standard pattern across all v1.2+ metrics. |
| Per-workspace lock keyed by `repo_id` | A bare `map[string]*sync.Mutex` with custom locking | `sync.Map` over `repo_id → *sync.Mutex`, or `keyedMutex` helper if pattern repeats | One pattern; tested. |
| LSP didChange wire-up for re-extraction | A direct LSP call from semantic | Phase 61 LSPQueue handoff (typed buffered queue) | D-03 keeps semantic out of kernel/lspool; only the daemon wires the LSP enrichment worker (Phase 61). |

**Key insight:** Every "deceptively complex" problem in this phase already has an in-tree solution Phase 60 must mirror. The unique work is the **glue** (signal → classifier → coalescer → overlay), not new infrastructure.

## Runtime State Inventory

> Phase 60 is greenfield code with one schema migration. There is no rename / rebrand / mass-string-replace. This section is included for completeness as the phase touches persisted state.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | DuckDB `semantic_live_overlay_*` tables (Phase 57 created them empty); `semantic_files.content_hash` populated by Phase 59. **No data migration needed** — the v2→v3 ADD COLUMN with `DEFAULT 0` backfills existing rows lazily (or eagerly, depending on DuckDB internals; either way the result is `current_epoch=0` everywhere, which is the intended initial state). | Code edit (migration body) only. No data migration. |
| Live service config | None — Phase 60 is the first live-update wiring. | None. |
| OS-registered state | None — Helix daemon state is process-local. | None — verified by `find . -name '*.plist' -o -name '*.service'` (none in repo). |
| Secrets/env vars | None — fsnotify needs no credentials. | None. |
| Build artifacts | None — Phase 60 adds Go packages but does not modify build pipeline; Phase 59.1's `CGO_ENABLED=1` single-mode is the build invariant. | None. |

**Concern (worth surfacing to the planner):** When DuckDB `ALTER TABLE ADD COLUMN ... DEFAULT 0` runs on a populated overlay (rare in practice — overlay is usually empty between compactions, but Phase 60 ships before Phase 63 so there's no compaction yet), DuckDB documentation does NOT explicitly state whether the default is materialized eagerly or lazily. [CITED: https://duckdb.org/docs/current/sql/statements/alter_table — "The new column will be filled with the specified default value, or NULL if none is specified."]. The "filled" verb suggests eager materialization. For a 100k-row overlay this is one full table scan per affected table (5 tables) — milliseconds in practice but worth the planner adding a metric/log line so we have evidence on real workspaces.

## Common Pitfalls

### Pitfall 1: fsnotify watcher death on atomic rename when watching the file directly

**What goes wrong:** Vim, JetBrains, and VS Code (when `files.atomicSave: true`) all rename the file during save. fsnotify's `Add(<filepath>)` watch is dropped immediately on the rename. Subsequent saves are silently missed.
**Why it happens:** inotify (Linux), FSEvents (macOS), and ReadDirectoryChangesW (Windows) all watch the inode/handle, not the path. After rename, the original handle/inode is gone.
**How to avoid:** Watch the **directory**, not the file. The directory's watch persists across atomic renames inside it. CONTEXT.md D-01 commits to directory-level. [VERIFIED upstream fsnotify Add() doc: "Watching individual files (rather than directories) is generally not recommended as many programs (especially editors) update files atomically: it will write to a temporary file which is then moved to destination, overwriting the original (or some variant thereof). The watcher on the original file is now lost, as that no longer exists. Watch the parent directory and use Event.Name to filter out files you're not interested in."]
**Warning signs:** A unit test that does `Add(tmpfile.Name())` rather than `Add(filepath.Dir(tmpfile.Name()))`.

### Pitfall 2: ENOSPC at `fsnotify.Add()` returning the bare errno from inotify_add_watch

**What goes wrong:** On Linux when `fs.inotify.max_user_watches` is exhausted (default 8192 across Ubuntu/Debian/Fedora; Linux 5.11+ kernels auto-tune up to 1048576 based on RAM but most distros still ship the older default), `fsnotify.Watcher.Add(dir)` returns the syscall error directly — no wrapping, no sentinel.
**Why it happens:** [VERIFIED via fsnotify v1.9.0 source `backend_inotify.go`: `wd, err := unix.InotifyAddWatch(w.fd, path, flags); if wd == -1 { return nil, err }`. The error from inotify_add_watch syscall (including ENOSPC = "no space left on device") is returned as-is without wrapping.] [CITED: man 2 inotify_add_watch: "ENOSPC — The user limit on the total number of inotify watches was reached or the kernel failed to allocate a needed resource."]
**How to avoid:** Detect with `errors.Is(err, syscall.ENOSPC)`. Emit one structured `slog.Warn` (use a `sync.Once` per workspace to dedupe — repeated `Add()` calls during recursive walk will all fail with the same ENOSPC). Set `WatcherStatus{Active:false, Reason:"inotify_enospc", RemediationHint:"echo fs.inotify.max_user_watches=524288 | sudo tee -a /etc/sysctl.conf"}`. Manifest scanner takes over correctness; SemanticIndexState stays `ready` (D-05).
**Warning signs:** A log spam under stress test → missing `sync.Once`.

### Pitfall 3: Empty coalesced flush opening a write tx and consuming an `overlay_epoch`

**What goes wrong:** `created+deleted` for the same path coalesces to a no-op (`MergeChange` returns `keep=false`); `pending` after coalesce is empty; if the dispatcher still calls `BeginOverlayTx` with no rows, `current_epoch` advances for no reason. Phase 63's `WHERE write_epoch <= captured_epoch` reads remain correct, but the epoch sequence becomes misleading for status reporting (`overlay_file_count` stays 0 but `current_epoch` keeps climbing).
**Why it happens:** The dispatcher doesn't check `len(coalesced) == 0` before opening tx.
**How to avoid:** Hard short-circuit in the dispatcher: `if len(coalesced) == 0 { return }`. Acceptance #8 explicitly tests this. The unit test injects `created+deleted`, asserts `meta.current_epoch` unchanged after flush.
**Warning signs:** A debug log "BeginOverlayTx: 0 rows committed" — that's the smoking gun.

### Pitfall 4: Manifest scanner double-walking — both watcher fires AND scanner sees the same file

**What goes wrong:** A user saves `auth.go`. Watcher fires `WorkspaceChangeSignal{Source:fsnotify, Paths:[auth.go]}`; 5s later the scanner also computes the new hash and emits `WorkspaceChangeSignal{Source:manifest_scan, Paths:[auth.go]}`. Both reach the classifier, which (correctly) sees the hash matches the freshly-stored one and returns `ChangeFileModified` for the second event too. The coalescer dedupes by path, so only one tx — but both events count for metric purposes, inflating the `helix_semantic_live_updates_total{kind=file_modified}` counter.
**Why it happens:** No source-aware deduplication. The classifier is correctly idempotent, but the metric counts inputs.
**How to avoid:** The metric increments on the dispatch path (after coalesce), not at signal-receive. Per the dispatcher rule "one event = one outcome label increment", duplicates collapse before metric emission. Acceptance #11's bounded-label test should also assert "100 watcher events + 100 manifest events for the same paths produce ≤ 100 metric increments" — that's the deduplication contract.
**Warning signs:** Sum of `helix_semantic_live_updates_total{outcome=applied}` exceeds the unique-file count over a window.

### Pitfall 5: JetBrains `___jb_old___` rename on Windows leaves stale watch state

**What goes wrong:** The fsnotify v1.9.0 doc explicitly notes: "A watch will be automatically removed if the watched path is deleted or renamed. **The exception is the Windows backend, which doesn't remove the watcher on renames.**" [VERIFIED upstream]. JetBrains' rename sequence on Windows can leave the watcher attached to a "ghost" path internally; this is the most platform-asymmetric behavior in this phase.
**Why it happens:** Windows ReadDirectoryChangesW retains the watch handle on the directory inode after rename within the directory.
**How to avoid:** Always re-add the parent directory after seeing a Rename event with the suffix `___jb_old___` or `___jb_tmp___`. The directory watch is stable; only file-level watches are at risk — and CONTEXT.md commits to directory-level only, so this is mostly defensive.
**Warning signs:** Editor-fixture test passes on Linux/macOS but the JetBrains scenario silently misses events on Windows.

### Pitfall 6: Cross-workspace event leak via a shared coalescer goroutine

**What goes wrong:** If the coalescer is implemented as a single global goroutine processing events from all workspaces (e.g., one channel keyed by repo_id), a slow handler for workspace A blocks workspace B.
**Why it happens:** "One goroutine per workspace" is in CONTEXT.md D-02 but is easy to miss when wiring.
**How to avoid:** `WatcherManager` holds `map[WorkspaceKey]*Coalescer`; each coalescer owns its own goroutine and channel. Cross-workspace `BeginOverlayTx` calls also do not serialize per CONTEXT D-04 ("per-workspace mutex; no global lock").
**Warning signs:** A stress test with 2 workspaces showing edit latency in workspace B correlated with edit storms in workspace A.

### Pitfall 7: DuckDB `ADD COLUMN ... NOT NULL DEFAULT 0` rejected at runtime

**What goes wrong:** CONTEXT.md D-04 SQL block uses `ALTER TABLE ... ADD COLUMN <name> UBIGINT NOT NULL DEFAULT 0`. Phase 59's `applyMigration002` already documented at line 380-396 that DuckDB rejects this exact form ("Parser Error: Adding columns with constraints not yet supported").
**Why it happens:** DuckDB's ALTER TABLE feature gap.
**How to avoid:** Drop `NOT NULL` from the migration SQL — `DEFAULT 0` alone backfills existing rows AND supplies the value for new INSERTs that omit the column. The application layer (Phase 60 overlay writer) explicitly sets `write_epoch` on every write, so the practical outcome is identical. [VERIFIED: `internal/semantic/store/migrations.go:380-396` and `:427-433`.]
**Warning signs:** Phase 60 P01 test fails on first run with "Adding columns with constraints not yet supported".

### Pitfall 8: `EditNotifier.OnEdit` blocking on extraction

**What goes wrong:** A naive impl does `OnEdit → classifier → re-extract synchronously`. The kernel edit tool now waits for tree-sitter to re-parse before returning. Acceptance #2 explicitly fails.
**Why it happens:** Forgetting that `OnEdit` is fire-and-forget; the channel send into the per-workspace coalescer is the entire impl body.
**How to avoid:** `OnEdit` is `liveService.signal(ctx, sig)`, which in turn is non-blocking enqueue (`select { case ch <- sig: default: drop+metric }`). Acceptance #2's integration test wires a slow extraction and asserts kernel returns within the foreground budget.
**Warning signs:** A blocking call chain visible in goroutine traces.

## Code Examples

### Example 1: fsnotify directory watch with atomic-rename re-attach (LIVE-02)

```go
// internal/semantic/live/watcher/watcher.go (sketch)
func (w *workspaceWatcher) loop(ctx context.Context) {
    var (
        timer   *time.Timer
        pending = make(map[string]struct{}) // path-set; D-01 paths-only
        mu      sync.Mutex
    )

    flush := func() {
        mu.Lock()
        if len(pending) == 0 { mu.Unlock(); return } // Pitfall 3 guard
        paths := make([]string, 0, len(pending))
        for p := range pending { paths = append(paths, p) }
        pending = make(map[string]struct{})
        mu.Unlock()
        w.dispatch(ctx, WorkspaceChangeSignal{
            WorkspaceID: w.ws,
            Paths:       paths,
            Source:      ChangeSourceFsnotify,
            ObservedAt:  time.Now(),
        })
    }

    for {
        select {
        case <-ctx.Done(): if timer != nil { timer.Stop() }; return
        case ev, ok := <-w.fw.Events:
            if !ok { return }
            // Skip JetBrains tempfile suffixes — the rename will fire
            // a Create on the destination path next.
            if strings.HasSuffix(ev.Name, "___jb_tmp___") ||
                strings.HasSuffix(ev.Name, "___jb_old___") {
                continue
            }
            mu.Lock(); pending[ev.Name] = struct{}{}; mu.Unlock()

            // Re-add directory if a Create on a subdir was observed
            // (matches internal/memory/watcher.go:93-98).
            if ev.Has(fsnotify.Create) {
                if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
                    _ = w.fw.Add(ev.Name)
                }
            }

            if timer != nil { timer.Stop() }
            timer = time.AfterFunc(w.cfg.DebounceMs, flush)

        case err, ok := <-w.fw.Errors:
            if !ok { return }
            if errors.Is(err, syscall.ENOSPC) {
                w.enospc.Do(func() {
                    w.logger.Warn("fsnotify ENOSPC; falling back to manifest scan",
                        "workspace", w.ws, "remediation",
                        "echo fs.inotify.max_user_watches=524288 | sudo tee -a /etc/sysctl.conf")
                    w.status.Set(WatcherStatus{Active:false, Reason:"inotify_enospc"})
                })
            } else {
                w.logger.Warn("fsnotify error", "error", err)
            }
        }
    }
}
```

[Source: structurally mirrors `internal/memory/watcher.go:60-118` with paths-only output per D-01, ENOSPC handling per D-05, and JetBrains tempfile filtering per LIVE-02.]

### Example 2: Per-workspace overlay tx with epoch allocation (D-04)

```go
// internal/semantic/store/overlay.go (FILL Phase 57 stub)
type OverlayWriter struct {
    db    *sql.DB
    locks sync.Map // repo_id → *sync.Mutex
}

func (w *OverlayWriter) BeginOverlayTx(ctx context.Context, repoID string) (*OverlayTx, error) {
    mu, _ := w.locks.LoadOrStore(repoID, &sync.Mutex{})
    mu.(*sync.Mutex).Lock()

    tx, err := w.db.BeginTx(ctx, nil)
    if err != nil { mu.(*sync.Mutex).Unlock(); return nil, err }

    var epoch uint64
    // Atomic increment + read-back.
    if _, err := tx.ExecContext(ctx,
        `UPDATE semantic_live_overlay_meta
         SET current_epoch = current_epoch + 1, updated_at = now()
         WHERE repo_id = ?`, repoID); err != nil {
        tx.Rollback(); mu.(*sync.Mutex).Unlock(); return nil, err
    }
    if err := tx.QueryRowContext(ctx,
        `SELECT current_epoch FROM semantic_live_overlay_meta WHERE repo_id = ?`,
        repoID).Scan(&epoch); err != nil {
        tx.Rollback(); mu.(*sync.Mutex).Unlock(); return nil, err
    }
    return &OverlayTx{tx: tx, repoID: repoID, epoch: epoch,
        unlock: func() { mu.(*sync.Mutex).Unlock() }}, nil
}

func (t *OverlayTx) UpsertOverlayFile(...) error {
    _, err := t.tx.ExecContext(ctx,
        `INSERT INTO semantic_live_overlay_files (..., write_epoch)
         VALUES (..., ?) ON CONFLICT (repo_id, path) DO UPDATE SET ...,
         write_epoch = excluded.write_epoch`, ..., t.epoch)
    return err
}
func (t *OverlayTx) Commit() error   { defer t.unlock(); return t.tx.Commit() }
func (t *OverlayTx) Rollback() error { defer t.unlock(); return t.tx.Rollback() }
```

[Source: structurally inherits from Phase 57 D-02 store conventions; `database/sql.Tx` semantics standard.]

### Example 3: Pure CoalesceEvents (D-02 + acceptance #4)

```go
// internal/semantic/live/coalescer/coalesce.go
// Pure function — unit-testable in isolation per D-02 invariant.
func CoalesceEvents(events []SourceChangeEvent, threshold int) []SourceChangeEvent {
    byPath := map[string]SourceChangeEvent{}
    for _, ev := range events {
        key := ev.Path
        if ev.Kind == ChangeFileRenamed { key = ev.OldPath + "→" + ev.Path }
        prev, ok := byPath[key]
        if !ok { byPath[key] = ev; continue }
        merged, keep := MergeChange(prev, ev)
        if !keep { delete(byPath, key); continue }
        byPath[key] = merged
    }
    result := make([]SourceChangeEvent, 0, len(byPath))
    for _, v := range byPath { result = append(result, v) }
    if len(result) > threshold {
        repo := ""
        if len(events) > 0 { repo = events[0].RepoID }
        return []SourceChangeEvent{{RepoID: repo, Kind: ChangeBulkUpdate, Source: "coalescer"}}
    }
    sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
    return result
}

func MergeChange(prev, cur SourceChangeEvent) (SourceChangeEvent, bool) {
    switch {
    case prev.Kind == ChangeFileModified && cur.Kind == ChangeFileModified:
        return cur, true
    case prev.Kind == ChangeFileCreated && cur.Kind == ChangeFileModified:
        return prev, true // created wins
    case prev.Kind == ChangeFileCreated && cur.Kind == ChangeFileDeleted:
        return SourceChangeEvent{}, false // no-op
    case prev.Kind == ChangeFileModified && cur.Kind == ChangeFileDeleted:
        return cur, true
    case prev.Kind == ChangeFileDeleted && cur.Kind == ChangeFileCreated:
        return SourceChangeEvent{Kind: ChangeFileModified, Path: cur.Path,
            RepoID: cur.RepoID, Source: cur.Source, ObservedAt: cur.ObservedAt}, true
    default:
        return cur, true // last-write-wins fallback
    }
}
```

[Source: SPEC §16.2 pseudocode at `SPEC-DRAFT.md:1462-1499`, ported verbatim to Go with named-rule explicit MergeChange function.]

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Polling-based file watch | Directory-level fsnotify with debounce | Phase 60 first | Sub-second freshness vs. multi-second; ENOSPC risk → manifest fallback. |
| Watching individual files | Directory watch + `Event.Name` filter | fsnotify upstream guidance (frozen since v1.0+) | Survives atomic-rename editors. |
| Phase 57 empty `overlay.go` stub | `BeginOverlayTx`-shaped writer with epoch stamp | Phase 60 fills it | Live overlay actually has a write path. |
| `ScheduleIncremental` sentinel JobID stub | Real body invoking the same extraction loop as initial walk | Phase 60 fills body (Phase 59 D-04 contract) | Dispatcher → scheduler in-process path works. |
| `helix_edit` event coupling kernel↔semantic at the import layer | `EditNotifier` interface in kernel + setter injection in daemon | Phase 60 D-03 | LIVE-07 acceptance: `internal/kernel/` does NOT import `internal/semantic/...`. |

**Deprecated/outdated:**
- The CONTEXT.md mention of `write_file` as a kernel tool name — not in the current registry; see Open Questions O-1 below.

## Project Constraints (from CLAUDE.md)

These project-wide directives must be honored:

- **Go-only, single binary; no Python, Docker, or runtime deps.** All Phase 60 code is Go.
- **`go test ./...` and `go vet ./...` must pass before completing any Go task.** Phase 60 tasks must include both gates.
- **`gofmt -w .` formatting.** Standard.
- **`internal/kernel/` MUST NOT import `internal/semantic/...`.** D-03 + LIVE-07 acceptance #1.
- **`duckdb-go` only allowed under `internal/semantic/store/...`.** Verified at `internal/lint/noduckdb/analyzer.go:16`. Phase 60 packages stay outside that prefix and call store APIs.
- **`internal/repomap` not imported by `internal/semantic/...`.** Phase 59 D-01 cascade.
- **Single `GrammarRegistry` injected from daemon bootstrap (BUG-04 / EXTRACT-05).** Re-extraction on file change reuses the daemon's singleton; do not create a new one.
- **Middleware install LIFO order MUST NOT change.** Phase 60 wires below the MCP layer; no middleware additions.
- **`CGO_ENABLED=1` single-mode (Phase 59.1 invariant).** No `_nocgo.go` stubs in Phase 60.
- **Bounded-label metrics with closed enums.** Phase 60's `helix_semantic_live_updates_total{kind, outcome}` follows the Phase 53 D-10 pattern verified in `internal/obs/metrics.go:339-351`.
- **GSD workflow enforcement.** Phase 60 work goes through `/gsd:execute-phase`; no direct edits.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| LIVE-01 | Directory-level fsnotify watcher, 250ms debounce, SPEC §16.2 coalesce, structured upserts/tombstones | Architecture Pattern 2 (debounced loop), Code Example 1 (atomic-rename re-attach), Code Example 3 (CoalesceEvents). Acceptance #4, #5. |
| LIVE-02 | Atomic-rename save patterns (Vim, JetBrains, VS Code) caught | Editor save-pattern dossier (below); Pitfall 1, 5; Validation Architecture #3. |
| LIVE-03 | Required (not best-effort) periodic content-hash scrub; status via `get_semantic_graph_status` | Architecture Pattern (manifest scanner), reuses `xxhash` and `semantic_files.content_hash`. Acceptance #10. Phase 64 wires the MCP wrapper; Phase 60 ships the data accessor. |
| LIVE-04 | Linux `inotify` ENOSPC fallback to manifest-poll with user-visible warning; semantic queries answer with stale-allowed reads | Pitfall 2 (verified ENOSPC propagation through fsnotify v1.9.0 source); Code Example 1 (sync.Once guard); Validation Architecture #9. |
| LIVE-05 | Bulk-change events > 200 collapse to single `bulk_update`; incremental rebuild rather than thrashing | Code Example 3 (threshold collapse in CoalesceEvents); Validation Architecture #5; the dispatcher invokes `HandleBulkUpdate` which calls `ScheduleIncremental` (Phase 59 D-04 contract). |
| LIVE-06 | Overlay write tx carries monotonic `overlay_epoch`; Phase 63 reads under CAS | Code Example 2 (BeginOverlayTx); Migration Pattern 3; Validation Architecture #6, #7, #8. |
| LIVE-07 | After every successful edit/fileops tool, `ChangeHelixEdit` event lands in the live queue via `postEditHook` (kernel does NOT import semantic) | Setter Pattern 1 (`EditNotifier`); Validation Architecture #1, #12. NB: 8th tool name discrepancy — see Open Questions O-1. |

## Editor Save-Pattern Dossier (LIVE-02 spine)

This is the load-bearing reference for the editor-fixture test suite.

### Vim — `backupcopy=auto` swap-rename

**Default behavior:** Vim's `backupcopy` option defaults to `auto`, which means: "When Vim sees that renaming the file is possible without side effects (the attributes can be passed on and the file is not a link) that is used. When problems are expected, a copy will be made." [CITED: vimhelp.org `:help 'backupcopy'`].

**Concrete file ops on save** (rename branch, common case on POSIX):
1. Vim writes the new content to `auth.go~` (or another temp file in the same directory).
2. Vim renames the original `auth.go` → backup (or removes it if no backup configured).
3. Vim renames the temp file → `auth.go`.

**fsnotify event sequence** (directory watch on `cwd/`):
```
RENAME    auth.go         (the original; old path → out of dir or to backup)
CREATE    auth.go~        (the temp file; if backup retained)
RENAME    auth.go~ → auth.go  (manifests as RENAME on auth.go~ + CREATE on auth.go)
WRITE     auth.go         (zero-or-more, depending on whether Vim flushes through fsync)
```

The watcher MUST NOT depend on a single Op flag — Vim emits a Rename followed by a Create, both on the directory. With directory-level watch and path-only signal (D-01), this normalizes correctly.

**Test fixture:** A small bash script that uses `vim -e -c 'set noswapfile' -c 'normal Goappended\n' -c ':wq' auth.go` to deterministically save, then assert overlay row exists for `auth.go` with the new content_hash.

### JetBrains IntelliJ — `___jb_tmp___` + `___jb_old___` safe write

**Default behavior:** "Safe write" is enabled by default. The save sequence is documented by JetBrains support:

1. Write new content to `auth.go___jb_tmp___` (in same directory).
2. Rename original `auth.go` → `auth.go___jb_old___` (backup).
3. Rename `auth.go___jb_tmp___` → `auth.go`.
4. Delete `auth.go___jb_old___`.

[CITED: JetBrains support article — "the file being saved is replaced with the saved file (technically, the original file is deleted and the temporary file is renamed)"; suffix etymology: "JetBrains temp" / "JetBrains old".]

**fsnotify event sequence:**
```
CREATE    auth.go___jb_tmp___
WRITE     auth.go___jb_tmp___
RENAME    auth.go              (→ auth.go___jb_old___)
CREATE    auth.go___jb_old___
RENAME    auth.go___jb_tmp___  (→ auth.go)
CREATE    auth.go              (post-rename)
REMOVE    auth.go___jb_old___
```

The watcher MUST filter out the `___jb_tmp___` and `___jb_old___` suffixes (they're not real source files); the path-set after dedup ends up `{auth.go}`. Code Example 1 above already does this.

**Test fixture:** A small Go program in `testdata/editors/jetbrains/save.go` that programmatically performs the four-step sequence using `os.WriteFile` + `os.Rename` + `os.Remove`. Run via `go run` from inside the test.

### VS Code — atomic save (when `files.atomicSave: true`) or default truncate-and-write

**Default behavior:** As of recent VS Code versions (covered in microsoft/vscode#98063), atomic save is **opt-in** via the `files.atomicSave` setting. When enabled, VS Code writes to a temp file in the same directory and uses `fs.rename` (single atomic op on POSIX) to replace.

When `files.atomicSave: false` (default for most users), VS Code uses the truncate-then-write pattern: open + truncate + write. The fsnotify event is a single WRITE on `auth.go`.

[CITED: github.com/microsoft/vscode/issues/98063 — "Add an option to save files atomically"; VS Code source `src/vs/platform/files/...` — the implementation detail wasn't fully extracted by WebFetch, but the issue confirms atomic save is the alternative path, not the default.]

**fsnotify event sequence (atomic save mode):**
```
CREATE    .vsctmp~auth.go-XXXX  (or similar; VS Code uses a randomized prefix)
WRITE     .vsctmp~auth.go-XXXX
RENAME    .vsctmp~auth.go-XXXX  (→ auth.go)
CREATE    auth.go               (post-rename)
```

**fsnotify event sequence (default truncate mode):**
```
WRITE     auth.go
```

**Test fixture:** A Go program in `testdata/editors/vscode/save.go` that performs the temp-file + `os.Rename` sequence. The watcher must catch BOTH the atomic-save and truncate-save patterns; the truncate-save case is the easy one.

[ASSUMED] The exact VS Code temp-file prefix may have changed across releases; the test fixture should not rely on the specific prefix string but rather on the rename-into-tracked-path observable.

## Validation Architecture

> Required: `nyquist_validation: true` in `.planning/config.json`.

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (+ `go test -race` for concurrency tests) |
| Config file | None — Go convention |
| Quick run command | `go test ./internal/semantic/live/... ./internal/semantic/store/... ./internal/kernel/edit/... ./internal/kernel/fileops/... -count=1` |
| Full suite command | `go test ./... -race -count=1` |

### Phase Requirements → Test Map (per CONTEXT.md Acceptance Criteria #1-#13)

| Crit | Behavior | Test Type | Automated Command | File Exists? |
|------|----------|-----------|-------------------|-------------|
| #1 | `internal/kernel/` does not import `internal/semantic/...` | unit (build-time greppable / vet analyzer) | `go test ./internal/lint/nokernel2semantic/... -run TestKernelDoesNotImportSemantic` (new analyzer modeled on `internal/lint/noduckdb/`) | ❌ Wave 0 — new `internal/lint/nokernel2semantic/` analyzer + `cmd/vet-nokernel2semantic/` |
| #2 | `EditNotifier.OnEdit` returns within O(microseconds) | integration | `go test ./internal/kernel/edit/... -run TestOnEditNonBlocking` | ❌ Wave 0 — `internal/kernel/edit/notifier_integration_test.go` |
| #3 | Editor-fixture suite (Vim, JetBrains, VS Code) | integration (editor fixtures) | `go test ./internal/semantic/live/watcher/... -run TestEditorFixtures -tags editor` | ❌ Wave 0 — `internal/semantic/live/testdata/editors/{vim,jetbrains,vscode}/` + `editor_fixtures_test.go` |
| #4 | `CoalesceEvents` 6 merge rules + bulk_update threshold | unit (table-driven) | `go test ./internal/semantic/live/coalescer/... -run TestCoalesceEvents` | ❌ Wave 0 — `internal/semantic/live/coalescer/coalesce_test.go` |
| #5 | bulk_update collapse fires when > 200 events at flush time | integration | `go test ./internal/semantic/live/coalescer/... -run TestBulkUpdateCollapse` | ❌ Wave 0 — `internal/semantic/live/coalescer/bulk_test.go` |
| #6 | Schema migration v2→v3 succeeds on clean / Phase-57 / Phase-59 stores | unit (migration round-trip) | `go test ./internal/semantic/store/... -run TestMigration003` | ❌ Wave 0 — extends `internal/semantic/store/migrations_test.go` |
| #7 | Per-tx `overlay_epoch` advancement under N concurrent BeginOverlayTx | stress / property | `go test ./internal/semantic/store/... -run TestOverlayEpochConcurrent -race` | ❌ Wave 0 — `internal/semantic/store/overlay_concurrent_test.go` |
| #8 | No-op coalesced batches do not advance epoch | unit | `go test ./internal/semantic/live/... -run TestNoOpDoesNotAdvanceEpoch` | ❌ Wave 0 — `internal/semantic/live/handler/noop_test.go` |
| #9 | ENOSPC simulation: one slog.Warn, Status() returns inotify_enospc | unit (with mock fsnotify) | `go test ./internal/semantic/live/watcher/... -run TestENOSPCFallback` | ❌ Wave 0 — `internal/semantic/live/watcher/enospc_test.go` |
| #10 | Manifest scan detects watcher misses within 2× interval | integration | `go test ./internal/semantic/live/scanner/... -run TestScannerCatchesWatcherMisses -timeout 30s` | ❌ Wave 0 — `internal/semantic/live/scanner/scanner_integration_test.go` |
| #11 | `helix_semantic_live_updates_total{kind, outcome}` bounded labels | unit (label allowlist) | `go test ./internal/obs/... -run TestMetricsLabelsAllowlist_LiveUpdates` | ❌ Wave 0 — extends `internal/obs/metrics_labels_test.go` + new helper test |
| #12 | 8 kernel edit/fileops tools each emit OnEdit on success | integration (mock notifier) | `go test ./internal/kernel/edit/... ./internal/kernel/fileops/... -run TestOnEditCalledOnSuccess` | ❌ Wave 0 — per-tool integration test |
| #13 | LIVE-01..LIVE-07 marked Done in REQUIREMENTS.md | manual (REQUIREMENTS.md edit + verifier check) | `grep -E '^- \[x\] \*\*LIVE-0[1-7]\*\*' .planning/REQUIREMENTS.md \| wc -l` | manual-only (REQUIREMENTS.md is human-edited at phase close) |

### Editor-Fixture Specifics (Criterion #3)

```
internal/semantic/live/testdata/editors/
├── vim/
│   ├── README.md           # Documents the swap-rename sequence
│   ├── save.sh             # Bash script invoking `vim -e -c '...'`
│   └── expected_events.txt # Literal fsnotify event sequence (golden)
├── jetbrains/
│   ├── README.md           # Documents the ___jb_tmp___ / ___jb_old___ sequence
│   ├── save.go             # Go program performing 4-step rename sequence
│   └── expected_events.txt # Literal fsnotify event sequence (golden)
└── vscode/
    ├── README.md           # Documents both atomic-save and truncate-save modes
    ├── save_atomic.go      # Go program: tempfile + os.Rename
    ├── save_truncate.go    # Go program: O_TRUNC + WriteFile
    └── expected_events.txt # Literal fsnotify event sequence (golden)
```

The harness in `editor_fixtures_test.go`:

1. Creates a temp workspace dir with a single `.go` source file.
2. Starts the Phase 60 watcher manager on that workspace.
3. Invokes the editor's save program via `os/exec` (with the temp dir's source file as the target).
4. Waits up to 2 seconds for the dispatcher's `UpdateChangedFile` to commit an overlay row.
5. Asserts: `semantic_live_overlay_files` row exists with `path=<source>` and `content_hash` matching the new content.

Skip predicates per platform: Vim test skips on Windows (no Vim binary in CI); JetBrains test runs on all platforms (it's a Go program reproducing the rename sequence). VS Code test runs on all platforms (Go program). The contract is the rename SEQUENCE, not the editor binary itself.

### Sampling Rate

- **Per task commit:** `go test ./internal/semantic/live/... ./internal/semantic/store/... -count=1` (subset; ~5s)
- **Per wave merge:** `go test ./... -race -count=1` (full suite; 30-90s including editor fixtures)
- **Phase gate:** Full suite green + 13 acceptance criteria mapped to passing tests + `go vet ./...` + `cmd/vet-noduckdb` clean + new `cmd/vet-nokernel2semantic` clean

### Wave 0 Gaps

- [ ] `internal/lint/nokernel2semantic/analyzer.go` + `cmd/vet-nokernel2semantic/main.go` — modeled on `internal/lint/noduckdb/` (acceptance #1)
- [ ] `internal/semantic/live/coalescer/coalesce_test.go` — table-driven (acceptance #4, #5)
- [ ] `internal/semantic/store/overlay_test.go` — concurrent BeginOverlayTx (acceptance #7)
- [ ] `internal/semantic/live/watcher/enospc_test.go` — uses a fake fsnotify backend (acceptance #9)
- [ ] `internal/semantic/live/scanner/scanner_integration_test.go` — uses real tmpfs (acceptance #10)
- [ ] `internal/semantic/live/testdata/editors/{vim,jetbrains,vscode}/` — fixture dirs (acceptance #3)
- [ ] Per-tool integration tests for the 8 hook entry points (acceptance #12) — extends existing `internal/kernel/edit/outcome_emission_test.go` and `internal/kernel/fileops/outcome_emission_test.go`

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All Phase 60 code | ✓ (assumed) | 1.21+ | — |
| fsnotify v1.9.0 | watcher | ✓ | v1.9.0 | — |
| xxhash/v2 | manifest scanner | ✓ | v2.3.0 | — |
| DuckDB driver | overlay store | ✓ | duckdb-go/v2 | — |
| `vim` binary | editor-fixture #3 (Vim only) | ✗ on most CI | — | Skip Vim fixture if `vim` not on PATH; JetBrains and VS Code fixtures cover atomic-rename otherwise. |
| Linux `inotify` | watcher (Linux only) | ✓ on Linux CI | kernel-provided | — |
| macOS FSEvents | watcher (macOS only) | ✓ on macOS CI | kernel-provided | — |
| Windows ReadDirectoryChangesW | watcher (Windows only) | ✓ on Windows CI | kernel-provided | — |

**Missing dependencies with no fallback:** None.
**Missing dependencies with fallback:** `vim` binary — fixture skip with a clear log message.

## Analog Pattern Locator

Per the research focus area C, here are the closest in-tree analogs for each new component:

| New package | Closest analog | File:line | What's reused | What MUST differ |
|-------------|----------------|-----------|---------------|------------------|
| `internal/semantic/live/watcher/` | `internal/memory/watcher.go` | `:60-118` | Debounce timer + recursive Add + per-event dispatch loop shape | (a) per-workspace, not per-process; (b) outputs `WorkspaceChangeSignal` (paths only), not `Upsert`/`Remove` calls; (c) ENOSPC handling + Status() accessor; (d) JetBrains tempfile-suffix filter |
| `internal/semantic/live/coalescer/` | `internal/memory/watcher.go` (timer flush pattern) | `:62-77, :109` | `time.AfterFunc` + `pending` map snapshot pattern | Per-workspace single goroutine; pure `CoalesceEvents` separate from impure dispatch loop; sequential dispatch with explicit kind switch |
| `internal/semantic/live/scanner/` | `internal/repomap/extractor.go` `walkAndExtract` | (file walker shape only) | filepath.Walk + ignore-rule filter | NEW CODE — must NOT import `internal/repomap`; reimplement ignore rules from SPEC §13.2 |
| `internal/semantic/live/classifier/` | None — net-new | — | — | First-class kind-decision owner; reads `semantic_files.content_hash` via store API |
| `internal/semantic/store/overlay.go` (FILL) | `internal/semantic/store/snapshot.go` (sibling) + `database/sql.Tx` patterns | `internal/semantic/store/duckdb.go:72-147` (Open shape) | `*sql.DB` + transaction lifecycle; per-workspace logger + metrics | Per-workspace mutex on `repo_id`; epoch increment + read-back inside tx; tombstone semantics (`MarkFileDeleted` etc.) |
| `internal/kernel/notifier.go` | `internal/kernel/.../skill_adapter.go` setter pattern | `internal/daemon/daemon.go:376` (`SetEnrichFn`) | Setter idiom; `atomic.Value` or single-writer guard | Interface lives in kernel pkg (not skill); fire-and-forget contract |
| `internal/phasegraph/pipelines/live.go` (FILL) | Same file (`pipelines/live.go:29-39`) Phase 57 stubs | direct extension | Typed phase IDs + Requires/Provides edges | Replace `noopRun` placeholders with bodies that wire watcher.Start, scanner.Start, dispatcher.Run |
| `internal/semantic/scheduler/scheduler.go` (FILL) | Same file `:81-85` (current stub) | direct extension | Existing `Scheduler` struct + lock + state machine | Real body translating `[]FileChange` → per-path dispatch via UpdateChangedFile/HandleFileDeleted/HandleFileRenamed |
| `helix_semantic_live_updates_total` metric | `EditOutcomeInc` helper | `internal/obs/metrics.go:339-351` | `switch`-drop closed-enum discipline; allowlist test | New label values; `kind` is a carve-out (like `strategy`) |

## Sources

### Primary (HIGH confidence)

- `internal/memory/watcher.go` — fsnotify watcher + debounce reference
- `internal/semantic/store/migrations.go:380-396, :421-445` — DuckDB `ALTER TABLE ADD COLUMN` constraint documented
- `internal/semantic/store/migrations_registry.go:27-30` — Migration registry append point
- `internal/semantic/store/migrations_types.go:16` — `CurrentSchemaVersion` bump
- `internal/semantic/scheduler/scheduler.go:25-33, :81-85` — `ExtractionScheduler` interface + stub body to fill
- `internal/semantic/scheduler/state.go:66-72` — `FileChange` struct already shipped
- `internal/phasegraph/pipelines/live.go:29-39` — Phase 57 D-04 typed-ID stubs to fill
- `internal/daemon/daemon.go:376, :463` — `SetEnrichFn` / `SetActivateCallback` setter pattern
- `internal/kernel/edit/replace.go:`(success path), `insert.go`, `rename.go:91-113`, `delete.go:32-78` — kernel edit success-path locations for the OnEdit hook
- `internal/kernel/fileops/replace.go`, `fuzzy_edit.go`, `write.go:38-72` — fileops success-path locations
- `internal/lint/noduckdb/analyzer.go:16-17` — duckdb-go allowlist; analyzer pattern for the new `nokernel2semantic` analyzer
- `internal/obs/metrics.go:333-351` — `EditOutcomeInc` closed-enum drop pattern
- `SPEC-DRAFT.md:1419-1622` — §16.1-§16.5 Live Update Pipeline canonical pseudocode
- `.planning/REQUIREMENTS.md` LIVE-01..LIVE-07 — phrasing contract
- `.planning/phases/60-live-update-pipeline/60-CONTEXT.md` — full decision/discretion/deferred context
- pkg.go.dev `time.Timer.Reset` / `time.AfterFunc` — official Go stdlib semantics
- fsnotify v1.9.0 GitHub source `backend_inotify.go:285-300, :385` — ENOSPC propagation verified unwrapped from `inotify_add_watch`
- pkg.go.dev `github.com/fsnotify/fsnotify` — Add/Watcher/Op/Event docs (current upstream)
- man 2 inotify_add_watch (kernel.org / man7.org) — ENOSPC vs ENOMEM distinction

### Secondary (MEDIUM confidence)

- vimhelp.org `:help 'backupcopy'` — `auto` default semantics ([CITED: vimhelp.org])
- JetBrains support article on `___jb_tmp___` / `___jb_old___` save sequence ([CITED: intellij-support.jetbrains.com])
- microsoft/vscode#98063 — VS Code atomic save is opt-in ([CITED: github.com/microsoft/vscode/issues/98063])
- Watchexec inotify-limits.html / multiple distro docs — `fs.inotify.max_user_watches` default 8192 across major distros, with kernel 5.11+ auto-tune; `524288` is the de facto remediation value

### Tertiary (LOW confidence)

- DuckDB ADD COLUMN materialization timing — docs say "filled" but don't specify lazy vs. eager. [ASSUMED] eager based on the verb; flagged as the only LOW-confidence claim. Validated by Phase 60 P01's migration test on a populated overlay (test fixture should populate ~1k overlay rows before running migration to bound the cost).
- Specific VS Code temp-file prefix format — varies across releases. The fixture tests rename-into-target-path observable, not the prefix.

## Assumptions Log

> Claims tagged `[ASSUMED]` requiring user/planner confirmation before execution.

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | DuckDB `ALTER TABLE ADD COLUMN UBIGINT DEFAULT 0` materializes the default eagerly (one full table scan per affected table) | Pitfall 7, Runtime State Inventory | If lazy: zero startup cost; if eager and overlay is huge: brief startup pause. Either way correctness holds; this is a perf observation only. Test on populated overlay in P01. |
| A2 | VS Code temp-file prefix is randomized but always includes a `.vsctmp` substring or similar | Editor save-pattern dossier | If prefix differs across releases: fixture should test the observable (rename-into-target), not the prefix string. The fixture's Go program controls its own prefix. |
| A3 | Single Helix daemon talks to a single workspace at a time in 99% of cases — multi-workspace coalescer concurrency is exercised by tests but rare in production | Pitfall 6 | If multi-ws concurrency is common: the per-workspace mutex design is still correct; just heavier-tested. |

**Empty otherwise:** all primary library/OS claims are tagged [VERIFIED] from upstream source or [CITED] from official docs.

## Out-of-Scope Confirmation (Cross-Phase Boundary Check)

Cross-checked CONTEXT.md `Out of scope (deferred)` against Phase 61, 62, 63, 64, 65 in `.planning/milestones/v1.10-ROADMAP.md:111-165`:

| Deferred item | Lands in | Phase 60 ships | No overlap? |
|---------------|----------|----------------|-------------|
| LSP enrichment worker (`LSPQueue.Enqueue` consumer) | Phase 61 | Typed buffered queue + producer | ✓ Phase 61 deps Phase 60 explicitly. |
| Compaction with `overlay_epoch` CAS | Phase 63 | Epoch contract + CAS read points | ✓ Phase 63 deps Phase 60, Phase 62. |
| Graph cache repair / score invalidation | Phase 62 | Typed no-op stubs | ✓ Phase 62 deps Phase 60, Phase 61. |
| `get_semantic_graph_status` / `refresh_semantic_graph` MCP tool wrapper | Phase 64 | `Status()` data accessor on watcher manager + live service | ✓ Phase 64 deps Phase 62, Phase 63. |
| `get_health` watcher integration | Phase 65 | `WatcherStatus` data accessor | ✓ Phase 65 deps Phase 64. |
| MCP push notifications | Phase 64+ | None — not started | ✓ No consumer in Phase 60. |

**Result:** No double-implementation risks. Phase 60's "data accessor / typed stub / typed queue" pattern is the cleanest possible deferral surface — every downstream phase has exactly one wiring point.

## Open Questions (RESOLVED)

1. **The `write_file` vs `create_file` discrepancy in CONTEXT.md D-03.** [O-1]
   - What we know: CONTEXT.md D-03 enumerates 8 hook entry points: 5 in `internal/kernel/edit/` + `replace_in_file` + `fuzzy_edit` + **`write_file`**. The kernel registry, verified at `internal/kernel/fileops/skill.go:32` and `tools.go:233`, ships `create_file`, NOT `write_file`. Acceptance #12 also says "8 kernel edit/fileops tools each emit `EditNotifier.OnEdit`".
   - What's unclear: Is "write_file" a typo for "create_file", or did CONTEXT.md anticipate a future tool that doesn't exist yet?
   - Recommendation: The planner should clarify with the user — either (a) treat the 8th hook as `create_file` (the closest existing tool that writes a brand-new file), or (b) add the OnEdit call inside the shared `OverwriteFile` helper at `internal/kernel/fileops/write.go:38-72` so every tool writing a file (create_file, replace_in_file, fuzzy_edit) gets the hook for free — that's actually 7 tools instead of 8 because `OverwriteFile` already covers replace_in_file and fuzzy_edit. Option (b) is simpler and structurally cleaner; option (a) preserves the literal CONTEXT count.
   - **RESOLVED:** Adopt per-tool wiring (option a-style); 8 hook insertion points = 5 in `internal/kernel/edit/` + `replace_in_file` + `fuzzy_edit` + `create_file`. `OverwriteFile` is a free function with no `*kernel.Kernel` handle, so the hook lives in each tool's registered handler closure. (See 60-03-PLAN.md `<objective>` and Tasks 2–3.)

2. **Configuration of editor-fixture skip on absent binaries.** [O-2]
   - What we know: Vim binary is not always available in CI.
   - What's unclear: Should the test skip with `t.Skip()` (silent) or fail with a clear "INSTALL VIM" message?
   - Recommendation: `t.Skip("vim not on PATH; the JetBrains and VS Code fixtures cover atomic-rename")` — preserves CI green while still surfacing in test output. The JetBrains and VS Code fixtures are pure Go programs, so they have no environment dependency.
   - **RESOLVED:** Vim test skips on Windows runners (and any host without `bash` on PATH) via `t.Skip` predicate; JetBrains and VS Code fixtures are Go programs that run on all platforms. (See 60-05-PLAN.md Task 2.)

3. **`live_updates.bulk_change_threshold = 200` empirical validation.** [O-3]
   - What we know: SPEC §25 declares the default 200; CONTEXT.md acceptance #5 tests `> 200 → bulk_update`.
   - What's unclear: Is 200 the right threshold? On a small repo a single `go fmt` could trigger 200+ files; on a large repo 200 is barely noticeable.
   - Recommendation: Phase 60 ships 200 as the default per SPEC. If real-world telemetry shows it's wrong, Phase 61+ adjusts via config — the threshold is bounded-config, not bounded-code.
   - **RESOLVED:** `bulk_change_threshold` default 200 ships unchanged; benchmarks deferred to Phase 61+ if dispatcher becomes the bottleneck. (See 60-04-PLAN.md `CoalesceEvents` test plan and CONTEXT.md "Deferred Ideas → Per-file fan-out".)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries already in `go.mod`; no new deps.
- Architecture: HIGH — every pattern has an in-tree analog with file:line evidence.
- fsnotify behavior: HIGH — verified directly against fsnotify v1.9.0 GitHub source for ENOSPC, official docs for atomic-rename guidance.
- Editor save patterns: MEDIUM-HIGH — Vim and JetBrains have official documentation; VS Code is partially documented (atomic save is opt-in per microsoft/vscode#98063, but the exact temp-file format is implementation detail).
- DuckDB ALTER TABLE materialization timing: LOW — docs say "filled" without specifying lazy vs. eager; flagged as A1.
- Pitfalls: HIGH — 8 pitfalls each with a verified root cause and an explicit guard.
- Validation Architecture: HIGH — every one of 13 acceptance criteria maps to a concrete `go test` invocation.

**Research date:** 2026-05-05
**Valid until:** 2026-06-04 (30 days; fsnotify, DuckDB, Go stdlib are stable; editor save patterns are stable across years)
