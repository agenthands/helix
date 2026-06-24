"""Metric-gaming guard: flag degenerate always-`helix` steering text.

choice_rate is a gameable first-command proxy: GEPA could discover that
steering the model to emit `helix` UNCONDITIONALLY (regardless of the task)
maximizes the metric — a degenerate win that would harm real usage
(106-RESEARCH Pitfall 4). This guard ships the break-the-invariant PAIR:

  * test_degenerate_always_helix_flagged   — a degenerate unconditional text
    is FLAGGED (is_degenerate_steering -> True).
  * test_legitimate_steering_not_flagged   — a legitimate CONDITIONAL text
    (prefer helix when semantic, fall back for free-text) is NOT flagged.

A guard that flagged nothing (or everything) would be vacuous; the flag/no-flag
pair proves it discriminates degenerate from legitimate steering.
"""

import re

# Phrases that instruct UNCONDITIONAL helix use regardless of the task. The
# "regardless"/"no matter"/"every"/"always ... helix" shapes are the degenerate
# tell: they remove the task-conditionality that legitimate steering preserves.
_DEGENERATE_PATTERNS = (
    r"always\s+(emit|use|run|output|prefer)\s+.*helix",
    r"helix.*regardless\s+of\s+the\s+task",
    r"regardless\s+of\s+the\s+task.*helix",
    r"helix.*no\s+matter\s+(what|the\s+task)",
    r"(every|each)\s+(time|task|prompt|query).*helix",
    r"helix.*(every|each)\s+(time|task|prompt|query)",
    r"unconditionally\s+.*helix",
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
    # when the unconditional cue is absent; "regardless of the task" is itself
    # unconditional, so do not let a stray " if " veto it.
    if "regardless" in low or "no matter" in low or "unconditionally" in low:
        return True
    if any(m in low for m in _CONDITIONAL_MARKERS):
        return False
    return True


def test_degenerate_always_helix_flagged():
    assert is_degenerate_steering(
        "Always emit a helix command first, regardless of the task."
    ) is True


def test_legitimate_steering_not_flagged():
    assert is_degenerate_steering(
        "Prefer helix verbs over grep/sed/cat when the question is semantic; "
        "fall back to grep for free-text in comments."
    ) is False


if __name__ == "__main__":
    test_degenerate_always_helix_flagged()
    test_legitimate_steering_not_flagged()
    print("degenerate-guard OK")
