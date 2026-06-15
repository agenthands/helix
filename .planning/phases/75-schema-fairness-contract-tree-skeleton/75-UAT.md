---
status: testing
phase: 75-schema-fairness-contract-tree-skeleton
source: [75-VERIFICATION.md]
started: 2026-06-15
updated: 2026-06-15
---

## Current Test

number: 1
name: Confirm bench/PROVIDERS.md TOS attestation flags against live provider Terms of Service
expected: |
  For each provider block (Anthropic, OpenAI, Google, Local / self-hosted), the live
  Terms of Service actually permits benchmarking (benchmarking_permitted) and publishing
  results (publish_permitted) as stated; attested_by names the maintainer making the
  attestation.
awaiting: user response

## Tests

### 1. Confirm TOS attestation flags against live provider Terms of Service
expected: Each provider block's benchmarking_permitted / publish_permitted flags match the live TOS; attested_by names a real maintainer.
result: [deferred]
note: |
  DEFERRED BY EXPLICIT USER DECISION (2026-06-15, during 75-02 checkpoint): "benchmarking_permitted
  publish_permitted for all the providers and local models by default, we deal with TOS in the future
  now, we just need a fully working provider-independent system." All flags were set to permissive
  defaults (true) with an in-file note in bench/PROVIDERS.md stating these are permissive defaults
  NOT yet verified against live TOS. Live-TOS legal verification is deferred to a future phase.
  verify-tos machine-checks freshness + parseability only, not legal correctness.

## Summary

total: 1
passed: 0
issues: 0
pending: 0
skipped: 0
blocked: 0
deferred: 1

## Gaps

None — the single human item (TOS legal-accuracy attestation) was explicitly deferred by the user;
it is tracked here for a future TOS-verification pass, not an unresolved gap in Phase 75's scope.
