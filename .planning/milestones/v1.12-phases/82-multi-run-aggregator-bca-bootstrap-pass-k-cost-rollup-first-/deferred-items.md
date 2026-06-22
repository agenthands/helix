# Phase 82 — Deferred / Out-of-Scope Items

Items discovered during execution that are NOT part of the current plan's scope.
Per the GSD SCOPE BOUNDARY rule, these are logged here and NOT fixed inline.

## Discovered during 82-07 (aggregate subcommand)

### `TestRunSubcommandWiresDeltaPass` is a pre-existing RED failure

- **File:** `cmd/helix-bench/run_cmd_test.go:144`
- **Symptom:** Under `go test ./...` (with a `helix` binary resolvable), the
  test fails: each real-mode `result.v2.json` row is "missing ablation_deltas
  (delta pass not wired into runBench)". The run itself reports `12/12 cells
  succeeded`; only the post-run `ablation_deltas` write-back assertion fails.
- **Root cause (out of scope):** the test was added as the Phase 80-05 "RED"
  half ("test(80-05): five-of-six smoke + runBench delta-pass wiring (RED)",
  commit `05db8792`); its GREEN (wiring the delta pass write-back into
  `runBench`) has not landed. It is the `bench/runtime` delta write-back
  feature, unrelated to the Phase 82 aggregator.
- **Proof it is pre-existing:** the identical failure reproduces on
  `4ed87ea7~1` (the commit BEFORE 82-07's first commit) in a clean worktree —
  82-07 touches only `cmd/helix-bench/aggregate.go`,
  `cmd/helix-bench/aggregate_test.go`, and the `main.go`/`main_test.go`
  registration + help-count assertion; it does not touch `runBench`, the
  daemon, or the delta pass.
- **Note:** without a `helix` binary on PATH the same test SKIPs with
  "helix binary not resolvable", which is why `go test ./...` is green in a
  bare environment but red once `helix` is built/on PATH — a HELIX_BIN
  false-green of the opposite polarity.
- **Disposition:** DEFER to a Phase-80 delta-pass GREEN follow-up. Do not fix
  in Phase 82.
