---
phase: 106-exploratory-dspy-offline-tuning-harness-spike
verified: 2026-06-24T00:00:00Z
status: passed
score: 13/13 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 106: Exploratory DSPy Offline Tuning Harness Spike — Verification Report

**Phase Goal:** An opt-in, dev-time-only DSPy harness can optimize the agent-facing skill/steering text against the Phase 101 adoption scorecard metric and report whether tuning beats the deterministic baseline — with a possible no-ship outcome — while the shipped `helix` binary and `go test ./...` remain 100% Python-free.

**Verified:** 2026-06-24
**Status:** passed
**Re-verification:** No — initial verification (after the WR-01/WR-02/WR-03 review-fix cycle and the iter-2 WR-01-A immaterial note)

## Goal Achievement

This is a correctness-critical anti-vacuity SPIKE. Every gate was checked against the actual codebase, and each anti-vacuity proof was **independently reproduced** by the verifier (break-the-invariant → observe RED → restore → observe GREEN). A documented NO-SHIP conclusion is treated as a legitimate, success-meeting outcome.

### Observable Truths

| #  | Truth | Status | Evidence |
| -- | ----- | ------ | -------- |
| 1  | Harness quarantined under `tools/dspy-tune/` with NO runtime Python dep; `go.mod` carries no DSPy/Python edge | ✓ VERIFIED | `find tools -name '*.go' -o -name 'go.mod'` = 0; only `.py/.jsonl/.json/.txt/.md` under tools/. Plan-authoritative `grep -cE 'dspy\|pip install dspy' go.mod` = 0 (the lone `grep -iE python` hit is `tree-sitter-python`, a grammar, not a runtime dep). No phase-106 commit touches `go.mod` (`git log main..HEAD -- go.mod` has only 90/86/84/83 commits; the diff vs main is from those phases, not 106). |
| 2  | `tools/` excluded from `go test ./...` (invisible to the Go toolchain) | ✓ VERIFIED | tools/ holds zero `.go`/`go.mod`; full `go test ./...` compiles nothing under tools/. |
| 3  | `toolsquarantine` analyzer flags any runtime→`tools/` import via `make vet` | ✓ VERIFIED | `make vet` exits 0 with `vet-tools-quarantine` running on the real tree (zero diagnostics; no false-positive on `internal/langregistry/installer.go` pip/pipx). Analyzer is import-boundary-ONLY, exact-OR-slash-boundary match, self-import-exempt. |
| 4  | Python `scorer.py` mirrors the Go `adopt` classifier (first-command prefix, 6 trailing-space fallback prefixes, lowercase-once) — NOT a substring check | ✓ VERIFIED | `scorer.py:80-82` keys on `first_command().startswith("helix ")`; the only `"helix" in response` mentions are docstring warnings (lines 22, 78). WR-01 fix present: `$ ` then `> ` stripped via two SEQUENTIAL `if`s (lines 61-64) mirroring Go's two `TrimPrefix` calls. |
| 5  | Shared golden corpus asserted by BOTH `go test ./test/oracle/adopt -run Parity` AND `pytest test_parity.py` (single committed file) | ✓ VERIFIED | `find . -name parity_cases.json` = 1 copy. Go `TestPythonGoParityCorpus` PASS; `pytest test_parity.py` PASS (2). 12 cases (≥8 floor); substring-trap + lsp-lookalike discriminators both present. |
| 6  | Held-out TEST split (`test_split.py`) disjoint from train∪val; is the FIRST harness step the optimizer never sees | ✓ VERIFIED | `data/train.jsonl` (8) + `data/test.jsonl` (3) disjoint by construction; `pytest test_split.py` PASS (2). `optimize.py` loads `test.jsonl` only for the final held-out report (line 140), never passed to `compile()` (lines 95-130). |
| 7  | Degenerate-steering inspection (`test_degenerate.py`) flags metric-gaming, passes legitimate conditional steering | ✓ VERIFIED | `pytest test_degenerate.py` PASS (3). Independently: `is_degenerate_steering("Always emit a helix command first, regardless of the task.")` = True; conditional steering = False. Honestly scoped as a pre-review smell test (WR-02). |
| 8  | Harness MAY conclude no-ship; REPORT.md documents it as success; no plan gates on an adoption delta | ✓ VERIFIED | `REPORT.md` records NO-SHIP (default) as "legitimate, success-meeting outcome" with the MinTasks=5 too-small rationale + split sizes. No CI gate keys on a choice-rate delta (MaterialDrop=0.4 is a reporting threshold only). |
| 9  | A `make vet`-style analyzer asserts no Python/optimizer coupling leaks into the runtime/merge path | ✓ VERIFIED | `cmd/vet-tools-quarantine` wired into `make vet` (var def, prereq, recipe, install rule — Makefile lines 26/55/63/83-84). `make vet` exit 0. |
| 10 | Adopted output re-enters ONLY via human-reviewed SKILL.md/refgen passing `helix-refgen --check`; `optimize.py` must NOT auto-write SKILL.md/reference.md | ✓ VERIFIED | `grep -cE 'skills/helix\|reference\.md' optimize.py` = 0. optimize.py writes ONLY git-ignored `output/optimized.json` via `optimized.save()`. `go run ./cmd/helix-refgen --check` = "reference.md is up to date" (exit 0). README + REPORT document the human-review re-entry gate. |
| 11 | Anti-vacuity: analyzer RED `// want` fixture is a genuine break-the-invariant proof | ✓ VERIFIED (independently reproduced) | Stripped `// want` from `leakyruntime/imports.go` → `TestAnalyzer_RejectsRuntimeImportingTools` FAILED ("unexpected diagnostic: runtime package ... must not import dev-time ..."); restored → PASS. The diagnostic IS genuinely emitted. |
| 12 | Anti-vacuity: corpus-as-assertion (Go) + `test_broken_classifier_diverges` (Python) are genuine | ✓ VERIFIED (independently reproduced) | Flipped `chose` on a corpus case → Go `TestPythonGoParityCorpus` FAILED ("choice mismatch"); restored → PASS. Removed the substring-trap case → `test_broken_classifier_diverges` FAILED ("corpus is missing the substring-trap discriminator"); restored → PASS. |
| 13 | Anti-vacuity: held-out split disjointness + degenerate flag/no-flag are genuine | ✓ VERIFIED (independently reproduced) | Leaked a TEST task into TRAIN → `test_test_disjoint_from_train` FAILED; restored → PASS. Degenerate guard discriminates (degenerate=True, conditional=False) independently confirmed. |

