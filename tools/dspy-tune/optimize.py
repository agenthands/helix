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

# Dev-time LM backend (SCALE-01). The optimizer's program LM AND reflection_lm
# are DeepSeek-primary / OpenAI-fallback, consistent with the Phase-107 agent
# (agent/llm.py). The model is a config var (DSPY_LM_MODEL); the default is the
# EXPLICIT successor id `deepseek-v4-flash` — the `deepseek-chat`/`deepseek-reasoner`
# aliases retire 2026-07-24 15:59 UTC (and `deepseek-reasoner` has no tool-calling),
# so pinning the explicit id makes the cutover a one-line change.
DEFAULT_LM_MODEL = "deepseek-v4-flash"
DEFAULT_FALLBACK_MODEL = "gpt-4.1-mini"
DEEPSEEK_BASE_URL = "https://api.deepseek.com"
# 429 backoff: DeepSeek limits by concurrency (HTTP 429), so the LM retries with
# litellm's exponential backoff (SCALE-02).
LM_NUM_RETRIES = 4


class LMConfig:
    """Resolved LM backend for the optimizer (program LM + reflection_lm)."""

    def __init__(self, model, api_key, api_base, num_retries=LM_NUM_RETRIES):
        self.model = model
        self.api_key = api_key
        self.api_base = api_base
        self.num_retries = num_retries


def resolve_lm(environ=None):
    """Resolve the optimizer LM: DeepSeek-primary / OpenAI-fallback (SCALE-01).

    Returns an LMConfig, or None when no usable key is set (the caller then takes
    the quiet hermetic-gate exit-0 path — symmetric with the agent's loud-fail but
    on the OPTIMIZER side, which must stay CI/executor-safe). Model precedence:
    explicit `DSPY_LM_MODEL` > `DEFAULT_LM_MODEL`. A bare `deepseek-*` id selects
    the DeepSeek provider when its key is present; otherwise it falls back to
    OpenAI; an explicit provider-prefixed id (`openai/…`, `deepseek/…`) is honored.
    """
    environ = environ if environ is not None else os.environ
    raw = environ.get("DSPY_LM_MODEL") or DEFAULT_LM_MODEL
    deepseek_key = environ.get("DEEPSEEK_API_KEY")
    openai_key = environ.get("OPENAI_API_KEY")
    wants_deepseek = raw.startswith("deepseek")  # "deepseek-v4-flash" or "deepseek/..."

    if wants_deepseek and deepseek_key:
        model = raw if raw.startswith("deepseek/") else f"deepseek/{raw}"
        return LMConfig(model=model, api_key=deepseek_key, api_base=DEEPSEEK_BASE_URL)
    # Fallback (or an explicitly OpenAI-pinned model) → OpenAI.
    if openai_key:
        if raw.startswith("openai/"):
            model = raw
        elif wants_deepseek:
            model = f"openai/{DEFAULT_FALLBACK_MODEL}"  # default deepseek id, but only openai key
        elif "/" in raw:
            model = raw  # some other litellm provider the user pinned explicitly
        else:
            model = f"openai/{raw}"
        return LMConfig(model=model, api_key=openai_key, api_base=None)
    return None


