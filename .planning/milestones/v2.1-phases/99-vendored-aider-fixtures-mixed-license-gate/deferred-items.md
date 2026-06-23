# Phase 99 — Deferred Items (out of scope, pre-existing)

These `cmd/helix-bench` test failures are PRE-EXISTING (fail identically on the
baseline commit `9ce7d83d~1`, before Plan 99-02 work). They are unrelated to the
license gate and were explicitly flagged out-of-scope by the 99-02 plan. NOT
fixed by this plan.

- `TestRunSubcommandWiresDeltaPass` — "delta pass not wired into runBench"
  (ablation_deltas missing in real-mode rows). Pre-existing runBench wiring gap.
- `crosscodeeval {python,java,typescript,csharp}` — HTTP 404 fetching parquet
  from huggingface.co (network-dependent dataset fetch).
- `repobench {python,java}` — HTTP 404 fetching parquet (network-dependent).
- `fetch-datasets` / `report --run-id ""` — downstream of the above network
  fetch failures.

Scope of the 99-02 green check: the `VerifyLicenses*` tests + `go vet` +
`make verify-licenses` (all GREEN).
