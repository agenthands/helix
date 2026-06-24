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

# TUNE-03: the held-out validation split must exceed this many tasks before ANY
# adoption recommendation is trustworthy. This is the explicit fix for the v2.2
# no-ship root cause (corpus too small: MinTasks=5, val≈3). A strict `> 50`
# boundary: val_size==50 NO-SHIPS, val_size>=51 may be adopted (see test_split.py
# anti-vacuity: 50 no-ship / 51 adoptable). Honors TUNE-FUT-01.
VAL_SIZE_GATE = 50

# Path to an Aider-polyglot task-success corpus (a dir of exercise task dirs,
# each with a `language` and gold tests). When set, GEPA optimizes REAL
# task-success (taskmetric.score_task via grade_aider); when unset, the harness
# refuses to optimize on the gameable choice_rate proxy and reports no-ship.
AIDER_TASKS_DIR = os.environ.get("AIDER_TASKS_DIR", "")


def adoption_allowed(val_size):
    """TUNE-03 gate: adoption is permissible ONLY when the held-out validation
    split exceeds VAL_SIZE_GATE. Strict `>` so val_size==50 no-ships."""
    return val_size > VAL_SIZE_GATE


def _task_key(ex):
    """Stable identity for an example across dspy.Example / dict / str forms.
    Prefers the unique `task_dir` (so two exercises that happen to share identical
    instructions text are not falsely merged), falling back to `task`. Used by the
    disjointness guard so the sequestered split check works whether optimize runs
    with dspy Examples or the hermetic test's plain dicts."""
    if isinstance(ex, dict):
        return ex.get("task_dir") or ex.get("task")
    return getattr(ex, "task_dir", None) or getattr(ex, "task", ex)


def _disjoint_ok(train, val, test):
    """True iff train / val / test are pairwise disjoint by task key. The
    sequestered held-out (`test`) split MUST share no task with train or val —
    GEPA reflects on train and selects candidates on val (val is leaked into
    selection), so a leak into either makes the held-out delta untrustworthy
    (research SUMMARY.md). A planted leak turns this False (test_corpus.py)."""
    ktrain = {_task_key(e) for e in train}
    kval = {_task_key(e) for e in val}
    ktest = {_task_key(e) for e in test}
    return ktrain.isdisjoint(kval) and ktrain.isdisjoint(ktest) and kval.isdisjoint(ktest)


