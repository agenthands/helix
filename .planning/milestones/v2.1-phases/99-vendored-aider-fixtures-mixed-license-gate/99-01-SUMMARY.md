---
phase: 99-vendored-aider-fixtures-mixed-license-gate
plan: 01
subsystem: bench/datasets/aider-polyglot (offline fixture vendoring)
tags: [vendoring, fixtures, mixed-license, MIT, Apache-2.0, sha256, manifest]
requires:
  - bench/datasets/aider-polyglot/pin.go (PinnedSHA + isHexSHA1 discipline)
  - bench/datasets/aider-polyglot/loader.go (per-exercise config.json shape)
  - network (one-time clone of two pinned SHAs)
provides:
  - bench/datasets/aider-polyglot/fixtures/{python,go,rust}/exercises/practice/<ex>/ (MIT, byte-identical @7e0611e)
  - bench/datasets/aider-polyglot/fixtures/_aider-edit-format/ (Apache-2.0, byte-identical @5dc9490)
  - bench/datasets/aider-polyglot/fixtures/{python,go,rust}/NOTICE (MIT attribution)
  - bench/datasets/aider-polyglot/fixtures/_aider-edit-format/{NOTICE,LICENSE} (Apache-2.0)
  - bench/datasets/aider-polyglot/VENDOR-MANIFEST.md (150-row real-sha256 reproducibility record)
  - bench/datasets/aider-polyglot/LICENSE-AUDIT.md (dual MIT-track + Apache-2.0-fixture disposition)
affects:
  - Phase 100 polyglot edit benchmark (consumes this offline fixture tree)
  - Phase 102 corpora (consume the vendored subset)
  - Phase 99 Plan 02 gate (verifies manifest-vs-disk sha256 + dual disposition)
tech-stack:
  added: []
  patterns:
    - "fetch-by-SHA + checkout (never a mutable ref); validate 40-lowercase-hex before clone"
    - "byte-identical vendoring (no inline SPDX in executed fixtures); disposition in manifest/audit/NOTICE"
    - "real crypto/sha256 per-file digest (no 40-zero placeholders)"
key-files:
  created:
    - bench/datasets/aider-polyglot/VENDOR-MANIFEST.md
    - bench/datasets/aider-polyglot/fixtures/python/NOTICE
    - bench/datasets/aider-polyglot/fixtures/go/NOTICE
    - bench/datasets/aider-polyglot/fixtures/rust/NOTICE
    - bench/datasets/aider-polyglot/fixtures/_aider-edit-format/NOTICE
    - bench/datasets/aider-polyglot/fixtures/_aider-edit-format/LICENSE
    - bench/datasets/aider-polyglot/fixtures/_aider-edit-format/search-replace-sample.txt
    - "bench/datasets/aider-polyglot/fixtures/_aider-edit-format/languages/{python,go,rust,java,javascript}/test.<ext>"
    - "bench/datasets/aider-polyglot/fixtures/{python,go,rust}/exercises/practice/<ex>/... (136 MIT files)"
  modified:
    - bench/datasets/aider-polyglot/LICENSE-AUDIT.md
decisions:
  - "Kept the pre-existing hermetic python/wordy + rust/leap stubs byte-unchanged; vendored real upstream wordy for go+rust only (python slot occupied) — preserves loader_test.go"
  - "Excluded .docs/.approaches/.articles/template/tests.toml/gen.go/test_template.tera/Cargo-example from the vendored exercise dirs (lean tree, Pitfall 4)"
  - "Trimmed the 27810-line aider gold to a 119-line excerpt (2 verbatim SEARCH/REPLACE blocks); chat-history.md not vendored"
  - "Apache-2.0 subtree under underscore-prefixed _aider-edit-format/ (loader-walk-safe; loader has no discovery walk)"
metrics:
  duration: ~25m
  completed: 2026-06-23
  tasks: 3
  files: 145
status: complete
---

# Phase 99 Plan 01: Vendor the Fixture Data Summary

Vendored a deterministic, offline, mixed-license fixture tree into `bench/datasets/aider-polyglot/fixtures/` — 136 MIT Exercism polyglot files (9 exercises × python/go/rust, byte-identical to polyglot-benchmark@7e0611e) plus the Apache-2.0 aider edit-format subset (5 small `languages/*` files + a trimmed SEARCH/REPLACE excerpt + NOTICE + LICENSE, byte-identical to aider@5dc9490), recorded in a 150-row real-sha256 `VENDOR-MANIFEST.md` and a dual-disposition `LICENSE-AUDIT.md`.

## What Was Built

