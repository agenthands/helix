"""Task-success GEPA metric (dev-time only): run the agent, grade, score.

DEV-TIME / OFFLINE ONLY — lives under tools/, NEVER linked into the helix
binary, `helix setup`, go.mod, or the default `go test ./...` path.

This replaces `scorer.score_choice_rate` as the GEPA optimization signal
(TUNE-02): the score is REAL Aider-polyglot task-success (hidden tests green in
a sandbox), not the gameable first-command proxy. `choice_rate` is retained
ONLY as a non-optimized diagnostic pre-screen (see optimize.py), never a
co-optimized reward.

The core `score_task(...)` is pure and dependency-light so the hermetic test
drives it with a fake agent + fake grader (no dspy, no network, no toolchain).
`make_gepa_metric(...)` wraps it into the dspy.Prediction(score, feedback)
shape lazily, so this module imports without dspy installed.
"""


def score_task(task, agent_runner, grader, on_error="zero"):
    """Score one task by REAL task-success.

    * `agent_runner(task) -> bool`: drives the Phase-107 ReAct agent against the
      task's sandbox and returns whether it produced a candidate solution
      (it does NOT decide success — the grader does).
    * `grader() -> (passed: bool, tests_run: int)`: runs the native hidden tests
      in the sandbox (grade_aider.grade_task) and may raise GradeError on a
      0-test vacuous run.

    Returns (score: float in {0.0, 1.0}, feedback: str). A GradeError (0 tests
    ran / unparseable) scores 0.0 with explicit feedback — a vacuous run is NOT
    a success. Honest: the score is the grader's verdict, never the agent's
    self-report.
    """
    try:
        agent_runner(task)
    except Exception as exc:  # agent crash is a failure, surfaced in feedback
        return 0.0, f"agent run failed: {exc}"

    try:
        passed, tests_run = grader()
    except Exception as exc:
        # GradeError (0 tests ran) or any grading failure => 0.0, never a pass.
        if on_error == "raise":
            raise
        return 0.0, f"grading failed (no honest pass possible): {exc}"

    if passed:
        return 1.0, f"task solved: {tests_run} test(s) passed in the sandbox."
    return 0.0, f"task NOT solved: {tests_run} test(s) ran, not all green."


def make_gepa_metric(agent_runner_for, grader_for):
    """Build a GEPA-shaped metric closure. `agent_runner_for(example)` and
    `grader_for(example)` resolve the per-example runner/grader. Returns a
    function with the GEPA metric signature returning dspy.Prediction."""
    import dspy  # lazy: this module must import without dspy installed

    def metric(gold, pred, trace=None, pred_name=None, pred_trace=None):
        score, feedback = score_task(gold, agent_runner_for(gold), grader_for(gold))
        return dspy.Prediction(score=score, feedback=feedback)

    return metric
