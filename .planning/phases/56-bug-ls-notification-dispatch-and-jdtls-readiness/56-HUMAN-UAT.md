---
status: complete
phase: 56-bug-ls-notification-dispatch-and-jdtls-readiness
source: [56-VERIFICATION.md]
started: 2026-04-25T00:00:00Z
updated: 2026-04-26T00:00:00Z
---

## Current Test

[testing complete]

## Tests

### 1. CI run of full Java fixture suite (`TestSymbols_JavaFixture`, `TestEdit_JavaFixture`)
expected: Both Java fixture tests pass on CI with a fresh jdtls install (CI is authoritative for JDTLS-RDY-02). Local-machine failures are pre-existing — verified via `git stash` at base commit `088b072f` reproducing identical failure mode (~2.3s, "no results"). The `waitJavaReady` gate itself fires correctly (both readiness gates close in ~2s).
result: issue
reported: |
  CI run on PR #1 (run 24959139216, 12m46s, ubuntu-latest, fresh jdtls install) shows both Java fixture tests FAILING.
  - TestSymbols_JavaFixture: FAIL (240.02s) — `java_test.go:49: LS readiness timeout after 4m0s (last: call error: context deadline exceeded)`
  - TestEdit_JavaFixture: timed out at 6m0s (panic: test timed out after 10m0s)
  - TestEdit_JavaFixture/rename: timed out at 2m0s
  Total integration package: FAIL after 600s.
  This falsifies the JDTLS-RDY-02 claim that the readiness gate makes CI green. The waitJavaReady gate itself may be firing locally in ~2s, but on a fresh CI runner JDTLS is not reaching ready state within the 4m budget — the gate times out before symbols become resolvable.
severity: major

## Summary

total: 1
passed: 0
issues: 1
pending: 0
skipped: 0
blocked: 0

## Gaps

- truth: "Both Java fixture tests pass on CI with a fresh jdtls install (JDTLS-RDY-02)"
  status: failed
  reason: "CI run 24959139216 on PR #1: TestSymbols_JavaFixture FAIL (LS readiness timeout 4m0s, context deadline exceeded); TestEdit_JavaFixture panic test timed out after 10m0s. Readiness gate is not closing within the 4-minute budget on fresh CI jdtls install."
  severity: major
  test: 1
  artifacts:
    - "https://github.com/postfix/serena/pull/1"
    - "https://github.com/postfix/serena/actions/runs/24959139216"
  missing: []
