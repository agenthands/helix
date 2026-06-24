"""ON-vs-OFF steering attribution delta + ship/no-ship REPORT (dev-time only).

DEV-TIME / OFFLINE ONLY — lives under tools/, NEVER linked into the helix
binary, `helix setup`, go.mod, or the default `go test ./...` path.

TUNE-04: any reported improvement MUST be an honest attribution delta
    delta = success(ON) - success(OFF)
measured on the SAME held-out split, recorded alongside `val_size` and per-arm
cost. A bare "ON scored X%" is not a result — only the ON-minus-OFF difference
on the held-out split attributes the change to the steering text rather than to
the task difficulty or the model.

The pure core (`ArmResult`, `Attribution`, `decide_ship`, `render_report`) is
dependency-light and fully hermetic: the test drives it with injected per-arm
results, no LLM / sandbox / harness / network. `compute_attribution(...)` runs
each arm through an injected `run_arm(arm, examples) -> ArmResult` callable so
the grader (grade_aider OR grade_swebench — grader-agnostic) and the steering
ON/OFF system-prompt switch are wired by the caller. `MeteredLLM` is a
non-invasive wrapper that records real token cost when the provider returns a
usage block (the Phase-107 agent/LLM is untouched).
"""

from dataclasses import dataclass

# Single source of truth for the held-out-split adoption gate (TUNE-03).
from optimize import VAL_SIZE_GATE, adoption_allowed


@dataclass
class ArmResult:
    """One steering arm's outcome over the held-out split."""

    arm: str  # "on" | "off"
    successes: int
    n: int
    cost_usd: float = 0.0

    @property
    def success_rate(self):
        return (self.successes / self.n) if self.n else 0.0


@dataclass
class Attribution:
    """The ON-vs-OFF comparison on a held-out split of `val_size` tasks."""

    on: ArmResult
    off: ArmResult
    val_size: int

    @property
    def delta(self):
        """success(ON) - success(OFF). Positive ⇒ steering helped on this split."""
        return self.on.success_rate - self.off.success_rate

    @property
    def total_cost_usd(self):
        return self.on.cost_usd + self.off.cost_usd


def decide_ship(attr, val_size_gate=VAL_SIZE_GATE):
    """Ship/no-ship verdict. Adoption is allowed ONLY when BOTH hold:

      1. the held-out split is large enough (`val_size > val_size_gate`, the
         strict TUNE-03 gate — `val_size == gate` no-ships), AND
      2. the attribution delta is strictly positive (steering measurably helped).

    Returns (verdict: "ship"|"no-ship", reason: str). no-ship is a legitimate,
    success-meeting outcome — never crash, never overstate.
    """
    if not adoption_allowed(attr.val_size):
        return (
            "no-ship",
            f"val_size={attr.val_size} <= {val_size_gate}: the held-out split is "
            f"too small for a trustworthy delta (the v2.2 no-ship cause). Grow the "
            f"corpus so val_size > {val_size_gate}.",
        )
    if attr.delta <= 0:
        return (
            "no-ship",
            f"attribution delta = {attr.delta:+.4f} (ON {attr.on.success_rate:.4f} "
            f"− OFF {attr.off.success_rate:.4f}) is not strictly positive: steering "
            f"did not measurably help on the held-out split.",
        )
    return (
        "ship",
        f"attribution delta = {attr.delta:+.4f} (ON {attr.on.success_rate:.4f} "
        f"− OFF {attr.off.success_rate:.4f}) > 0 on a held-out split of "
        f"{attr.val_size} (> {val_size_gate}) tasks.",
    )


def compute_attribution(examples, run_arm, val_size=None):
    """Run the held-out split through the OFF then ON arm and build the
    Attribution. `run_arm(arm, examples) -> ArmResult` is injected so the agent /
    grader / sandbox wiring lives in the caller (and the test injects fakes).

    Both arms see the SAME `examples` — that is what makes the delta an honest
    attribution. `val_size` defaults to len(examples)."""
    off = run_arm("off", examples)
    on = run_arm("on", examples)
    if off.n != on.n:
        raise ValueError(
            f"ON and OFF arms must score the SAME held-out split (off n={off.n}, "
            f"on n={on.n}) — an unequal split makes the delta uninterpretable."
        )
    return Attribution(on=on, off=off, val_size=val_size if val_size is not None else len(examples))


