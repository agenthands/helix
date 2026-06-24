"""Honest SWE-bench task-success grader (dev-time only).

DEV-TIME / OFFLINE ONLY — lives under tools/, NEVER linked into the helix
binary, `helix setup`, go.mod, or the default `go test ./...` path.

This is the heaviest v2.3 grader (ORACLE-02): it wraps the upstream
`swebench==4.1.0` harness (`python -m swebench.harness.run_evaluation`) on
Podman and grades a single SWE-bench instance by the FAIL_TO_PASS + PASS_TO_PASS
resolution contract. It plugs in behind `taskmetric.score_task` via the SAME
`grader() -> (passed, tests_run)` shape that `grade_aider.grade_task` exposes,
so the GEPA metric is grader-agnostic.

The argv builder, the 2-name dataset-org allowlist, and the env allowlist are a
PARITY MIRROR of the Go authority `bench/evaluators/swebench/harness.go`
(`RunArgs` / `allowedDatasetNames` / `envAllowlist`), pinned by the shared
golden `golden/swebench_argv.json` (asserted by test_swebench.py here AND
argv_parity_test.go on the Go side). The same discipline grade_aider.py keeps
against loader.go.

Honesty invariants (ORACLE-02; the v2.2/Phase-81/108 vacuous-pass class):
  * A run where ZERO FAIL_TO_PASS + PASS_TO_PASS tests were evaluated is a hard
    GradeError, NEVER "resolved". A harness that produced no report, or a report
    with an empty test set, cannot honestly score as success.
  * "resolved" requires EVERY FAIL_TO_PASS test to pass AND EVERY PASS_TO_PASS
    test to keep passing — the upstream resolution contract, enforced here so a
    partial pass never reads as success.
  * A crafted dataset name, run id, instance id, or predictions path can never
    cross os/exec: each is total-validated (regexp-free, leading-'-' refused)
    BEFORE a single argv element is produced, exactly as harness.go does.
  * The subprocess gets a STRICT env allowlist (never the full parent env);
    DOCKER_HOST carries the Podman socket. PATH empty => fail-closed.

SWE-bench is NOT blocked on Podman: start the docker-compat socket with
`podman system service --time=0 &` and point the harness at it via
`DOCKER_HOST=unix://$XDG_RUNTIME_DIR/podman/podman.sock` — configuration, not a
blocker.
"""

import json
import os
import subprocess
from dataclasses import dataclass, field

# Reuse grade_aider's GradeError so BOTH graders raise the one honest-failure
# type the metric catches (a 0-test vacuous run is identical in spirit).
from grade_aider import GradeError

# --- parity-mirrored constants (authority: harness.go) --------------------- #

# allowedDatasetNames (harness.go): the FIXED 2-name org allowlist. Closes the
# `princeton-nlp/` (dataset org) vs `SWE-bench/` (repo org) drift trap — a name
# outside this set is refused BEFORE any argv is produced.
ALLOWED_DATASET_NAMES = frozenset(
    {
        "princeton-nlp/SWE-bench_Verified",
        "Bertsekas/SWE-Bench_Verified_UTBoost",
    }
)

# allowedCacheLevels (harness.go).
ALLOWED_CACHE_LEVELS = frozenset({"none", "base", "env", "instance"})

# envAllowlist (harness.go): set-keys-only forwarding, never the full parent env.
# PATH/HOME/HELIX_CACHE_DIR base set; DOCKER_* forwarded WHEN SET so the Podman
# socket (DOCKER_HOST) and a non-default daemon are reachable. An UNLISTED key is
# never forwarded.
ENV_ALLOWLIST = (
    "PATH",
    "HOME",
    "HELIX_CACHE_DIR",
    "DOCKER_HOST",
    "DOCKER_TLS_VERIFY",
    "DOCKER_CERT_PATH",
)

# The fixed module entry point — never a shell, never an inherited argv.
_HARNESS_MODULE = "swebench.harness.run_evaluation"


# --- total, regexp-free validators (parity: harness.go) -------------------- #

def _is_valid_run_id(s):
    """Non-empty [A-Za-z0-9_-]+ that does NOT start with '-' (flag-smuggling
    refusal). Mirror of isValidRunID."""
    if not s or s[0] == "-":
        return False
    for c in s:
        if not (c.isascii() and (c.isalnum() or c in "_-")):
            return False
    return True


