---
phase: 102-repomap-quality-fuzzy-robustness-evals-baselines
verified: 2026-06-23T20:30:00Z
status: passed
score: 15/15 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 102: RepoMap-Quality + Fuzzy-Robustness Evals + Baselines Verification Report

**Phase Goal:** Two stdlib-only leaf evaluators measure Helix's existing `internal/repomap` ranking quality and `internal/fuzzy` strategy-selection/ambiguity-refusal against broad multi-language gold/drift corpora authored from task ground truth (not tool output), each guarded by a reversed/random-ranker discriminator that must fail the corpus, with committed byte-reproducible baselines.
**Verified:** 2026-06-23T20:30:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| #  | Truth | Status | Evidence |
| -- | ----- | ------ | -------- |
| 1 | repomapeval leaf computes recall@10 / MRR / nDCG@10 / budget-fit from committed gold + committed captured ranking, no binary/no network | ✓ VERIFIED | `go test ./bench/evaluators/repomapeval/` 13 tests PASS hermetic; `repomapeval.go` exports RecallAtK/MRR/NDCGAtK/BudgetFitRatio (stdlib `math` only); TestScoreCorpus is the hermetic golden |
| 2 | repomapeval imports Go stdlib ONLY, proven by TestLeafImports | ✓ VERIFIED | `go list` shows zero agenthands imports for the package; TestLeafImports passes AND **bites** — injecting `_ "internal/repomap"` made it FAIL (`forbidden non-stdlib import`), restored→green |
| 3 | Gold corpus authored from `.meta/example.*` ground truth, committed file:symbol JSON, never from get-repo-map output | ✓ VERIFIED | `testdata/gold/go.json` is symbol-level `file:symbol`; `wordy.go:Answer`, `bowling.go:Game.Roll`/`Game.Score`/`rollsThisFrame`/`NewGame`/`Game` all match the real `.meta/example.go` symbols; gold is plain committed JSON, leaf imports no extractor |
| 4 | Reversed AND seeded-random rankers BOTH fail gold; forward beats max(rev,random) by committed margin | ✓ VERIFIED | TestDiscriminator passes: asserts `fwd-max(rev,rnd) >= DiscriminatorMargin (0.30)` AND `rev>=fwd`/`rnd>=fwd` both fail (both bite) AND empty-corpus fatals; observed fwd=1.0 rev=0.0048 rnd=0.5243 (spread ≈0.48) |
| 5 | Corpus covers py/go/rust at per-language floor enforced by TestCorpusFloor | ✓ VERIFIED | TestCorpusFloor passes (go≥9/py≥9/rust≥10); rust gold=10 exercises confirmed; n=28 total |
| 6 | fuzzyrobust leaf scores 4-strategy selection + ambiguity refusal reusing editsim.ES | ✓ VERIFIED | `go test ./bench/evaluators/fuzzyrobust/` 15 tests PASS; `fuzzyrobust.go` ScoreCase uses `editsim.ES`; TestStrategy_{Match,Mismatch,Refusal} pass |
| 7 | Each drift case = real fixture block transformed by deterministic per-tier perturbation; expected strategy derived structurally, never from observed behavior | ✓ VERIFIED | `perturb.go` Perturb{Whitespace,Indent,Ellipsis,Exact} stdlib `strings` only, RNG-free; rust drift = 4 per strategy (exact/ws/indent/ellipsis) + 1 ambiguous; expected≠captured divergence documented (TrimSpace superset of TrimLeft) — honest measurement |
| 8 | ≥1 duplicate-block ambiguous case asserted `ambiguous_match` (not no_match) | ✓ VERIFIED | TestAmbiguousRefused fatals on no_match (Pitfall 6) and on absent; TestAmbiguousBites proves the gate fails on a flipped no_match outcome; 3 ambiguous cases (1/lang) all refused |
| 9 | fuzzyrobust imports stdlib + exactly editsim (no internal/fuzzy), proven by TestLeafImports | ✓ VERIFIED | `go list` shows exactly `bench/evaluators/editsim`; TestLeafImports + self-discriminator pass |
| 10 | Drift corpus covers py/go/rust at floor (≥4/strategy/lang + ≥1 ambiguous), TestCorpusFloor | ✓ VERIFIED | TestCorpusFloor passes; 17 cases/lang × 3 = 51; rust = 4×4 + 1 ambiguous confirmed |
| 11 | Two committed baseline trees force-tracked (NOT gitignored) | ✓ VERIFIED | `git check-ignore` rc=1 (NOT ignored) for both; all 4 files in `git ls-files`; `.gitignore` lines 279-282 allowlist both trees |
| 12 | Each renderer pure sort-before-emit, deterministic-metrics-only, fail-CLOSES on missing load-bearing key | ✓ VERIFIED | repomap renderer fail-closes on `NDCGAt10==nil`; fuzzy renderer fail-closes on `AmbiguousRefused==nil` OR false; result.v2.json carry no latency/timestamp/abs-path |
| 13 | Each baseline byte-reproducible (double-render diff-empty + golden match) | ✓ VERIFIED | TestRepoMapEvalBaselineByteReproducible + TestFuzzyRobustBaselineByteReproducible pass: double-render + byte-match committed BENCH-RESULTS.md golden |
| 14 | Anti-vacuity: stripped load-bearing metric fails assertion AND renderer fail-closes | ✓ VERIFIED | Both *AntiVacuity tests pass: strip `ndcg_at_10`/`ambiguous_refused`, assert present-check false AND renderer returns error |
| 15 | Makefile regen targets build helix then run //go:build ignore regenerators HELIX_BIN-gated | ✓ VERIFIED | `make -n bench-repomap-eval`/`bench-fuzzy-robust` resolve; both regenerators `//go:build ignore`, excluded from GoFiles, use `os.Exit(2)` fail-not-skip, HELIX_BIN-gated |

