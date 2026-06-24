"""Hermetic tests for the honest Aider task-success grader + sandbox.

No toolchain, no network: grade_output() is pure (canned native-tool output)
and grade_task() takes an injectable runner. Mirrors the flat, fixture-free,
script-runnable convention of test_parity.py / test_degenerate.py.

Ships the Phase-108 anti-vacuity break-the-invariant proofs:
  * 0 tests executed => GradeError (a vacuous "pass" is refused).
  * a do-nothing Rust stub (compiles, ignored acceptance tests FAIL under
    --include-ignored) does NOT score as success.
  * agent test-tampering inside the sandbox cannot force green — the gold test
    is restored from an agent-unwritable source before grading.
"""

import json
import os

import pytest

from grade_aider import (
    GradeError,
    grade_output,
    grade_task,
    native_test_command,
)
import sandbox as sb

_HERE = os.path.dirname(__file__)


# --------------------------------------------------------------------------- #
# Parity: the Python native-cmd mirror == the shared golden (Go authority).    #
# --------------------------------------------------------------------------- #


def test_native_cmd_parity_with_golden():
    with open(os.path.join(_HERE, "golden", "aider_native_cmd.json"), encoding="utf-8") as fh:
        golden = {k: v for k, v in json.load(fh).items() if not k.startswith("_")}
    assert len(golden) == 6, "golden must cover the full 6-language map (vacuous-guard)"
    for lang, want in golden.items():
        assert native_test_command(lang) == want, f"{lang} cmd drift vs golden"
    # The load-bearing Rust flag must be present.
    assert native_test_command("rust") == ["cargo", "test", "--", "--include-ignored"]


def test_unknown_language_fails_closed():
    with pytest.raises(GradeError):
        native_test_command("cobol")


# --------------------------------------------------------------------------- #
# Honest pass/fail + the 0-tests-ran = hard ERROR invariant.                   #
# --------------------------------------------------------------------------- #


def test_python_pass_and_fail():
    ok, n = grade_output("python", 0, "===== 5 passed in 0.10s =====")
    assert ok and n == 5
    bad, n2 = grade_output("python", 1, "==== 3 passed, 2 failed in 0.2s ====")
    assert not bad and n2 == 5


def test_zero_tests_is_hard_error_python():
    # pytest "no tests ran" must NEVER be coerced into a pass (the Phase-81 class).
    with pytest.raises(GradeError):
        grade_output("python", 0, "no tests ran in 0.01s")


def test_zero_tests_is_hard_error_rust():
    # A do-nothing stub that merely COMPILES produces no "test result:" line under
    # cargo => 0 tests => GradeError, not a vacuous pass.
    with pytest.raises(GradeError):
        grade_output("rust", 0, "   Compiling foo v0.1.0\n    Finished test profile\n")


def test_do_nothing_fails_rust_under_include_ignored():
    # With --include-ignored the ignored acceptance tests RUN and FAIL for a
    # do-nothing stub: cargo reports failures => not a pass (the WR-02 invariant).
    out = "running 4 tests\ntest result: FAILED. 1 passed; 3 failed; 0 ignored"
    passed, n = grade_output("rust", 101, out)
    assert not passed and n == 4, "a do-nothing Rust stub must NOT score as success"


def test_rust_real_pass():
    out = "running 4 tests\ntest result: ok. 4 passed; 0 failed; 0 ignored"
    passed, n = grade_output("rust", 0, out)
    assert passed and n == 4


def test_grade_task_uses_native_cmd_and_runner():
    captured = {}

    def fake_runner(argv, cwd, timeout):
        captured["argv"] = argv
        return 0, "test result: ok. 2 passed; 0 failed", ""

    passed, n = grade_task("/nonexistent-sandbox", "rust", runner=fake_runner)
    assert passed and n == 2
    # Proves the grader uses the parity-pinned native argv (no shell).
    assert captured["argv"] == ["cargo", "test", "--", "--include-ignored"]


# --------------------------------------------------------------------------- #
# Anti-tamper: gold tests restored from an agent-unwritable source.            #
# --------------------------------------------------------------------------- #


def test_gold_test_restore_defeats_tampering(tmp_path):
    # Pristine (agent-unwritable) source with a real gold test.
    gold_src = tmp_path / "gold"
    (gold_src / "tests").mkdir(parents=True)
    gold_test = gold_src / "tests" / "lib_test.rs"
    gold_test.write_text("assert_eq!(add(2,2), 4);  // real assertion\n")

    # The sandbox the agent edited: it TAMPERED the test into a no-op.
    sandbox_dir = tmp_path / "sandbox"
    (sandbox_dir / "tests").mkdir(parents=True)
    tampered = sandbox_dir / "tests" / "lib_test.rs"
    tampered.write_text("// agent deleted the assertions\n")

    sb.restore_gold_tests(str(sandbox_dir), str(gold_src), ["tests/lib_test.rs"])
    assert "real assertion" in tampered.read_text(), "gold test must overwrite the tampered one"


def test_restore_missing_gold_is_hard_error(tmp_path):
    sandbox_dir = tmp_path / "sandbox"
    sandbox_dir.mkdir()
    with pytest.raises(FileNotFoundError):
        sb.restore_gold_tests(str(sandbox_dir), str(tmp_path / "absent"), ["tests/x_test.rs"])


def _run(fn):
    import inspect
    sig = inspect.signature(fn)
    if "tmp_path" in sig.parameters:
        import tempfile
        import pathlib
        d = tempfile.mkdtemp()
        try:
            fn(pathlib.Path(d))
        finally:
            import shutil
            shutil.rmtree(d, ignore_errors=True)
    else:
        fn()


if __name__ == "__main__":
    _run(test_native_cmd_parity_with_golden)
    _run(test_unknown_language_fails_closed)
    _run(test_python_pass_and_fail)
    _run(test_zero_tests_is_hard_error_python)
    _run(test_zero_tests_is_hard_error_rust)
    _run(test_do_nothing_fails_rust_under_include_ignored)
    _run(test_rust_real_pass)
    _run(test_grade_task_uses_native_cmd_and_runner)
    _run(test_gold_test_restore_defeats_tampering)
    _run(test_restore_missing_gold_is_hard_error)
    print("grade OK")
