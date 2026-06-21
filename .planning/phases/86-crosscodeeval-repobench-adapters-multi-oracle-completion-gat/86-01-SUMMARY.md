---
phase: 86-crosscodeeval-repobench-adapters-multi-oracle-completion-gat
plan: 01
subsystem: bench/evaluators
tags: [crosscodeeval, scorers, em, edit-similarity, identifier-match, pure-logic, tdd]
requires:
  - bench/evaluators/metrics.go (MetricError shape; consumed conceptually, not imported)
provides:
  - bench/evaluators/exactmatch.EM (pure CCE CM-EM exact-match)
  - bench/evaluators/editsim.ES (pure CCE CM-ES normalized Levenshtein ratio in [0,1])
  - bench/evaluators/identmatch.Match (pure CCE IM-EM + IM-F1 identifier-set scorer)
affects:
  - 86-02 (completion gate consumes EM/ES/Match outputs)
  - VERIFIED.md (Plan 02 cites the identmatch tokenizer + keyword list documented here)
tech-stack:
  added: []
  patterns:
    - stdlib-only leaf scorer subpackages (editsim has ZERO imports; identmatch imports regexp only)
    - local two-row DP Levenshtein over runes (no patch_validator/suggest_lev reuse)
key-files:
  created:
    - bench/evaluators/exactmatch/exactmatch.go
    - bench/evaluators/exactmatch/exactmatch_test.go
    - bench/evaluators/editsim/editsim.go
    - bench/evaluators/editsim/editsim_test.go
    - bench/evaluators/identmatch/identmatch.go
    - bench/evaluators/identmatch/identmatch_test.go
  modified: []
decisions:
  - "editsim.ES is normalized Levenshtein 1-lev/max(runeLen), NOT git-numstat line distance; proven by a numstat discriminator test + an import-level gate (editsim has zero imports, so patch_validator/suggest_lev are provably absent)."
  - "Rune-length (not byte-length) normalization so multi-byte unicode counts once (unicode_runes fixture: café vs cafe = 0.75)."
  - "EM/ES apply NO input normalization (raw-string oracles); any trimming is the caller's job and must mirror the live reference. EM-on-empty = true (empty==empty)."
  - "identmatch tokenizer = [A-Za-z_][A-Za-z0-9_]* (ASCII; non-ASCII letters act as separators) minus a language-agnostic keyword set (common Python/Java/TS/C# subset), documented in the doc comment for VERIFIED.md to cite."
  - "Empty identifier-set convention: empty-vs-empty => (em=true, f1=1.0); exactly-one-empty => (em=false, f1=0.0). Keyword-only inputs reduce to empty sets, proving keyword exclusion."
metrics:
  duration: ~14min
  completed: 2026-06-21
---

# Phase 86 Plan 01: EM / edit-similarity / identifier-match scorers Summary

Three PURE, stdlib-only CrossCodeEval completion scorers — `exactmatch.EM` (CM-EM), `editsim.ES` (CM-ES normalized Levenshtein ratio in [0,1]), and `identmatch.Match` (IM-EM + IM-F1 over identifier sets) — each proven SOLELY by a hermetic CCE-paper-shaped fixture test, with `editsim.ES` provably the character-level edit ratio (not git-numstat line distance) via a discriminator test and a zero-import gate.

## What Was Built

- **`bench/evaluators/exactmatch`** — `EM(pred, gold string) bool`: raw string equality oracle (no normalization), both-empty == true, total on empty/unicode. CCE CM-EM.
- **`bench/evaluators/editsim`** — `ES(pred, gold string) float64`: `1 - levenshtein(pred,gold)/max(runeLen)` in [0,1], computed with a local two-row DP edit distance over runes. Both-empty short-circuits to 1.0; one-empty yields 0.0. CCE CM-ES. **Zero imports** (pure stdlib logic, no `import` block at all).
- **`bench/evaluators/identmatch`** — `Match(pred, gold string) (em bool, f1 float64)`: tokenizes each string to `[A-Za-z_][A-Za-z0-9_]*` identifiers minus a documented language-agnostic keyword set, set-compares for EM, and returns the precision/recall harmonic-mean F1. Empty-vs-empty => (true, 1.0); one-empty => (false, 0.0). Imports `regexp` only. CCE IM-EM / IM-F1.

## TDD Gate Compliance

Both tasks followed RED → GREEN as atomic commits:

| Task | RED (test) | GREEN (impl) |
|------|-----------|--------------|
| 1 (EM + ES) | `8d02adda` | `c1da9afc` |
| 2 (identmatch) | `e7bc2e65` | `eba4a751` |

Each RED commit was confirmed failing-to-compile (`undefined: EM/ES/Match`) before the GREEN implementation. No REFACTOR commit was needed.

## Provenance of Fixture Values

All fixture rows are sourced from / shaped by the CrossCodeEval paper (Ding et al., NeurIPS 2023, arXiv:2310.11248), cited in each test's doc comment. The expected EM booleans are mechanical equalities; the ES ratios and IM F1 values are computed by hand from the closed-form definitions (e.g. `ES("kitten","sitten")` = 1 - 1/6; partial-overlap IM F1 = 2/3 over size-3 sets with intersection 2). No metric output was fabricated — every expected value is derivable from the documented formula. The hermetic tests are the SOLE authoritative proof (no network, no `HELIX_BIN`, no new dependency).

## Wrong-Metric Guard (T-86-01-02 mitigation)

- `editsim.ES` re-implements Levenshtein locally and does NOT import `patch_validator.EditDistancePatch` (git `diff --numstat` line distance) nor the unexported `internal/mcp` tool-name-typo Levenshtein.
- **`TestES_NumstatDiscriminator`** asserts `ES("return a + b", "return a - b")` = 1 - 1/12 ≈ 0.9166 (a character-level ratio strictly in (0,1)), which a line-count distance (an integer >= 1) could never produce.
- **Import-level gate:** `go list -f '{{.Imports}}' ./bench/evaluators/editsim/` returns zero imports, so `patch_validator`/`suggest_lev` are provably absent. (The package doc comment mentions those names in prose to explain the pitfall; the `^import` key-link is satisfied at the import level.)

## Deviations from Plan

None — plan executed exactly as written. The plan's `grep -rn "suggest_lev\|EditDistancePatch" bench/evaluators/editsim/` check matches explanatory doc-comment prose by design; the load-bearing guarantee (no IMPORT of the wrong metric) was verified at the import level via `go list`, which is the stronger and intended check (key_link pattern `^import`).

## Verification

- `go build ./...` — clean.
- `go vet ./bench/evaluators/...` — clean.
- `go test ./bench/evaluators/...` — all packages OK (exactmatch, editsim, identmatch + the 7 pre-existing graders).
- `make vet` — clean (all 6 custom vettools pass).
- Import gate: editsim has zero imports; identmatch imports `regexp` only.

## Known Stubs

None. All three scorers are fully implemented pure functions with complete fixture coverage.

## Self-Check: PASSED

All 6 created files exist on disk; all 4 task commits (8d02adda, c1da9afc, e7bc2e65, eba4a751) are present in git history.
