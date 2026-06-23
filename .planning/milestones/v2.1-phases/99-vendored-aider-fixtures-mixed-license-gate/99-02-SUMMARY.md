---
phase: 99-vendored-aider-fixtures-mixed-license-gate
plan: 02
subsystem: bench-licensing-gate
tags: [licensing, supply-chain, tamper-detection, sha256, dual-disposition, anti-vacuity]
requires:
  - "99-01: vendored fixtures/ tree (150 files) + VENDOR-MANIFEST.md (real sha256) + dual-disposition LICENSE-AUDIT.md"
provides:
  - "Dual-disposition (MIT + Apache-2.0) license gate over the full vendored tree"
  - "Bidirectional manifest-vs-disk crypto/sha256 walk (tamper/missing/extra detection)"
  - "Anti-vacuity tamper test proving RED on every sabotage class + a GREEN good-tree control"
affects:
  - "make verify-licenses (CI hard-fail build gate)"
tech-stack:
  added: []
  patterns:
    - "Extend, not rebuild: reused splitOnHorizontalRules + KnownFields(true) strict-decode engine"
    - "validatePathSegment-style containment on manifest paths + fixture sidecar paths (T-99-02-04)"
    - "stdlib crypto/sha256 + io/fs.WalkDir; zero new Go deps"
key-files:
  created: []
  modified:
    - cmd/helix-bench/verify_licenses.go
    - cmd/helix-bench/verify_licenses_test.go
    - Makefile
decisions:
  - "Dual-disposition assertion keys on Apache-2.0 specifically (apacheFixtures>=1), not any fixture block, so flipping the only Apache block's license away fails closed"
  - "Added a per-file manifest-license vs audit-disposition cross-check (catches a flipped MIT->GPL manifest row); the non-SPDX 'internal — repo license' stub disposition is a recognized sentinel"
  - "--manifest/--tree are optional flags; the command stays OFF the root tree (preserves --help subcommand count, same rule as verify-tos)"
metrics:
  duration: "~25 min"
  completed: "2026-06-23"
  tasks: 3
  files: 3
status: complete
---

# Phase 99 Plan 02: Dual-Disposition License Gate + Anti-Vacuity Tamper Test Summary

Extended the existing `cmd/helix-bench verify-licenses` strict-decode gate (VENDOR-03) to scan the full mixed-license vendored tree under BOTH the MIT `track:` and Apache-2.0 `fixture:` dispositions, added a bidirectional manifest-vs-disk `crypto/sha256` walk that fails closed on any tamper/missing/extra file, and shipped an anti-vacuity tamper test proving the gate goes RED on every sabotage class while a good-tree control stays GREEN.

## What was built

- **`FixtureLicense` struct + `hasTopLevelFixtureKey` pre-filter** (`verify_licenses.go`) — the Apache-2.0 second disposition, strict-decoded with `KnownFields(true)` (unknown key fails closed), mirroring the existing `TrackLicense`/`hasTopLevelTrackKey` path.
- **`verifyLicensesFull(audit, manifest, tree)`** — runs the existing per-track audit, decodes every `fixture:` block, asserts ≥1 **Apache-2.0** fixture block (dual disposition required), asserts each fixture's `notice_path`/`license_path` sidecars exist (path-containment checked), then runs the manifest-vs-disk walk.
- **`verifyManifestVsDisk` + `parseManifestRows`** — parses the per-file `| `path` | sha256 | license | … |` table (64-hex column-2 filter isolates it from the upstream-sources table), walks the `--tree` root recomputing `crypto/sha256`, and hard-fails in BOTH directions: an on-disk file with no manifest row ("no manifest entry"), a digest mismatch ("sha256 mismatch"), and a manifest row with no on-disk file. Also cross-checks each manifest per-file license against the audit dispositions (flipped MIT→GPL fails; the `internal — repo license` stub sentinel is allowed).
- **`--manifest` / `--tree` flags** on `newVerifyLicensesCmd` (NOT mounted on root — preserves the `--help` subcommand count).
- **Anti-vacuity tamper test suite** (`verify_licenses_test.go`) — `vendorTempTree` helper materializes a good MIT+Apache tree + real-sha256 manifest + dual-disposition audit; a good-tree control + 8 tamper/strict cases.
- **`make verify-licenses`** wired with `--manifest …/VENDOR-MANIFEST.md --tree …/fixtures` over the full Plan 01 tree; stays a hard-fail gate.

