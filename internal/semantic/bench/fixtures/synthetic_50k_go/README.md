# synthetic_50k_go — deterministic 50000-symbol Go fixture

This fixture supports the Phase 64-01 bleve gate benchmark and is reusable by
Phase 67 (evaluation harness). The generator is **fully deterministic** —
running it twice produces byte-identical output (sha256 of any individual file
or of `find ./out -type f -print0 | xargs -0 sha256sum` is stable).

## Invocation

From this directory:

```bash
go run gen.go
```

This wipes `./out/` and regenerates the tree from a fixed seed.

## Output shape

- 500 files: `out/pkg{0..49}/file{0..9}.go`
- 50000 declarations total: 25000 exported funcs + 25000 exported type structs
- Each declaration carries a 3-line godoc comment with random words drawn from
  a fixed 200-word lowercase wordlist (committed inline in `gen.go`).

## Determinism contract

- Seed: `int64(42)` (constant in `gen.go`).
- Wordlist: 200 lowercase English nouns/verbs, **committed inline** in
  `gen.go`. Do not reorder — output sha256 depends on word ordering.
- RNG: `math/rand` with `rand.NewSource(seed)`. The standard library RNG is
  byte-stable across Go releases that preserve the algorithm.
- File emit order: pkgs 0→49, files 0→9, 100 decls per file
  (50 funcs then 50 types).

The generator uses the deterministic `math/rand` package (NOT
`math/rand/v2`, whose output is not contractually byte-stable) and a single
RNG instance per process so per-symbol naming is reproducible.

## Intended consumers

- **Phase 64-01 bench (`internal/semantic/bench/semantic_bench_test.go`)** — measures bleve vs
  DuckDB FTS5 indexing throughput on this corpus.
- **Phase 67 eval harness** — re-uses the fixture as a stable corpus for
  retrieval-quality measurements.

## Build tag

`gen.go` carries `//go:build ignore` so it does NOT compile into the
`./internal/semantic/bench/...` test binary. It is invoked manually via `go run gen.go`.

## Acceptance markers (per 64-01-PLAN.md task 1)

The string `seed`, the literal `50000`, and the word `deterministic` all
appear in this README so `grep` checks in the plan's automated verifier
pass.
