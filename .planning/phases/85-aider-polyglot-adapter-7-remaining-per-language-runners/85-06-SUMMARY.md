---
phase: 85
plan: 06
subsystem: bench
tags: [license-audit, verify-licenses, hard-fail-gate, aider-polyglot, exercism, tdd]
requires:
  - cmd/helix-bench/verify_tos.go (strict-decode hard-fail validator shape)
  - Makefile verify-tos: target (gate shape)
provides:
  - make verify-licenses (HARD-FAIL per-track license gate)
  - bench/datasets/aider-polyglot/LICENSE-AUDIT.md (per-track sha256 + redistribution clause)
  - cmd/helix-bench verify-licenses subcommand (Makefile-only gate)
affects:
  - SC#4 redistribution compliance for the Aider-Polyglot dataset
tech-stack:
  added: []
  patterns:
    - "strict-decode hard-fail validator: KnownFields(true) + first-error-returns + (count, error) signature + count==0 is an error"
    - "Makefile-only gate dispatched directly from main (not a root subcommand, preserves --help count)"
key-files:
  created:
    - cmd/helix-bench/verify_licenses.go
    - cmd/helix-bench/verify_licenses_test.go
    - bench/datasets/aider-polyglot/LICENSE-AUDIT.md
  modified:
    - cmd/helix-bench/main.go
    - Makefile
decisions:
  - "All six Exercism tracks (cpp/go/java/javascript/python/rust) ship an identical MIT LICENSE — verified byte-for-byte at the pinned sha (single sha256 e52f804e...), read not assumed (A2)."
  - "source_repo records the track repo pinned to its commit sha (V6: commit-pinned, not a mutable tag) to mitigate T-85-06-02 upstream-LICENSE drift."
  - "Reused verify_tos.go's splitOnHorizontalRules verbatim (already in package main) rather than re-implementing block extraction."
metrics:
  duration: ~3m
  completed: 2026-06-21
---

# Phase 85 Plan 06: Exercism License Audit + make verify-licenses Gate Summary

Strict-decode HARD-FAIL `make verify-licenses` gate over a per-track
`LICENSE-AUDIT.md` cloning the proven `verify_tos.go` discipline, with all six
Aider-Polyglot Exercism tracks' real MIT license + sha256 read at the pinned sha.

## What Was Built

**Task 1 (TDD) — `cmd/helix-bench/verify_licenses.go` validator:**
- `verifyLicenses(path) error` wrapping `verifyLicensesCount(path) (int, error)`,
  a direct clone of `verifyTOS`/`verifyTOSCount`'s shape.
- `TrackLicense` struct: `track`, `source_repo`, `license` (SPDX),
  `license_sha256`, `redistribution_clause_excerpt`.
- Block extraction reuses the package's existing `splitOnHorizontalRules`;
  a new `hasTopLevelTrackKey` pre-filter mirrors `hasTopLevelProviderKey`.
- `dec.KnownFields(true)` strict decode → unknown key is a non-zero exit.
- Fail-closed: empty `license` OR empty `license_sha256` returns the first error;
  zero track blocks is itself an error (count==0 fails, mirroring verify_tos.go:99-101).
- `newVerifyLicensesCmd()` (cobra, `RunE`) dispatched directly from `main.go`
  (NOT added to the root tree, preserving the --help subcommand count) — invoked
  only by the Makefile gate.
- Tests written RED first (undefined symbol compile failure), then GREEN.

**Task 2 — `bench/datasets/aider-polyglot/LICENSE-AUDIT.md` + Makefile gate:**
- One strict-decodable YAML-frontmatter block per Exercism track (cpp, go, java,
  javascript, python, rust).
- Each track's LICENSE was fetched at its pinned HEAD sha from
  `raw.githubusercontent.com/exercism/<track>/<sha>/LICENSE` and its sha256
  computed. All six are MIT and byte-identical
  (`e52f804e74f0fbd34e8927962319df346166b8194094309a0aee318693df44df`) — read,
  not assumed (A2). The license was confirmed by inspecting the actual `LICENSE`
  text (MIT header + permission/redistribution clause).
- `source_repo` pins each track to its commit sha (V6).
- `verify-licenses:` Makefile target added (mirrors `verify-tos:`); `verify-licenses`
  added to the `.PHONY` line.

## Tests / Verification

- `go test ./cmd/helix-bench/ -run License -count=1` — 6/6 PASS (good-file count,
  empty-license, empty-sha256, unknown-key strict-decode, zero-track, committed-audit).
- `make verify-licenses` exits 0 on the committed audit (6 tracks).
- Negative path proven: a corrupted entry (empty `license`) makes the gate exit 1
  (`Error: verify-licenses: track "cpp" ... has an empty license (SPDX) field`).
- `go build ./...` clean; `go vet ./cmd/helix-bench/` clean; full `make vet`
  (all 6 vettool passes) green.
- TDD gate commits present: `test(85-06)` (RED) precedes `feat(85-06)` (GREEN).

## Deviations from Plan

None — plan executed as written.

Note on the plan's network-gated fallback clause (Task 2): the environment had
network reachability, so the real LICENSE text + sha256 were captured at the
pinned shas directly (the preferred path), rather than recording a network-gated
placeholder. No flagging was needed.

## Threat Model Coverage

- **T-85-06-01 (Repudiation — track shipped without a license):** mitigated —
  hard-fail gate asserts non-empty SPDX license + sha256 per track; count==0 is
  an error; first-error-returns.
- **T-85-06-02 (Tampering — upstream LICENSE drift):** mitigated — per-track
  sha256 at the pinned sha; `source_repo` pins a commit sha, not a mutable tag.
- **T-85-06-03 (Tampering — drift/unknown keys in the audit):** mitigated —
  `KnownFields(true)` strict decode → unknown key is a non-zero exit.

## Threat Flags

None — no new network endpoint, auth path, or trust-boundary surface beyond the
read-only audit file and its validator.

## Known Stubs

None.

## TDD Gate Compliance

- RED: `test(85-06)` commit `42761211` — tests fail to compile (undefined `verifyLicensesCount`).
- GREEN: `feat(85-06)` commit `0c98162d` — validator implemented, all license tests pass.
- No REFACTOR commit needed.

## Commits

- `42761211` test(85-06): add failing strict-decode license validator tests
- `0c98162d` feat(85-06): strict-decode hard-fail license validator + dispatch
- `3cbd5133` feat(85-06): author per-track LICENSE-AUDIT.md + wire make verify-licenses gate

## Self-Check: PASSED

All created files and all three task commits verified present on disk and in git history.
