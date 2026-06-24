"""Overfit guard: the held-out TEST split must be disjoint from TRAIN.

The held-out TEST split is the FIRST harness step (spike discipline): it is
created and locked BEFORE any optimization can run, or overfit is undetectable
(106-RESEARCH Pitfall 3). GEPA draws trainset/valset ONLY from train.jsonl —
val is carved from TRAIN — so the TRAIN task universe IS trainset∪valset. This
test asserts TEST ⊄ trainset∪valset by checking TEST ∩ TRAIN == ∅.

If a future edit leaks a TEST task into TRAIN, the intersection becomes
non-empty and this test turns RED — that is the break-the-invariant proof this
overfit guard ships.
"""

import json
import os

_HERE = os.path.dirname(__file__)
_TRAIN = os.path.join(_HERE, "data", "train.jsonl")
_TEST = os.path.join(_HERE, "data", "test.jsonl")


def _load_tasks(path: str):
    tasks = set()
    with open(path, encoding="utf-8") as fh:
        for line in fh:
            line = line.strip()
            if not line:
                continue
            tasks.add(json.loads(line)["task"])
    return tasks


def test_train_and_test_non_empty():
    """A sequestered split must actually hold data on both sides."""
    train = _load_tasks(_TRAIN)
    test = _load_tasks(_TEST)
    assert train, "train.jsonl must be non-empty (the optimizer's TRAIN universe)"
    assert test, "test.jsonl must be non-empty (a held-out split must hold data)"


def test_test_disjoint_from_train():
    """TEST ⊄ trainset∪valset: no TEST task may appear in TRAIN."""
    train = _load_tasks(_TRAIN)
    test = _load_tasks(_TEST)
    leaked = test & train
    assert not leaked, f"TEST tasks leaked into TRAIN (overfit risk): {sorted(leaked)}"


def test_val_size_adoption_gate_boundary():
    """TUNE-03 (v2.2 no-ship root-cause fix): the val_size>50 adoption gate is a
    STRICT boundary — val_size==50 NO-SHIPS, val_size==51 is adoptable.

    Break-the-invariant: weakening the gate to `>=50` (or any vacuous
    always-true form) turns the val_size==50 assertion below RED.
    """
    from optimize import VAL_SIZE_GATE, adoption_allowed

    assert VAL_SIZE_GATE == 50
    assert adoption_allowed(51) is True, "val_size=51 must be adoptable"
    assert adoption_allowed(50) is False, "val_size=50 must NO-SHIP (strict > gate)"
    assert adoption_allowed(0) is False
    # The current tiny corpus must no-ship today (the v2.2 cause is gated, not repeated).
    assert adoption_allowed(len(_load_tasks(_TRAIN))) is False


if __name__ == "__main__":
    test_train_and_test_non_empty()
    test_test_disjoint_from_train()
    test_val_size_adoption_gate_boundary()
    print("split-disjoint OK")
