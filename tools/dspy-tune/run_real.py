"""Real cost-aware run driver: smoke + ON/OFF attribution (dev-time only).

DEV-TIME / OFFLINE ONLY — lives under tools/, NEVER linked into the helix binary,
`helix setup`, go.mod, or the default `go test ./...` path.

Modes:
  smoke [N]       — run the agent OFF on N corpus tasks for real + grade (de-risk).
  attribution     — RUN-02: run the agent ON (candidate steering = the embedded
                    SKILL.md) vs OFF (control) over the SEQUESTERED held-out split,
                    metering per-arm cost, and write the honest ship/no-ship
                    artifacts (output/attribution.json + output/REPORT-RUN.md).

Honest by construction: the score is the grader's verdict (0 tests => not a pass),
the delta is ON−OFF on the SAME held-out split, a low/zero/negative delta is a
valid NO-SHIP, and per-arm cost is real (metered token usage). The held-out split
is loaded from the git-ignored output/heldout_test.json when present (written by
optimize.py), else carved from the corpus via the same split_corpus (so the split
is identical to what compile() never saw).
"""

import json
import os
import sys

import runlib
from optimize import split_corpus, VAL_SIZE_GATE
from attribution import ArmResult, compute_attribution, decide_ship, render_report

_HERE = os.path.dirname(os.path.abspath(__file__))
_CORPUS = os.path.join(_HERE, "corpus")
_OUT = os.path.join(_HERE, "output")
_HELDOUT = os.path.join(_OUT, "heldout_test.json")
_SKILL = os.path.join(_HERE, "..", "..", "internal", "cli", "skills", "helix", "SKILL.md")


def _load_corpus_descriptors(corpus_dir):
    out = []
    for name in sorted(os.listdir(corpus_dir)):
        if not name.endswith(".json") or name == "corpus_manifest.json":
            continue
        with open(os.path.join(corpus_dir, name), encoding="utf-8") as fh:
            d = json.load(fh)
        if "task" in d:
            out.append(d)
    return out


def _heldout():
    if os.path.isfile(_HELDOUT):
        with open(_HELDOUT, encoding="utf-8") as fh:
            return json.load(fh)
    examples = _load_corpus_descriptors(_CORPUS)
    _train, _val, test = split_corpus(examples)
    return test


def _steering_text():
    with open(_SKILL, encoding="utf-8") as fh:
        return fh.read()


def smoke(n=1, max_turns=6):
    descs = _load_corpus_descriptors(_CORPUS)[:n]
    for d in descs:
        r = runlib.run_one(d, "", "off", max_turns=max_turns)
        print(f"[smoke] {d['language']:7} {os.path.basename(d['task_dir']):24} "
              f"passed={r['passed']} tests={r['tests_run']} reason={r['agent_reason']} "
              f"steps={r['steps']} cost=${r['cost_usd']:.4f}")
    print("[smoke] done")
    return 0


def attribution(max_turns=20):
    if not (os.environ.get("DEEPSEEK_API_KEY") or os.environ.get("OPENAI_API_KEY")):
        print("No LM key — attribution is a real billed run and needs one. no-ship stands.")
        return 0
    heldout = _heldout()
    steering = _steering_text()
    print(f"[attribution] held-out split = {len(heldout)} tasks (gate > {VAL_SIZE_GATE}); "
          f"steering candidate = embedded SKILL.md ({len(steering)} chars); max_turns={max_turns}")

    def run_arm(arm, examples):
        text = steering if arm == "on" else ""
        successes = 0
        cost = 0.0
        for i, d in enumerate(examples):
            r = runlib.run_one(d, text, arm, max_turns=max_turns)
            successes += 1 if r["passed"] else 0
            cost += r["cost_usd"]
            print(f"[{arm}] {i+1}/{len(examples)} {d['language']:7} "
                  f"{os.path.basename(d['task_dir']):24} passed={r['passed']} "
                  f"tests={r['tests_run']} cost=${r['cost_usd']:.4f}", flush=True)
        return ArmResult(arm=arm, successes=successes, n=len(examples), cost_usd=cost)

    attr = compute_attribution(heldout, run_arm, val_size=len(heldout))
    verdict, reason = decide_ship(attr)
    os.makedirs(_OUT, exist_ok=True)
    with open(os.path.join(_OUT, "attribution.json"), "w", encoding="utf-8") as fh:
        json.dump({
            "verdict": verdict, "reason": reason,
            "val_size": attr.val_size, "delta": attr.delta,
            "on": {"success_rate": attr.on.success_rate, "successes": attr.on.successes,
                   "n": attr.on.n, "cost_usd": attr.on.cost_usd},
            "off": {"success_rate": attr.off.success_rate, "successes": attr.off.successes,
                    "n": attr.off.n, "cost_usd": attr.off.cost_usd},
            "total_cost_usd": attr.total_cost_usd,
        }, fh, indent=2)
    with open(os.path.join(_OUT, "REPORT-RUN.md"), "w", encoding="utf-8") as fh:
        fh.write(render_report(attr, verdict, reason))
    print(f"\n[attribution] VERDICT={verdict.upper()} delta={attr.delta:+.4f} "
          f"ON={attr.on.success_rate:.4f} OFF={attr.off.success_rate:.4f} "
          f"val_size={attr.val_size} cost=${attr.total_cost_usd:.4f}")
    print(f"[attribution] wrote {_OUT}/attribution.json + REPORT-RUN.md")
    return 0


def main(argv):
    mode = argv[0] if argv else "smoke"
    if mode == "smoke":
        n = int(argv[1]) if len(argv) > 1 else 1
        return smoke(n)
    if mode == "attribution":
        mt = int(os.environ.get("AGENT_MAX_TURNS") or 20)
        return attribution(max_turns=mt)
    print(f"unknown mode {mode!r}; use: smoke [N] | attribution")
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
