---
phase: 64-new-mcp-tools
plan: 01
subsystem: infra
tags: [bleve, duckdb, fts5, benchmark, semantic-index, gate]

# Dependency graph
requires:
  - phase: 63-compaction-retention
    provides: snapshot-write API + compaction gate (consumed by Wave 1 retrieval engine)
provides:
  - Deterministic 50k-symbol Go fixture generator (seed=42, byte-stable across runs)
  - Bleve scorch indexing throughput benchmark
  - DuckDB FTS5 indexing throughput benchmark (build-tag gated to honor noduckdb)
  - PASS verdict committed in bench/semantic_bench_REPORT.md — bleve cleared
    both gates (binary growth 8.81 MiB / 50 MiB; throughput ratio 0.61x / 5x)
  - bleve v2.4.4 dependency pinned in go.mod / go.sum
affects: [64-07-retrieval-engine, 67-eval-harness]

# Tech tracking
tech-stack:
  added:
    - github.com/blevesearch/bleve/v2 v2.4.4 (Apache-2.0; scorch backend)
  patterns:
    - "benchfts build tag isolates duckdb-go consumers in bench paths so cmd/vet-noduckdb is satisfied"
    - "//go:build neverbuild on generated fixture .go files keeps them out of the module compile graph"

key-files:
  created:
    - bench/fixtures/synthetic_50k_go/gen.go
    - bench/fixtures/synthetic_50k_go/README.md
    - bench/fixtures/synthetic_50k_go/.gitignore
    - bench/_blevprobe/probe.go
    - bench/semantic_bench_test.go
    - bench/semantic_bench_fts_test.go
    - bench/semantic_bench_REPORT.md
    - internal/semantic/store/bench_fts_probe.go
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "Bleve incremental binary cost measured via empty-vs-bleve probe pair (cleanest apples-to-apples) instead of helix-with-bleve vs helix-without; helix itself cannot detect bleve until a helix package imports it (Wave 1)."
  - "DuckDB FTS5 baseline kept inside internal/semantic/store/ via a benchfts-tagged shim to honor STORE-06 / cmd/vet-noduckdb. Bench file imports the shim, never duckdb-go directly."
  - "Generated fixture files carry //go:build neverbuild so go vet ./... and go build ./... ignore them (random naming produces identifier collisions that aren't a real problem — the corpus is read-only data)."
  - "Bench IDs include source line number so DuckDB's PRIMARY KEY constraint is satisfied; bleve doesn't care but the unified ID shape keeps both backends comparing apples-to-apples."

patterns-established:
  - "Wave-0 gate benchmark pattern: empirical measurement → REPORT.md verdict → orchestrator reads at Wave 1 entry to confirm or re-route"
  - "STORE-06 boundary preservation in bench code via benchfts build tag + in-store shim"

requirements-completed: [TOOL-04]

# Metrics
duration: ~22min
completed: 2026-05-08
---

# Phase 64 Plan 01: Bleve Gate Benchmark Summary

**Bleve v2.4.4 cleared both D-08 gates (binary growth 8.81 MiB, indexing 0.61x DuckDB FTS5) — PASS verdict committed; Wave 1 P64-07 retrieval engine proceeds with bleve, no DuckDB FTS5 fallback.**

## Performance

- **Duration:** ~22 min
- **Started:** 2026-05-08T00:00:00Z
- **Completed:** 2026-05-08T00:22:00Z (approximate)
- **Tasks:** 2 of 2
- **Files modified:** 10 (8 created, 2 modified — go.mod / go.sum)

## Accomplishments

- Deterministic 50k-symbol Go fixture (500 files × 100 decls; 25k funcs + 25k types) with byte-stable sha256 across runs.
- Apples-to-apples binary-size measurement: empty Go probe (1.52 MiB) vs bleve probe (10.33 MiB) → **8.81 MiB incremental cost**, 17.6% of 50 MiB budget.
- Apples-to-apples ingest-throughput measurement on the same 50k corpus: **bleve 21,440 sym/s** vs DuckDB FTS5 13,072 sym/s; bleve is 1.6x *faster* (ratio 0.61x, well inside the ≤5x ceiling).
- Verdict report (`bench/semantic_bench_REPORT.md`) committed and machine-readable — opens with `# Phase 64 — Bleve Gate Report (D-08)` and ends with the verbatim `**PASS — proceed with bleve**` string the plan's grep verifier expects.
- bleve v2.4.4 pinned in go.mod / go.sum; `cmd/vet-noduckdb` boundary preserved via benchfts-tagged shim inside `internal/semantic/store/`.

