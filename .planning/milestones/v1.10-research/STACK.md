# Technology Stack — v1.10 Live Semantic Index (Additions)

**Project:** Helix v1.10 Live Semantic Index
**Researched:** 2026-05-03
**Scope:** NEW dependencies only — existing v1.9 stack (Go 1.25.1, MCP SDK, koanf, modernc.org/sqlite, go-tree-sitter, gRPC, Prometheus, OTel, cobra, koanf, fsnotify v1.9.0) is fixed and not re-evaluated.

---

## TL;DR — Recommended Additions

| Component | Library | Version | CGO | Confidence |
|-----------|---------|---------|-----|------------|
| Fact store driver | `github.com/duckdb/duckdb-go` | v2.10502.0 (DuckDB 1.5.2) | **REQUIRES CGO=1** | HIGH |
| File watcher | `github.com/fsnotify/fsnotify` (already in go.mod) | v1.9.0 | none | HIGH |
| Graph algorithms (validation only) | `gonum.org/v1/gonum` | v0.16.x | none | HIGH |
| PageRank / clustering (production) | hand-rolled in `internal/semantic/rank/` and `internal/semantic/graph/` | — | none | HIGH |
| Pipeline DAG | stdlib only (Kahn topo sort, ~80 LOC) | — | none | HIGH |
| Token counting (eval) | `github.com/tiktoken-go/tokenizer` | v0.6.x | none (pure Go, embedded vocab) | MEDIUM |
| Patch apply (eval) | `github.com/bluekeyes/go-gitdiff` | v0.8.x | none | MEDIUM |

**CGO posture impact:** v1.10 BREAKS the CGO=0 stub policy at the runtime level for the semantic-index feature. Must extend the existing Phase 51.1 stub pattern (`//go:build cgo` / `!cgo`) to `internal/semantic/store/duckdb.go` so CGO=0 builds compile but `semantic_index.enabled=true` produces a structured "feature requires CGO=1 build" error at activation. This is a continuation of policy, not a violation — the stub path was *designed* for exactly this case.

---

## 1. DuckDB Go Bindings

### Recommended: `github.com/duckdb/duckdb-go` v2.10502.0

