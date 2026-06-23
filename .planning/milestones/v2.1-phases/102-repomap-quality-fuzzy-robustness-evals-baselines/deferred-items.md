# Deferred Items — Phase 102

Out-of-scope discoveries logged during execution (NOT fixed by the discovering plan).

## 102-01: pre-existing cmd/helix-bench failures (out of scope)

Discovered during the Plan 102-01 full-suite regression gate (`go test ./...`).
NOT caused by this plan — 102-01 added `bench/evaluators/repomapeval/` + an
ignore-tagged regenerator and never touched `cmd/helix-bench`. Both reproduce on
the pre-plan baseline tree.

- `TestRunSubcommandWiresDeltaPass` (`cmd/helix-bench/run_cmd_test.go:144`):
  real-mode rows missing `ablation_deltas` ("delta pass not wired into runBench").
  A pre-existing ablation-delta wiring gap in the helix-bench `run` subcommand.
- `cmd/helix-bench` dataset-fetch tests: crosscodeeval / repobench parquet
  fetches return HTTP 404 (network-gated; the upstream HF dataset revisions
  appear moved/removed). Environment/network-dependent, not a code regression.

Scope: the authoritative hermetic gate for this plan
(`go test ./bench/evaluators/repomapeval/`, no binary, no network) is GREEN, as
is `go vet ./bench/...` and `gofmt -l`.

## 102-02: same pre-existing cmd/helix-bench failure (out of scope)

Re-confirmed during the Plan 102-02 full-suite regression gate. 102-02 added the
disjoint `bench/evaluators/fuzzyrobust/` leaf + an ignore-tagged
`bench/runtime/fuzzy_robust_capture_regen.go` harness and never touched
`cmd/helix-bench`. `TestRunSubcommandWiresDeltaPass` fails identically on the
102-02 baseline commit (`e743a0c8~1`), proving it is pre-existing and unrelated.
The authoritative hermetic gate for this plan
(`go test ./bench/evaluators/fuzzyrobust/`, no binary, no network) is GREEN, as
is `go vet ./bench/...` and `gofmt -l`.
