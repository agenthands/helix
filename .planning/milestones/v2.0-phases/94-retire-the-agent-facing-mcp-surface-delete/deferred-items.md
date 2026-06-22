# Deferred / Out-of-Scope Items — Phase 94

## Pre-existing flaky test (NOT caused by 94-01)

- **`internal/guardrails` `TestNewReceiptID/consecutive_IDs_are_monotonically_non-decreasing`**
  — intermittently FAILs under the full-tree `go test ./...` run (parallel load),
  but passes deterministically when the package or the test is run alone
  (`go test ./internal/guardrails/ -count=1` is green on rerun). Root cause is a
  timing/monotonicity assumption in the ULID-style receipt-ID generator under
  concurrent scheduling — unrelated to this plan, which touches no files in
  `internal/guardrails`. Logged per the executor scope boundary; not fixed here.

## Pre-existing flaky test (NOT caused by 94-02)

- **`internal/obs` `TestConnCallProducesLspoolSpan`** — intermittently FAILs
  under the full-tree `go test ./...` run (parallel load) but passes
  deterministically in isolation (`go test ./internal/obs/ -count=1` green 3/3 on
  rerun). Same class as the `internal/guardrails` flake above (a span/timing
  assertion under concurrent scheduling). 94-02 touches no files in `internal/obs`
  and the test references none of the deleted symbols. Logged per the executor
  scope boundary; not fixed here.

## Pre-existing env-dependent failures (NOT caused by 94-02)

- **`test/oracle/scenario` (`TestScenario_Collision_NoCrossContamination`,
  `TestScenario_Polyglot_NoCrossLanguageContamination`) and
  `test/oracle/runtime`** — fail under `-tags integration` with gopls
  cross-file/LS-readiness errors ("(no results)", "LS readiness timeout"). Verified
  PRE-EXISTING: `TestScenario_Collision_NoCrossContamination` fails identically on
  the pre-deletion baseline `c61dbca0` (94-01). These reference none of the deleted
  symbols — they are LSP/gopls environment failures (per the project's known
  integration-tagged env-dependent LS-fixture failures). The transport gate the
  plan specifies (`./test/oracle/protocol/...`) is fully GREEN. Not fixed here.