**Why this one:**
- Official DuckDB org repository — `marcboeker/go-duckdb` was donated to DuckDB Labs and v2.5.0+ lives at `duckdb/duckdb-go`. Use the canonical path going forward; pin to a specific tag.
- Versioning encodes upstream DuckDB: `v2.MAJOR_MINOR_PATCH.x` → `v2.10502.0` ⇒ DuckDB 1.5.2.
- Implements `database/sql.Driver` (works with the existing `database/sql` patterns we already use for `modernc.org/sqlite`), plus a lower-level Appender API for bulk-loading symbol/reference rows during full reindex.
- Pre-built static libs bundled for darwin/{amd64,arm64}, linux/{amd64,arm64}, windows/amd64 — matches our 6-archive goreleaser matrix exactly. **No FreeBSD** (dropped at v2; we don't ship FreeBSD).
- Default build links the bundled static lib — no `libduckdb.so` required on user systems. Single-binary property preserved at the goreleaser archive level.

**CGO Reality (HARD CONSTRAINT):**
- `CGO_ENABLED=1` REQUIRED. There is no pure-Go DuckDB driver and there will not be one — DuckDB itself is a 200kLOC C++ analytical engine; the maintainers explicitly rejected a native-Go port discussion (see Discussion #232).
- Cross-compilation requires `CC=<cross-toolchain> CGO_ENABLED=1`. Goreleaser already runs per-arch builders for the v1.9 release matrix, so this is incremental, not net-new infra.
- Build tags: default = bundled static link (what we want). `-tags=duckdb_use_lib` (system dynamic link) and `-tags=duckdb_use_static_lib` (custom prebuilt) are alternatives we should NOT use — bundled static is the single-binary path.
- `-tags=duckdb_arrow` is **opt-in** at v2 — leave OFF (Arrow connections are not pool-safe and we don't need Arrow IPC).

**Concurrency model:**
- Single DuckDB database file is a process-wide singleton. Use one `*sql.DB` per workspace, with `MaxOpenConns=N` to leverage `database/sql`'s pool.
- DuckDB is *single-writer, multi-reader* at the file level. Live overlay writes must be serialized (single goroutine queue feeding the writer) — fits ADR-002's overlay model naturally. Snapshot reads run on read-only connections.
- Snapshot/checkpoint operations (`CHECKPOINT`, `EXPORT DATABASE`) require quiescence — coordinate with the live update queue's compaction trigger.

### Alternatives Considered (and rejected)

| Alternative | Why Not |
|-------------|---------|
| `marcboeker/go-duckdb` v1 | Donated upstream; v1 is unmaintained going forward. Use `duckdb/duckdb-go`. |
| `duckdb/duckdb-go-bindings` | Lower-level CGO-only bindings, no `database/sql` driver. Too much surface area for us. |
| Pure-Go SQLite + manual columnar | We already use `modernc.org/sqlite` for FTS5 memory. Re-purposing for analytical workloads (200k symbols × millions of references with PageRank-friendly aggregation) loses 10–100× on the queries SPEC §8 implies. ADR-001 explicitly chose DuckDB; revisiting that is out of scope. |
| `chDB-go` (ClickHouse embedded) | Same CGO requirement as DuckDB but heavier runtime, less mature Go binding, and no `database/sql` driver. No advantage. |
| Gonum-based in-memory store | Loses durability (ADR-002 requires committed snapshots survive restart) and reproducibility (ADR-007 eval needs deterministic snapshots). |
| BadgerDB / Pebble (KV) | Wrong shape — we need analytical SQL with joins across symbols/references/edges, not KV. |

**Decision rationale:** ADR-001 already made this call. The research question for v1.10 is *which DuckDB binding*, not *whether DuckDB*. Answer: official `duckdb/duckdb-go` v2.10502.0, default static-bundled build.

### CGO Policy Reconciliation

v1.9 Phase 51.1 established the `//go:build cgo` stub pattern so `CGO_ENABLED=0` builds still compile (with a runtime refusal). v1.10 extends that pattern:

```
internal/semantic/store/duckdb.go        // //go:build cgo
internal/semantic/store/duckdb_stub.go   // //go:build !cgo
```

The stub path returns a `Kind: Unsupported` error when `semantic_index.enabled=true` on a CGO=0 build, with remediation text pointing at the CGO=1 install instructions. CGO=0 builds keep working for everything except the new semantic feature — same shape as the v1.9 tree-sitter stub. **Document this in EMBED-AUDIT.md and CONTRIBUTING.md.**

---

## 2. File Watcher

### Recommended: `github.com/fsnotify/fsnotify` v1.9.0 (already vendored)

**Why no change:**
- Already in `go.mod` (used by memory FTS index watcher). Adding a second watching library is gratuitous.
- Cross-platform backend selection is automatic: inotify (Linux), FSEvents (macOS), kqueue (BSD), ReadDirectoryChangesW (Windows).
- v1.9.0 (current) is the actively maintained line.

**Critical gaps fsnotify imposes (we own the workarounds):**

| Gap | Impact | Mitigation |
|-----|--------|------------|
| No recursive watch on Linux/Windows | Each subdirectory is a separate watch FD. 10k-file repo ≈ ~1k–3k dirs. | Walk repo once at start, register each dir; on `Create`(dir) events, register the new dir; on `Remove`(dir), unregister. Mirrors the pattern in `rfsnotify` but in-tree (≈150 LOC). |
| Linux `inotify` per-user watch limit (`fs.inotify.max_user_watches`, default 8192–524288) | Repos with >8k dirs blow the limit on stock Ubuntu. | (a) Detect `ENOSPC` from fsnotify and degrade to manifest polling for that workspace. (b) Surface the limit in `get_semantic_graph_status` and include a sysctl-tuning runbook. (c) Already covered by SPEC §27.2 "Watcher Misses" mitigation. |
| macOS FSEvents coalesces and may drop events under load | Edit storms can lose events | SPEC §27.2 already mandates "periodic manifest check" + content-hash check on query. Implement these as belt-and-suspenders. |
| Symlinks not followed automatically | `node_modules`-style symlinked sub-projects miss events | Resolve symlinks during the initial walk, watch the resolved target if inside workspace; otherwise skip and document. |
| No event-ordering guarantees | Rename = (Remove, Create) pair, possibly out of order | Coalesce by path with debounce (SPEC §16.2 already specifies `debounce_ms: 250`, `bulk_change_threshold: 200`). |

### Alternatives Considered

| Alternative | Why Not |
|-------------|---------|
| `andreaskoch/go-fswatch` (polling) | Avoids inotify limits but O(N) polling cost on 10k files. Dead-last on latency. Useful only as a fallback. |
| `rjeczalik/notify` | Has recursive watch on macOS/Windows but Linux still emulates by walk-and-register, and the project is much less actively maintained than fsnotify. Not worth the swap. |
| `rfsnotify` wrapper | Thin wrapper over fsnotify; we'd inherit the same fd-per-dir cost AND a third-party dep. Implement the wrapper logic in-tree instead. |

**Decision:** Keep fsnotify v1.9.0. Build the recursive walker + ENOSPC fallback in `internal/semantic/live/watcher.go` (already in SPEC §6 package layout). No new dependency.

---

## 3. Graph Algorithms

### Recommended: hand-rolled in production, gonum for **validation/testing only**

**Production code (in `internal/semantic/rank/` and `internal/semantic/graph/`):**

| Algorithm | Why hand-rolled |
|-----------|-----------------|
| Weighted PageRank (multiple projections) | SPEC §18.2 specifies edge-weight + damping + sparse iteration. Existing repomap PageRank is ~60 LOC. v1.10 needs (a) per-projection weight, (b) personalized restart vector, (c) **incremental local repair** (SPEC §18.4). Gonum's `network.PageRank` does (a) but not (b) or (c). Forking it is more code than writing it. |
| Personalized PageRank | Not in gonum. Custom restart vector ⇒ trivial extension of weighted PR (~20 LOC delta). |
| Incremental local PageRank repair | Not in gonum. Bounded-BFS frontier + local power iteration. Custom — this is core v1.10 IP per ADR-005. |
| Weak components | Stdlib-friendly union-find, ~40 LOC. Gonum has it but we don't want to load gonum's `graph.Graph` adapter just for this. |
| Label propagation | SPEC §19.3 needs a specific tie-break + freshness-aware variant. Not in gonum. ~80 LOC. |
| Bounded BFS / reverse reachability | Stdlib. ~30 LOC each. |

ADR-005 explicitly says: *"Implement hot graph operations directly in Go… Gonum may be used for validation or non-critical algorithms, but not as the core storage or graph model."* This research confirms that's the right call:
- Gonum's graph model uses `int64` node IDs through its `graph.Node` interface — forcing a translation layer between our compact symbol IDs (FNV-64 of stable key, SPEC §11.1) and gonum's internal IDs.
- Gonum's `network.PageRank` is dense-vector-friendly; our graphs are sparse and projection-filtered. We'd be adapting around it more than benefiting from it.

### Recommended: `gonum.org/v1/gonum` v0.16+ — **test-only dependency**

Use cases:
- Cross-check our weighted PageRank against `network.PageRank` on small synthetic graphs as an oracle in unit tests.
- `topo.ConnectedComponents` as an oracle for our weak-component impl.
- Validate clustering output against `community.Modularize` on small fixtures.

This keeps gonum out of the runtime closure (it pulls in a chunk of `gonum/blas/cgo`-adjacent transitive deps if you're not careful — but the pure-Go subset under `gonum/graph` and `gonum/graph/network` does NOT require CGO).

```go
// in test file only
require gonum.org/v1/gonum v0.16.0 // test-only oracle
```

### Alternatives Considered

| Alternative | Why Not |
|-------------|---------|
| Gonum in production | ADR-005 already rejected. Translation layer overhead, doesn't cover personalized/incremental cases. |
| `alixaxel/pagerank` | Single-file, weighted only, no personalized/incremental. Strictly subset of what we need. |
| `dominikbraun/graph` | Generic graph library, but we'd still write PR ourselves; doesn't help. |

---

## 4. Eval Harness Dependencies

### Token Counting: `github.com/tiktoken-go/tokenizer` v0.6.x

**Why this one:**
- **Pure Go**, no CGO, embeds OpenAI vocabularies as Go maps at compile time (no runtime download, important for our offline/single-binary stance).
- Covers `cl100k_base`, `o200k_base`, `p50k_base`, `r50k_base` (GPT-3.5/4/4o family).
- For Anthropic Claude: we already have `github.com/anthropics/anthropic-sdk-go v1.35.0` in go.mod, which has the official `messages.CountTokens` server-side endpoint. Use that for Claude exact counts; use tiktoken locally as a heuristic fallback when we don't want to hit the network.

**NOT recommended:** `pkoukk/tiktoken-go` — downloads vocab to a cache dir on first use. Breaks the offline/airgapped story and adds a network failure mode to eval runs. Strictly worse than `tiktoken-go/tokenizer` for our use case.

**For DeepSeek / other OpenAI-compatible providers:** they re-use `cl100k_base` or `o200k_base`; tiktoken-go/tokenizer covers them.

### Patch Application: `github.com/bluekeyes/go-gitdiff` v0.8.x

**Why this one:**
- Pure Go, parses git-style and standard unified diffs, exposes an `Apply` function for both text and binary patches.
- Maintained by Palantir (active 2025–2026 commits on main).
- The eval harness needs to apply LLM-emitted patches to a repo snapshot, run tests, and score. `go-gitdiff` is the cleanest "patch in, mutated bytes out" API in Go.

**NOT recommended:** `sourcegraph/go-diff` is parser-only (no apply), `sourcegraph/go-diff-patch` only generates patches. Both are incomplete for the eval use case.

**Note:** The original prompt asked about `bluekeyes/go-patch` — that's not the actual repo name. The library is `bluekeyes/go-gitdiff`.

### Test Runner Orchestration: stdlib `os/exec` + `context`

The eval harness runs `go test`, `pytest`, etc. as subprocesses with bounded timeouts. No new library needed — `os/exec.CommandContext` + `errgroup` (already in go.mod via `golang.org/x/sync`) covers it. Anything heavier (e.g., a dedicated test-runner abstraction) is over-engineering for SPEC §32's eval modes.

---

## 5. Pipeline DAG (ADR-010)

### Recommended: stdlib only

**Rationale:** Kahn's topological sort over `map[Phase][]Phase` adjacency is ~30 LOC. Cycle detection is the same pass (if not all nodes are emitted, there's a cycle). DOT-format dump for `dump_dot_on_error: true` (SPEC §25) is another ~20 LOC. Total: ~80 LOC in `internal/semantic/phasegraph/`.

**No third-party DAG/workflow library is justified.** Anything we'd consider (`graphkit`, `dag`, etc.) brings:
- Generics gymnastics or `interface{}`-flavored APIs.
- Extra abstractions (Pipeline, Step, Worker) we don't want.
- Test-and-maintenance burden for code we'd write in an afternoon.

This matches the v1.6 RepoMap precedent: hand-rolled PageRank in ~60 LOC was cheaper and clearer than pulling gonum.

---

## 6. Type Resolution / Fixpoint Iteration

### Recommended: no library

**Rationale:** Fixpoint iteration is a `for { changed := false; ... if !changed { break } }` loop. The complexity is in the *resolution rules per language* (JSDoc/PHPDoc/YARD/Python typing comments per ADR-009), not in the iteration scaffolding.

Worth studying for patterns:
- **Go's `go/types` package** — its iterative method-set resolution is a clean reference for tiered confidence + fixpoint iteration. Stdlib, no dep.
- **gopls' `internal/typeparams`** — similar.

For **comment-based fallback parsing** (JSDoc, PHPDoc, YARD, Python type comments):

| Language | Parser source |
|----------|---------------|
| JSDoc | Hand-roll lightweight comment scanner; full JSDoc is huge but we only need `@param {Type}` / `@returns {Type}` / `@type` / `@typedef`. ~200 LOC. |
| PHPDoc | Same shape as JSDoc. Hand-roll. |
| YARD (Ruby) | `# @param [Type] name` — regex-tractable. Hand-roll. |
| Python typing comments / docstrings | `# type: T` + `:type x:` — regex-tractable. Hand-roll. For full docstring parsing later, defer to v1.11. |

Pulling JS/PHP/Ruby AST libraries to parse comments is overkill — comments are line-based and the syntax we need is a tiny subset. Tree-sitter already gives us the comment node positions; we just regex inside.

**No new dependency.**

---

## 7. Trace / Metrics Additions

### Existing infra reused

All v1.10 metrics (SPEC §28.1, 25 new families) plug into the existing `internal/obs/` Prometheus registry. The v1.9 PromQL validator (registry-driven, fail-closed) extends to cover them automatically — same `RegisterCounter`/`RegisterHistogram` pattern, same bounded-label discipline.

All v1.10 tracing spans (SPEC §28.2, 14 new spans) use the existing OTel tracer; spans nest under the v1.9 `tools/call` parent span. No new exporter or instrumentation library needed.

### Cardinality concerns flagged

The bounded-label allowlist must extend to:

| Label | Allowed values | Cardinality risk if not bounded |
|-------|----------------|----------------------------------|
| `language` | enum of 23 grammars + `unknown` | LOW (closed set) |
| `mode` (live update kind) | `change`, `create`, `delete`, `rename`, `bulk` | LOW |
| `outcome` | `success`, `timeout`, `error`, `circuit_open`, `skipped`, `degraded` | LOW |
| `projection` | `imports`, `references`, `calls`, `types`, `mixed` | LOW |
| `algorithm` | `pagerank`, `personalized_pagerank`, `weak_components`, `label_propagation` | LOW |
| `edge_kind` | enum from SPEC §12.1/§12.2 (~15 values) | LOW |
| `confidence_tier` | `high`, `medium`, `low`, `unresolved` | LOW |
| `query_kind` | enum of named query templates | **MEDIUM — must be a closed allowlist, not free-form SQL hashes** |
| `phase` | enum of registered phase names | LOW |
| `repo_state` | `idle`, `bulk_change`, `live`, `compacting` | LOW |

**Anti-cardinality rules (must encode in `internal/obs/labels.go`):**
- `tool_name` must remain restricted to the registered MCP tool set (already enforced in v1.9).
- NEVER include `repo_id`, `file_path`, `symbol_name`, `cluster_id`, `snapshot_id` as label values. Use trace span attributes for those (high-cardinality, but traces are sampled). SPEC §28.1's bounded-labels list does not include any of these — keep it that way.
- Histograms cost 10× their label cardinality (one series per bucket). Watch `helix_semantic_lsp_enrichment_duration_seconds{language}` — 23 languages × 10 buckets = 230 series, fine.

The v1.9 cardinality test (`TestMetricsBoundedCardinality` per Phase 53) must extend to the new families. Add a test fixture enumerating the allowed label combinations and assert no unbounded labels are registered.

---

## Installation / go.mod Diff (projected)

```diff
require (
+   github.com/duckdb/duckdb-go v2.10502.0
+   github.com/tiktoken-go/tokenizer v0.6.0
+   github.com/bluekeyes/go-gitdiff v0.8.0
    // ... existing v1.9 deps unchanged
)

require (
+   gonum.org/v1/gonum v0.16.0 // test-only, used in internal/semantic/.../*_test.go
)
```

**Build matrix impact:**
- `make build` (CGO=1, default): adds DuckDB static-bundle link step, ~5–10s extra compile, ~12 MB binary size increase.
- `make build-nocgo` (existing CGO=0 stub path): unchanged binary size; semantic feature returns `Unsupported` at activation.
- Goreleaser: existing 6-archive matrix already runs CGO-aware per-arch builders for tree-sitter — no new release-pipeline work.

---

## Integration with Existing Daemon Bootstrap

`internal/daemon/daemon.go` currently has 14+ steps (kernel init, skill init, middleware install, etc.). v1.10 inserts:

1. **Step 9.5 (post-kernel, pre-skill):** `semantic.NewService(deps)` — opens DuckDB at `<workspace>/.helix/semantic.duckdb`, runs migrations, validates schema version. Fail-fast on corruption (per SPEC §29.1 — but with the timestamped-`.corrupt` rename + degraded-mode advance, NOT a hard daemon abort). On CGO=0, the stub `NewService` returns a degraded-mode service that refuses semantic operations with `Kind: Unsupported`.
2. **Step 9.6:** Wire `service.LiveQueue()` into the file watcher; start the watcher goroutine under the daemon errgroup.
3. **Step 9.7:** Start the LSP revalidation worker pool (single goroutine + bounded channel, per SPEC §21).
4. **Step 9.8:** Start the idle compaction worker (single goroutine, idle-debounced, per SPEC §22).
5. **Step 13.5 (post-skill registration):** Register the 10 new semantic MCP tools (SPEC §23) via the existing `ToolProvider` skill adapter pattern.
6. **Step 14.5 (post-middleware):** Install the guardrail middleware (SPEC §24's edit-tool integration) — runs *after* `LazyInitMiddleware`, *before* the tool handler. Order: `LazyInit → Suggestion → ProfileFilter → Telemetry → **Guardrail** → handler`.
7. **Shutdown order:** semantic service shuts down BEFORE the kernel (live update queue must drain into a clean overlay state before LSP workers go away). Add to the kernel-first shutdown sequence in `daemon.Stop()`.

The pipeline DAG (ADR-010) describes exactly this ordering and is validated on startup (`phase_graph.validate_on_startup: true`). A cycle or missing dep dumps a `.dot` file and fails fast — this *replaces* the comment-driven middleware-order doc currently in `internal/mcp/lazy_init.go`.

---

## What NOT to Pull In

| Library | Why we don't want it |
|---------|----------------------|
| Any pure-Go DuckDB clone (chDB-go, etc.) | Doesn't exist in mature form; ADR-001 commits to DuckDB. |
| Heavy DAG/workflow engines (`temporalio/sdk-go`, `mvdan/sh`-style) | 80 LOC of Kahn's algorithm. |
| Embedding/vector libs (`milvus`, `chroma-go`, FAISS bindings) | Out of Scope per PROJECT.md ("Vector/embedding search — Augment Context Engine does this better"). |
| Graph DBs (`dgraph`, `neo4j-go-driver`) | ADR-001: graph DBs are optional later accelerators, not source of truth. |
| Python interop (`go-python`, gopy) | Out of Scope: native Go, no Python interop. |
| Docker / container libs | Out of Scope: single binary. |
| `pkoukk/tiktoken-go` | Network-on-first-use breaks offline; use `tiktoken-go/tokenizer`. |
| `sourcegraph/go-diff` | Parser-only; we need apply. |
| `gonum` in production code | ADR-005. |
| Recursive-watch wrappers (`rfsnotify`) | Implement in-tree on existing fsnotify. |

---

## Confidence Assessment

| Claim | Confidence | Source |
|-------|------------|--------|
| `duckdb/duckdb-go` v2.10502.0 = current, official, CGO-required | HIGH | Direct fetch from github.com/duckdb/duckdb-go README + pkg.go.dev |
| Bundled static libs cover darwin/linux × amd64/arm64 + windows/amd64 | HIGH | duckdb-go README distribution table |
| fsnotify lacks recursive watch; inotify watch-limit is real | HIGH | Multiple project issues, fsnotify own docs |
| Gonum `network.PageRank` exists, supports weighted, lacks personalized/incremental | HIGH | pkg.go.dev/gonum.org/v1/gonum/graph/network direct read |
| `tiktoken-go/tokenizer` is pure-Go with embedded vocab | MEDIUM | Project README; not directly verified at file level |
| `bluekeyes/go-gitdiff` supports apply for text + binary | MEDIUM | Project README; recent activity confirmed via libraries.io |
| ADR-005 (custom Go graph) is the right call vs gonum-in-prod | HIGH | ADR is in SPEC-DRAFT; research confirms gonum's missing capabilities (personalized, incremental) |
| CGO=0 stub policy compatible via existing Phase 51.1 pattern | HIGH | Direct read of v1.9 phase outcome in PROJECT.md + CLAUDE.md |
| Bounded-label discipline extends cleanly to new metrics | HIGH | SPEC §28.1 explicit allowlist + v1.9 PromQL validator already enforces |

## Sources

- [duckdb/duckdb-go (official, post-donation)](https://github.com/duckdb/duckdb-go)
- [marcboeker/go-duckdb (legacy, pre-donation)](https://github.com/marcboeker/go-duckdb)
- [duckdb-go on pkg.go.dev (v2)](https://pkg.go.dev/github.com/marcboeker/go-duckdb/v2)
- [go-duckdb V2 General Discussion #232](https://github.com/marcboeker/go-duckdb/discussions/232)
- [DuckDB Go Client Documentation](https://duckdb.org/docs/current/clients/go)
- [fsnotify/fsnotify](https://github.com/fsnotify/fsnotify)
- [fsnotify Issue #18: User-space recursive watcher](https://github.com/fsnotify/fsnotify/issues/18)
- [farmergreg/rfsnotify (recursive wrapper reference)](https://github.com/farmergreg/rfsnotify)
- [gonum graph/network package](https://pkg.go.dev/gonum.org/v1/gonum/graph/network)
- [gonum/gonum repository](https://github.com/gonum/gonum)
- [tiktoken-go/tokenizer (pure Go, embedded vocab)](https://github.com/tiktoken-go/tokenizer)
- [pkoukk/tiktoken-go (rejected — downloads vocab)](https://github.com/pkoukk/tiktoken-go)
- [bluekeyes/go-gitdiff](https://github.com/bluekeyes/go-gitdiff)
- [bluekeyes/go-gitdiff on Libraries.io](https://libraries.io/go/github.com%2Fbluekeyes%2Fgo-gitdiff)
- [sourcegraph/go-diff (parser-only, rejected)](https://github.com/sourcegraph/go-diff)
- [SPEC-DRAFT.md §4 ADRs, §6 Package Layout, §25 Configuration, §28 Observability](file:///Users/Janis_Vizulis/go/src/github.com/agenthands/helix/SPEC-DRAFT.md)
- [PROJECT.md v1.10 milestone declaration](file:///Users/Janis_Vizulis/go/src/github.com/agenthands/helix/.planning/PROJECT.md)
- [go.mod v1.9 dependency state](file:///Users/Janis_Vizulis/go/src/github.com/agenthands/helix/go.mod)
