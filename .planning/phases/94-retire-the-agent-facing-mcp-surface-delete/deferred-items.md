# Deferred / Out-of-Scope Items — Phase 94

## Pre-existing flaky test (NOT caused by 94-01)

- **`internal/guardrails` `TestNewReceiptID/consecutive_IDs_are_monotonically_non-decreasing`**
  — intermittently FAILs under the full-tree `go test ./...` run (parallel load),
  but passes deterministically when the package or the test is run alone
  (`go test ./internal/guardrails/ -count=1` is green on rerun). Root cause is a
  timing/monotonicity assumption in the ULID-style receipt-ID generator under
  concurrent scheduling — unrelated to this plan, which touches no files in
  `internal/guardrails`. Logged per the executor scope boundary; not fixed here.
