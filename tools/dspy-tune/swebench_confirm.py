"""RUN-03: SWE-bench Verified confirming leg (dev-time only).

DEV-TIME / OFFLINE ONLY — lives under tools/, NEVER linked into the helix binary,
go.mod, or the default `go test ./...` path.

Confirms the SWE-bench task-success ORACLE (grade_swebench) works end-to-end on
real `princeton-nlp/SWE-bench_Verified` instances on Podman, by running the
upstream harness on GOLD patches (each instance's own `patch`) — which MUST
resolve. This is the secondary confirming grader the user selected ("both
tracks"); a gold patch that resolves + grade_report agreeing (tests_checked>0,
resolved=True) proves the oracle is honest on real reports, not just hermetic
fakes (Phase 109/112 already proved the decision logic hermetically).

fail-not-skip: a *requested* run that yields no result is recorded as an explicit
error per instance (never a silent pass). K is small by default (image builds are
GB-scale and minutes each); the achieved K is recorded honestly.
"""

import json
import os
import sys
import tempfile

from grade_swebench import HarnessRun, grade_task

DATASET = "princeton-nlp/SWE-bench_Verified"
_HERE = os.path.dirname(os.path.abspath(__file__))
_OUT = os.path.join(_HERE, "output")


def _safe_run_id(iid):
    s = "".join(c if (c.isalnum() or c in "_-") else "_" for c in iid)
    return ("confirm_" + s)[:60]


def main():
    k = int(os.environ.get("SWEBENCH_K") or 2)
    if not os.environ.get("DOCKER_HOST"):
        print("DOCKER_HOST not set — the swebench harness needs the Podman socket. "
              "Start `podman system service` and export DOCKER_HOST. (fail-not-skip: "
              "recording an explicit error, not a silent pass.)")
        os.makedirs(_OUT, exist_ok=True)
        json.dump({"k": k, "error": "DOCKER_HOST unset", "results": []},
                  open(os.path.join(_OUT, "swebench_confirm.json"), "w"), indent=2)
        return 1

    from datasets import load_dataset  # swebench dep; lazy so the guard above runs key-free

    ds = load_dataset(DATASET, split="test")
    # Stratify-ish: smallest gold patches first (faster image builds / runs), and
    # spread across distinct repos so we don't confirm a single env image only.
    rows = sorted(ds, key=lambda r: len(r["patch"]))
    picked, seen_repos = [], set()
    for r in rows:
        if r["repo"] in seen_repos:
            continue
        seen_repos.add(r["repo"])
        picked.append(r)
        if len(picked) >= k:
            break

    work = tempfile.mkdtemp(prefix="swebench-confirm-")
    preds_path = os.path.join(work, "predictions.jsonl")
    results = []
    for r in picked:
        iid = r["instance_id"]
        with open(preds_path, "w", encoding="utf-8") as fh:
            fh.write(json.dumps({
                "instance_id": iid,
                "model_name_or_path": "gold",
                "model_patch": r["patch"],
            }) + "\n")
        run = HarnessRun(
            dataset_name=DATASET,
            predictions_path=preds_path,
            run_id=_safe_run_id(iid),
            instance_ids=[iid],
        )
        try:
            passed, n = grade_task(run, work_dir=work, timeout=1800)
            results.append({"instance_id": iid, "repo": r["repo"], "resolved": bool(passed),
                            "tests_checked": int(n)})
            print(f"[swebench] {iid} ({r['repo']}) resolved={passed} tests_checked={n}", flush=True)
        except Exception as e:  # fail-not-skip: record the explicit error
            results.append({"instance_id": iid, "repo": r["repo"], "error": str(e)[:300]})
            print(f"[swebench] {iid} ({r['repo']}) ERROR: {str(e)[:200]}", flush=True)

    gold_resolved = sum(1 for x in results if x.get("resolved"))
    summary = {
        "dataset": DATASET,
        "k_requested": k,
        "k_run": len(results),
        "gold_resolved": gold_resolved,
        "all_gold_resolved": gold_resolved == len(results) and len(results) > 0,
        "results": results,
    }
    os.makedirs(_OUT, exist_ok=True)
    json.dump(summary, open(os.path.join(_OUT, "swebench_confirm.json"), "w"), indent=2)
    print(f"\n[swebench] gold_resolved={gold_resolved}/{len(results)} "
          f"(all_gold_resolved={summary['all_gold_resolved']}); wrote output/swebench_confirm.json")
    return 0


if __name__ == "__main__":
    sys.exit(main())
