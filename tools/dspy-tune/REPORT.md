# Spike REPORT — DSPy offline tuning harness (Phase 106)

**Spike:** exploratory-dspy-offline-tuning-harness-spike
**Requirement:** TUNE-01
**Date:** 2026-06-24
**This file IS the spike deliverable conclusion** — it records the outcome
whether or not the LM optimization run executes.

## Conclusion: NO-SHIP (default)

A **no-ship** conclusion is the spike's expected, **legitimate, success-meeting**
outcome (not a failure, not a hard adoption-delta gate). The DSPy GEPA harness,
the parity-pinned Python scorer, the held-out split, and the overfit /
metric-gaming / planted-divergence guards are all built and green; the spike's
honest finding is that the **corpus is too small to trust a tuned adoption
delta**, so no optimizer output should be adopted into the shipped surface at
this time.

### Why no-ship

- **`MinTasks=5` is the floor; the corpus is far below a trustworthy size.** The
  committed adoption bucket is 6 intact + 6 sabotaged transcripts (`test/oracle/
  adopt/testdata/transcripts/`), and this harness's own split is small
  (TRAIN=8, TEST=3). A 6+6 / 8+3 corpus cannot yield a statistically
  trustworthy adoption delta on a held-out TEST split.
- **A meaningful held-out split starves the optimizer.** Reserving even 2–3 TEST
  tasks leaves GEPA almost no signal to evolve against — so a "win" on TEST
  would be noise, not a real improvement.
- **The proxy is gameable.** `choice_rate` keys on the first emitted command; an
  optimizer can win it with degenerate unconditional-`helix` steering. The
  `test_degenerate.py` guard flags that, but the residual risk (no quality/
  correctness joiner) is real — full defense is TUNE-FUT-02 (deferred).
- **The clean fallback keeps the milestone 100% Go.** A hand-rolled Go
  candidate-search loop avoids the Python quarantine cost entirely.

### Gate before a real tuning run (TUNE-FUT-01)

Grow the corpus to `val_size > 50` and (per TUNE-FUT-02) join the adoption
metric with a task-success/quality oracle before a GEPA/MIPROv2 run can beat a
deterministic baseline with confidence.

## Split sizes (this harness)

| Split | Source | Count | Seen by optimizer? |
|-------|--------|-------|--------------------|
| TRAIN (train∪val) | `data/train.jsonl` | 8 | yes (val carved from train) |
| TEST (held-out)   | `data/test.jsonl`  | 3 | **no — sequestered** |

The held-out split was created as the FIRST harness step; `test_split.py`
asserts TEST ⊄ train∪val, and `optimize.py` never passes `test.jsonl` to
`optimizer.compile()`.

## Gates (all green, LM-free)

| Gate | File | Proof it is non-vacuous |
|------|------|-------------------------|
| Python↔Go parity | `test_parity.py::test_python_go_parity` | asserts scorer vs the SAME `golden/parity_cases.json` the Go `parity_test.go` pins |
| Planted-divergence (anti-vacuity) | `test_parity.py::test_broken_classifier_diverges` | a `"helix" in response` substring classifier DIVERGES from the real scorer on the substring-trap case |
| Overfit guard | `test_split.py` | TEST disjoint from train∪val; RED on a leak |
| Metric-gaming guard | `test_degenerate.py` | flag/no-flag pair: degenerate always-`helix` flagged, conditional steering not |

## If the dev runs `optimize.py` with an LM key

> Placeholder — fill in after a dev-time run with `OPENAI_API_KEY` set.

- Baseline (deterministic SKILL.md steering) TEST `choice_rate`: _TBD_
- GEPA-optimized TEST `choice_rate`: _TBD_
- Delta on held-out TEST: _TBD_ (adopt only if it beats baseline by a material
  margin on data the optimizer never saw — mirror `MaterialDrop=0.4` as a
  reporting threshold, not a CI gate)