def render_report(attr, verdict=None, reason=None):
    """Render the ship/no-ship REPORT (markdown) recording the ON/OFF rates, the
    delta, val_size, and per-arm cost — the TUNE-04 deliverable. If `verdict`/
    `reason` are omitted they are computed via decide_ship."""
    if verdict is None or reason is None:
        verdict, reason = decide_ship(attr)
    lines = [
        "# v2.3 Steering ON/OFF Attribution REPORT",
        "",
        f"**Verdict: {verdict.upper()}** — {reason}",
        "",
        "| Arm | success_rate | successes / n | cost (USD) |",
        "|-----|-------------|---------------|------------|",
        f"| OFF | {attr.off.success_rate:.4f} | {attr.off.successes} / {attr.off.n} | {attr.off.cost_usd:.4f} |",
        f"| ON  | {attr.on.success_rate:.4f} | {attr.on.successes} / {attr.on.n} | {attr.on.cost_usd:.4f} |",
        "",
        f"- **Attribution delta (ON − OFF): {attr.delta:+.4f}**",
        f"- Held-out val_size: {attr.val_size} (TUNE-03 gate: > {VAL_SIZE_GATE})",
        f"- Total per-arm cost: OFF {attr.off.cost_usd:.4f} + ON {attr.on.cost_usd:.4f} = {attr.total_cost_usd:.4f} USD",
        "",
        "> Adoption (if SHIP) is still human-gated: re-enters the shipped surface",
        "> ONLY via a human-reviewed skill edit passing `helix-refgen --check`",
        "> (Phase 110). The optimizer never auto-writes the shipped skill/doc text.",
    ]
    return "\n".join(lines) + "\n"


class MeteredLLM:
    """Non-invasive cost meter wrapping a Phase-107 `agent.llm.LLM` (or any
    duck-typed `.chat(messages, tools)`), recording REAL token cost when the
    provider returns a usage block. The wrapped LLM is UNTOUCHED — this only
    observes. `cost_usd` accumulates across calls.

    `price_per_1k` maps (kind) -> USD per 1K tokens; default 0.0 so cost is an
    honest 0.0 when no price table is supplied rather than a fabricated number.
    The wrapper returns the underlying message verbatim so the agent loop sees no
    difference. Usage is read defensively (getattr) so a fake message without a
    usage block does not crash — it simply contributes 0 cost.
    """

    def __init__(self, inner, price_per_1k=None):
        self._inner = inner
        self._price = price_per_1k or {"prompt": 0.0, "completion": 0.0}
        self.cost_usd = 0.0

    def chat(self, messages, tools=None):
        msg = self._inner.chat(messages, tools=tools)
        usage = getattr(msg, "usage", None) or getattr(self._inner, "last_usage", None)
        if usage is not None:
            prompt = getattr(usage, "prompt_tokens", 0) or 0
            completion = getattr(usage, "completion_tokens", 0) or 0
            self.cost_usd += (prompt / 1000.0) * self._price.get("prompt", 0.0)
            self.cost_usd += (completion / 1000.0) * self._price.get("completion", 0.0)
        return msg


def _main():
    """Gated live entry point (dev-time only). Mirrors optimize.py's guards: a
    missing LM key or absent corpus prints an informative message and exits 0
    (CI / executor friendly, hermetic gates are key-free) — it NEVER fabricates a
    delta. The actual ship/no-ship REPORT artifact (filled with real numbers) is
    the Phase-110 deliverable; this entry point wires the run that produces it.

    The held-out split is the sequestered `data/test.jsonl` (NEVER passed to
    optimizer.compile). A real run drives the Phase-107 agent ON and OFF over that
    split, grades via grade_aider / grade_swebench, meters per-arm cost, and
    renders the REPORT to the git-ignored output/ dir.
    """
    import os

    here = os.path.dirname(os.path.abspath(__file__))
    if not (os.environ.get("DEEPSEEK_API_KEY") or os.environ.get("OPENAI_API_KEY")):
        print(
            "No LM key (DEEPSEEK_API_KEY / OPENAI_API_KEY) set — the ON/OFF "
            "attribution run is dev-time/offline only and needs a key.\n"
            "The hermetic gates run WITHOUT a key:\n"
            "  uv run pytest test_attribution.py test_swebench.py\n"
            "no-ship is a legitimate, success-meeting outcome (README.md / REPORT.md)."
        )
        return 0
    if not (os.environ.get("AIDER_TASKS_DIR") or os.environ.get("SWEBENCH_INSTANCES")):
        print(
            "No held-out task corpus set (AIDER_TASKS_DIR for the Aider grader, "
            "SWEBENCH_INSTANCES for the SWE-bench grader) — refusing to fabricate "
            "an attribution delta. The corpus is still below the val_size>50 gate "
            "(the v2.2 no-ship cause / TUNE-FUT-01). no-ship is a legitimate, "
            "success-meeting outcome."
        )
        return 0
    # A real wired run lands with the grown corpus (TUNE-FUT-01) + Phase 110.
    # Until then we refuse rather than emit a fabricated REPORT.
    print(
        "Live attribution wiring requires a task-success corpus sized so "
        f"val_size > {VAL_SIZE_GATE}; the committed corpus is below it. "
        "Grow the corpus (TUNE-FUT-01) before a real ON/OFF run. no-ship stands."
    )
    _ = here
    return 0


if __name__ == "__main__":
    import sys

    sys.exit(_main())
