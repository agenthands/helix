"""Hermetic tests for the honest SWE-bench task-success grader.

No live python-swebench, no Podman, no network: build_argv/build_env/grade_report
are pure, and grade_task takes an injectable runner + report_reader. Mirrors the
flat, fixture-free, script-runnable convention of test_grade.py.

Ships the Phase-109 anti-vacuity break-the-invariant proofs:
  * a dataset name outside the 2-name org allowlist => GradeError (org-drift refusal).
  * flag-smuggling values (leading-'-' run_id / instance id, relative or '..'
    predictions path) can never reach argv (GradeError before any argv element).
  * the env crossing the boundary is the STRICT allowlist (DOCKER_HOST carried,
    an unlisted key dropped); empty PATH fails closed.
  * 0 FAIL_TO_PASS+PASS_TO_PASS tests evaluated => GradeError (vacuous-pass refusal).
  * the resolution contract: a single FAIL_TO_PASS/PASS_TO_PASS failure => NOT resolved.
  * fail-not-skip: a requested real run that produced no report.json => GradeError
    (the Phase-81 false-green class), never a silent pass.
"""

import json
import os

import pytest

import taskmetric
from grade_aider import GradeError
from grade_swebench import (
    ALLOWED_DATASET_NAMES,
    ENV_ALLOWLIST,
    HarnessRun,
    build_argv,
    build_env,
    grade_report,
    grade_task,
)

_HERE = os.path.dirname(__file__)
_GOOD_IID = "sympy__sympy-20590"


def _good_run(**over):
    base = dict(
        dataset_name="princeton-nlp/SWE-bench_Verified",
        predictions_path="/tmp/helix-swebench/preds.jsonl",
        run_id="helix-v2_3-on",
        instance_ids=[_GOOD_IID],
        max_workers=1,
        cache_level="env",
    )
    base.update(over)
    return HarnessRun(**base)


# --------------------------------------------------------------------------- #
# Parity: build_argv == the shared golden (Go authority RunArgs).             #
# --------------------------------------------------------------------------- #


def test_argv_parity_with_golden():
    with open(os.path.join(_HERE, "golden", "swebench_argv.json"), encoding="utf-8") as fh:
        golden = json.load(fh)
    run = _good_run(**golden["input"])
    assert build_argv(run) == golden["argv"], "build_argv drift vs golden (RunArgs authority)"
    # Anti-vacuity: the golden pins the dataset org and a real instance id.
    assert golden["input"]["dataset_name"] == "princeton-nlp/SWE-bench_Verified"
    assert golden["input"]["instance_ids"], "golden must carry a real instance id"


# --------------------------------------------------------------------------- #
# Dataset-org allowlist + flag-smuggling refusals (fail-closed before argv).  #
# --------------------------------------------------------------------------- #


def test_dataset_org_drift_refused():
    # The org-drift trap: princeton-nlp/ (dataset org) vs SWE-bench/ (repo org).
    with pytest.raises(GradeError):
        build_argv(_good_run(dataset_name="SWE-bench/SWE-bench_Verified"))
    # The two allowlisted names DO pass.
    assert "princeton-nlp/SWE-bench_Verified" in ALLOWED_DATASET_NAMES
    assert "Bertsekas/SWE-Bench_Verified_UTBoost" in ALLOWED_DATASET_NAMES


def test_flag_smuggling_run_id_refused():
    with pytest.raises(GradeError):
        build_argv(_good_run(run_id="-rf"))


def test_flag_smuggling_instance_id_refused():
    with pytest.raises(GradeError):
        build_argv(_good_run(instance_ids=["-x"]))
    # A non-instance-shaped id is also refused.
    with pytest.raises(GradeError):
        build_argv(_good_run(instance_ids=["not-an-instance"]))


