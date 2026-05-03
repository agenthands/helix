---
status: partial
phase: 58-v1-9-carryover-release-distribution
source: [58-VERIFICATION.md]
started: 2026-05-03T19:13:11Z
updated: 2026-05-03T19:13:11Z
---

## Current Test

[awaiting human testing]

## Tests

### 1. Push v1.10.0-rc1 (or v1.10.0) git tag and verify CI release workflow runs end-to-end
expected: `.github/workflows/release.yml` completes: cosign-installer pinned SHA loads, GitHub Actions OIDC mints Fulcio cert, cosign sign-blob produces `.sigstore.json` bundle for every archive, GitHub release uploads all archives + bundles + checksums.txt + checksums.txt.sigstore.json.
result: [pending]

### 2. Run `helix upgrade` against the real published v1.10.0 release artifact
expected: `helix upgrade` succeeds end-to-end against the real v1.10.0 release; the binary is atomically swapped (`syscall.Exec` on Unix or `os.Exit(0)` on Windows). Verifier accepts the production trust root and rejects any tampered or wrong-identity bundle.
result: [pending]

## Summary

total: 2
passed: 0
issues: 0
pending: 2
skipped: 0
blocked: 0

## Gaps

- truth: "REL-02, REL-03, REL-04 are explicitly recorded as won't-do under Phase 58 D-01 across PROJECT.md / REQUIREMENTS.md / v1.10-ROADMAP.md / CONTRIBUTING.md (SC-4)"
  status: failed
  reason: "REQUIREMENTS.md, v1.10-ROADMAP.md, and CONTRIBUTING.md updated; PROJECT.md was deliberately deferred to a post-phase update_project_md step that has not yet run."
  remedy: "Either let the post-phase update_project_md step evolve PROJECT.md (auto), or run /gsd-plan-phase 58 --gaps to plan explicit PROJECT.md edits."