## Sabotage-class proof (the whole point — SC4 anti-vacuity)

Every sabotage class makes the gate go RED, and the committed good tree stays GREEN:

| Sabotage class | Test | Result |
|----------------|------|--------|
| (control) committed good tree | `TestVerifyLicensesFull_GoodTreeControl` / `_CommittedTreePasses` | GREEN (nil) |
| (a) flipped license in manifest (MIT→GPL) | `_TamperFlippedManifestLicense` | RED |
| (a') flipped license in audit (only Apache block→MIT) | `_TamperFlippedAuditLicense` | RED |
| (b) dropped manifest entry (disk file uncovered) | `_TamperDroppedManifestEntry` | RED ("no manifest entry") |
| (c) extra manifest entry (no disk file) | `_TamperExtraManifestEntry` | RED |
| (d) mutated vendored byte (sha mismatch) | `_TamperMutatedByte` | RED ("sha256 mismatch") |
| (e) removed NOTICE/LICENSE sidecar | `_TamperRemovedSidecar` | RED |
| (f) zero Apache-2.0 blocks (single-disposition regression) | `_ZeroApacheBlocksIsError` | RED |
| (g) unknown key in fixture block | `_FixtureUnknownKeyFailsStrictDecode` | RED ("strict decode") |

All 16 `VerifyLicenses*` tests GREEN (6 pre-existing unchanged + 10 new). A manual sabotage smoke confirmed: mutate one committed vendored byte → `make verify-licenses` exits 1; revert → exits 0.

## Verification results

- `go vet ./...` — clean (exit 0).
- `go test ./cmd/helix-bench/... -run VerifyLicenses` — `ok` (all 16 pass).
- `make verify-licenses` — exit 0 over the real committed 150-file tree (6 MIT track blocks + 1 Apache-2.0 fixture block validate; every file's on-disk sha256 matches its manifest row, both directions).
- `git diff go.mod go.sum` — empty (zero new Go deps).
- No `internal/kernel` / `internal/semantic` import in `verify_licenses.go`.

## Deviations from Plan

None functional. One additive decision beyond the literal plan text: a per-file **manifest-license vs audit-disposition cross-check** was added inside `verifyManifestVsDisk` so the "flipped license value" tamper class is caught even when the flip is in the *manifest* (not just the audit). This is squarely within the plan's "flipped license value" sabotage requirement and is fail-closed (Rule 2 — correctness completeness). The non-SPDX `internal — repo license` value the committed manifest records for the 8 pre-existing hermetic stubs is a recognized sentinel so the committed-tree test passes.

## Deferred Issues (out of scope, pre-existing)

Logged in `deferred-items.md`. These `cmd/helix-bench` tests fail identically on the baseline commit (`9ce7d83d~1`, before this plan) and are network/wiring issues unrelated to the license gate — NOT regressed by this plan:
- `TestRunSubcommandWiresDeltaPass` (ablation delta pass not wired into runBench).
- `crosscodeeval` / `repobench` parquet fetches (HTTP 404, network-dependent) and the downstream `fetch-datasets` / `report` cases.

## Self-Check: PASSED

- `cmd/helix-bench/verify_licenses.go` — present, contains `FixtureLicense`, `verifyLicensesFull`, `verifyManifestVsDisk`.
- `cmd/helix-bench/verify_licenses_test.go` — present, contains `Tamper`, `vendorTempTree`, good-tree control.
- `Makefile` `verify-licenses` — wired with `--manifest`/`--tree`.
- Commits `9ce7d83d` (test/RED), `a7a08cc1` (feat/GREEN), `b580bab8` (chore/Makefile) all in `git log`.
