# Phase 60: Live Update Pipeline - Pattern Map

**Mapped:** 2026-05-05
**Files analyzed:** 22 (11 NEW + 11 MODIFIED)
**Analogs found:** 22 / 22

This map enumerates every file Phase 60 creates or modifies, classifies it by
role + data flow, and pairs it with a concrete in-tree analog (file:line +
excerpt). Pattern assignments are concrete enough that the planner can copy
the analog's shape directly into each plan's action section.

## File Classification

### NEW packages

| New File | Role | Data Flow | Closest Analog | Match Quality |
|----------|------|-----------|----------------|---------------|
| `internal/semantic/live/signal.go` | model (struct + enum) | request-response | `internal/semantic/scheduler/state.go` `FileChange` (lines 66-72) | exact |
| `internal/semantic/live/classifier.go` | service (single-purpose decision fn) | transform | net-new — closest is `internal/semantic/store/effective.go` (read-then-decide) | partial |
| `internal/semantic/live/coalescer/coalescer.go` | service (event aggregator) | event-driven | `internal/memory/watcher.go` Run loop (lines 60-118) | exact |
| `internal/semantic/live/coalescer/coalesce.go` | utility (pure fn) | transform | net-new (SPEC §16.2 pseudocode) | n/a (spec drives shape) |
| `internal/semantic/live/watcher/watcher.go` | service (fsnotify watcher) | event-driven | `internal/memory/watcher.go` (lines 17-118) | exact |
| `internal/semantic/live/watcher/enospc.go` | utility (error classifier + Status) | request-response | `internal/semantic/store/duckdb.go` `classifyExisting` (lines 153-159) | role-match |
| `internal/semantic/live/scanner/scanner.go` | service (periodic file walker) | batch | `internal/skill/repomap/skill.go` `walkAndExtract` (lines 386-411) — STRUCTURALLY (no import) | role-match |
| `internal/semantic/live/handler/handler.go` | service (per-event handlers) | CRUD | `internal/semantic/store/effective.go` (read-then-write API shape) | partial |
| `internal/semantic/live/service.go` | service (glue: OnEdit impl + dispatcher + scheduler integration) | event-driven | `internal/skill/repomap/skill.go` (post-init wiring + setters, lines 100-145) | role-match |
| `internal/kernel/notifier.go` | model (interface) | request-response | `internal/skill/repomap/skill.go` `SetEnrichFn` (lines 113-119) | role-match |
| `internal/lint/nokernel2semantic/analyzer.go` | utility (go/analysis Analyzer) | transform | `internal/lint/noduckdb/analyzer.go` (lines 1-40) | exact |
| `cmd/vet-nokernel2semantic/main.go` | config (singlechecker entry) | config | `cmd/vet-noduckdb/main.go` (lines 1-12) | exact |

### MODIFIED files

| Modified File | Role | Data Flow | Action | Closest Analog Within File |
|---------------|------|-----------|--------|----------------------------|
| `internal/semantic/store/overlay.go` | service (transactional writer) | CRUD | FILL stub | sibling `internal/semantic/store/snapshot.go` shape doc + `database/sql.Tx` patterns |
| `internal/semantic/store/migrations.go` | migration | batch | EXTEND | `applyMigration002` (lines 404-445) |
| `internal/semantic/store/migrations_registry.go` | config (registry slice) | config | EXTEND | line 27-30 (slice literal) |
| `internal/semantic/store/migrations_types.go` | config (constant) | config | EXTEND | line 16 (`CurrentSchemaVersion`) |
| `internal/phasegraph/pipelines/live.go` | controller (DAG bodies) | event-driven | FILL Run/Validate | `pipelines/live.go` itself (PhaseSpec slice, lines 29-39) |
| `internal/daemon/daemon.go` | controller (bootstrap) | request-response | EXTEND post-init | `SetEnrichFn` + `SetActivateCallback` (lines 376, 463-496) |
| `internal/kernel/edit/replace.go`, `insert.go`, `rename.go`, `delete.go` | controller (tool handler) | request-response | INSERT OnEdit call | success path; analog = `mcp.RecordEditOutcome` defer (tools.go:309, 391, 455, 521, 578) |
| `internal/kernel/fileops/replace.go`, `fuzzy_edit.go`, `write.go` | controller (tool handler) | request-response | INSERT OnEdit call | tools.go:370, 452 (`RecordEditOutcome` defer) |
| `internal/kernel/edit/tools.go` | controller (registration) | config | EXTEND signature | `RegisterTools` (line 131) |
| `internal/kernel/fileops/tools.go` | controller (registration) | config | EXTEND signature | `RegisterTools` (line 169) |
| `internal/config/defaults.go` | config (koanf defaults) | config | INSERT 3 keys | `live_updates.*` block (lines 78-87) |
| `internal/semantic/scheduler/scheduler.go` | service (extraction scheduler) | event-driven | FILL `ScheduleIncremental` body | `ScheduleInitialExtraction` (lines 67-79) |

---

## Pattern Assignments

### `internal/semantic/live/signal.go` (model, struct + enum)

**Why this analog:** `FileChange` is the closest existing typed-event struct
that crosses the same boundary (extract scheduler ↔ caller). Same shape:
exported struct, string-typed kind enum, no methods, used as a value type
across goroutines.

**Analog:** `internal/semantic/scheduler/state.go:66-72`

```go
// FileChange describes a single workspace file mutation passed to
// ScheduleIncremental. Phase 60 fills the body; Phase 59 ships the type so
// consumers can compile against the interface today.
type FileChange struct {
    Path string
    Kind string // "modified" | "created" | "deleted"
}
```

**What's reused:** Exported struct value type, string-typed `Kind` field.

**What MUST differ:**
- `WorkspaceChangeSignal` is paths-only (no Kind — classifier owns Kind decision per D-01).
- `ChangeSource` is a typed string enum (not free-form), with const declarations:
  `ChangeSourceHelixEdit | ChangeSourceFsnotify | ChangeSourceManifestScan`.
- Carries `WorkspaceID workspace.WorkspaceKey` and `ObservedAt time.Time` —
  signals are observed, classified later.
- Sibling `SourceChangeEvent` (post-classify) has the `Kind ChangeKind` field;
  the kind-decision boundary between the two types is the spine of D-01.

---

### `internal/semantic/live/coalescer/coalescer.go` (service, event-driven)

**Why this analog:** `internal/memory/watcher.go` is the canonical in-tree
debounce timer pattern, listed by both CONTEXT.md `<code_context>` and
RESEARCH.md §Analog Pattern Locator. Same shape: `time.AfterFunc` debounce,
`pending` map snapshot at flush, single goroutine drains channel.

**Analog:** `internal/memory/watcher.go:60-118`

