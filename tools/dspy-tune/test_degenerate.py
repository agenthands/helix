"""Metric-gaming guard: flag degenerate always-`helix` steering text.

choice_rate is a gameable first-command proxy: GEPA could discover that
steering the model to emit `helix` UNCONDITIONALLY (regardless of the task)
maximizes the metric — a degenerate win that would harm real usage
(106-RESEARCH Pitfall 4). This guard ships the break-the-invariant PAIR:

  * test_degenerate_always_helix_flagged   — degenerate unconditional texts
    are FLAGGED (is_degenerate_steering -> True), across several realistic
    always-helix phrasings GEPA could plausibly emit.
  * test_legitimate_steering_not_flagged   — a legitimate CONDITIONAL text
    (prefer helix when semantic, fall back for free-text) is NOT flagged.

A guard that flagged nothing (or everything) would be vacuous; the flag/no-flag
pair proves it discriminates degenerate from legitimate steering.

SCOPE (WR-02): `is_degenerate_steering` is a deliberately broadened HEURISTIC
matcher over common always-helix phrasings — NOT a complete metric-gaming
classifier. It can still be evaded by phrasings outside its pattern list. The
real adoption backstop is the human-review re-entry gate: optimize.py never
writes SKILL.md/reference.md, and adopted steering re-enters the shipped surface
ONLY via a human-reviewed commit. Treat this guard as a pre-review smell test,
not a sound gate. (It is not wired into optimize.py; it is exercised only here.)
"""

import re

# Phrases that instruct UNCONDITIONAL helix use regardless of the task. The
# "always/start/begin/respond ... helix", "regardless"/"no matter",
# "every/each/any/all <task-noun>" shapes are the degenerate tell: they remove
# the task-conditionality that legitimate steering preserves.
#
# This is a HEURISTIC pattern list, deliberately broadened (WR-02) to catch the
# realistic always-helix phrasings GEPA could plausibly emit (e.g. "Always start
# your answer with helix.", "Output a helix command for every input.", "Begin
# with a helix verb in all cases."). It is NOT a complete metric-gaming
# classifier — the human-review re-entry gate (no auto-adopt; optimize.py never
# writes SKILL.md/reference.md) remains the real backstop. See the module
# docstring.
_DEGENERATE_PATTERNS = (
    # always/start/begin verb-phrases that lead with helix
    r"(always|start|begin|respond|answer)\b.*\bhelix",
    r"\bhelix\b.*(always|start with|begin with)",
    # "<verb> a helix command/verb ..." with no task-conditionality
    r"(emit|use|run|output|prefer|start|begin)\b.*\bhelix\b",
    # "for every/any/each/all <task-noun>" universally-quantified scopes
    r"helix.*(for\s+)?(every|any|each|all)\s+(time|task|prompt|query|input|case|answer)",
    r"(for\s+)?(every|any|each|all)\s+(time|task|prompt|query|input|case|answer).*helix",
    r"\bin\s+all\s+cases\b.*helix",
    r"helix.*\bin\s+all\s+cases\b",
    # explicit regardless / no-matter / unconditionally cues
    r"helix.*regardless\s+of\s+the\s+task",
    r"regardless\s+of\s+the\s+task.*helix",
    r"helix.*no\s+matter\s+(what|the\s+task)",
    r"no\s+matter\s+(what|the\s+task).*helix",
    r"unconditionally\s+.*helix",
    r"helix.*unconditionally",
)

# Conditionality markers that signal LEGITIMATE, task-aware steering. Their
# presence vetoes a degenerate flag: legitimate steering tells the model WHEN
# to prefer helix and WHEN to fall back.
_CONDITIONAL_MARKERS = (
    "when the question is semantic",
    "fall back",
    "fallback",
    "for free-text",
    "for free text",
    "in comments",
    " when ",
    " if ",
)


def is_degenerate_steering(text: str) -> bool:
    """Return True iff text instructs unconditional helix use (metric-gaming).

    A text is degenerate if it matches an unconditional-helix pattern AND does
    not carry a conditionality marker that scopes the steering to a task class.
    """
    low = text.lower()
    matched = any(re.search(p, low) for p in _DEGENERATE_PATTERNS)
    if not matched:
        return False
    # A conditional marker rescues an otherwise-degenerate-looking phrasing only
    # when an UNCONDITIONAL cue is absent. "regardless of the task", "no matter",
    # "unconditionally", and universal quantifiers ("every/any/each/all <task>",
    # "in all cases") are themselves unconditional, so do not let a stray " if "
    # or "fall back" veto them.
    unconditional_cues = (
        "regardless",
        "no matter",
        "unconditionally",
        "in all cases",
    )
    if any(c in low for c in unconditional_cues):
        return True
    if re.search(r"(every|any|each|all)\s+(time|task|prompt|query|input|case|answer)", low):
        return True
    if any(m in low for m in _CONDITIONAL_MARKERS):
        return False
    return True


def test_degenerate_always_helix_flagged():
    assert is_degenerate_steering(
        "Always emit a helix command first, regardless of the task."
    ) is True


# Realistic always-helix phrasings GEPA could plausibly emit. The original
# narrow pattern list (WR-02) let 4 of these 5 slip through; the broadened
# heuristic now flags them. Kept as an explicit list so a future pattern
# regression turns this test RED.
_DEGENERATE_SAMPLES = (
    "Output a helix command for every input.",
    "Always start your answer with helix.",
    "For any task, emit helix first.",
    "Begin with a helix verb in all cases.",
    "Just respond with helix no matter what.",
)


def test_realistic_degenerate_phrasings_flagged():
    for text in _DEGENERATE_SAMPLES:
        assert is_degenerate_steering(text) is True, f"missed degenerate: {text!r}"


def test_legitimate_steering_not_flagged():
    assert is_degenerate_steering(
        "Prefer helix verbs over grep/sed/cat when the question is semantic; "
        "fall back to grep for free-text in comments."
    ) is False


if __name__ == "__main__":
    test_degenerate_always_helix_flagged()
    test_realistic_degenerate_phrasings_flagged()
    test_legitimate_steering_not_flagged()
    print("degenerate-guard OK")