**Score:** 15/15 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `bench/evaluators/repomapeval/repomapeval.go` | stdlib metric math (NDCGAtK) | ✓ VERIFIED | exists, NDCGAtK present, stdlib-only |
| `bench/evaluators/repomapeval/rankers.go` | reversed + PCG random rankers | ✓ VERIFIED | math/rand/v2 NewPCG, DiscriminatorMargin=0.30 |
| `bench/evaluators/repomapeval/corpus.go` | LoadGold + path-segment validation | ✓ VERIFIED | LoadGold/LoadCaptured, TestLoadRejectsTraversal passes |
| `bench/evaluators/repomapeval/discriminator_test.go` | TestDiscriminator anti-vacuity | ✓ VERIFIED | both rankers must bite + margin asserted |
| `bench/evaluators/repomapeval/leafimports_test.go` | TestLeafImports | ✓ VERIFIED | self-test bites on injected forbidden import |
| `bench/evaluators/repomapeval/testdata/gold/*.json` | committed file:symbol gold | ✓ VERIFIED | go/python/rust, symbol-level, ground-truth-keyed |
| `bench/runtime/repomap_eval_capture_regen.go` | //go:build ignore HELIX_BIN regen | ✓ VERIFIED | ignore-tagged, excluded from build, os.Exit(2)×6 |
| `bench/evaluators/fuzzyrobust/fuzzyrobust.go` | editsim.ES scoring | ✓ VERIFIED | ScoreCase uses editsim.ES |
| `bench/evaluators/fuzzyrobust/perturb.go` | per-tier transforms | ✓ VERIFIED | PerturbWhitespace/Indent/Ellipsis/Exact deterministic |
| `bench/evaluators/fuzzyrobust/corpus.go` | LoadDrift + validation | ✓ VERIFIED | LoadDrift/LoadCaptured present |
| `bench/evaluators/fuzzyrobust/ambiguous_test.go` | TestAmbiguousRefused | ✓ VERIFIED | asserts ambiguous_match not no_match + bites |
| `bench/evaluators/fuzzyrobust/leafimports_test.go` | TestLeafImports | ✓ VERIFIED | stdlib + editsim only |
| `bench/runtime/fuzzy_robust_capture_regen.go` | //go:build ignore non-leaf harness | ✓ VERIFIED | calls fuzzy.Match, ErrAmbiguous→ambiguous_match, excluded from build |
| `bench/aggregator/repomap_eval_baseline.go` | RenderRepoMapEvalBaseline fail-closed | ✓ VERIFIED | nil-nDCG fail-close, sort-before-emit |
| `bench/aggregator/fuzzy_robust_baseline.go` | RenderFuzzyRobustBaseline fail-closed | ✓ VERIFIED | absent/false ambiguous_refused fail-close |
| `bench/reports/{repomap-eval,fuzzy-robust}-baseline/result.v2.json` | committed deterministic-metrics-only | ✓ VERIFIED | git-tracked, no latency/timestamp/abs-path |
| `.gitignore` | allowlist force-tracking | ✓ VERIFIED | lines 279-282 |
| `Makefile` | bench-repomap-eval + bench-fuzzy-robust | ✓ VERIFIED | both `make -n` resolve |

### Key Link Verification

