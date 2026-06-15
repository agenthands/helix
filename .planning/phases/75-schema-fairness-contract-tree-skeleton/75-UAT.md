---
status: passed
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
awaiting: none — resolved

## Tests

### 1. Confirm TOS attestation flags against live provider Terms of Service
expected: Each provider block's benchmarking_permitted / publish_permitted flags match the live TOS; attested_by names a real maintainer.
result: [passed]
note: |
  VERIFIED BY USER 2026-06-15. The user confirmed the TOS attestation flags
  (benchmarking_permitted / publish_permitted) for the provider blocks in bench/PROVIDERS.md.
  The single human-only verification item is satisfied; verify-tos continues to machine-check
  freshness + parseability on every run.

## Summary

total: 1
passed: 1
issues: 0
pending: 0
skipped: 0
blocked: 0
deferred: 0

## Gaps

None — all verification items resolved (the TOS legal-accuracy attestation was confirmed by the user).