def _is_valid_instance_id(s):
    """SWE-bench instance shape "<owner>__<repo>-<num>" over [A-Za-z0-9_.-],
    not starting with '-'. Mirror of isValidInstanceID."""
    if not s or s[0] == "-":
        return False
    sep = s.find("__")
    if sep <= 0:
        return False
    dash = s.rfind("-")
    if dash <= sep + 2 or dash == len(s) - 1:
        return False
    num = s[dash + 1:]
    if not num.isdigit() or not num.isascii():
        return False
    for c in s:
        if not (c.isascii() and (c.isalnum() or c in "_-.")):
            return False
    return True


def _is_valid_predictions_path(p):
    """Clean, absolute, no ':' and no '..' (post-canonicalization). Mirror of
    isValidPredictionsPath — checked against the slash form for cross-OS parity."""
    if not p:
        return False
    if ":" in p:
        return False
    s = p.replace(os.sep, "/")
    if not s.startswith("/"):
        return False
    # os.path.normpath collapses redundant separators / resolves '..'; if the
    # canonical form differs from the input, the path was not already clean.
    if os.path.normpath(s).replace(os.sep, "/") != s:
        return False
    return True


# --- the validated run config (parity: HarnessRun) ------------------------- #

@dataclass
class HarnessRun:
    """Validated config for one `python -m swebench.harness.run_evaluation`
    invocation. Every field is total-validated by build_argv BEFORE any argv
    element is produced. Mirror of harness.go HarnessRun."""

    dataset_name: str
    predictions_path: str
    run_id: str
    instance_ids: list = field(default_factory=list)
    max_workers: int = 1
    cache_level: str = "env"


def build_argv(run):
    """Build the FIXED argv for the swebench harness, fail-closing on the FIRST
    bad input BEFORE producing any argv (parity: RunArgs). Raises GradeError
    (not a silent skip) so a crafted value can never reach os/exec.

    argv = ["-m", _HARNESS_MODULE,
            "--dataset_name", <name>, "--predictions_path", <path>,
            "--run_id", <id>, "--max_workers", <n>, "--cache_level", <lvl>,
            "--instance_ids", <iid>...]
    """
    if run.dataset_name not in ALLOWED_DATASET_NAMES:
        raise GradeError(
            f"dataset_name {run.dataset_name!r} not in allowlist "
            f"{sorted(ALLOWED_DATASET_NAMES)} (org-drift refusal)"
        )
    if not _is_valid_predictions_path(run.predictions_path):
        raise GradeError(
            f"predictions_path {run.predictions_path!r} must be a clean absolute "
            f"path without '..' or ':'"
        )
    if not _is_valid_run_id(run.run_id):
        raise GradeError(
            f"run_id {run.run_id!r} must match [A-Za-z0-9_-]+ and not start with '-'"
        )
    if run.max_workers < 1:
        raise GradeError(f"max_workers {run.max_workers} must be >= 1")
    if run.cache_level not in ALLOWED_CACHE_LEVELS:
        raise GradeError(
            f"cache_level {run.cache_level!r} not in {sorted(ALLOWED_CACHE_LEVELS)}"
        )
    if not run.instance_ids:
        raise GradeError("at least one instance id is required")
    for iid in run.instance_ids:
        if not _is_valid_instance_id(iid):
            raise GradeError(
                f"instance id {iid!r} is not a valid <owner>__<repo>-<num>"
            )

    argv = [
        "-m", _HARNESS_MODULE,
        "--dataset_name", run.dataset_name,
        "--predictions_path", run.predictions_path,
        "--run_id", run.run_id,
        "--max_workers", str(run.max_workers),
        "--cache_level", run.cache_level,
        "--instance_ids",
    ]
    argv.extend(run.instance_ids)
    return argv


def build_env(environ=None):
    """Build the STRICT env for the harness subprocess: only ENV_ALLOWLIST keys,
    set-keys-only, never the full parent env (parity: allowlistEnv). Fail-closed
    (GradeError) on empty PATH — the harness shells out to the container engine,
    which it locates via PATH."""
    src = os.environ if environ is None else environ
    if not src.get("PATH"):
        raise GradeError(
            "PATH is empty — the harness needs it to locate the container engine "
            "(docker/podman)"
        )
    env = {}
    for k in ENV_ALLOWLIST:
        v = src.get(k)
        if v:
            env[k] = v
    return env


# --- resolution-contract classifier (FAIL_TO_PASS + PASS_TO_PASS) ---------- #

