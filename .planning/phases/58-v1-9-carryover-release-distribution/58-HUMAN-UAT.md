---
status: complete
phase: 58-v1-9-carryover-release-distribution
source: [58-VERIFICATION.md]
started: 2026-05-03T19:13:11Z
updated: 2026-05-04T11:59:00Z
---

## Current Test

[all tests passed]

## Tests

### 1. Push v1.10.0-rc1 (or v1.10.0) git tag and verify CI release workflow runs end-to-end
expected: `.github/workflows/release.yml` completes: cosign-installer pinned SHA loads, GitHub Actions OIDC mints Fulcio cert, cosign sign-blob produces `.sigstore.json` bundle for every archive, GitHub release uploads all archives + bundles + checksums.txt + checksums.txt.sigstore.json.
result: pass — verified against v1.10.0-rc1 on 2026-05-04 (workflow run 25310013651, 4m45s; release tag at commit df02da5e). 14 assets present, all sigstore.json bundles in proto schema with RFC3161 timestamps. Required four post-rc1 fixes to close: WR-07 (reproducibility-gate Linux drift), WR-08 (--new-bundle-format), WR-09 (TSA timestamp required by verifier), WR-10 (use sigstore public-good TSA + refresh trust root from live TUF).

### 2. Run `helix upgrade` against the real published v1.10.0 release artifact
expected: `helix upgrade` succeeds end-to-end against the real v1.10.0 release; the binary is atomically swapped (`syscall.Exec` on Unix or `os.Exit(0)` on Windows). Verifier accepts the production trust root and rejects any tampered or wrong-identity bundle.
result: pass — verified against v1.10.0-rc1 on 2026-05-04. Built fake-old `v1.9.999-dev` helix from current source, ran `helix upgrade --version v1.10.0-rc1 --prerelease`. Dry-run succeeded ("downloaded + verified v1.10.0-rc1"); real upgrade swapped binary atomically (152MB dev → 26MB release, `--version` reports `1.10.0-rc1`); stage dir cleaned post-swap (WR-05 verified). Negative test: 1-byte corruption of archive rejected by both cosign CLI and sigstore-go verifier with `invalid signature when validating ASN.1 encoded signature`, exit 1 from both. Three independent verify paths agreed end-to-end: cosign verify-blob CLI, sigstore-go strict in-process verifier, helix upgrade.

## Summary

total: 2
passed: 2
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

- truth: "REL-02, REL-03, REL-04 are explicitly recorded as won't-do under Phase 58 D-01 across PROJECT.md / REQUIREMENTS.md / v1.10-ROADMAP.md / CONTRIBUTING.md (SC-4)"
  status: failed
  reason: "REQUIREMENTS.md, v1.10-ROADMAP.md, and CONTRIBUTING.md updated; PROJECT.md was deliberately deferred to a post-phase update_project_md step that has not yet run."
  remedy: "Either let the post-phase update_project_md step evolve PROJECT.md (auto), or run /gsd-plan-phase 58 --gaps to plan explicit PROJECT.md edits."