def build_gepa_kwargs(environ=None):
    """Cost-bounded GEPA kwargs (SCALE-02). Default `auto="light"` (~600 rollouts);
    an explicit `GEPA_MAX_METRIC_CALLS` replaces it with a hard rollout cap.
    `num_threads` is bounded (default 4, `GEPA_NUM_THREADS`) so the in-process
    evaluation never opens unbounded concurrent LM connections (DeepSeek 429).
    `seed` is fixed for reproducibility; `track_stats` on for the attribution report.
    """
    environ = environ if environ is not None else os.environ
    kwargs = {"track_stats": True, "seed": 0}
    max_calls = environ.get("GEPA_MAX_METRIC_CALLS")
    if max_calls:
        kwargs["max_metric_calls"] = int(max_calls)  # explicit hard cap wins over auto
    else:
        kwargs["auto"] = "light"
    num_threads = int(environ.get("GEPA_NUM_THREADS") or 4)
    kwargs["num_threads"] = max(1, min(num_threads, 8))  # bounded, never unbounded
    return kwargs

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
    # SCALE-01: DeepSeek-primary / OpenAI-fallback (config var DSPY_LM_MODEL,
    # default deepseek-v4-flash). None ⇒ no usable key ⇒ quiet hermetic-gate exit-0.
    lm_cfg = resolve_lm()
    if lm_cfg is None:
        # Dev-env-only guard: never crash when no key is present (CI / executor).
        print(
            "No LM key set (DEEPSEEK_API_KEY primary / OPENAI_API_KEY fallback).\n"
            "The GEPA optimization run is dev-time/offline only and needs an LM "
            "key.\nThe hermetic gates run WITHOUT a key:\n"
            "  uv run pytest test_split.py test_parity.py test_degenerate.py "
            "test_corpus.py test_scale.py\n"
            "Set DEEPSEEK_API_KEY (and optionally DSPY_LM_MODEL) in your dev shell "
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

    def _build_lm(temperature=None):
        kw = {"model": lm_cfg.model, "api_key": lm_cfg.api_key, "num_retries": lm_cfg.num_retries}
        if lm_cfg.api_base:
            kw["api_base"] = lm_cfg.api_base  # litellm passthrough (deepseek base url)
        if temperature is not None:
            kw["temperature"] = temperature
        return dspy.LM(**kw)

    dspy.configure(lm=_build_lm())

    # The Signature's instruction text IS the evolving steering artifact GEPA
    # mutates. AgentProgram.forward injects it into the REAL Phase-107 agent as the
    # ON steering text (the candidate→agent thread — without this every candidate
    # scored identically and the optimization was vacuous). The agent shells
    # `helix <verb>` over a per-task sandbox and the native hidden tests grade it.
    class Solve(dspy.Signature):
        """Drive the helix CLI verbs to make the task's hidden tests pass. Prefer
        the symbolic verbs (read-file, search-symbols, fuzzy-edit, ...) over
        guessing; read before you edit."""

        task = dspy.InputField()
        response = dspy.OutputField(desc="the solution / final answer")

    _gepa_max_turns = int(os.environ.get("AGENT_MAX_TURNS") or 8)

    class AgentProgram(dspy.Module):
        def __init__(self):
            super().__init__()
            self.solve = dspy.Predict(Solve)

        def forward(self, task, task_dir="", gold_src="", gold_tests=None, language="python"):
            import runlib

            steering = self.solve.signature.instructions or ""
            desc = {
                "task": task, "task_dir": task_dir, "gold_src": gold_src,
                "gold_tests": gold_tests or [], "language": language,
            }
            r = runlib.run_one(desc, steering, "on", max_turns=_gepa_max_turns)
            fb = (
                f"task solved: {r['tests_run']} test(s) passed"
                if r["passed"]
                else f"task NOT solved: {r['tests_run']} test(s) ran, not all green"
            )
            # HARNESS-04: Build trace from transcript for GEPA reflective mutation.
            # The trace shows step-level agent behavior that GEPA can reflect on.
            transcript = r.get("transcript")
            trace_steps = []
            if transcript:
                trace_steps = [
                    {"verb": s.verb, "argv": s.argv, "exit": s.exit, "stdout": s.stdout[:200] if s.stdout else ""}
                    for s in transcript.steps
                ]
            return dspy.Prediction(
                passed=r["passed"],
                tests_run=r["tests_run"],
                feedback=fb,
                trace=trace_steps,  # HARNESS-04: emit trace for GEPA reflection
            )

    program = AgentProgram()

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

    # The metric reads the program's prediction: AgentProgram.forward already ran
    # the real agent (ON, with the candidate steering) over the task's sandbox and
    # graded it (honest: 0 tests => not passed). GEPA evolves the Solve instruction
    # (the steering), so candidates now score differently — non-vacuous.
    def metric(gold, pred, trace=None, pred_name=None, pred_trace=None):
        passed = bool(getattr(pred, "passed", False))
        return dspy.Prediction(
            score=1.0 if passed else 0.0,
            feedback=getattr(pred, "feedback", "no prediction"),
        )

    optimizer = GEPA(
        metric=metric,
        reflection_lm=_build_lm(temperature=1.0),
        **build_gepa_kwargs(),  # SCALE-02: rollout cap + bounded num_threads + seed
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
            ).with_inputs("task", "task_dir", "gold_src", "gold_tests", "language")
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
