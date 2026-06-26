"""Shared real-run primitives: per-task workspace activation + agent run + grade.

DEV-TIME / OFFLINE ONLY — lives under tools/, NEVER linked into the helix
binary, `helix setup`, go.mod, or the default `go test ./...` path.

The single honest unit of work is `run_one(desc, steering_text, steering, ...)`:
  1. copy the pristine task dir into a throwaway sandbox (agent edits the copy),
  2. restore the gold tests from the agent-unwritable pristine source,
  3. `helix activate` the sandbox so the CLI file/edit verbs have a workspace
     (the Phase-113 activate.go fix makes `helix activate` set the file-tool
     workspace; without it the verbs return no_workspace),
  4. drive the Phase-107 ReAct agent (ON = candidate steering text, OFF = control)
     over the sandbox, metering real token cost via attribution.MeteredLLM,
  5. grade honestly via grade_aider.grade_task (0 tests => not a pass),
  6. clean up the sandbox.

Used by optimize.py (the GEPA program's forward) AND run_real.py (the ON/OFF
attribution). The agent shells `helix <verb>` as a subprocess — no Go, no new
deps; the no-runtime-Python / single-binary boundary is unchanged.
"""

import subprocess

# deepseek-v4-flash cache-miss pricing, USD per 1K tokens (research SUMMARY.md:
# $0.14 / $0.28 per 1M => /1000). MeteredLLM multiplies tokens/1000 by these.
DEEPSEEK_PRICE_PER_1K = {"prompt": 0.00014, "completion": 0.00028}


def activate_workspace(sandbox_dir, timeout=60):
    """`helix activate --workspace <sandbox>` so the CLI file/edit verbs resolve a
    workspace. Returns (ok, output). Best-effort: a failure is surfaced to the
    caller but does not raise (the agent will observe no_workspace and the grade
    will reflect it)."""
    proc = subprocess.run(
        ["helix", "activate", "--workspace", sandbox_dir],
        capture_output=True, text=True, timeout=timeout,
    )
    return proc.returncode == 0, (proc.stdout + proc.stderr)


def run_one(desc, steering_text, steering, max_turns=None, provider="deepseek"):
    """Run + grade one task. `desc` is a corpus descriptor
    {task, language, task_dir, gold_src, gold_tests}. `steering` is "on"|"off";
    when "on" the candidate `steering_text` is injected as the agent system
    prompt. Returns a dict {passed, tests_run, cost_usd, agent_reason, steps, transcript}."""
    from agent.react import ReActAgent, build_system_prompt, Transcript
    from agent.llm import LLM
    from sandbox import make_sandbox, restore_gold_tests, cleanup_sandbox
    from attribution import MeteredLLM
    import grade_aider

    sb = make_sandbox(desc["task_dir"])
    try:
        if desc.get("gold_tests"):
            restore_gold_tests(sb, desc["gold_src"], desc["gold_tests"])
        activate_workspace(sb)
        metered = MeteredLLM(LLM(provider=provider), price_per_1k=DEEPSEEK_PRICE_PER_1K)
        agent = ReActAgent(system_prompt=build_system_prompt(steering_text, steering), llm=metered)
        transcript = agent.run(desc["task"], cwd=sb, max_turns=max_turns)
        try:
            passed, tests_run = grade_aider.grade_task(sb, desc["language"])
        except grade_aider.GradeError:
            passed, tests_run = False, 0
        return {
            "passed": bool(passed),
            "tests_run": int(tests_run),
            "cost_usd": float(metered.cost_usd),
            "agent_reason": transcript.reason,
            "steps": len(transcript.steps),
            "transcript": transcript,  # HARNESS-04: Return full transcript for trace
        }
    finally:
        cleanup_sandbox(sb)