## Task Commits

1. **Task 1: deterministic 50k-symbol fixture generator** — `e811fa3d` (feat)
2. **Task 2: bleve gate benchmark + verdict** — `674cc64d` (feat)

## Files Created/Modified

- `bench/fixtures/synthetic_50k_go/gen.go` — `//go:build ignore` generator; emits 500 fixture .go files from seed=42 with inline 200-word lowercase wordlist; each generated file carries `//go:build neverbuild` so the go toolchain ignores it.
- `bench/fixtures/synthetic_50k_go/README.md` — pins the determinism contract (seed, 50000, deterministic — exact strings the plan grep-checks).
- `bench/fixtures/synthetic_50k_go/.gitignore` — excludes `out/` so the regenerable fixture tree never ships in commits.
- `bench/_blevprobe/probe.go` — minimal `import _ ".../bleve/v2"` main used to measure bleve's incremental linker footprint.
- `bench/semantic_bench_test.go` — corpus walker (`loadCorpus`) + `BenchmarkBleveIndexThroughput_50k` (scorch index, batches of 1k); ships compile-only `TestNothing` so `go test ./bench/...` is green in default builds.
- `bench/semantic_bench_fts_test.go` — `benchfts`-tagged `BenchmarkDuckDBFTSIndexThroughput_50k` calling the in-store shim.
- `bench/semantic_bench_REPORT.md` — verdict (PASS), measurement tables, reproduction notes, project-rule reminder that benchmarks remain local-only per `feedback_no_ci_benchmarks` memory.
- `internal/semantic/store/bench_fts_probe.go` — `benchfts`-tagged `BenchSymbolDoc` + `BenchDuckDBFTSIngest`; the only consumer of `duckdb-go` in the bench path; satisfies `cmd/vet-noduckdb` (STORE-06).
- `go.mod` / `go.sum` — pin `github.com/blevesearch/bleve/v2 v2.4.4`.

## Decisions Made

