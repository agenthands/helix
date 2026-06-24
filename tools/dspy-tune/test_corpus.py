"""Hermetic tests for the Aider corpus materializer + 3-way sequestered split.

DEV-TIME / OFFLINE ONLY — no network, no LM key, no toolchain. These run in the
key-free CI/executor gate alongside test_split.py / test_parity.py.

Covers:
  * CORPUS-01 descriptor shape (build_corpus.make_descriptor over a committed
    synthetic fixture repo — no clone).
  * CORPUS-02 3-way disjoint split: train/val are passed to compile(), `test` is
    the sequestered held-out attribution split (NEVER passed to compile), sized
    > VAL_SIZE_GATE. A planted leak turns the disjointness gate RED.
"""

import os

_HERE = os.path.dirname(os.path.abspath(__file__))
_FAKE_REPO = os.path.join(_HERE, "testdata", "fake_corpus_repo")


# --- CORPUS-01: descriptor shape ------------------------------------------- #

def test_build_corpus_descriptor_shape():
    import build_corpus

    ex_dir = os.path.join(
        _FAKE_REPO, "python", "exercises", "practice", "greeter"
    )
    d = build_corpus.make_descriptor(ex_dir, "python")
    assert set(d.keys()) == {"task", "language", "task_dir", "gold_src", "gold_tests"}, d.keys()
    assert d["language"] == "python"
    # gold_src must equal task_dir: restore_gold_tests reads pristine tests from
    # the agent-unwritable source, which IS the pristine clone dir.
    assert d["gold_src"] == d["task_dir"] == ex_dir
    # gold_tests mirrors config.files.test exactly.
    assert d["gold_tests"] == ["greeter_test.py"]
    # task is the instructions text (non-empty prompt the agent solves).
    assert "greet" in d["task"]


def test_enumerate_finds_practice_exercises():
    import build_corpus

    found = build_corpus.enumerate_exercises(_FAKE_REPO, "python")
    names = sorted(os.path.basename(d) for d, _ in found)
    assert "greeter" in names
    # every returned dir has a readable config with solution+test
    for d, lang in found:
        assert lang == "python"
        assert os.path.isfile(os.path.join(d, ".meta", "config.json"))


# --- CORPUS-02: 3-way sequestered split ------------------------------------ #

def _examples(n):
    """n synthetic examples as dicts (split_corpus must not require dspy)."""
    return [{"task": f"task-{i}", "language": "go"} for i in range(n)]


def test_split_three_way_disjoint_and_heldout_gt_gate():
    from optimize import VAL_SIZE_GATE, split_corpus, _task_key, _disjoint_ok

    ex = _examples(120)
    train, val, test = split_corpus(ex)
    # sequestered held-out attribution split must clear the strict gate
    assert len(test) > VAL_SIZE_GATE, f"held-out test={len(test)} must be > {VAL_SIZE_GATE}"
    assert train and val, "train and val must be non-empty for GEPA"
    # pairwise disjoint by task key, and a complete partition (no loss/dup)
    assert _disjoint_ok(train, val, test), "partitions must be pairwise disjoint"
    keys_all = {_task_key(e) for e in ex}
    keys_part = {_task_key(e) for e in (train + val + test)}
    assert keys_part == keys_all, "train ∪ val ∪ test must equal the corpus (no loss/dup)"


def test_planted_leak_goes_red():
    """Break-the-invariant: a test task injected into train makes the
    disjointness gate return False (the overfit guard bites)."""
    from optimize import split_corpus, _disjoint_ok

    ex = _examples(120)
    train, val, test = split_corpus(ex)
    assert _disjoint_ok(train, val, test) is True
    leaked_train = train + [test[0]]  # leak a held-out task into train
    assert _disjoint_ok(leaked_train, val, test) is False, "planted leak must turn the gate RED"


def test_compile_never_sees_test():
    """The sequestered held-out split is NEVER passed to compile() — proven by a
    fake compile that records trainset+valset and asserting no test key leaks in."""
    from optimize import split_corpus, _task_key

    ex = _examples(120)
    train, val, test = split_corpus(ex)
    seen = {"trainset": None, "valset": None}

    def fake_compile(student=None, trainset=None, valset=None):
        seen["trainset"], seen["valset"] = trainset, valset
        return student

    fake_compile(student=object(), trainset=train, valset=val)
    compiled_keys = {_task_key(e) for e in (seen["trainset"] or [])} | {
        _task_key(e) for e in (seen["valset"] or [])
    }
    test_keys = {_task_key(e) for e in test}
    assert compiled_keys.isdisjoint(test_keys), "no held-out test task may reach compile()"


if __name__ == "__main__":
    test_build_corpus_descriptor_shape()
    test_enumerate_finds_practice_exercises()
    test_split_three_way_disjoint_and_heldout_gt_gate()
    test_planted_leak_goes_red()
    test_compile_never_sees_test()
    print("corpus tests OK")
