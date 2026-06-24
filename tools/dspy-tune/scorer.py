"""Python re-implementation of the Phase 101 adopt classifier.

This module mirrors test/oracle/adopt/scorecard.go EXACTLY. It is the
parity-pinned Python scorer the DSPy optimizer (optimize.py) wraps as its
metric. The Go package is the SINGLE SOURCE OF TRUTH; this is a faithful
port, asserted case-for-case against the SHARED golden corpus
(tools/dspy-tune/golden/parity_cases.json) by both test_parity.py here and
test/oracle/adopt/parity_test.go on the Go side.

Critical invariants (each a Phase 101 Pitfall; see 106-RESEARCH.md):
  * Classification keys on the PREFIX of the FIRST emitted command via
    first_command() -> startswith(...), NEVER `"helix" in response` over the
    whole transcript. The injected SKILL.md is saturated with the word
    "helix"; a substring/Contains check would inflate the choice rate
    (the substring trap, 101-RESEARCH Pitfall 2).
  * The trailing space in every FALLBACK_PREFIXES entry is LOAD-BEARING: it
    keys on the invocation token, so "lsp" is NOT an "ls " fallback.
  * Lowercasing happens ONCE, on the extracted first command, never on the
    whole response.
"""

# The standard tools the helix skill is meant to displace. A first command
# starting with one of these is a FALLBACK. The trailing space matters: it
# keys on the invocation token, not a substring (e.g. "ls " not "ls", so
# "lsp" is not a fallback). Mirrors scorecard.go:77 fallbackPrefixes.
FALLBACK_PREFIXES = ("grep ", "sed ", "cat ", "find ", "rg ", "ls ")


def first_command(response: str) -> str:
    """Return the first non-empty command line of response with common
    command-line decoration stripped: surrounding code fences, surrounding
    backticks, and a leading "$ "/"> " shell prompt.

    Verbatim port of FirstCommand (scorecard.go:59-72): iterate split("\\n");
    per line strip(); skip "" or startswith("```"); then strip surrounding
    backticks and re-strip; drop a leading "$ " or "> " prompt; return the
    first surviving line. Returns "" if none.
    """
    for raw in response.split("\n"):
        line = raw.strip()
        if line == "" or line.startswith("```"):
            continue
        line = line.strip("`").strip()
        if line.startswith("$ "):
            line = line[2:]
        elif line.startswith("> "):
            line = line[2:]
        return line.strip()
    return ""


def classify_choice(response: str):
    """Classify a response by the PREFIX of its FIRST emitted command.

    Returns (chose, fell_back). chose is True iff the first command starts with
    "helix "; fell_back is True iff it starts with a fallback tool prefix. A
    prose or unrecognized first line yields (False, False) — unclassified.

    Mirrors ClassifyChoice (scorecard.go:85-95): lowercase ONCE on the
    extracted command (NOT the whole response), then HasPrefix — deliberately
    NOT a `"helix" in response` substring check (the substring trap).
    """
    cmd = first_command(response).lower()
    chose = cmd.startswith("helix ")
    fell_back = any(cmd.startswith(t) for t in FALLBACK_PREFIXES)
    return chose, fell_back


def score_choice_rate(response: str):
    """Thin wrapper returning classify_choice(response) for optimize.py to
    consume as its parity-checked metric. Kept distinct so the optimizer wraps
    a named scoring entry point rather than reaching into classify_choice
    directly.
    """
    return classify_choice(response)