**Run-loop / debounce / flush pattern** (lines 60-118):
```go
func (w *Watcher) Run(ctx context.Context) error {
    var (
        timer   *time.Timer
        pending = make(map[string]fsnotify.Op)
        mu      sync.Mutex
    )

    flush := func() {
        mu.Lock()
        batch := pending
        pending = make(map[string]fsnotify.Op)
        mu.Unlock()

        for path, op := range batch {
            w.processEvent(path, op)
        }
    }

    for {
        select {
        case <-ctx.Done():
            if timer != nil {
                timer.Stop()
            }
            return ctx.Err()

        case event, ok := <-w.watcher.Events:
            if !ok { return nil }
            // ... filter ...
            mu.Lock()
            pending[event.Name] = event.Op
            mu.Unlock()

            if timer != nil { timer.Stop() }
            timer = time.AfterFunc(w.debounce, flush)

        case err, ok := <-w.watcher.Errors:
            if !ok { return nil }
            w.logger.Warn("watcher error", "error", err)
        }
    }
}
```

**What's reused:** Snapshot-then-reset map pattern; `time.AfterFunc(debounce)`
reset on each event; `select` on ctx + events + errors; mutex around
`pending` (because timer goroutine reads it).

**What MUST differ:**
- ONE coalescer goroutine PER WORKSPACE (key on `workspace.WorkspaceKey`),
  not one per process. Lifecycle bound to workspace activation/deactivation.
- Input channel carries `SourceChangeEvent` (post-classifier), not
  `fsnotify.Event` (the watcher's classifier turns those into signals first).
- Flush invokes pure `CoalesceEvents([]SourceChangeEvent) []SourceChangeEvent`
  in a separate file (`coalesce.go`), then sequentially dispatches each
  result event to handler.UpdateChangedFile / HandleFileDeleted / HandleFileRenamed
  / HandleBulkUpdate based on Kind.
- Add a `max_batch_delay_ms` (1500ms default) hard ceiling: separate
  `time.AfterFunc` armed on the FIRST pending event, never reset, fires the
  flush even if events keep coming. Memory watcher has no such ceiling.
- Bulk-update collapse decision lives inside `CoalesceEvents` itself —
  if `len(events) > cfg.BulkChangeThreshold`, return a single
  `SourceChangeEvent{Kind: ChangeBulkUpdate}` slice.
- Per-event errors logged via structured `slog`; a single failed dispatch
  MUST NOT abort the batch — continue to the next event (D-02 invariant).

---

### `internal/semantic/live/coalescer/coalesce.go` (utility, pure fn)

**Why this analog:** No closely-shaped in-tree analog exists; SPEC §16.2
prescribes the pseudocode verbatim. Use the migration-statements helper
shape (deterministic ordering, pure function, no side effects, no logger
parameter) as the structural template.

**Analog:** `internal/semantic/store/migrations.go:421-445` (`schema2Statements`)

```go
// schema2Statements returns the v1→v2 DDL in deterministic order: 6 ALTER
// TABLE statements on semantic_files, 2 each on semantic_symbols and
// semantic_references, then one INSERT into semantic_schema_version.
func schema2Statements() []string {
    return []string{
        `ALTER TABLE semantic_files ADD COLUMN extraction_status TEXT DEFAULT ''`,
        // ... etc ...
    }
}
```

**What's reused:** Pure function, deterministic output, no logger / no I/O,
unit-testable in isolation.

**What MUST differ:**
- Signature: `CoalesceEvents(events []SourceChangeEvent) []SourceChangeEvent`.
- Implements SPEC §16.2 merge rules:
  `modified+modified → modified`,
  `created+deleted → no-op (drop)`,
  `created+modified → created`,
  `modified+deleted → deleted`,
  `deleted+created → modified`,
  rename pairing.
- Final-pass collapse: when `len(merged) > bulkChangeThreshold`, return a
  single-element slice `[]SourceChangeEvent{{Kind: ChangeBulkUpdate}}`.
- MUST be deterministic per input — same input slice yields same output
  slice, ordered (group by path, then last-write-wins per the merge table).

---

### `internal/semantic/live/watcher/watcher.go` (service, event-driven)

**Why this analog:** `internal/memory/watcher.go` is the only in-tree
fsnotify consumer; it already encodes recursive directory add, debounce,
graceful shutdown — exactly Phase 60's needs.

**Analog:** `internal/memory/watcher.go:28-58, 176-189`

**Constructor / fsnotify init** (lines 28-58):
```go
func NewWatcher(index *Index, projectDir, globalDir string, logger *slog.Logger) (*Watcher, error) {
    if logger == nil { logger = slog.Default() }

    fw, err := fsnotify.NewWatcher()
    if err != nil { return nil, err }

    w := &Watcher{
        index:      index,
        projectDir: projectDir,
        globalDir:  globalDir,
        watcher:    fw,
        logger:     logger,
        debounce:   300 * time.Millisecond,
    }

    for _, dir := range []string{projectDir, globalDir} {
        if dir != "" {
            if err := w.addRecursive(dir); err != nil {
                fw.Close()
                return nil, err
            }
        }
    }
    return w, nil
}
```

**Recursive add** (lines 176-189):
```go
func (w *Watcher) addRecursive(dir string) error {
    return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
        if err != nil { return nil }
        if info.IsDir() {
            if err := w.watcher.Add(path); err != nil {
                w.logger.Warn("watcher: failed to watch dir", "path", path, "error", err)
            }
        }
        return nil
    })
}
```

**What's reused:** `fsnotify.NewWatcher()` constructor, recursive directory
add at Start, `fw.Close()` on construction failure, debounce field on the
watcher value, `addRecursive` shape via `filepath.Walk`.

**What MUST differ:**
- ONE watcher PER WORKSPACE (keyed on `workspace.WorkspaceKey`); the manager
  type owns a `map[workspace.WorkspaceKey]*Watcher` with a `sync.RWMutex`.
- Output crosses the watcher→classifier boundary as `[]string` paths only
  (NOT `Op`-tagged) — fsnotify's `Op` flags collapse to "this path changed,
  classifier decides what kind" per D-01.
- Atomic-rename re-attach: when fsnotify emits a `Rename` event on a watched
  path, immediately re-`Add` the renamed-to path (within the same parent dir).
  Filter helpers detect JetBrains `___jb_tmp___` suffix and the renamed-to
  observable per RESEARCH.md.
- ENOSPC handling: at `fsnotify.NewWatcher()` or `fw.Add(...)` returning a
  `*os.PathError` wrapping `syscall.ENOSPC`, emit ONE `slog.Warn` with
  remediation hint and set `Status().Active = false, Reason = "inotify_enospc"`.
- Apply Phase 59 D-04 ignore rules (`.git/`, `node_modules/`, `vendor/`,
  `dist/`, `build/`, `target/`, `coverage/`, files above `indexing.max_file_size`)
  at recursive-add time AND at event-receive time.

---

### `internal/semantic/live/watcher/enospc.go` (utility, error classifier + Status)

**Why this analog:** `classifyExisting` follows the same "inspect runtime
state, return reason string + error" shape Phase 60 needs for ENOSPC
classification.

**Analog:** `internal/semantic/store/duckdb.go:149-159`

```go
// classifyExisting opens the DB and inspects semantic_schema_version. Returns
// ("", nil) when the DB is clean; ("<reason>", err) otherwise. The error
// carries the underlying cause for diagnostic logging — the caller does not
// re-surface it to the user.
func classifyExisting(ctx context.Context, path string) (string, error) {
    db, err := sql.Open("duckdb", path)
    if err != nil {
        return reasonCorruptFile, err
    }
    defer db.Close()
```

**What's reused:** Two-value `(reason string, err error)` return shape, named
reason constants (`reasonCorruptFile`, `reasonUnknown` analogs).

**What MUST differ:**
- New function `IsENOSPC(err error) bool` — uses `errors.Is(err, syscall.ENOSPC)`
  (verified in RESEARCH.md as unwrapped from `inotify_add_watch` per fsnotify
  v1.9.0 source `backend_inotify.go:285-300`).
- New struct `WatcherStatus { Active bool; Reason string; LastError error;
  RemediationHint string }`.
- New method `(*Watcher).Status() WatcherStatus` reading the watcher's atomic
  state. Return value is a value (not pointer) so callers can't mutate.
- Reason constants: `"inotify_enospc"`, `"closed"`, `"running"` (closed enum).

---

### `internal/semantic/live/scanner/scanner.go` (service, batch)

**Why this analog:** RESEARCH.md §Analog Pattern Locator nominates
`internal/repomap/extractor.go` `walkAndExtract` for the walker SHAPE;
`internal/skill/repomap/skill.go:386-411` is where that shape is actually
implemented. The structural template — `filepath.WalkDir` + skip-dir set +
symlink rejection — is reused. The hard rule is that
`internal/semantic/live/scanner/` MUST NOT import `internal/repomap`
(Phase 59 D-01 cascade).

**Analog (structural only — no import):** `internal/skill/repomap/skill.go:386-411`

```go
// walkAndExtract walks the workspace root and populates TagCache using TagExtractor.
func (s *RepoMapSkill) walkAndExtract(ctx context.Context, root string) error {
    return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            return nil // skip entries with errors
        }
        if d.IsDir() && skipDirs[d.Name()] {
            return filepath.SkipDir
        }
        if d.IsDir() {
            return nil
        }
        // T-30-02: skip symlinks to prevent symlink escape.
        if d.Type()&fs.ModeSymlink != 0 {
            return nil
        }

        lang := repomap.LangFromExt(path)
        if lang == "" {
            return nil // skip unsupported file types
        }
        // ...
        _, extractErr := s.cache.GetOrExtract(path, func() ([]repomap.Tag, error) { /* ... */ })
        // ...
    })
}
```

**Reference also:** `internal/kernel/fileops/find.go:35-67` — same
`filepath.WalkDir` + `skipDirs` pattern in fileops, in-tree precedent for
"net-new walker that ignores common build artifacts".

**What's reused:** `filepath.WalkDir` outer loop, `skipDirs` set for excluded
directories (`.git`, `node_modules`, `vendor`, `dist`, `build`, `target`,
`coverage` per SPEC §13.2), symlink rejection, return-nil-on-err to keep
walking past unreadable entries.

**What MUST differ:**
- NEW CODE — `internal/semantic/live/scanner/` MUST NOT import
  `internal/repomap` (Phase 59 D-01 cascade). Reimplement the `skipDirs` set
  in package-private form.
- Per-file work is xxhash64 of contents (via `github.com/cespare/xxhash/v2`,
  reused from Phase 59), then compare against `semantic_files.content_hash`
  via `internal/semantic/store/` API call.
- Output: emit `WorkspaceChangeSignal{Source: ChangeSourceManifestScan, Paths: ...}`
  for every mismatch (modified/created), or for every "in DB but missing on
  disk" (deleted). Single signal per scan cycle preferred (paths slice can be
  multi-element), but per-path signals also legal.
