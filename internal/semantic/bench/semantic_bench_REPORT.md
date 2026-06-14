# Phase 64 — Bleve Gate Report (D-08)

Date: 2026-05-07
Bleve version: v2.4.4
Host: Darwin arm64 (Apple M4 Pro, 14 logical CPUs)

## Binary size

Method: built two minimal Go programs with identical surrounding flags. The
control is `package main; func main(){}` to capture the unavoidable Go
runtime cost; the probe at `internal/semantic/bench/_blevprobe/probe.go` adds a single blank
import of `github.com/blevesearch/bleve/v2` and nothing else. The
*incremental* cost of bringing bleve into a binary's reachable graph is the
delta between the two. (Building `./cmd/helix` itself with bleve added to
go.mod produces a byte-identical binary because no helix package imports
bleve yet — verified at 153,972,994 bytes both before and after
`go get github.com/blevesearch/bleve/v2@v2.4.4`.)

| Build                                  | Bytes      | MiB     |
| -------------------------------------- | ---------: | ------: |
| empty probe (`package main; func main()`) | 1,593,618  |   1.52  |
| bleve probe (`import _ ".../bleve/v2"`)   | 10,828,658 |  10.33  |
| **delta (bleve incremental)**             |            | **8.81** |

Threshold: < 50 MiB. Result: **PASS**.

## Indexing throughput (50k symbols)

Both benchmarks run on the same 50,000-row deterministic corpus walked from
`internal/semantic/bench/fixtures/synthetic_50k_go/out/` (500 .go files × 100 declarations,
seed=42). The bench measures the full ingest envelope: bleve scorch index
build with batches of 1,000 + close; DuckDB FTS5 single-tx insert + FTS
index build. Run via:

```
go test -tags=benchfts -run=^$ \
  -bench='BenchmarkBleveIndexThroughput_50k|BenchmarkDuckDBFTSIndexThroughput_50k' \
  -benchtime=1x -timeout=15m ./internal/semantic/bench/...
```

| Backend       |        ns/op | Time (s) | Symbols/s |
| ------------- | -----------: | -------: | --------: |
| bleve scorch  | 2,331,981,542 |    2.332 |    21,440 |
| duckdb fts5   | 3,824,810,500 |    3.825 |    13,072 |
| **ratio (bleve/duckdb)** |  | **0.61x**  |           |

Threshold: bleve ≤ 5x slower than DuckDB FTS5 (i.e. ratio ≤ 5.0). Result:
**PASS** — bleve is in fact ~1.6x *faster* than DuckDB FTS5 on this
corpus, leaving ample headroom.

## Verdict

**PASS — proceed with bleve**

Subsequent Wave 1 plans (P64-07 retrieval engine) consume this verdict.
Both gate criteria from CONTEXT.md D-08 are satisfied with comfortable
headroom (binary growth 8.81 MiB / 50 MiB = 17.6% of budget; throughput
ratio 0.61x / 5.0x = 12.2% of budget). No conditional re-route to DuckDB
FTS5 inside `*Store` is required.

## Reproduction notes

- The fixture must be generated before the benchmarks run:
  `cd internal/semantic/bench/fixtures/synthetic_50k_go && go run gen.go`. The generator is
  deterministic (seed=42) so the corpus is byte-stable across machines.
- The DuckDB FTS5 baseline benchmark is gated behind the `benchfts` build
  tag because the `cmd/vet-noduckdb` analyzer forbids `duckdb-go` imports
  outside `internal/semantic/store/`. The shim
  `internal/semantic/store/bench_fts_probe.go` (also `benchfts`-gated) is
  the only consumer of the duckdb driver in the bench path; this honors
  the STORE-06 boundary while still letting the bench measure the real
  DuckDB FTS5 ingest path.
- Per project rule (`feedback_no_ci_benchmarks` memory): this benchmark is
  local-only. It is NOT wired into any CI workflow. Re-run by hand on a
  representative dev machine when bleve is bumped.
