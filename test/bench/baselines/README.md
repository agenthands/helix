# `test/bench/baselines/` — local benchmark baselines

This directory is the conventional capture location for **local** benchmark
runs. It is intentionally empty in the repo (apart from this README) —
captured baselines are personal scratch artifacts and are **gitignored**.

Benchmarks run locally only. The project does not run benchmarks on CI
runners because shared GitHub-hosted runners produce noisy, untrustable
baselines. See `CONTRIBUTING.md` → "Running Benchmarks" for the full
rationale.

## Capturing a baseline

From the repo root:

```sh
make bench-baseline
```

This runs the bench suite once and writes the benchstat-formatted output
to `test/bench/baselines/local.txt` (overwrites on each run). The path is
listed in `.gitignore` so the file never enters the working tree.

## Comparing two captures

Install upstream `benchstat` and invoke it directly:

```sh
go install golang.org/x/perf/cmd/benchstat@latest
benchstat old.txt new.txt
```

Save copies under any name you like (e.g. `cp local.txt before.txt`,
re-run `make bench-baseline`, then `benchstat before.txt local.txt`) —
everything in this directory other than this README is gitignored.