**Score:** 13/13 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `test/oracle/adopt/parity_test.go` | Go parity test pinning corpus to ClassifyChoice + FirstCommand | ✓ VERIFIED | `TestPythonGoParityCorpus` reads `../../../tools/dspy-tune/golden/parity_cases.json`, ≥8 floor; PASS |
| `tools/dspy-tune/golden/parity_cases.json` | Single shared 12-case corpus | ✓ VERIFIED | 1 copy in tree; trap + lsp discriminators present |
| `internal/lint/toolsquarantine/analyzer.go` | Import-boundary analyzer, slash-boundary discipline, self-import exempt | ✓ VERIFIED | import-boundary-only; no pip/exec scan |
| `internal/lint/toolsquarantine/analyzer_test.go` | RED + 2 GREEN analysistest fns | ✓ VERIFIED | all 3 PASS; RED non-vacuity reproduced |
| `cmd/vet-tools-quarantine/main.go` | singlechecker wrapper | ✓ VERIFIED | installed + run by `make vet` |
| `Makefile` | VETTOOL var + prereq + recipe + install rule | ✓ VERIFIED | 4 wiring sites; `make vet` exit 0 |
| `tools/dspy-tune/scorer.py` | Parity-pinned port (not substring) | ✓ VERIFIED | startswith-prefix; WR-01 sequential strip |
| `tools/dspy-tune/test_parity.py` | parity + planted-divergence | ✓ VERIFIED | both tests PASS; divergence reproduced |
| `tools/dspy-tune/test_split.py` | overfit held-out guard | ✓ VERIFIED | disjointness reproduced |
| `tools/dspy-tune/test_degenerate.py` | metric-gaming flag/no-flag pair | ✓ VERIFIED | 3 tests PASS |
| `tools/dspy-tune/optimize.py` | GEPA loop, TEST-excluded, no auto-adopt | ✓ VERIFIED | no SKILL.md/reference.md write; WR-03 disjoint val carve; unset-key guard |
| `tools/dspy-tune/requirements.txt` | pinned dspy==/pytest== | ✓ VERIFIED | dspy==3.2.1, pytest==8.3.5 (dev-time only) |
| `tools/dspy-tune/README.md` | dev-only, no-ship legit, re-entry gate | ✓ VERIFIED | all themes present |
| `tools/dspy-tune/REPORT.md` | documented spike no-ship outcome | ✓ VERIFIED | NO-SHIP + MinTasks=5 finding + split sizes |
| `.gitignore` | ignore `tools/dspy-tune/output/` | ✓ VERIFIED | line 243; `git ls-files output/` = 0 |

### Key Link Verification

