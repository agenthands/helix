"""Per-task sandbox + gold-test restore (dev-time only).

DEV-TIME / OFFLINE ONLY — lives under tools/, NEVER linked into the helix
binary, `helix setup`, go.mod, or the default `go test ./...` path.

The agent edits a throwaway COPY of the task, never the pristine source. Before
grading, the gold test files are restored from an agent-UNWRITABLE source path
(the pristine task dir, which the agent never sees) so agent test-tampering
inside the sandbox cannot force a green grade (ORACLE-01 anti-tamper).
"""

import os
import shutil
import tempfile


def make_sandbox(src_task_dir, prefix="aider-task-"):
    """Copy a pristine task dir into a fresh temp sandbox the agent may edit.
    Returns the sandbox path (caller cleans up via cleanup_sandbox)."""
    if not os.path.isdir(src_task_dir):
        raise FileNotFoundError(f"task dir does not exist: {src_task_dir}")
    dest = tempfile.mkdtemp(prefix=prefix)
    sandbox = os.path.join(dest, os.path.basename(os.path.normpath(src_task_dir)))
    shutil.copytree(src_task_dir, sandbox)
    return sandbox


def restore_gold_tests(sandbox_dir, gold_src_dir, test_rel_paths):
    """Overwrite the sandbox's test files with the pristine gold copies from an
    agent-unwritable source (`gold_src_dir`), defeating in-sandbox test tampering.

    `test_rel_paths` are paths relative to the task root (e.g. ["tests/lib_test.rs"]).
    A gold file that does not exist is a hard error — we never grade against a
    test the agent could have deleted.
    """
    restored = []
    for rel in test_rel_paths:
        gold = os.path.join(gold_src_dir, rel)
        if not os.path.isfile(gold):
            raise FileNotFoundError(
                f"gold test file missing from the unwritable source: {gold} "
                f"(refusing to grade against an agent-deletable test)"
            )
        dst = os.path.join(sandbox_dir, rel)
        os.makedirs(os.path.dirname(dst), exist_ok=True)
        shutil.copyfile(gold, dst)
        restored.append(rel)
    if not restored:
        raise ValueError("no gold test files specified — cannot guarantee an untampered grade")
    return restored


def cleanup_sandbox(sandbox_dir):
    """Remove a sandbox tree (best-effort; the parent temp dir too)."""
    parent = os.path.dirname(os.path.normpath(sandbox_dir))
    shutil.rmtree(parent, ignore_errors=True)
