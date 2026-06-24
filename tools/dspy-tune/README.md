# `tools/dspy-tune/` — DSPy offline tuning harness (dev-time only)

An **opt-in, dev-time-only** DSPy prompt-optimization harness for evolving the
agent-facing helix steering text against the Phase 101 adoption metric.

> **This tree is NEVER in the shipped product.** No `.go` files, no `go.mod`. It
> is invisible to `go test ./...`, adds no runtime Python dependency to the
> `helix` binary or `helix setup`, and never enters `go.mod`. The
> `cmd/vet-tools-quarantine` analyzer + the no-`.go`-under-`tools/` invariant
> keep it that way.

## What's here

| File | Role |
|------|------|
| `scorer.py` | Python re-impl of the Go `test/oracle/adopt` classifier (`first_command`, `classify_choice`, `score_choice_rate`) — parity-pinned. |
| `golden/parity_cases.json` | The SHARED golden corpus. Read by BOTH `test_parity.py` here and `test/oracle/adopt/parity_test.go` (no duplication). |
| `data/train.jsonl`, `data/test.jsonl` | The held-out split. TEST is sequestered; the optimizer draws train/val from TRAIN only. |
| `test_split.py` | Overfit guard: asserts TEST ⊄ train∪val. |
| `test_parity.py` | Parity over the shared corpus **+** the planted-divergence anti-vacuity test (a broken substring classifier must diverge). |
| `test_degenerate.py` | Metric-gaming guard: flags degenerate always-`helix` steering; passes legitimate conditional steering. |
| `optimize.py` | The DSPy **GEPA** harness loop (dev-time; needs an LM key). |
| `requirements.txt` | Pinned dev deps (`dspy`, `pytest`). |
| `REPORT.md` | The documented spike outcome (no-ship/ship). |

## Setup (dev-time only — ALWAYS use `uv`, never bare `python`/`pip`)

```bash
cd tools/dspy-tune
uv venv                              # creates .venv/ (git-ignored)
uv pip install -r requirements.txt
```

The `.venv/` and `output/` directories are git-ignored. All commands below run through `uv run` (which uses `.venv/` automatically); `uvx` for one-off tools.

## Run the hermetic gates (LM-free — no API key needed)

```bash
cd tools/dspy-tune
uv run pytest test_split.py test_parity.py test_degenerate.py test_agent.py
# each file is also runnable as a plain script under the env:
uv run python test_split.py
```

## Run the optimization (dev env only — needs an LM key)

```bash
cd tools/dspy-tune
export OPENAI_API_KEY=sk-...        # DEV ENVIRONMENT ONLY — never committed, never read by any Go code
export DSPY_LM_MODEL=openai/gpt-4.1-mini   # optional; any litellm-supported provider
uv run python optimize.py
```

If `OPENAI_API_KEY` is unset, `optimize.py` prints an informative message and
exits 0 (it never crashes). The optimizer writes a git-ignored
`output/optimized.json`; it never writes `SKILL.md` or `reference.md`.

## NO-SHIP is a legitimate, success-meeting outcome

This is a **spike**. A **no-ship conclusion is an explicit success-meeting
outcome**, not a failure. The committed adoption bucket floor is `MinTasks=5`
and the corpus is honestly **too small** for a statistically trustworthy
adoption delta on a held-out TEST split: reserving even 2–3 TEST tasks leaves
GEPA almost no signal. The clean fallback — and the default conclusion — is a
hand-rolled Go candidate-search loop that keeps the milestone **100% Go**. See
`REPORT.md`. Growing the corpus to `val_size > 50` (TUNE-FUT-01) is the gate
before a real tuning run can beat a deterministic baseline with confidence.

## The artifact re-entry gate (how adopted text ships)

The raw `output/optimized.json` is **git-ignored and NEVER pasted** into the
product. Adopted steering text re-enters the shipped surface **only** via:

1. a **human-reviewed edit** to `internal/cli/skills/helix/SKILL.md`, preserving
   the `## Decision matrix` anchor (the `StripDecisionMatrix` anchor) and the
   **SKILL-04 idle-cost cap of 1,536 chars**, **or**
2. a refgen override regenerated through `cmd/helix-refgen`,

and it must pass `go run ./cmd/helix-refgen --check` byte-for-byte. The
optimizer can PROPOSE; a human transcribes. The gate is on the committed
**artifact**, never the optimizer **process** (LLM output is not bit-reproducible).