def test_relative_or_dotdot_predictions_path_refused():
    with pytest.raises(GradeError):
        build_argv(_good_run(predictions_path="preds.jsonl"))  # relative
    with pytest.raises(GradeError):
        build_argv(_good_run(predictions_path="/tmp/../etc/preds.jsonl"))  # '..'
    with pytest.raises(GradeError):
        build_argv(_good_run(predictions_path="/tmp/a:b/preds.jsonl"))  # ':'


def test_bad_cache_level_and_workers_refused():
    with pytest.raises(GradeError):
        build_argv(_good_run(cache_level="evil"))
    with pytest.raises(GradeError):
        build_argv(_good_run(max_workers=0))
    with pytest.raises(GradeError):
        build_argv(_good_run(instance_ids=[]))


# --------------------------------------------------------------------------- #
# Env allowlist: strict forwarding, DOCKER_HOST carried, empty PATH closed.   #
# --------------------------------------------------------------------------- #


def test_env_allowlist_forwards_only_listed_set_keys():
    src = {
        "PATH": "/usr/bin",
        "HOME": "/home/dev",
        "DOCKER_HOST": "unix:///run/user/1000/podman/podman.sock",
        "SECRET_TOKEN": "leak-me",  # unlisted => must be dropped
        "OPENAI_API_KEY": "sk-leak",  # unlisted => must be dropped
    }
    env = build_env(src)
    assert env["PATH"] == "/usr/bin"
    assert env["DOCKER_HOST"].endswith("podman.sock"), "DOCKER_HOST (podman socket) must be carried"
    assert "SECRET_TOKEN" not in env and "OPENAI_API_KEY" not in env
    # Only ENV_ALLOWLIST keys may appear.
    assert set(env).issubset(set(ENV_ALLOWLIST))


def test_empty_path_fails_closed():
    with pytest.raises(GradeError):
        build_env({"HOME": "/home/dev"})  # PATH absent => fail-closed


# --------------------------------------------------------------------------- #
# Resolution contract (FAIL_TO_PASS + PASS_TO_PASS) + vacuous-pass refusal.    #
# --------------------------------------------------------------------------- #


def _report(iid, f2p_pass, f2p_fail, p2p_pass, p2p_fail):
    return {
        iid: {
            "tests_status": {
                "FAIL_TO_PASS": {"success": f2p_pass, "failure": f2p_fail},
                "PASS_TO_PASS": {"success": p2p_pass, "failure": p2p_fail},
            }
        }
    }


def test_resolved_when_all_green():
    rep = _report(_GOOD_IID, ["t_a", "t_b"], [], ["t_c"], [])
    resolved, n = grade_report(rep, _GOOD_IID)
    assert resolved and n == 3


def test_not_resolved_on_any_fail_to_pass_failure():
    rep = _report(_GOOD_IID, ["t_a"], ["t_b"], ["t_c"], [])
    resolved, n = grade_report(rep, _GOOD_IID)
    assert not resolved and n == 3


def test_not_resolved_on_pass_to_pass_regression():
    rep = _report(_GOOD_IID, ["t_a"], [], [], ["t_c"])
    resolved, n = grade_report(rep, _GOOD_IID)
    assert not resolved and n == 2


def test_ignores_harness_resolved_flag_on_zero_tests():
    # SCALE-03 (research footgun): the upstream harness's compute_fail_to_pass()
    # returns 1.0 on total==0, so a parsed-but-empty eval can carry resolved=True.
    # grade_report MUST recompute from tests_status and still hard-error on 0 tests
    # — never trusting the harness's vacuous resolved flag.
    rep = {
        _GOOD_IID: {
            "resolved": True,  # the harness's vacuous flag — must be ignored
            "tests_status": {
                "FAIL_TO_PASS": {"success": [], "failure": []},
                "PASS_TO_PASS": {"success": [], "failure": []},
            },
        }
    }
    with pytest.raises(GradeError):
        grade_report(rep, _GOOD_IID)