| Task | Output | Commit |
|------|--------|--------|
| 1 | MIT polyglot subset (9 ex × py/go/rust, byte-identical @7e0611e) + 3 per-track NOTICE sidecars | `52acc395` |
| 2 | Apache-2.0 aider edit-format subset (`_aider-edit-format/`) + NOTICE + Apache-2.0 LICENSE copy | `aff157bd` |
| 3 | VENDOR-MANIFEST.md (150 real sha256 rows) + dual-disposition LICENSE-AUDIT.md (`fixture:` block) | `97dd0aa6` |

### Manifest / exercise counts

- **Total fixtures tree:** 150 files (all covered by a manifest row, bidirectionally verified).
- **MIT (Exercism polyglot, vendored):** 134 files. Exercise dirs: python 8 upstream (+1 hermetic stub = 9), go 9, rust 9 (+1 hermetic leap = 10).
- **Apache-2.0 (aider edit-format):** 8 files.
- **internal — repo license (pre-existing hermetic stubs):** 8 files (python/wordy ×4, rust/leap ×4).
- **NOTICE sidecars:** 4 (python, go, rust, _aider-edit-format).

## Key Decisions Made

1. **python/wordy conflict resolution.** The upstream `wordy` exercise collides with the pre-existing repo-authored hermetic stub at `fixtures/python/exercises/practice/wordy/` that `loader_test.go` hardcodes. Resolved per plan Recommendation (a): kept the hermetic stub byte-unchanged, vendored the real upstream `wordy` for go+rust only. Recorded distinctly in the manifest as `internal — repo license`.
2. **Lean vendored exercise dirs.** Copied only the loader-required files (`.meta/config.json`, `.meta/example.<ext>`, solution stub, test file(s), go.mod/Cargo.toml). Excluded instructional/generator files.
3. **Trimmed Apache gold.** A 119-line excerpt (two complete verbatim SEARCH/REPLACE blocks, gold lines 3–89 and 419–439) replaces the 27810-line gold; the ~100k-line chat-history.md is absent.
4. **No inline SPDX in executed bytes.** Disposition lives in VENDOR-MANIFEST.md + LICENSE-AUDIT.md + NOTICE sidecars only; executed `.py/.go/.rs/.java/.js/.json` fixtures are byte-identical to upstream.

## Provenance / integrity

- Both pinned SHAs were the live HEAD of each repo at execute time; validated against the 40-lowercase-hex `isHexSHA1` shape, fetched by SHA + checkout (never a mutable ref) — T-99-01.
- Every vendored file verified byte-identical (`cmp`) against the clone before commit — T-99-03 substrate.
- The exercism MIT LICENSE re-fetched at the pinned python SHA hashed to `e52f804e…df44df`, matching the audit's recorded `license_sha256`.
- Temp clones removed after vendoring (offline-self-sufficient tree).

## Deviations from Plan

None — plan executed exactly as written. The python/wordy slot conflict was handled by the plan's pre-specified Recommendation (a) (not a deviation).

## Verification

- `go build ./...` — clean.
- `go vet ./bench/datasets/aider-polyglot/... ./cmd/helix-bench/...` — clean.
- `go test ./bench/datasets/aider-polyglot/...` — PASS (loader_test.go's pinned `python/wordy` + `Files.Solution==[wordy.py]` assertions hold; Apache subtree loader-walk-safe).
- `go test ./cmd/helix-bench/... -run VerifyLicenses` — PASS (extended audit strict-decodes; 6 MIT track blocks intact).
- `go run ./cmd/helix-bench verify-licenses bench/datasets/aider-polyglot/LICENSE-AUDIT.md` — exit 0.
- Hermetic stubs `python/wordy` + `rust/leap` — byte-unchanged (no git diff).
- No `SPDX-License-Identifier` in any executed fixture under `*/exercises/` or `_aider-edit-format/languages/`.
- `git diff go.mod` — empty.
- Manifest-vs-disk: all 150 on-disk files have a real-sha256 manifest row (Plan 02 gate-ready, bidirectional).

### Pre-existing unrelated failures (out of scope)

The full `go test ./cmd/helix-bench/...` package run fails on `Repobench`/`FetchDatasets`/`Report` tests due to network (HTTP 404 on huggingface dataset fetch) and an empty `--run-id` arg test. These are pre-existing, env-dependent, and touch no file changed by this plan (data + markdown only). Logged here, not fixed (scope boundary).

## For Plan 02 (Wave 2 gate)

- `--tree` root: `bench/datasets/aider-polyglot/fixtures/` (stated in the manifest header).
- `--manifest`: `bench/datasets/aider-polyglot/VENDOR-MANIFEST.md` (150 rows, real sha256).
- The `fixture:` block in `LICENSE-AUDIT.md` carries exactly the 5 FixtureLicense keys (`fixture`/`source_repo`/`license`/`notice_path`/`license_path`); `notice_path`/`license_path` resolve to committed files.

## Self-Check: PASSED
