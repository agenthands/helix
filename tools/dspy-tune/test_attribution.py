"""Hermetic tests for the ON/OFF attribution delta + ship/no-ship REPORT.

No LLM, no sandbox, no harness, no network: the pure core is driven with
injected per-arm ArmResults. Mirrors the flat, fixture-free, script-runnable
convention of test_grade.py / test_split.py.

Ships the Phase-109 anti-vacuity break-the-invariant proofs (TUNE-04):
  * the verdict is gated on the held-out split size — val_size==50 NO-SHIPS,
    val_size==51 may ship (the strict TUNE-03 boundary, shared with optimize.py).
  * a non-positive delta NO-SHIPS even on a large split (steering must measurably
    help, attributed ON-minus-OFF on the SAME split).
  * compute_attribution refuses unequal arms (an apples-to-oranges delta).
  * the REPORT records the delta, val_size, and per-arm cost, and keeps the
    human-gated-adoption note (Phase 110 owns the actual SKILL.md edit).
"""

import pytest

from attribution import (
    ArmResult,
    Attribution,
    MeteredLLM,
    compute_attribution,
    decide_ship,
    render_report,
)
from optimize import VAL_SIZE_GATE


def test_delta_is_on_minus_off():
    attr = Attribution(
        on=ArmResult("on", successes=40, n=51),
        off=ArmResult("off", successes=30, n=51),
        val_size=51,
    )
    assert attr.delta == pytest.approx(40 / 51 - 30 / 51)


def test_ship_when_positive_delta_and_big_split():
    attr = Attribution(
        on=ArmResult("on", successes=40, n=51, cost_usd=1.2),
        off=ArmResult("off", successes=30, n=51, cost_usd=1.0),
        val_size=51,
    )
    verdict, reason = decide_ship(attr)
    assert verdict == "ship" and "delta" in reason


def test_val_size_gate_boundary_50_noship_51_ship():
    # The strict TUNE-03 boundary (shared with optimize.adoption_allowed): 50
    # no-ships, 51 may ship. A positive delta on both so ONLY the gate differs.
    on = ArmResult("on", successes=10, n=50)
    off = ArmResult("off", successes=5, n=50)
    assert VAL_SIZE_GATE == 50
    assert decide_ship(Attribution(on=on, off=off, val_size=50))[0] == "no-ship"
    # Same positive delta but val_size 51 => may ship.
    on51 = ArmResult("on", successes=11, n=51)
    off51 = ArmResult("off", successes=5, n=51)
    assert decide_ship(Attribution(on=on51, off=off51, val_size=51))[0] == "ship"


def test_nonpositive_delta_noships_even_with_big_split():
    # Steering did not help (equal rates) on a large split => no-ship.
    attr = Attribution(
        on=ArmResult("on", successes=30, n=200),
        off=ArmResult("off", successes=30, n=200),
        val_size=200,
    )
    verdict, reason = decide_ship(attr)
    assert verdict == "no-ship" and "not strictly positive" in reason
    # A NEGATIVE delta also no-ships.
    worse = Attribution(
        on=ArmResult("on", successes=20, n=200),
        off=ArmResult("off", successes=30, n=200),
        val_size=200,
    )
    assert decide_ship(worse)[0] == "no-ship"


def test_compute_attribution_runs_both_arms_on_same_split():
    examples = list(range(60))
    seen = {}

    def run_arm(arm, exs):
        seen[arm] = list(exs)
        successes = 40 if arm == "on" else 30
        return ArmResult(arm, successes=successes, n=len(exs))

    attr = compute_attribution(examples, run_arm)
    assert attr.on.n == 60 and attr.off.n == 60
    assert seen["on"] == seen["off"] == examples, "both arms must see the SAME split"
    assert attr.val_size == 60
    assert attr.delta == pytest.approx(40 / 60 - 30 / 60)


def test_compute_attribution_refuses_unequal_arms():
    def run_arm(arm, exs):
        n = 50 if arm == "on" else 49  # mismatched splits
        return ArmResult(arm, successes=1, n=n)

    with pytest.raises(ValueError):
        compute_attribution(list(range(50)), run_arm)


def test_report_records_delta_valsize_cost_and_human_gate():
    attr = Attribution(
        on=ArmResult("on", successes=40, n=51, cost_usd=1.23),
        off=ArmResult("off", successes=30, n=51, cost_usd=0.98),
        val_size=51,
    )
    report = render_report(attr)
    assert "Attribution delta" in report
    assert "val_size: 51" in report or "val_size" in report
    assert "1.23" in report and "0.98" in report, "per-arm cost must be recorded"
    assert "SHIP" in report.upper()
    assert "helix-refgen --check" in report, "must keep the human-gated adoption note"


def test_metered_llm_records_real_cost_and_passes_message_through():
    class FakeUsage:
        prompt_tokens = 1000
        completion_tokens = 500

    class FakeMsg:
        content = "hi"
        usage = FakeUsage()

    class FakeLLM:
        def chat(self, messages, tools=None):
            return FakeMsg()

    metered = MeteredLLM(FakeLLM(), price_per_1k={"prompt": 0.001, "completion": 0.002})
    msg = metered.chat([{"role": "user", "content": "x"}])
    assert msg.content == "hi", "wrapper must return the underlying message verbatim"
    # 1000/1000*0.001 + 500/1000*0.002 = 0.001 + 0.001 = 0.002
    assert metered.cost_usd == pytest.approx(0.002)


def test_metered_llm_zero_cost_without_usage_block():
    class FakeMsg:
        content = "hi"  # no usage attribute

    class FakeLLM:
        def chat(self, messages, tools=None):
            return FakeMsg()

    metered = MeteredLLM(FakeLLM(), price_per_1k={"prompt": 0.001, "completion": 0.002})
    metered.chat([])
    assert metered.cost_usd == 0.0, "no usage block => honest 0.0, never fabricated"


if __name__ == "__main__":
    for name, obj in sorted(globals().items()):
        if name.startswith("test_") and callable(obj):
            obj()
    print("attribution OK")