- Driven by a `time.Ticker(cfg.ManifestScanInterval)` in the Scanner's `Run`
  goroutine. Default 10s. Always-on (per D-05 invariant), not gated on
  watcher state.
- Walk concurrency: launch `cfg.MaxParallelFiles` goroutines (default 4 to
  match `extraction.max_parallel_files`) consuming a channel of `(path, size)`
  records emitted by the WalkDir loop. Use `errgroup.Group` for shutdown.

---

### `internal/semantic/live/handler/handler.go` (service, CRUD)

**Why this analog:** No exact in-tree analog (Phase 60 IS the first overlay
writer). Use `internal/semantic/store/effective.go` (read-then-decide) as the
package-shape template, and SPEC §16.3-§16.5 as the body spec.

**Analog (shape only):** `internal/semantic/store/effective.go` (sibling file
showing the per-operation handler shape inside `internal/semantic/store/`).
Phase 60 mirrors: one exported function per logical operation, each taking
ctx + identifiers + payload, returning error.

**What's reused:** Per-operation function signature
`func XYZ(ctx context.Context, repoID semantic.RepoID, ...) error`, internal
helpers private to the handler package.

**What MUST differ:**
- Bodies follow SPEC §16.3-§16.5 verbatim:
  - `UpdateChangedFile(ctx, repoID, path, contentHash)` — re-extract via
    Phase 59 `extract.Registry`, diff against effective facts, write to
    overlay tx, enqueue `LSPQueue.Enqueue(RevalidateFileJob{...})` as
    typed buffered handoff (Phase 61 fills consumer).
  - `HandleFileDeleted(ctx, repoID, path)` — `MarkFileDeleted` + cascade
    `MarkSymbolsDeleted` / `MarkReferencesDeleted` / `MarkEdgesDeleted` in
    one tx.
  - `HandleFileRenamed(ctx, repoID, oldPath, newPath)` — content-hash
    lineage check; on match preserve identity, on mismatch fall back to
    delete + create (SPEC §16.5).
  - `HandleBulkUpdate(ctx, repoID)` — call
    `scheduler.ScheduleIncremental(workspaceID, []FileChange{...})`
    instead of opening an overlay tx; the scheduler's incremental path
    handles the bulk re-walk.
- Each body uses `store.BeginOverlayTx(ctx, repoID)` then `tx.Commit()` —
  per-tx epoch is allocated inside `BeginOverlayTx`, not by the handler.