| From | To | Via | Status |
| ---- | -- | --- | ------ |
| repomapeval_test.go | testdata/gold/*.json | LoadGold scores gold vs captured | ✓ WIRED (TestScoreCorpus hermetic golden passes) |
| discriminator_test.go | rankers.go | reversed/random applied, NDCGAtK | ✓ WIRED (both rankers bite) |
| repomap_eval_capture_regen.go | testdata/captured/*.json | HELIX_BIN-gated parse, os.Exit(2) | ✓ WIRED (ignore-tagged, fail-not-skip) |
| fuzzyrobust.go | editsim.go | editsim.ES per-case similarity | ✓ WIRED (sole in-repo import) |
| ambiguous_test.go | testdata/captured/*.json | duplicate-block == ambiguous_match | ✓ WIRED |
| repomap_eval_baseline.go | result.v2.json | decode nDCG@10, fail-close | ✓ WIRED |
| Makefile | repomap_eval_capture_regen.go | build helix + HELIX_BIN regen | ✓ WIRED |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Leaf-import self-test bites | inject `_ "internal/repomap"` then `go test -run TestLeafImports` | rc=1, `forbidden non-stdlib import` | ✓ PASS |
| repomapeval hermetic | `go test ./bench/evaluators/repomapeval/ -count=1` | 13 PASS, no HELIX_BIN/network | ✓ PASS |
| fuzzyrobust hermetic | `go test ./bench/evaluators/fuzzyrobust/ -count=1` | 15 PASS | ✓ PASS |
| aggregator baselines | `go test ./bench/aggregator/ -count=1` | 6 baseline tests PASS | ✓ PASS |
| Full bench gate | `go test ./bench/... -count=1` | rc=0 all green | ✓ PASS |
| Regen excluded from build | `go list -f '{{.GoFiles}}' ./bench/runtime` | capture_regen NOT in GoFiles | ✓ PASS |
| Baselines not gitignored | `git check-ignore` both trees | rc=1 (not ignored) | ✓ PASS |
| Build/vet clean | `go build ./...` + `go vet ./bench/...` | rc=0 / rc=0 | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ----------- | ----------- | ------ | -------- |
| REPOEVAL-01 | 102-01 | repomapeval leaf measures recall/MRR/nDCG + budget-fit, stdlib-only, no-kernel-import | ✓ SATISFIED | Truths 1-2, TestLeafImports bites |
| REPOEVAL-02 | 102-01 | multi-lang gold from ground truth, size floor, reversed/random discriminator MUST fail | ✓ SATISFIED | Truths 3-5, TestDiscriminator |
| FUZZBENCH-01 | 102-02 | fuzzyrobust leaf measures 4-strategy + refusal via editsim.ES, leaf boundary | ✓ SATISFIED | Truths 6, 9 |
| FUZZBENCH-02 | 102-02 | drift corpus, expected strategy from drift type, ≥1 must-refuse ambiguous, floor | ✓ SATISFIED | Truths 7-8, 10 |
| BASELINE-02 | 102-03 | committed baselines fail-not-skip, byte-reproducible, deterministic-metrics-only | ✓ SATISFIED | Truths 11-15 |

All 5 declared requirement IDs accounted for; REQUIREMENTS.md maps exactly these 5 to Phase 102 (all marked Complete). No orphaned requirements.

### Anti-Patterns Found

None blocking. The `indentation_flexible` strategy count of 0 in the fuzzy baseline is an honest, documented measured divergence (cascade tier-2 TrimSpace is a superset of tier-3 TrimLeft), not a stub — the corpus carries 4 indentation_flexible *expected* drift cases per language and the leaf grades selection, so expected≠captured by design. Documented 11× in CORPUS.md.

### Pre-Existing Failure (NOT attributable to Phase 102)

`cmd/helix-bench TestRunSubcommandWiresDeltaPass` fails on `go test ./...` due to an unwired `ablation_deltas` (tech debt from prior phases). Confirmed: `git log origin/HEAD..HEAD -- cmd/helix-bench` shows zero Phase 102 commits (only Phase 89/99/85/86). The phase's authoritative gate `go test ./bench/...` is fully green (rc=0).

### Gaps Summary

No gaps. All 15 must-haves verified against the actual codebase. The two stdlib-only leaves exist, are import-boundary-enforced (self-test demonstrably bites), score gold/drift corpora authored from ground truth (cross-checked against `.meta/example.*`), the reversed+seeded-random discriminator and the duplicate-block must-refuse gate both bite, and both committed baselines are force-tracked, byte-reproducible, and fail-closed on a stripped metric.

---

_Verified: 2026-06-23T20:30:00Z_
_Verifier: Claude (gsd-verifier)_