- Degenerate-steering inspection of the optimized text: _TBD_ (must pass
  `test_degenerate.py`'s `is_degenerate_steering(optimized_text) is False`)

## Re-entry (if any text is ever adopted)

Adopted steering re-enters the shipped surface **only** via a human-reviewed
`internal/cli/skills/helix/SKILL.md` edit (≤1536 chars, `## Decision matrix`
anchor preserved) or a refgen override, passing `go run ./cmd/helix-refgen
--check`. The raw `output/optimized.json` is git-ignored and **never
auto-adopted**.

## Toolchain note

Research pinned `dspy==3.1.3`; the harness uses `dspy==3.2.1` (the then-current
release at plan time). The GEPA top-level API used by `optimize.py` is unchanged
across 3.1.x -> 3.2.x.

---

# v2.3 Ship/No-Ship REPORT — Task-Success-Driven Skill Optimization (Phase 110)

**Milestone:** v2.3 (Phases 107–110)
**Requirements:** ADOPT-03 (human-gated adoption), ADOPT-04 (single-binary /
no-runtime-Python re-verification)
**Date:** 2026-06-24

## Verdict: NO-SHIP (by design — legitimate, success-meeting)

The full task-success optimization pipeline is **built, gated, and green**, but
**no optimized steering text is adopted into the shipped surface** at this time.

This is the same honest conclusion the Phase-106 spike reached, now reached
through the *real* task-success machinery rather than the gameable `choice_rate`
proxy: there is **no trustworthy positive ON-vs-OFF attribution delta on a
held-out split large enough to trust** (`val_size > 50`), so adopting any tuned
steering would be premature. NO-SHIP is a success-meeting outcome, not a failure.

### What was built (107–109) and is ready behind the gate
- **Phase 107** — dev-time DeepSeek/OpenAI ReAct agent driving the real `helix`
  CLI; loud-fail on missing key; first-class steering ON/OFF.
- **Phase 108** — honest Aider-polyglot task-success oracle (0-tests = hard
  error, anti-tamper gold-test restore); the `choice_rate → task-success` GEPA
  metric swap; sequestered held-out TEST split + strict `val_size > 50` gate.
- **Phase 109** — heaviest grader: SWE-bench task-success via the upstream
  `swebench==4.1.0` harness on Podman (FAIL_TO_PASS + PASS_TO_PASS, dataset-org
  pin, 0-tests refusal); ON-vs-OFF attribution delta + per-arm cost machinery
  (`attribution.py`).

### The recorded ON/OFF attribution (what the REPORT records)

| Field | Value |
|-------|-------|
| ON success_rate | _not measured — no live run cleared the gate_ |
| OFF success_rate | _not measured — no live run cleared the gate_ |
| Attribution delta (ON − OFF) | _n/a (no trustworthy held-out delta)_ |
| Held-out `val_size` | below the strict `> 50` gate (the v2.2 corpus is ~3) |
| Per-arm cost | $0.00 (no live optimization run executed) |

No numbers are fabricated. `attribution.render_report(...)` is the live renderer
that fills this table from a real ON/OFF run; `attribution._main()` refuses to
emit a delta without an LM key AND a corpus, and the corpus is still below the
gate. Growing the corpus past `val_size > 50` (TUNE-FUT-01) is the precondition
for a real ship decision.

## Adoption gate (ADOPT-03) — live + non-vacuous

- Optimized text re-enters the shipped surface **only** via a human-reviewed
  `internal/cli/skills/helix/SKILL.md` edit (the `## Decision matrix` anchor and
  the SKILL-04 ≤1536-char description cap preserved) or a `helix-refgen` override,
  passing `go run ./cmd/helix-refgen --check`.
- The optimizer writes **only** git-ignored `tools/dspy-tune/output/`
  (`.gitignore:243`); it never auto-writes `SKILL.md` / the generated reference.
- **Break-the-invariant proof:** a hand-edited reference out of sync with the
  generator makes `helix-refgen --check` exit `1` (verified live; durable guard:
  `cmd/helix-refgen/main_test.go::TestCheckRoundTrip`).

## Single-binary / no-runtime-Python (ADOPT-04) — re-verified end-to-end

- **Zero new Go deps:** `go.mod` / `go.sum` last changed at `7f20a874 feat(90-01)`
  (v2.0) — untouched through v2.1, v2.2, and v2.3.
- **No optimizer→Python coupling in the binary:** 0 Go import edges from
  `internal/`/`cmd/` into `tools/dspy-tune`; 0 `.go` files under `tools/dspy-tune`
  (off the default `go test ./...`); the only Go `pip` exec is the documented
  `internal/langregistry/installer.go` LS installer (toolsquarantine-exempt).
- `make vet` (incl. the `vet-tools-quarantine` import-boundary analyzer) green.
- `grep -E 'skills/helix|reference\.md' tools/dspy-tune/optimize.py == 0`.
- New deps are dev-venv Python pins only: `dspy==3.2.1`, `openai==2.43.0`,
  `swebench==4.1.0` — all quarantined out of the binary / module / `helix setup`
  / merge path.

## Re-entry (if a future tuned run ever ships)

Grow the corpus so `val_size > 50` (TUNE-FUT-01), run the ON/OFF attribution,
and — only on a materially positive held-out delta — transcribe the adopted
steering into `SKILL.md` by hand and pass `helix-refgen --check`. The raw
`output/optimized.json` is git-ignored and never pasted.
