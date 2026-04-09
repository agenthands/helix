# Phase 08 Deferred Items

Out-of-scope issues discovered while executing phase 08 plans. NOT fixed in
plan execution because they pre-date the plan's touched files and fixing them
would violate the scope boundary rule.

## From 08-03 (concurrency strategy)

### DEFERRED-01: Data race in `internal/kernel/jsonrpc.TestConn_Call`

- **Discovered:** 2026-04-08 during `go test -race ./... -count=1` regression
  run for plan 08-03.
- **Symptom:** `FAIL: TestConn_Call — race detected during execution of test`
- **Scope:** Pre-existing race in the jsonrpc codec test, unrelated to the
  worker pool concurrency work covered by plan 08-03.
- **Action:** Log for a future hardening plan. Do NOT block 08-03 on it.
  The race is in a goroutine started inside the test itself
  (`codec_test.go:198`), not in code paths exercised by 08-03's three-tier
  concurrency strategy.
- **Suggested follow-up:** Small fix-up plan to audit jsonrpc.Conn test
  helpers for missing synchronization (likely a write from the test goroutine
  that needs a channel ack before the main test goroutine reads).
