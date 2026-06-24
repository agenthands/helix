"""Honest Aider-polyglot task-success grader (dev-time only).

DEV-TIME / OFFLINE ONLY — lives under tools/, NEVER linked into the helix
binary, `helix setup`, go.mod, or the default `go test ./...` path.

The per-language native test argv is a PARITY MIRROR of the Go authority
`bench/datasets/aider-polyglot/loader.go` nativeTestCommand(), pinned via the
shared corpus `golden/aider_native_cmd.json` (asserted by test_grade.py here AND
native_cmd_parity_test.go on the Go side).

Honesty invariants (ORACLE-01; the v2.2/Phase-81 vacuous-pass class):
  * A run where ZERO tests executed is a hard ERROR (GradeError), NEVER a pass.
    A do-nothing stub that merely compiles must not score as success.
  * Rust uses `cargo test -- --include-ignored` — Exercism marks acceptance
    tests #[ignore]; without it a compile-only stub passes vacuously.
  * Gold test files are restored from an agent-unwritable source before grading
    (see sandbox.py) so agent test-tampering cannot force green.
"""

import json
import os
import re
import subprocess

_HERE = os.path.dirname(os.path.abspath(__file__))
_GOLDEN = os.path.join(_HERE, "golden", "aider_native_cmd.json")


class GradeError(Exception):
    """Raised when task-success cannot be honestly determined (e.g. 0 tests ran,
    or the test output could not be parsed for a test count)."""


def _load_golden():
    with open(_GOLDEN, encoding="utf-8") as fh:
        data = json.load(fh)
    return {k: v for k, v in data.items() if not k.startswith("_")}


_NATIVE_CMD = _load_golden()


def native_test_command(language):
    """Return the native test argv for a language (parity mirror of loader.go).
    Fail-closed (GradeError) on an unknown language — never guess a command."""
    argv = _NATIVE_CMD.get(language)
    if argv is None:
        raise GradeError(f"no native test command for language {language!r} (fail-closed)")
    return list(argv)


# --- per-language pass/test-count parsers ---------------------------------- #
# Each returns (passed: bool, tests_run: int). They key on the native tool's
# own summary line. When no test-count signal is found, callers raise GradeError
# (cannot confirm tests ran => not a pass).

def _parse_python(exit_code, out):
    # pytest summary: "N passed", "N failed", "N error(s)", "no tests ran".
    if re.search(r"\bno tests ran\b", out):
        return (False, 0)
    passed = sum(int(m) for m in re.findall(r"(\d+) passed", out))
    failed = sum(int(m) for m in re.findall(r"(\d+) failed", out))
    errors = sum(int(m) for m in re.findall(r"(\d+) errors?", out))
    total = passed + failed + errors
    return (exit_code == 0 and failed == 0 and errors == 0 and passed > 0, total)


def _parse_rust(exit_code, out):
    # cargo: "test result: ok. N passed; M failed; ..."
    runs = re.findall(r"test result:\s*(\w+)\.\s*(\d+) passed;\s*(\d+) failed", out)
    if not runs:
        return (False, 0)
    passed = sum(int(p) for _, p, _ in runs)
    failed = sum(int(f) for _, _, f in runs)
    return (exit_code == 0 and failed == 0 and passed > 0, passed + failed)


def _parse_go(exit_code, out):
    # go test: count "--- PASS:" / "--- FAIL:" lines; "no test files" => 0.
    if re.search(r"\[no test files\]", out) and "--- PASS:" not in out and "--- FAIL:" not in out:
        return (False, 0)
    passed = len(re.findall(r"--- PASS:", out))
    failed = len(re.findall(r"--- FAIL:", out))
    total = passed + failed
    if total == 0:
        # Fall back to the package-level ok/FAIL summary only if a test count is
        # otherwise unknowable — but with no per-test markers we cannot confirm a
        # test actually ran, so report 0 (callers turn that into a GradeError).
        return (False, 0)
    return (exit_code == 0 and failed == 0 and passed > 0, total)


def _parse_generic(exit_code, out):
    # java/gradlew, js, cpp: look for an explicit "N tests" style count. If none
    # is found we cannot confirm tests ran honestly.
    m = re.search(r"(\d+)\s+tests?\b", out)
    if not m:
        return (False, 0)
    n = int(m.group(1))
    failed_m = re.search(r"(\d+)\s+(?:failures?|failed)\b", out)
    failed = int(failed_m.group(1)) if failed_m else (0 if exit_code == 0 else 1)
    return (exit_code == 0 and failed == 0 and n > 0, n)


_PARSERS = {
    "python": _parse_python,
    "rust": _parse_rust,
    "go": _parse_go,
    "java": _parse_generic,
    "javascript": _parse_generic,
    "cpp": _parse_generic,
}


def grade_output(language, exit_code, stdout, stderr=""):
    """Classify a native-test run into (passed, tests_run). Raises GradeError if
    ZERO tests executed (vacuous pass refused) or the language is unknown."""
    parser = _PARSERS.get(language)
    if parser is None:
        raise GradeError(f"no grader for language {language!r} (fail-closed)")
    combined = (stdout or "") + "\n" + (stderr or "")
    passed, tests_run = parser(exit_code, combined)
    if tests_run <= 0:
        raise GradeError(
            f"0 tests executed for {language} (exit={exit_code}) — refusing a "
            f"vacuous pass; a compile-only/do-nothing solution must not score."
        )
    return passed, tests_run


def grade_task(task_dir, language, timeout=180, runner=None):
    """Run the native test command in `task_dir` and grade it honestly.

    `runner(argv, cwd, timeout) -> (exit_code, stdout, stderr)` is injectable so
    hermetic tests drive canned output with no real toolchain/network. Returns
    (passed: bool, tests_run: int); raises GradeError on a 0-test (vacuous) run.
    """
    argv = native_test_command(language)

    def _default_runner(a, cwd, t):
        # Fixed-argv list passed to subprocess; no shell interpretation.
        proc = subprocess.run(a, cwd=cwd, capture_output=True, text=True, timeout=t)
        return proc.returncode, proc.stdout, proc.stderr

    run = runner or _default_runner
    exit_code, stdout, stderr = run(argv, task_dir, timeout)
    return grade_output(language, exit_code, stdout, stderr)
