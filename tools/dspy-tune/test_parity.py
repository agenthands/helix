"""Python<->Go parity over the SHARED golden corpus + planted-divergence guard.

This is the Python half of the cross-language parity contract. It reads the
SAME committed file the Go parity test reads
(tools/dspy-tune/golden/parity_cases.json — also asserted by
test/oracle/adopt/parity_test.go::TestPythonGoParityCorpus). Because both sides
assert against one file, agreement on the corpus implies semantic parity on
every listed case (106-RESEARCH "The Parity Contract Design").

Two required tests:
  * test_python_go_parity        — scorer agrees with every corpus case.
  * test_broken_classifier_diverges — the named CR-01 anti-vacuity proof: a
    deliberately-wrong `"helix" in response` substring classifier produces a
    DIFFERENT result from the real scorer on the substring-trap case, proving
    the corpus discriminates a real classifier from a broken one. Without such a
    discriminating case, the parity check would be vacuous.

Both functions are plain pytest-discoverable (no fixtures) and the module is
also runnable as a script (`python3 test_parity.py`) so the parity is provable
even when pytest is not installed.
"""

import json
import os

import scorer

_CORPUS = os.path.join(os.path.dirname(__file__), "golden", "parity_cases.json")


def _load_corpus():
    with open(_CORPUS, encoding="utf-8") as fh:
        return json.load(fh)


def _broken_classify(response: str):
    """A deliberately-wrong classifier: the substring trap. It keys `chose` on
    whether the word "helix" appears ANYWHERE in the (lowercased) response,
    rather than on the first emitted command's prefix. This is exactly the bug
    101-RESEARCH Pitfall 2 warns against; it MUST diverge from scorer on the
    substring-trap corpus case (prose mentioning helix, then a grep).
    """
    return "helix" in response.lower()


def test_python_go_parity():
    """scorer.first_command / classify_choice agree with EVERY corpus case —
    the same cases test/oracle/adopt/parity_test.go pins to the Go truth.
    """
    cases = _load_corpus()
    assert len(cases) >= 8, "corpus must cover every classifier branch"
    for c in cases:
        assert scorer.first_command(c["response"]) == c["first_command"], (
            f"first_command mismatch on {c['response']!r}"
        )
        assert scorer.classify_choice(c["response"]) == (c["chose"], c["fell_back"]), (
            f"classify_choice mismatch on {c['response']!r}"
        )


def test_broken_classifier_diverges():
    """Planted-divergence anti-vacuity: the broken substring classifier must
    disagree with the real scorer on at least one corpus case (the substring
    trap). Deleting that case from the corpus would make this test fail to find
    a discriminating case — so the corpus's discriminative power is itself
    under test.
    """
    cases = _load_corpus()
    # The substring-trap case: prose mentioning "helix" on the first line, then
    # a grep, classified as NOT a choice (chose == False) by the real scorer.
    trap = [
        c
        for c in cases
        if "helix" in c["response"].lower()
        and not c["chose"]
        and "\n" in c["response"]
    ]
    assert trap, "corpus is missing the substring-trap discriminator case"
    diverged = False
    for c in trap:
        real_chose, _ = scorer.classify_choice(c["response"])
        broken_chose = _broken_classify(c["response"])
        if broken_chose != real_chose:
            # broken says True (substring "helix" present) while the real scorer
            # says False (first command is prose, not "helix ").
            assert broken_chose is True and real_chose is False
            diverged = True
    assert diverged, (
        "the broken substring classifier did NOT diverge from scorer on any "
        "corpus case — parity would be vacuous"
    )


if __name__ == "__main__":
    test_python_go_parity()
    test_broken_classifier_diverges()
    print("parity OK")