def split_corpus(examples, heldout=None, val_frac=0.5):
    """3-way disjoint split of the Aider reward corpus (CORPUS-02).

    Returns (train, val, test). `train` + `val` are passed to `optimizer.compile()`
    (train = reflective updates, val = Pareto candidate selection). `test` is the
    SEQUESTERED held-out attribution split — carved FIRST and locked (spike
    discipline: the held-out split exists before any optimization) and NEVER
    passed to compile(); attribution.py runs the honest ON/OFF delta over it.

    The held-out split is sized to clear the strict `val_size > 50` adoption gate
    when the corpus is large enough (default `max(VAL_SIZE_GATE+1, n//3)`), while
    leaving at least one train and one val example.
    """
    n = len(examples)
    if heldout is None:
        heldout = max(VAL_SIZE_GATE + 1, n // 3)
    heldout = max(0, min(heldout, n - 2))  # always leave >=2 for train+val
    test = examples[:heldout]
    remainder = examples[heldout:]
    if len(remainder) >= 2:
        k = max(1, min(len(remainder) - 1, int(round(len(remainder) * (1 - val_frac)))))
    else:
        k = len(remainder)
    train, val = remainder[:k], remainder[k:]
    return train, val, test


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

    # choice_rate is now a DIAGNOSTIC PRE-SCREEN ONLY (TUNE-02): it is never the
    # GEPA optimization reward. The optimization reward is REAL Aider-polyglot
    # task-success (taskmetric.score_task via grade_aider), which requires a
    # task-success corpus at AIDER_TASKS_DIR.
    if not AIDER_TASKS_DIR:
        # task-success optimization needs a real Aider corpus; choice_rate
        # (scorer.py) remains available as a diagnostic pre-screen but is never
        # the optimization reward.
        test_rows = _load_jsonl(TEST_PATH)
        print(
            "AIDER_TASKS_DIR is not set — refusing to optimize on the gameable "
            "choice_rate proxy (the v2.2 no-ship cause). choice_rate is a "
            "DIAGNOSTIC pre-screen only; the GEPA reward is real task-success on "
            "an Aider-polyglot corpus.\n"
            f"Held-out TEST tasks available for a future task-success run: {len(test_rows)}.\n"
            "Set AIDER_TASKS_DIR to a task-success corpus to optimize. "
            "no-ship is a legitimate, success-meeting outcome (README.md / REPORT.md)."
        )
        return 0

    # Imported lazily so the unset-key guard above works even if dspy is not
    # installed in the current (non-dev) environment.
    import dspy
    from dspy import GEPA

    from taskmetric import make_gepa_metric

    dspy.configure(lm=dspy.LM(model=LM_MODEL, api_key=api_key))

    class Solve(dspy.Signature):
        """Solve the coding task by driving helix verbs to a passing solution."""

        task = dspy.InputField()
        response = dspy.OutputField(desc="the solution / final answer")

    program = dspy.Predict(Solve)

    # Load the Aider task-success corpus, split train/val (val carved from train).
    # TEST stays sequestered and is NEVER passed to compile().
    examples = _load_aider_corpus(dspy, AIDER_TASKS_DIR)
    if len(examples) < 2:
        print(
            "Aider corpus has <2 tasks; cannot form a disjoint train/val split. "
            "no-ship is a legitimate outcome (README.md / REPORT.md)."
        )
        return 0
    # 3-way disjoint split (CORPUS-02): train + val feed compile(); `heldout` is
    # the SEQUESTERED attribution split, carved first and NEVER passed to compile.
    trainset, valset, heldout = split_corpus(examples)
    assert trainset and valset, "train and val must be non-empty after split"
    assert _disjoint_ok(trainset, valset, heldout), (
        "train/val/heldout must be pairwise disjoint (sequestered held-out split leaked)"
    )

    # TUNE-03 ADOPTION GATE on the SEQUESTERED held-out attribution split. Research
    # (SUMMARY.md): GEPA leaks valset into candidate selection, so the trustworthy
    # gate is the split compile() never sees. Refuse before burning a GEPA run.
    if not adoption_allowed(len(heldout)):
        print(
            f"held-out val_size={len(heldout)} <= {VAL_SIZE_GATE}: NO-SHIP. The "
            f"sequestered held-out split is too small for a trustworthy task-success "
            f"delta (the v2.2 no-ship cause). Grow the Aider corpus so the held-out "
            f"split > {VAL_SIZE_GATE} (TUNE-FUT-01) before any adoption recommendation. "
            f"no-ship is a legitimate, success-meeting outcome."
        )
        return 0

    # Persist the sequestered held-out descriptors (git-ignored) so attribution.py
    # runs the honest ON/OFF delta over the EXACT split compile() never saw.
    os.makedirs(os.path.dirname(OUTPUT_PATH), exist_ok=True)
    heldout_path = os.path.join(os.path.dirname(OUTPUT_PATH), "heldout_test.json")
    with open(heldout_path, "w", encoding="utf-8") as _fh:
        json.dump(
            [
                {
                    "task": e.task,
                    "language": e.language,
                    "task_dir": getattr(e, "task_dir", ""),
                    "gold_src": getattr(e, "gold_src", ""),
                    "gold_tests": getattr(e, "gold_tests", []),
                }
                for e in heldout
            ],
            _fh,
            indent=2,
        )

    metric = make_gepa_metric(
        agent_runner_for=lambda ex: _agent_runner_for_example(ex),
        grader_for=lambda ex: _grader_for_example(ex),
    )
    optimizer = GEPA(
        metric=metric,
        auto="light",
        track_stats=True,
        reflection_lm=dspy.LM(model=LM_MODEL, temperature=1.0, api_key=api_key),
    )
    optimized = optimizer.compile(student=program, trainset=trainset, valset=valset)

    os.makedirs(os.path.dirname(OUTPUT_PATH), exist_ok=True)
    # git-ignored; re-enters the shipped surface ONLY via a human-reviewed
    # SKILL.md/refgen commit passing `helix-refgen --check`.
    optimized.save(OUTPUT_PATH)
    print(
        f"train={len(trainset)} val={len(valset)} heldout={len(heldout)} "
        f"(held-out > {VAL_SIZE_GATE} gate passed; held-out sequestered at {heldout_path})"
    )
    print(f"Saved optimized program to {OUTPUT_PATH} (git-ignored).")
    print(
        "Re-entry is human-review-only: transcribe adopted steering into the "
        "embedded SKILL.md (<=1536 chars, '## Decision matrix' anchor "
        "preserved) and pass `go run ./cmd/helix-refgen --check`."
    )
    return 0


def _load_aider_corpus(dspy, tasks_dir):
    """Load Aider task-success examples from a corpus dir. Each task is a JSON
    descriptor (task prompt + language + gold-test rel paths). Returns dspy
    Examples. Dev-time only; the live corpus is wired in a future milestone."""
    examples = []
    for name in sorted(os.listdir(tasks_dir)):
        path = os.path.join(tasks_dir, name)
        if not name.endswith(".json") or name == "corpus_manifest.json":
            continue  # skip the provenance manifest (not a task descriptor)
        with open(path, encoding="utf-8") as fh:
            spec = json.load(fh)
        if "task" not in spec:
            continue  # defensive: only consume well-formed descriptors
        examples.append(
            dspy.Example(
                task=spec["task"],
                language=spec["language"],
                task_dir=spec.get("task_dir", ""),
                gold_src=spec.get("gold_src", ""),
                gold_tests=spec.get("gold_tests", []),
            ).with_inputs("task")
        )
    return examples


def _agent_runner_for_example(ex):
    """Return an agent_runner(task) closure that drives the Phase-107 ReAct agent
    in a sandbox for this example. Live dev-time wiring (needs an LM key)."""
    def _run(_task):
        from agent import LLM, ReActAgent, build_system_prompt
        from sandbox import make_sandbox, restore_gold_tests
        llm = LLM(provider="deepseek")
        sb = make_sandbox(ex.task_dir)
        if ex.gold_tests:
            restore_gold_tests(sb, ex.gold_src, ex.gold_tests)
        agent = ReActAgent(system_prompt=build_system_prompt("", "off"), llm=llm)
        ex._sandbox = sb  # grader reads it
        return agent.run(ex.task, cwd=sb)
    return _run


def _grader_for_example(ex):
    """Return a grader() closure that runs the native hidden tests in the
    example's sandbox and grades honestly (0 tests => GradeError)."""
    def _grade():
        from grade_aider import grade_task
        return grade_task(getattr(ex, "_sandbox", ex.task_dir), ex.language)
    return _grade


if __name__ == "__main__":
    sys.exit(main())