def test_zero_tests_is_hard_error():
    # Empty test set must NEVER read as success (the Phase-81/108 vacuous class).
    rep = _report(_GOOD_IID, [], [], [], [])
    with pytest.raises(GradeError):
        grade_report(rep, _GOOD_IID)


def test_empty_fail_to_pass_is_not_resolved():
    # A patch that fixes nothing (no FAIL_TO_PASS passes) cannot resolve the issue,
    # even if PASS_TO_PASS stays green.
    rep = _report(_GOOD_IID, [], [], ["t_c", "t_d"], [])
    resolved, n = grade_report(rep, _GOOD_IID)
    assert not resolved and n == 2


def test_missing_instance_is_hard_error():
    rep = _report("other__repo-1", ["t"], [], [], [])
    with pytest.raises(GradeError):
        grade_report(rep, _GOOD_IID)


# --------------------------------------------------------------------------- #
# grade_task: injected runner proves the exact argv crosses the boundary.      #
# --------------------------------------------------------------------------- #


def test_grade_task_passes_exact_argv_and_grades():
    captured = {}

    def fake_runner(argv, env, cwd, timeout):
        captured["argv"] = argv
        captured["env"] = env
        return 0

    def fake_reader(run, work_dir):
        return _report(_GOOD_IID, ["t_a", "t_b"], [], ["t_c"], [])

    # Ensure build_env has a PATH to forward.
    os.environ.setdefault("PATH", "/usr/bin")
    passed, n = grade_task(_good_run(), work_dir="/tmp/wd",
                           runner=fake_runner, report_reader=fake_reader)
    assert passed and n == 3
    # The runner received the FIXED, validated argv (no shell).
    assert captured["argv"][:2] == ["-m", "swebench.harness.run_evaluation"]
    assert "--instance_ids" in captured["argv"] and _GOOD_IID in captured["argv"]
    # The env crossing the boundary is the strict allowlist.
    assert set(captured["env"]).issubset(set(ENV_ALLOWLIST))


def test_fail_not_skip_default_reader_no_report(tmp_path):
    # A *requested* real run that produced NO report.json must FAIL LOUDLY, never
    # silently pass (the Phase-81 false-green / HELIX_BIN fail-not-skip class).
    def noop_runner(argv, env, cwd, timeout):
        return 0  # ran nothing, wrote nothing

    os.environ.setdefault("PATH", "/usr/bin")
    with pytest.raises(GradeError):
        grade_task(_good_run(), work_dir=str(tmp_path), runner=noop_runner)


# --------------------------------------------------------------------------- #
# Pluggable behind taskmetric.score_task (grader-agnostic).                    #
# --------------------------------------------------------------------------- #


def test_plugs_into_taskmetric_score_task():
    def agent_runner(_task):
        return True  # the grader, not the agent, decides success

    def grader_resolved():
        rep = _report(_GOOD_IID, ["t_a"], [], ["t_b"], [])
        return grade_report(rep, _GOOD_IID)

    score, fb = taskmetric.score_task(object(), agent_runner, grader_resolved)
    assert score == 1.0 and "passed" in fb

    def grader_vacuous():
        return grade_report(_report(_GOOD_IID, [], [], [], []), _GOOD_IID)

    score0, fb0 = taskmetric.score_task(object(), agent_runner, grader_vacuous)
    assert score0 == 0.0, "a GradeError (vacuous) must score 0.0, never a pass"


def _run(fn):
    import inspect
    if "tmp_path" in inspect.signature(fn).parameters:
        import tempfile, pathlib, shutil
        d = tempfile.mkdtemp()
        try:
            fn(pathlib.Path(d))
        finally:
            shutil.rmtree(d, ignore_errors=True)
    else:
        fn()


if __name__ == "__main__":
    for name, obj in sorted(globals().items()):
        if name.startswith("test_") and callable(obj):
            _run(obj)
    print("swebench OK")
