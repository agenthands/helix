---
status: complete
phase: 56-bug-ls-notification-dispatch-and-jdtls-readiness
source: [56-VERIFICATION.md]
started: 2026-04-25T00:00:00Z
updated: 2026-04-27T00:00:00Z
re_verified: 2026-04-27T00:00:00Z
---

## Current Test

[testing complete]

## Tests

### 1. CI run of full Java fixture suite (`TestSymbols_JavaFixture`, `TestEdit_JavaFixture`)
expected: Both Java fixture tests pass on CI with a fresh jdtls install (CI is authoritative for JDTLS-RDY-02). Local-machine failures are pre-existing — verified via `git stash` at base commit `088b072f` reproducing identical failure mode (~2.3s, "no results"). The `waitJavaReady` gate itself fires correctly (both readiness gates close in ~2s).
result: pass
evidence: |
  CI run 24984110102 on branch gsd/phase-50-toolchain-go1.25-gopls-ci (2026-04-27 08:16, ubuntu-latest, fresh jdtls cache miss confirmed):
  - `ok  github.com/postfix/serena/test/integration  47.361s`
  - `ok  github.com/postfix/serena/test/integration/jdtlscache  0.007s`
  - Cache lookup: `Cache not found for input keys: jdtls-warm-Linux-...` (cold-start path exercised)
  - Java 21 (Temurin) configured per fix commit 84506d8d
  - LS-readiness budget raised per 9bd5345b
  - Three integration bugs uncovered by readiness gate fixed in faa3c2fc

  Original failure mode (LS readiness timeout 4m0s; 10m panic) no longer reproduces.
  JDTLS-RDY-02 satisfied on CI with fresh jdtls install.
re_verified: 2026-04-27
prior_result: issue (CI run 24959139216, resolved by faa3c2fc + 84506d8d + 9bd5345b)

## Summary

total: 1
passed: 1
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none — all resolved 2026-04-27]

## Re-verification Note

Initial UAT on 2026-04-25 logged a `major` issue: CI Java fixtures timed out under the original readiness gate. Three follow-up commits on 2026-04-26/27 (`faa3c2fc`, `84506d8d`, `9bd5345b`) addressed the root causes: jdtls indexing budget, Java toolchain version on CI, and three latent integration bugs the working readiness gate finally exposed. Today's CI run (24984110102) confirms Java fixtures pass in 47s on a cold jdtls cache. A separate CI failure on the legacy Python `Tests` workflow (which had been masking the result) was unrelated to phase 56 and was removed in commit `36289554`.