def grade_report(report, instance_id):
    """Classify a swebench per-run report dict into (resolved, tests_checked),
    enforcing the FAIL_TO_PASS + PASS_TO_PASS resolution contract.

    `report` is the parsed swebench report (keyed by instance id), e.g.
        {"<iid>": {"resolved": <bool>,
                   "tests_status": {"FAIL_TO_PASS": {"success": [...],
                                                     "failure": [...]},
                                    "PASS_TO_PASS": {"success": [...],
                                                     "failure": [...]}}}}

    Raises GradeError if the instance is absent or ZERO FAIL_TO_PASS+PASS_TO_PASS
    tests were evaluated (the vacuous-pass refusal). "resolved" is recomputed
    from tests_status (never trusted from a bare top-level flag) so a partial
    pass cannot read as success.
    """
    entry = report.get(instance_id)
    if entry is None:
        raise GradeError(
            f"no report entry for instance {instance_id!r} — the harness produced "
            f"no result for it (refusing a vacuous pass)"
        )
    status = entry.get("tests_status") or {}
    f2p = status.get("FAIL_TO_PASS") or {}
    p2p = status.get("PASS_TO_PASS") or {}
    f2p_pass = list(f2p.get("success") or [])
    f2p_fail = list(f2p.get("failure") or [])
    p2p_pass = list(p2p.get("success") or [])
    p2p_fail = list(p2p.get("failure") or [])

    tests_checked = len(f2p_pass) + len(f2p_fail) + len(p2p_pass) + len(p2p_fail)
    if tests_checked <= 0:
        raise GradeError(
            f"0 FAIL_TO_PASS+PASS_TO_PASS tests evaluated for {instance_id!r} — "
            f"refusing a vacuous pass; an empty test set must not score as success."
        )

    # Resolution contract: every FAIL_TO_PASS now passes AND every PASS_TO_PASS
    # still passes. A non-empty FAIL_TO_PASS set is required (a patch that fixes
    # nothing cannot resolve an issue).
    resolved = (
        len(f2p_pass) > 0
        and not f2p_fail
        and not p2p_fail
    )
    return resolved, tests_checked


# --- default on-disk report reader ----------------------------------------- #

def _default_report_reader(run, work_dir):
    """Locate and parse the swebench per-instance report.json for the single
    graded instance. swebench writes
        logs/run_evaluation/<run_id>/<model>/<instance_id>/report.json
    where <model> is the prediction's model_name_or_path with '/'→'__'. We glob
    over <model> so we need not reconstruct the sanitized name. FAIL-LOUD: a
    missing report is a GradeError, never a silent skip (a *requested* real run
    that produced no result must fail — the Phase-81 fail-not-skip class)."""
    import glob

    iid = run.instance_ids[0]
    base = os.path.join(work_dir or ".", "logs", "run_evaluation", run.run_id)
    matches = glob.glob(os.path.join(base, "*", iid, "report.json"))
    if not matches:
        raise GradeError(
            f"no report.json under {base!r} for instance {iid!r} — the harness "
            f"produced no result (fail-not-skip: a requested real run must fail "
            f"loudly, never silently pass)."
        )
    with open(sorted(matches)[0], encoding="utf-8") as fh:
        return json.load(fh)


# --- the grader the metric plugs into -------------------------------------- #

def grade_task(run, work_dir=None, timeout=1800, runner=None, report_reader=None):
    """Run the swebench harness for `run` and grade the single instance honestly.

    Returns (passed: bool, tests_run: int) — the SAME contract grade_aider exposes
    so taskmetric.score_task is grader-agnostic. `tests_run` is the count of
    FAIL_TO_PASS+PASS_TO_PASS tests evaluated.

    Both subprocess and report I/O are injectable so the hermetic test drives
    canned argv/output with no live python/swebench/podman/network:
      * `runner(argv, env, cwd, timeout) -> int` runs `python -m <argv>` and
        returns the exit code (default: subprocess.run, no shell, strict env).
      * `report_reader(run, work_dir) -> dict` returns the parsed report
        (default: _default_report_reader, glob over the swebench output tree).

    Raises GradeError on any bad argv, a missing report, or a 0-test run.
    """
    # Build + validate argv and env BEFORE crossing os/exec (fail-closed).
    argv = build_argv(run)
    env = build_env()

    def _default_runner(a, e, cwd, t):
        import sys
        proc = subprocess.run(
            [sys.executable, *a], env=e, cwd=cwd, timeout=t,
            capture_output=True, text=True,
        )
        return proc.returncode

    run_fn = runner or _default_runner
    read_fn = report_reader or _default_report_reader

    # The harness exit code is informational; the resolution report is the
    # authority. A non-zero exit with a usable report is still graded honestly;
    # a missing/empty report is a GradeError (fail-not-skip).
    run_fn(argv, env, work_dir, timeout)
    report = read_fn(run, work_dir)
    return grade_report(report, run.instance_ids[0])