- No-op coalesced batches (e.g., empty `[]SourceChangeEvent`) MUST NOT
  open a tx — early-return before `BeginOverlayTx` (D-04 invariant: "Empty
  coalesced flushes do not advance epoch").
- Emits `helix_semantic_live_updates_total{kind, outcome}` metric on each
  path: `outcome ∈ {applied, no_op, error, dropped}`.

---

### `internal/semantic/live/service.go` (service, event-driven glue)

**Why this analog:** `RepoMapSkill` is the closest sibling: a service with
multiple post-init setters, internal mutex, dependency on the kernel
without circular imports.

**Analog:** `internal/skill/repomap/skill.go:113-145`

```go
// SetEnrichFn sets the optional LSP enrichment callback.
// Called by the daemon after kernel creation to enable cross-file LSP references.
func (s *RepoMapSkill) SetEnrichFn(fn func(graph *repomap.FileGraph)) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.enrichFn = fn
}

// SetFallbackDeps sets the fallback extraction dependencies for languages without tree-sitter grammars.
// Called by the daemon after kernel creation to enable LSP documentSymbol fallback (D-33-01).
func (s *RepoMapSkill) SetFallbackDeps(deps *FallbackDeps) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.fallbackDeps = deps
}
```

**What's reused:** Setter + `sync.Mutex` discipline; daemon-post-init wiring
boundary; one type owning multiple lifecycle phases.

**What MUST differ:**
- `liveService` exposes `OnEdit(ctx, ws, paths) error` implementing the
  `kernel.EditNotifier` interface — MUST return immediately after enqueueing
  into the per-workspace coalescer (D-03 invariant).
- Owns: classifier, per-workspace coalescer registry (`map[workspace.WorkspaceKey]*Coalescer`),
  per-workspace overlay-writer locks (`map[semantic.RepoID]*sync.Mutex`),
  scheduler reference (for `ScheduleIncremental` call from
  `HandleBulkUpdate`), watcher manager reference, scanner reference.
- `OnWorkspaceChanged(ctx, sig) error` is the public single entry point all
  three sources call (helix_edit hook, watcher dispatch, scanner dispatch);
  it normalizes paths, calls classifier, then enqueues into the coalescer.
- No global state — workspace registry keyed on `workspace.WorkspaceKey`.

---

### `internal/kernel/notifier.go` (model, interface)

**Why this analog:** The setter idiom is `SetEnrichFn`'s; the interface lives
inside `internal/kernel/` to avoid importing semantic from kernel. The
constructor on `Kernel` (`NewKernel`) plus a setter (`SetEditNotifier`)
mirrors the daemon's existing post-init wiring of repomap.

**Analog (setter idiom):** `internal/skill/repomap/skill.go:113-119` (above)
**Analog (kernel handle access pattern):** `internal/kernel/kernel.go:22-53`
(Kernel struct + accessor methods)

```go
// internal/kernel/kernel.go
type Kernel struct {
    workspaces map[string]*WorkspaceRuntime
    pool       *lspool.Pool
    // ...
    mu         sync.RWMutex
}

func (k *Kernel) Tracer() trace.Tracer { return k.tracer }
```

**What's reused:** Setter discipline (mutex-guarded), accessor returning the
stored interface, kernel as the single owner of the field.

**What MUST differ:**
- Interface lives in `internal/kernel/notifier.go` (NEW FILE) so
  `internal/kernel/edit/` and `internal/kernel/fileops/` can call it
  WITHOUT importing `internal/semantic/...`:
  ```go
  // internal/kernel/notifier.go
  type EditNotifier interface {
      OnEdit(ctx context.Context, workspaceID workspace.WorkspaceKey, paths []string) error
  }
  ```
- Stored on `*Kernel` as `editNotifier atomic.Value` (or mutex-guarded field).
  Accessor: `func (k *Kernel) EditNotifier() EditNotifier` returns nil-safe
  zero value when unset. Setter: `func (k *Kernel) SetEditNotifier(n EditNotifier)`.
- Edit-tool call site is fire-and-forget:
  ```go
  if n := k.EditNotifier(); n != nil {
      _ = n.OnEdit(ctx, wsKey, []string{relPath})
  }
  ```
  Errors are swallowed by design (fast-path, optional, watcher+scanner
  provide correctness coverage).

---

### `internal/lint/nokernel2semantic/analyzer.go` (utility, go/analysis Analyzer)

**Why this analog:** `internal/lint/noduckdb/analyzer.go` is the EXACT
template — same boundary-enforcement intent (one package subtree may not
import another package subtree), same go/analysis Analyzer shape.

**Analog:** `internal/lint/noduckdb/analyzer.go:1-40`

```go
// Package noduckdb provides a go/analysis Analyzer that fails the build if
// the duckdb-go module is imported from any package outside the allowlisted
// internal/semantic/store/ subtree.
package noduckdb

import (
    "strings"

    "golang.org/x/tools/go/analysis"
)

const allowedPkgPrefix = "github.com/agenthands/helix/internal/semantic/store"
const forbiddenImport = "github.com/duckdb/duckdb-go" // D-12 lock; prefix form

var Analyzer = &analysis.Analyzer{
    Name: "noduckdb",
    Doc:  "fails if duckdb-go is imported outside internal/semantic/store/",
    Run: func(pass *analysis.Pass) (interface{}, error) {
        if strings.HasPrefix(pass.Pkg.Path(), allowedPkgPrefix) {
            return nil, nil
        }
        for _, file := range pass.Files {
            for _, imp := range file.Imports {
                path := strings.Trim(imp.Path.Value, `"`)
                if strings.HasPrefix(path, forbiddenImport) {
                    pass.Reportf(imp.Pos(),
                        "duckdb-go may only be imported from %s (got %s)",
                        allowedPkgPrefix, pass.Pkg.Path())
                }
            }
        }
        return nil, nil
    },
}
```

**What's reused:** Entire structure verbatim — `Analyzer = &analysis.Analyzer{...}`
with `Name`, `Doc`, `Run`; package-prefix string check; `pass.Reportf` on
violation.

**What MUST differ:**
- INVERTED direction: kernel is the FORBIDDEN IMPORTER (any package whose
  path starts with `github.com/agenthands/helix/internal/kernel/`), and the
  FORBIDDEN IMPORT prefix is `github.com/agenthands/helix/internal/semantic`.
- Constants:
  ```go
  const checkedPkgPrefix = "github.com/agenthands/helix/internal/kernel"
  const forbiddenImportPrefix = "github.com/agenthands/helix/internal/semantic"
  ```
- Run logic: report only when `strings.HasPrefix(pass.Pkg.Path(), checkedPkgPrefix)`
  AND any import has prefix `forbiddenImportPrefix`. Reverse of the
  noduckdb early-return.
- Companion `analyzer_test.go` mirrors `noduckdb/analyzer_test.go` with
  `testdata/src/badpkg/` (faked kernel pkg importing semantic) and
  `testdata/src/goodpkg/` (faked non-kernel pkg or kernel pkg with no
  semantic import).

---

### `cmd/vet-nokernel2semantic/main.go` (config, singlechecker)

**Analog:** `cmd/vet-noduckdb/main.go:1-12` (entire file)

```go
// Command vet-noduckdb is a singlechecker binary wrapping the noduckdb
// Analyzer. It is wired into `make vet` via `go vet -vettool=...` to enforce
// STORE-06 on every test run.
package main