- **Probe-vs-empty for binary size, not helix-vs-helix.** Building `./cmd/helix` before vs after `go get bleve` produces byte-identical binaries (153,972,994 bytes both times) because no helix package imports bleve until Wave 1. The actually-meaningful measurement of bleve's marginal linker cost is a minimal probe binary that imports bleve and nothing else, compared against a minimal probe that imports nothing. That is the comparison the REPORT documents.
- **DuckDB FTS5 baseline lives in-store via build tag.** The plan suggested calling `internal/semantic/store.Open(...)` directly from the bench file, but the store's `Open` is a heavyweight workspace-bound constructor that doesn't expose a generic FTS5 path, and the bench file cannot import `duckdb-go` (cmd/vet-noduckdb / STORE-06). Resolution: a `benchfts`-tagged shim `BenchDuckDBFTSIngest` lives inside `internal/semantic/store/` (the only allowlisted package), opens DuckDB via `database/sql.Open("duckdb", ...)`, installs+loads the FTS extension, ingests, and builds the FTS index. Bench file calls the shim. Boundary preserved.
- **Generated fixture files carry `//go:build neverbuild`.** `go vet ./bench/...` initially errored on duplicate identifiers in the generated tree because each 100-decl file draws names from a 200-word list with replacement, producing collisions. Rather than complicate name generation (which would break determinism), the generator now emits `//go:build neverbuild` on every fixture file so the go toolchain skips them entirely. Bench code reads them as raw bytes via `filepath.WalkDir`.
- **Per-row IDs include source line number.** DuckDB's `PRIMARY KEY` rejected duplicates from the random-name corpus. Adding the line number to the ID disambiguates without changing the underlying corpus content; bleve doesn't require this but uses the same ID shape so both backends compare like-for-like.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] generated fixture files broke `go vet ./bench/...`**
- **Found during:** Task 2 (running `go vet ./bench/...` per the plan's verify command)
- **Issue:** Generated `out/pkg{0..49}/file{0..9}.go` files contained duplicate exported identifiers (random names with collisions across a 200-word pool), so `go vet` rejected them as ill-formed packages. The plan's verify step requires `go vet ./bench/... exits 0`.
- **Fix:** Edited `bench/fixtures/synthetic_50k_go/gen.go` to emit `//go:build neverbuild` at the top of every generated file. Regenerated the fixture; sha256 changed (expected, since the file content changed) but determinism is preserved (re-run twice → identical sha). The generator's intentional change is documented in the file header comment and in `bench/semantic_bench_REPORT.md`.
- **Files modified:** `bench/fixtures/synthetic_50k_go/gen.go`
- **Verification:** `go vet ./bench/...` now exits 0; `go vet -tags=benchfts ./bench/... ./internal/semantic/store/...` also exits 0; bench tests still find and parse the fixture (the random `func`/`type` lines are stable; the new build-tag header is just a leading comment block).
- **Committed in:** `674cc64d` (Task 2 commit)

**2. [Rule 3 - Blocking] DuckDB FTS5 PRIMARY KEY collisions on random-name corpus**
- **Found during:** Task 2 (first DuckDB FTS bench run)
- **Issue:** `BenchSymbolDoc.ID` was constructed as `<rel>::<name>` from the deterministic random corpus, which produced identifier collisions across the 100-decl-per-file × 200-word-pool space. DuckDB rejected the second insert with a primary-key violation; bleve had quietly upserted on the same key.
- **Fix:** Edit `bench/semantic_bench_test.go::loadCorpus` to include the source line number in the ID (`<rel>::<lineNo>::<name>`). Disambiguates uniquely; both backends now ingest 50000 distinct rows, making the throughput numbers directly comparable.
- **Files modified:** `bench/semantic_bench_test.go`
- **Verification:** Both benchmarks ran to completion in the same `go test` invocation; `BenchmarkDuckDBFTSIndexThroughput_50k` succeeded; `BenchmarkBleveIndexThroughput_50k` finished with the same row count.
- **Committed in:** `674cc64d` (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (2 blocking)
**Impact on plan:** Both fixes were necessary to satisfy the plan's own automated verify step. No scope creep — both stayed inside the two task files the plan already names.

## Issues Encountered

- `go mod tidy` failed mid-run on an unrelated upstream package issue (`github.com/go-openapi/testify/v2/assert/yaml` not present in v2.5.0). Did not block: bleve was already added cleanly via `go get github.com/blevesearch/bleve/v2@v2.4.4` and is fully present in go.mod / go.sum. Tidy can be re-run when the unrelated upstream issue is sorted; not in 64-01's scope.
- `go build ./...` reports pre-existing C compile errors in `tmp/GitNexus/...` and `tmp/graphify/...` fixture trees. These are external test fixtures unrelated to this plan; not addressed (Rule 3 boundary — out of scope).

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- **Wave 1 P64-07 retrieval engine** can proceed with bleve immediately. The verdict in `bench/semantic_bench_REPORT.md` is the orchestrator-readable signal at Wave 1 entry.
- **Phase 67 evaluation harness** can reuse `bench/fixtures/synthetic_50k_go/` as a stable corpus once it lands.
- **No blockers.** No conditional re-route to DuckDB FTS5 inside `*Store`.

## Self-Check: PASSED

Verification steps:
- `bench/fixtures/synthetic_50k_go/gen.go` — FOUND
- `bench/fixtures/synthetic_50k_go/README.md` — FOUND
- `bench/fixtures/synthetic_50k_go/.gitignore` — FOUND
- `bench/_blevprobe/probe.go` — FOUND
- `bench/semantic_bench_test.go` — FOUND
- `bench/semantic_bench_fts_test.go` — FOUND
- `bench/semantic_bench_REPORT.md` — FOUND (contains `**PASS — proceed with bleve**`)
- `internal/semantic/store/bench_fts_probe.go` — FOUND
- Commit `e811fa3d` (Task 1) — FOUND in `git log`
- Commit `674cc64d` (Task 2) — FOUND in `git log`
- `go vet ./bench/...` — exit 0
- `go vet -tags=benchfts ./bench/... ./internal/semantic/store/...` — exit 0
- Default `go test ./bench/... -count=1 -run=TestNothing -timeout=30s` — exit 0
- Both benchmarks ran to completion with `b.N=1`; numbers recorded in REPORT.md.

---
*Phase: 64-new-mcp-tools*
*Completed: 2026-05-08*