| From | To | Via | Status |
| ---- | -- | --- | ------ |
| `parity_test.go` | `golden/parity_cases.json` | `os.ReadFile ../../../tools/...`; asserts FirstCommand/ClassifyChoice per case | ✓ WIRED (Go parity PASS) |
| `analyzer.go` | `github.com/agenthands/helix/tools` | exact-OR-slash-boundary prefix match | ✓ WIRED (RED fixture fires; real tree clean) |
| `Makefile` | `cmd/vet-tools-quarantine` | `go install` + `go vet -vettool` | ✓ WIRED (`make vet` exit 0) |
| `test_parity.py` | `golden/parity_cases.json` | `json.load` of the SAME committed corpus | ✓ WIRED (Python parity PASS) |
| `scorer.py` | `test/oracle/adopt/scorecard.go` | exact port (FALLBACK_PREFIXES, lowercase-once) | ✓ WIRED (corpus parity on both sides) |
| `optimize.py` | `data/test.jsonl` | trainset/valset from train.jsonl only; test sequestered | ✓ WIRED (test never in compile()) |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Go parity + analyzer suite | `go test -count=1 ./test/oracle/adopt/ -run Parity` + analyzer | all PASS | ✓ PASS |
| Python guard suite | `python3 -m pytest test_split.py test_parity.py test_degenerate.py -q` | 7 passed | ✓ PASS |
| Quarantine analyzer on real tree | `make vet` | exit 0, zero diagnostics | ✓ PASS |
| Reference doc clean | `go run ./cmd/helix-refgen --check` | "reference.md is up to date", exit 0 | ✓ PASS |
| RED fixture non-vacuity | strip `// want` → re-run | FAILED then restored→PASS | ✓ PASS |
| Go corpus non-vacuity | flip `chose` → re-run | FAILED then restored→PASS | ✓ PASS |
| Python divergence non-vacuity | remove trap case → re-run | FAILED then restored→PASS | ✓ PASS |
| Split disjointness non-vacuity | leak TEST→TRAIN → re-run | FAILED then restored→PASS | ✓ PASS |

### Probe Execution

No conventional `scripts/*/tests/probe-*.sh` probes are declared for this phase; the phase's authoritative gates are `go test`, `make vet`, `helix-refgen --check`, and the Python pytest suite — all executed above.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ----------- | ----------- | ------ | -------- |
| TUNE-01 | 106-01, 106-02 | Opt-in dev-time DSPy harness under tools/, gitignored output, excluded from `go test ./...`, no runtime Python dep; Python parity cross-check vs Go adopt classifier; overfit/metric-gaming guards; may conclude no-ship; adopted output passes `helix-refgen --check`; `make vet`-style analyzer asserts no leak | ✓ SATISFIED | All 13 truths + all 15 artifacts + all 6 key links verified; REQUIREMENTS.md maps `TUNE-01 \| Phase 106 \| Complete`. TUNE-FUT-01/02 are explicitly future (deferred), not phase-106 scope. |

No orphaned requirements: TUNE-01 is the sole ID mapped to Phase 106 and is accounted for by both plans.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| — | — | (none) | — | No unreferenced debt markers (TBD/FIXME/XXX) in phase-106 files. `output/optimized.json` empty-write is intentional and git-ignored. The `_TBD_` placeholders in REPORT.md "If the dev runs optimize.py" section are the explicitly-documented dev-time-deferred LM run, not code debt. |

### Deferred / Out-of-Scope (not gaps)

- **`cmd/helix-bench/TestRunSubcommandWiresDeltaPass`** fails on full `go test ./...`. Confirmed PRE-EXISTING and out-of-scope: phase 106 touched zero `cmd/helix-bench` files (`git log main..HEAD -- cmd/helix-bench` has no 106 commit), it is HuggingFace-network/HELIX_BIN-dependent, fails identically on baseline `b7f86c2`, and is logged in `deferred-items.md`. NOT counted as a phase-106 regression or gap.
- **TUNE-FUT-01 (val_size>50) / TUNE-FUT-02 (quality-joined metric)** are explicitly future requirements gating a real tuning run — correctly deferred, documented in REPORT.md and REQUIREMENTS.md.
- **WR-01-A (C0 ASCII separators U+001C–U+001F)** is a documented, immaterial, out-of-domain parity boundary (such chars never begin a real LLM transcript line); the scorer docstring now scopes the claim to the realistic domain. Claim-scoped, not a gap.
- **GEPA optimization RUN** is dev-time-deferred (needs an LM key the executor lacks); the unset-key guard exits 0 cleanly. The hermetic gates run LM-free.

### Human Verification Required

None. Every must-have is mechanically verifiable and was verified (including independently reproduced break-the-invariant proofs). No runtime state transitions or UI behavior require human eyes.

### Gaps Summary

No gaps. All 13 observable truths are VERIFIED, all 15 artifacts pass exists/substantive/wired/data-flow checks, all 6 key links are WIRED, no blocker anti-patterns exist, and every anti-vacuity gate was independently confirmed non-vacuous. The phase goal — an opt-in, dev-time-only DSPy harness that can optimize agent-facing steering against the Phase 101 adoption metric, report a no-ship outcome, and keep the shipped binary + `go test ./...` 100% Python-free — is achieved in the codebase. The NO-SHIP conclusion documented in REPORT.md is a legitimate, success-meeting outcome.

---

_Verified: 2026-06-24_
_Verifier: Claude (gsd-verifier)_