import (
    "github.com/agenthands/helix/internal/lint/noduckdb"
    "golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(noduckdb.Analyzer) }
```

**What's reused:** Entire file pattern, singlechecker.Main wrapper.

**What MUST differ:** Import path swap to
`github.com/agenthands/helix/internal/lint/nokernel2semantic`; `Doc` updated
in the analyzer file. Also: `Makefile` `vet` target gets a parallel
`go vet -vettool=$(go env GOPATH)/bin/vet-nokernel2semantic ./...` call.

---

### `internal/semantic/store/overlay.go` (FILL stub, service / CRUD)

**Why this analog:** `internal/semantic/store/snapshot.go` (sibling) is the
documented analog — it sits in the same package, comments out the same shape
of "BeginX → Insert* → Commit/Abort" API for snapshots that overlay needs
for live writes. `database/sql.Tx` lifecycle inside `internal/semantic/store/`
is established via `Open` (`internal/semantic/store/duckdb.go:72-147`).

**Analog (sibling stub doc):** `internal/semantic/store/snapshot.go:1-15`

```go
package store

// snapshot.go is the future home of the snapshot-write API (P59):
//
//   - BeginSnapshot(ctx) (SnapshotID, error)
//   - InsertFiles / InsertSymbols / InsertReferences / InsertEdges
//   - CommitSnapshot(ctx, SnapshotID) error
//   - AbortSnapshot(ctx, SnapshotID) error
```

**Existing overlay.go stub (target):** `internal/semantic/store/overlay.go:1-17`

```go
package store

// overlay.go is the future home of the live-overlay-write API (P60):
//
//   - UpsertOverlayFile(ctx, FileFact) error
//   - UpsertOverlaySymbol(ctx, SymbolFact) error
//   - DeleteOverlayFile(ctx, FileID) error  // tombstone
//   - FlushOverlay(ctx) error               // periodic / on-shutdown drain
```

**What's reused:** Lives inside `internal/semantic/store/` (vet-noduckdb
boundary holds); `*sql.DB` + `*sql.Tx` patterns from `Open`; Phase 57
metric helpers.

**What MUST differ:**
- New API:
  ```go
  type OverlayTx interface {
      UpsertOverlayFile(ctx context.Context, fact FileFact) error
      UpsertOverlaySymbol(ctx context.Context, fact SymbolFact) error
      UpsertOverlayReference(ctx context.Context, fact ReferenceFact) error
      UpsertOverlayEdge(ctx context.Context, fact EdgeFact) error
      MarkFileDeleted(ctx context.Context, fileID semantic.FileID) error
      MarkSymbolsDeleted(ctx context.Context, fileID semantic.FileID) error
      MarkReferencesDeleted(ctx context.Context, fileID semantic.FileID) error
      MarkEdgesDeleted(ctx context.Context, fileID semantic.FileID) error
      WriteInvalidations(...) error // no-op stub for Phase 60; Phase 62 fills
      Commit() error
      Rollback() error
  }
  func (s *Store) BeginOverlayTx(ctx context.Context, repoID semantic.RepoID) (OverlayTx, error)
  func (s *Store) FlushOverlay(ctx context.Context) error
  ```
- Per-workspace mutex registry: `s.overlayLocks map[semantic.RepoID]*sync.Mutex`,
  protected by `s.overlayLocksMu sync.Mutex`. `BeginOverlayTx` acquires
  the per-`repoID` mutex; `Commit/Rollback` release it. Cross-workspace
  txs do NOT serialize.
- Inside `BeginOverlayTx`, atomically increment
  `semantic_live_overlay_meta.current_epoch` and read it back into the tx
  as `writeEpoch`. Every `Upsert/Mark*` writes `write_epoch = writeEpoch`.
- All write SQL: `INSERT OR REPLACE INTO semantic_live_overlay_*` with the
  v3 columns including `write_epoch`.
- ROLLBACK does NOT rewind `current_epoch` — D-04 invariant: "no rollbacks
  on tx abort; the increment commits independently of row writes; if the
  tx rolls back, that epoch is just unused".

---

### `internal/semantic/store/migrations.go` (EXTEND, migration)

**Analog (within same file):** `applyMigration002` (lines 404-445)

```go
func applyMigration002(ctx context.Context, db *sql.DB) error {
    stmts := schema2Statements()
    for i, stmt := range stmts {
        if _, err := db.ExecContext(ctx, stmt); err != nil {
            return fmt.Errorf("applyMigration002: stmt %d (%s): %w", i+1, firstLine(stmt), err)
        }
    }
    return nil
}

func schema2Statements() []string {
    return []string{
        // semantic_files: 6 partial-extraction columns (D-05).
        // DuckDB rejects NOT NULL on ADD COLUMN even with DEFAULT (Parser
        // Error: "Adding columns with constraints not yet supported"); use
        // DEFAULT alone — see applyMigration002 doc comment.
        `ALTER TABLE semantic_files ADD COLUMN extraction_status TEXT DEFAULT ''`,
        // ... 9 more ALTER TABLE statements ...
        `INSERT INTO semantic_schema_version (version, applied_at) VALUES (2, now())`,
    }
}
```

**What's reused:** Pair of (`applyMigration00N`, `schema<N>Statements`) functions;
deterministic statement ordering; `db.ExecContext` per-statement loop;
`firstLine(stmt)` for error messages; final
`INSERT INTO semantic_schema_version (version, applied_at) VALUES (N, now())`
to stamp the new version.

**What MUST differ:**
- New pair `applyMigration003` + `schema3Statements`.
- DDL per CONTEXT.md D-04:
  ```sql
  ALTER TABLE semantic_live_overlay_meta ADD COLUMN current_epoch UBIGINT DEFAULT 0;
  ALTER TABLE semantic_live_overlay_files ADD COLUMN write_epoch UBIGINT DEFAULT 0;
  ALTER TABLE semantic_live_overlay_symbols ADD COLUMN write_epoch UBIGINT DEFAULT 0;
  ALTER TABLE semantic_live_overlay_references ADD COLUMN write_epoch UBIGINT DEFAULT 0;
  ALTER TABLE semantic_live_overlay_edges ADD COLUMN write_epoch UBIGINT DEFAULT 0;
  CREATE INDEX idx_overlay_files_write_epoch ON semantic_live_overlay_files(repo_id, write_epoch);
  CREATE INDEX idx_overlay_symbols_write_epoch ON semantic_live_overlay_symbols(repo_id, write_epoch);
  CREATE INDEX idx_overlay_references_write_epoch ON semantic_live_overlay_references(repo_id, write_epoch);
  CREATE INDEX idx_overlay_edges_write_epoch ON semantic_live_overlay_edges(repo_id, write_epoch);
  INSERT INTO semantic_schema_version (version, applied_at) VALUES (3, now());
  ```
- Same DuckDB constraint applies: `ADD COLUMN ... NOT NULL DEFAULT 0` is
  rejected; use `DEFAULT 0` alone (the application layer always sets
  `write_epoch` on every write).

---

### `internal/semantic/store/migrations_registry.go` (EXTEND, config)

**Analog (within same file):** lines 27-30

```go
var migrations = []Migration{
    {From: 0, To: 1, Kind: MigrationInPlace, Apply: applyMigration001},
    {From: 1, To: 2, Kind: MigrationInPlace, Apply: applyMigration002},
}
```

**What's reused:** Slice-literal entry shape.

**What MUST differ:** Append exactly one entry:
```go
{From: 2, To: 3, Kind: MigrationInPlace, Apply: applyMigration003},
```

---

### `internal/semantic/store/migrations_types.go` (EXTEND, config)

**Analog (within same file):** line 16

```go
const CurrentSchemaVersion = 2
```

**What's reused:** Single constant.

**What MUST differ:** Bump to `3`. Update doc comment to add Phase 60 line.

---

### `internal/phasegraph/pipelines/live.go` (FILL Run/Validate, controller)

**Analog (within same file):** `LiveUpdatePhases` slice (lines 29-39)

```go
var LiveUpdatePhases = []phasegraph.PhaseSpec{
    {ID: PhaseCollectEvents, Requires: nil, Provides: []string{"events"}, Run: noopRun},
    {ID: PhaseCoalesceEvents, Requires: []phasegraph.PhaseID{PhaseCollectEvents}, Provides: []string{"coalesced_events"}, Run: noopRun},
    // ... 7 more phases ...
    {ID: PhaseEnqueueLSPRevalidation, Requires: []phasegraph.PhaseID{PhaseMarkScoresClusters}, Provides: []string{"lsp_revalidation_queue"}, Run: noopRun},
}
```

**What's reused:** Phase IDs (typed constants — DO NOT rename), Requires
edges (linear chain), Provides labels.

**What MUST differ:**
- Replace `Run: noopRun` with real `Run: <func>` bodies that wire to the
  Phase 60 components:
  - `PhaseCollectEvents` → bind to `liveService.OnWorkspaceChanged`-fed
    coalescer queue.
  - `PhaseCoalesceEvents` → invoke `coalescer.CoalesceEvents`.
  - `PhaseClassifyEvents` → invoke `live.ClassifyPathChange` per path.
  - `PhaseParseChangedFiles` → invoke `extract.Registry` per language.
  - `PhaseDiffEffectiveFacts` → diff against `semantic_files` via
    `internal/semantic/store/effective.go`.
  - `PhaseWriteOverlay` → `store.BeginOverlayTx` + tx.Upsert*/Mark* + tx.Commit.
  - `PhaseRepairGraphCache` → typed no-op stub call to
    `GraphCache.ApplyRepair(repair)` (Phase 62 fills body).
  - `PhaseMarkScoresClusters` → typed no-op stub call to
    `MarkAffectedScoresAndClusters(...)` (Phase 62 fills body).
  - `PhaseEnqueueLSPRevalidation` → typed buffered handoff to
    `LSPQueue.Enqueue(RevalidateFileJob{...})` (Phase 61 fills consumer).
- Add `Validate` and `Shutdown` bodies per
  `phasegraph.PhaseSpec` interface (the existing struct shape covers Run
  + Requires + Provides; this phase adds the rest).
- Phase IDs and `Requires`/`Provides` strings DO NOT CHANGE — Phase 57 D-04
  shape locks them (acceptance #6 from CONTEXT.md downstream phases).

---

### `internal/daemon/daemon.go` (EXTEND post-init, controller)

**Analog (within same file):** `SetEnrichFn` post-init block (lines 372-385)
and `SetActivateCallback` (lines 463-496)

**Pattern 1 — Post-init setter wiring** (lines 372-385):
```go
// 12b. Wire repomap skill LSP enrichment callback (RMAP-08).
if rs := repomapSkill.GetRepoMapSkill(); rs != nil {
    tagCache := rs.Cache()
    rs.SetEnrichFn(func(g *repomapPkg.FileGraph) {
        wsKey := activeWSKey
        if wsKey.RepoRoot == "" {
            return
        }
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        enrichRepoMapFromLSP(ctx, k, wsKey, g, tagCache, logger)
    })
}
```

**Pattern 2 — Workspace-activate callback** (lines 463-496):
```go
mcpServer.SetActivateCallback(func(ctx context.Context, repoPath string) error {
    rt, err := k.ActivateWorkspace(ctx, repoPath)
    if err != nil {
        return err
    }
    activeWSKey = workspace.WorkspaceKey{RepoRoot: repoPath}
    // ... per-workspace setup ...
    if semanticScheduler != nil {
        semanticScheduler.ScheduleInitialExtraction(
            semantic.WorkspaceID(repoPath),
            scheduler.InitialExtraction{
                Reason: "workspace_activation",
                Mode:   scheduler.ModeAuto,
            },
        )
    }
    logger.Info("kernel workspace activated",
        "root", repoPath,
        "languages", rt.Languages(),
    )
    return nil
})
```

**What's reused:** Numbered comment block for new step (e.g. "12e. Wire live
update service"); nil-guarded setter pattern; per-workspace activation
inside `SetActivateCallback`; `semanticScheduler` reference (already in
scope post Phase 59).

**What MUST differ:** Add three new bootstrap blocks AFTER the Phase 59
scheduler block:
1. **Construct live service** (after Phase 59 scheduler open, before
   middleware install):
   ```go
   liveService := live.NewService(store, scheduler, classifier, logger, observability.Metrics())
   ```
2. **Wire kernel.SetEditNotifier:**
   ```go
   k.SetEditNotifier(liveService)
   ```
3. **Construct watcher manager + scanner manager:**
   ```go
   watcherMgr := watcher.NewManager(liveService, cfg.SemanticIndex.LiveUpdates, logger)
   scannerMgr := scanner.NewManager(liveService, store, cfg.SemanticIndex.LiveUpdates, logger)
   ```
4. **Inside the existing `SetActivateCallback` body**, after
   `ScheduleInitialExtraction` and gated on `cfg.SemanticIndex.LiveUpdates.Enabled`:
   ```go
   if cfg.SemanticIndex.LiveUpdates.Enabled {
       if cfg.SemanticIndex.LiveUpdates.WatcherEnabled {
           watcherMgr.Start(workspace.WorkspaceKey{RepoRoot: repoPath})
       }
       if cfg.SemanticIndex.LiveUpdates.ManifestScanEnabled {
           scannerMgr.Start(workspace.WorkspaceKey{RepoRoot: repoPath})
       }
   }
   ```
5. **Register `helix_semantic_live_updates_total` metric** via
   `observability.Metrics()` path (Phase 57 D-07 closed-enum registration).

---

### `internal/kernel/edit/replace.go`, `insert.go`, `rename.go`, `delete.go` (INSERT OnEdit, controller)

**Analog (within `internal/kernel/edit/tools.go`):** lines 308-309 (and
similar at 391, 455, 521, 578) — the `RecordEditOutcome` defer pattern shows
the canonical post-handler hook insertion point in the registered handler.

```go
// Phase 53 D-16 emission: defer captures the live (outcome, strategy)
// values; each error branch updates `outcome` (strategy stays "none"
// on the error path), and the success branch updates `strategy` from
// fuzzyInfo when fuzzy ran.
outcome, strategy := "success", "none"
defer func() { mcp.RecordEditOutcome(ctx, "replace_symbol_body", outcome, strategy) }()
```

**What's reused:** Hook-emitted-from-tool-handler pattern; success path is
the call site; the kernel handle (`k *kernel.Kernel`) is in scope at the
registration closure.

**What MUST differ:**
- The OnEdit hook must fire ONLY on success path (not in defer — per D-03
  the hook is fire-and-forget for successful edits, not a metric on every
  outcome).
- Insertion point per file: at the success path AFTER the underlying
  edit/write operation returns nil error AND BEFORE the function returns.
  Pattern:
  ```go
  // After successful edit:
  if n := k.EditNotifier(); n != nil {
      _ = n.OnEdit(ctx, wsKey, []string{args.Path})
  }
  return textResult("..."), nil, nil
  ```
- Files affected (5 in edit/, 3 in fileops/):
  - `replace.go` — `replace_symbol_body` registration (around tools.go:309)
  - `insert.go` — `insert_before_symbol`, `insert_after_symbol` (391, 455)
  - `rename.go` — `rename_symbol` (521); also rename emits `replaceCount` paths
  - `delete.go` — `safe_delete_symbol` (578)
  - `fileops/replace.go` — `replace_in_file` (370)
  - `fileops/fuzzy_edit.go` — `fuzzy_edit` (452)
  - `fileops/write.go` — wrapped through registration, plus `create_file`
    success in tools.go:245 (NOTE: RESEARCH.md §Open Questions O-1 flags
    "write_file vs create_file" discrepancy — planner to resolve).
- The hook MUST accept multiple paths (e.g., rename_symbol can affect many
  files via WorkspaceEdit); collect from the underlying result struct and
  pass `[]string` to `OnEdit`.

---

### `internal/kernel/edit/tools.go` and `internal/kernel/fileops/tools.go` (EXTEND, controller)

**Analog:**
- `internal/kernel/edit/tools.go:131` — `RegisterTools` signature
- `internal/kernel/fileops/tools.go:169` — `RegisterTools` signature

```go
// internal/kernel/edit/tools.go:131
func RegisterTools(server *mcp.SerenaMCPServer, k *kernel.Kernel, extractor *BodyExtractor, diagStore *diag.DiagnosticStore, wsKeyFn func() workspace.WorkspaceKey) {
    tracer := k.Tracer()
    registerReplaceBody(server, k, extractor, diagStore, wsKeyFn, tracer)
    // ...
}
```

**What's reused:** Existing parameter list; the `*kernel.Kernel` handle is
ALREADY threaded through every `register*` call. Phase 60 reuses
`k.EditNotifier()` from inside each registered handler — NO new parameter
needed.

**What MUST differ:**
- NO signature change. The handlers acquire the notifier via
  `k.EditNotifier()` accessor at call time. This preserves backwards
  compatibility for tests that construct `RegisterTools` calls today.
- Optional: each `registerXYZ` function may close over the notifier once
  via `notifier := k.EditNotifier()` at registration time; planner's call.

---

### `internal/config/defaults.go` (INSERT 3 keys, config)

**Analog (within same file):** `live_updates.*` block (lines 78-87)

```go
// live_updates.*
"semantic_index.live_updates.enabled":                      true,  // SPEC §25
"semantic_index.live_updates.debounce_ms":                  250,   // SPEC §25
"semantic_index.live_updates.max_batch_delay_ms":           1500,  // SPEC §25
"semantic_index.live_updates.bulk_change_threshold":        200,   // SPEC §25
"semantic_index.live_updates.compact_after_idle_ms":        5000,  // SPEC §25
"semantic_index.live_updates.lsp_revalidate_after_idle_ms": 750,   // SPEC §25
"semantic_index.live_updates.lsp_compaction_max_wait_ms":   3000,  // SPEC §25
"semantic_index.live_updates.max_overlay_files":            1000,  // SPEC §25
"semantic_index.live_updates.max_overlay_age":              "30m", // SPEC §25
```

**What's reused:** Block heading comment `// live_updates.*`; aligned
key/value/comment formatting; SPEC reference comment style.

**What MUST differ:**
- Three new keys ADDED to the same block:
  ```go
  "semantic_index.live_updates.watcher_enabled":         true,    // Phase 60 D-05
  "semantic_index.live_updates.manifest_scan_enabled":   true,    // Phase 60 D-05
  "semantic_index.live_updates.manifest_scan_interval":  "10s",   // Phase 60 D-05
  ```
- Companion test in `internal/config/defaults_test.go` (per Phase 57 D-09
  pattern): `TestLoad_LiveUpdatesDefaults` — load fresh koanf, assert all
  three new keys at expected default values.
- `SerenaConfig.SemanticIndex.LiveUpdates` struct field already reserved
  in Phase 57 P02/P03 — Phase 60 adds three new struct fields:
  `WatcherEnabled bool`, `ManifestScanEnabled bool`, `ManifestScanInterval time.Duration`.

---

### `internal/semantic/scheduler/scheduler.go` (FILL `ScheduleIncremental`, service)

**Analog (within same file):** `ScheduleInitialExtraction` (lines 67-79) +
existing `ScheduleIncremental` stub (lines 81-85)

```go
// ScheduleInitialExtraction kicks an initial-walk extraction for ws. Idempotent
// per workspace: if a job is already in-flight, the existing JobID is
// returned and req is ignored (the first call's reason wins). The state
// transitions to SemanticIndexing on the first admission and Subscribers
// receive the new status.
func (s *Scheduler) ScheduleInitialExtraction(ws semantic.WorkspaceID, req InitialExtraction) JobID {
    s.mu.Lock()
    defer s.mu.Unlock()
    if existing, ok := s.jobs[ws]; ok {
        return existing // idempotent: return the in-flight job ID
    }
    job := JobID(generateJobID(ws, req.Reason))
    s.jobs[ws] = job
    s.transitionUnlocked(ws, SemanticIndexing)
    return job
}

// ScheduleIncremental is a Phase 60 stub — returns a sentinel JobID and does
// not mutate state. Wave 2 ships the interface so consumers compile today.
func (s *Scheduler) ScheduleIncremental(ws semantic.WorkspaceID, changes []FileChange) JobID {
    return JobID("phase60-incremental-stub")
}
```

**What's reused:** `s.mu.Lock` / `defer s.mu.Unlock` discipline; `JobID`
generation via `generateJobID(ws, reason)`; `s.jobs` admission map;
`transitionUnlocked` publish.

**What MUST differ:**
- Body produces a real JobID, dispatches per `FileChange.Kind`:
  ```go
  func (s *Scheduler) ScheduleIncremental(ws semantic.WorkspaceID, changes []FileChange) JobID {
      s.mu.Lock()
      defer s.mu.Unlock()
      job := JobID(generateJobID(ws, "incremental"))
      // Per-FileChange dispatch via the existing handler module's entry points:
      for _, ch := range changes {
          switch ch.Kind {
          case "modified", "created":
              go s.dispatchUpdate(ws, ch)  // calls handler.UpdateChangedFile
          case "deleted":
              go s.dispatchDelete(ws, ch)  // calls handler.HandleFileDeleted
          // rename handled via paired changes by the caller
          }
      }
      s.transitionUnlocked(ws, SemanticIndexing)
      return job
  }
  ```
- This is the in-process API the dispatcher (D-02) calls into — also the
  path Phase 64's `refresh_semantic_graph` MCP tool will reach.

---

## Shared Patterns

### Cross-package import boundary enforcement

**Source:** `internal/lint/noduckdb/analyzer.go:1-40`
**Apply to:** `internal/lint/nokernel2semantic/analyzer.go` (boundary
between `internal/kernel/...` and `internal/semantic/...`)

The two-constant + Run pattern (`allowedPkgPrefix`, `forbiddenImport`,
prefix check, `pass.Reportf`) is the shared template. All net-new analyzer
files in the project follow it.

### Setter-based post-init wiring

**Source:** `internal/skill/repomap/skill.go:113-145`
**Apply to:** `internal/kernel/notifier.go` `SetEditNotifier`,
`internal/semantic/live/service.go` setter methods

Mutex-guarded setter, nil-safe accessor returning a value type or interface,
called from `internal/daemon/daemon.go` AFTER all fundamental subsystems
are open.

### Closed-enum metric labels with drop-on-unknown

**Source:** `internal/obs/metrics.go:333-351` (`EditOutcomeInc`)
**Apply to:** `helix_semantic_live_updates_total{kind, outcome}` registration
in `internal/obs/metrics.go`

```go
func (m *Metrics) LiveUpdateInc(kind, outcome string) {
    switch kind {
    case "file_created", "file_modified", "file_deleted",
        "file_renamed", "helix_edit", "bulk_update":
    default:
        return // drop unknown
    }
    switch outcome {
    case "applied", "no_op", "error", "dropped":
    default:
        return // drop unknown
    }
    m.LiveUpdate.WithLabelValues(kind, outcome).Inc()
}
```

Closed enum + early-return on unknown is the project-wide invariant (T-53-01
mitigation, Phase 57 D-07 cascade). Allowlist test in
`internal/obs/metrics_labels_test.go` extends.

### Schema migration via registry

**Source:** `internal/semantic/store/migrations.go:404-445` (`applyMigration002`),
`migrations_registry.go:27-30` (registry slice),
`migrations_types.go:16` (`CurrentSchemaVersion`)
**Apply to:** Phase 60 v2→v3 migration (three-touch: function pair + registry
slice append + version-bump constant)

DuckDB ALTER TABLE constraint: `ADD COLUMN ... DEFAULT 0` works,
`ADD COLUMN ... NOT NULL DEFAULT 0` rejects. Always use `DEFAULT` alone;
application layer always sets the value on every write.

### Per-workspace lifecycle bound to `SetActivateCallback`

**Source:** `internal/daemon/daemon.go:463-496`
**Apply to:** Watcher manager `Start(ws)` and Scanner manager `Start(ws)`
calls inside the existing callback body — gated on
`cfg.SemanticIndex.LiveUpdates.{Enabled, WatcherEnabled, ManifestScanEnabled}`.

---

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/semantic/live/classifier.go` | service (decision fn) | transform | Net-new logic — combines filesystem state read + content-hash compare + source-tagging into a single function. Closest read-then-decide is `classifyExisting` (`internal/semantic/store/duckdb.go:153`); shape similar but domain different. Use SPEC §16.1's classification table verbatim. |
| `internal/semantic/live/coalescer/coalesce.go` | utility (pure fn) | transform | SPEC §16.2 prescribes the merge rules verbatim; no in-tree pure-event-merge analog exists. Implement to spec. |
| `internal/semantic/live/handler/handler.go` | service (overlay writer) | CRUD | First overlay-write code in tree. SPEC §16.3-§16.5 is the body spec. Sibling `internal/semantic/store/effective.go` provides shape only. |

---

## Metadata

**Analog search scope:**
- `internal/memory/`
- `internal/semantic/store/`
- `internal/semantic/scheduler/`
- `internal/skill/repomap/`
- `internal/repomap/`
- `internal/kernel/`, `internal/kernel/edit/`, `internal/kernel/fileops/`
- `internal/lint/noduckdb/`, `cmd/vet-noduckdb/`
- `internal/daemon/daemon.go`
- `internal/phasegraph/pipelines/`
- `internal/config/defaults.go`
- `internal/obs/metrics.go`

**Files scanned:** ~28 with concrete excerpt extraction; ~15 cross-referenced.

**Pattern extraction date:** 2026-05-05

**Coverage check (NEW packages, all 11 listed):**
- `internal/semantic/live/signal.go` ✓ (FileChange analog)
- `internal/semantic/live/classifier.go` ✓ (effective.go shape; spec drives body)
- `internal/semantic/live/coalescer/coalescer.go` ✓ (memory/watcher.go run-loop)
- `internal/semantic/live/coalescer/coalesce.go` ✓ (schema2Statements pure-fn shape; SPEC §16.2 body)
- `internal/semantic/live/watcher/watcher.go` ✓ (memory/watcher.go full)
- `internal/semantic/live/watcher/enospc.go` ✓ (classifyExisting reason+err shape)
- `internal/semantic/live/scanner/scanner.go` ✓ (skill/repomap walkAndExtract structural; fileops/find.go in-tree precedent)
- `internal/semantic/live/handler/handler.go` ✓ (effective.go shape; SPEC §16.3-§16.5 body)
- `internal/semantic/live/service.go` ✓ (RepoMapSkill setter discipline)
- `internal/kernel/notifier.go` ✓ (SetEnrichFn setter idiom + Kernel accessor pattern)
- `internal/lint/nokernel2semantic/analyzer.go` ✓ (noduckdb full template)
- `cmd/vet-nokernel2semantic/main.go` ✓ (vet-noduckdb full template)

All 11 NEW package files have an analog excerpt with file:line.
