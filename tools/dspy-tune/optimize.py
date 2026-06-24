"""DSPy GEPA offline tuning harness (dev-time only).

Evolves a single agent-facing steering instruction against the parity-checked
adoption metric (scorer.score_choice_rate, a faithful Python mirror of the Go
test/oracle/adopt classifier). This file lives under tools/ — it is NEVER
linked into the helix binary, `helix setup`, go.mod, or `go test ./...`.

Quarantine invariants enforced here:
  * The LM key is read from the DEV environment ONLY (OPENAI_API_KEY). If it is
    unset (as in CI / the executor environment), the harness prints an
    informative message and exits 0 — it does NOT crash. The hermetic gates
    (test_split / test_parity / test_degenerate) are LM-free and run without it.
  * The held-out TEST split (data/test.jsonl) is sequestered: GEPA's
    trainset/valset are drawn ONLY from data/train.jsonl (val carved from
    train). test.jsonl is loaded solely for a final held-out report and is
    NEVER passed to optimizer.compile().
  * Output is written to a git-ignored output/optimized.json. This file MUST NOT
    write to the embedded skill bundle or the generated reference doc. Adopted
    text re-enters the shipped surface ONLY via a human-reviewed SKILL.md edit
    (<=1536 chars, "## Decision matrix" anchor preserved) or a refgen override
    passing `go run ./cmd/helix-refgen --check`. The raw dump is never pasted.
"""

import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
TRAIN_PATH = os.path.join(HERE, "data", "train.jsonl")
TEST_PATH = os.path.join(HERE, "data", "test.jsonl")
OUTPUT_PATH = os.path.join(HERE, "output", "optimized.json")  # git-ignored

# Dev-time LM backend. Illustrative; any litellm-supported provider works.
LM_MODEL = os.environ.get("DSPY_LM_MODEL", "openai/gpt-4.1-mini")


def _load_jsonl(path):
    rows = []
    with open(path, encoding="utf-8") as fh:
        for line in fh:
            line = line.strip()
            if line:
                rows.append(json.loads(line))
    return rows


def main():
    api_key = os.environ.get("OPENAI_API_KEY")
    if not api_key:
        # Dev-env-only guard: never crash when the key is absent (CI / executor).
        print(
            "OPENAI_API_KEY is not set in the dev environment.\n"
            "The GEPA optimization run is dev-time/offline only and needs an LM "
            "key.\nThe hermetic gates run WITHOUT a key:\n"
            "  pytest tools/dspy-tune/test_split.py tools/dspy-tune/test_parity.py "
            "tools/dspy-tune/test_degenerate.py\n"
            "Set OPENAI_API_KEY (and optionally DSPY_LM_MODEL) in your dev shell "
            "to run the optimization.\n"
            "NOTE: no-ship is a legitimate, success-meeting outcome (see "
            "README.md / REPORT.md)."
        )
        return 0

    # Imported lazily so the unset-key guard above works even if dspy is not
    # installed in the current (non-dev) environment.
    import dspy
    from dspy import GEPA

    from scorer import score_choice_rate

    dspy.configure(lm=dspy.LM(model=LM_MODEL, api_key=api_key))

    class Steer(dspy.Signature):
        """Answer the coding-intel task by emitting the single best command."""

        task = dspy.InputField()
        response = dspy.OutputField(desc="the single command line to run")

    program = dspy.Predict(Steer)

    def adopt_metric(gold, pred, trace=None, pred_name=None, pred_trace=None):
        # pred.response is the model's emitted transcript; score 1.0 iff the
        # FIRST emitted command is a helix verb (the parity-checked classifier).
        chose, _fell = score_choice_rate(pred.response)
        score = 1.0 if chose else 0.0
        feedback = (
            "Emitted a helix verb first (good)."
            if chose
            else "Fell back to grep/sed/cat — steer toward a helix verb."
        )
        return dspy.Prediction(score=score, feedback=feedback)

    # trainset/valset drawn ONLY from TRAIN (val carved from train). TEST is
    # sequestered and NEVER passed to compile().
    train_rows = _load_jsonl(TRAIN_PATH)
    examples = [
        dspy.Example(task=r["task"]).with_inputs("task") for r in train_rows
    ]
    # A real train/val split needs at least 2 TRAIN tasks: one to optimize on,
    # one to validate on. With fewer, fail loudly rather than collapse val into
    # train. The previous `examples[split:] or examples[:split]` fallback (WR-03)
    # silently set valset == trainset when len(examples) == 1 (split=1, so
    # examples[1:] == [] and the `or` substituted examples[:1]), making GEPA
    # validate on its own training example — a degenerate, overfit-prone config
    # with no error. Guard it explicitly and keep trainset/valset DISJOINT.
    if len(examples) < 2:
        print(
            "TRAIN has <2 tasks; cannot form a disjoint train/val split. "
            "Add tasks to data/train.jsonl before optimizing.\n"
            "no-ship is a legitimate outcome (see README.md / REPORT.md)."
        )
        return 0
    split = max(1, len(examples) // 2)
    trainset, valset = examples[:split], examples[split:]
    # Disjointness is structural here (examples[:split] and examples[split:]
    # partition the list), but assert it so a future refactor that reintroduces
    # an overlapping carve fails loudly instead of silently overfitting.
    assert valset, "valset must be non-empty after the >=2-task guard"
    _train_ids = {id(e) for e in trainset}
    assert not any(id(e) in _train_ids for e in valset), (
        "trainset and valset must be disjoint (no example may appear in both)"
    )

    optimizer = GEPA(
        metric=adopt_metric,
        auto="light",
        track_stats=True,
        reflection_lm=dspy.LM(model=LM_MODEL, temperature=1.0, api_key=api_key),
    )
    optimized = optimizer.compile(student=program, trainset=trainset, valset=valset)

    os.makedirs(os.path.dirname(OUTPUT_PATH), exist_ok=True)
    # git-ignored; re-enters the shipped surface ONLY via a human-reviewed
    # SKILL.md/refgen commit passing `helix-refgen --check`. This script does NOT
    # write the embedded skill bundle or the generated reference doc.
    optimized.save(OUTPUT_PATH)

    # Final HELD-OUT report only: load TEST purely to report the optimized
    # choice_rate on data the optimizer never saw. Not passed to compile().
    test_rows = _load_jsonl(TEST_PATH)
    print(f"train={len(trainset)} val={len(valset)} test(held-out)={len(test_rows)}")
    print(f"Saved optimized program to {OUTPUT_PATH} (git-ignored).")
    print(
        "Re-entry is human-review-only: transcribe adopted steering into the "
        "embedded SKILL.md (<=1536 chars, '## Decision matrix' anchor "
        "preserved) and pass `go run ./cmd/helix-refgen --check`. "
        "no-ship is a legitimate outcome."
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
