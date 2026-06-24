"""Python re-implementation of the Phase 101 adopt classifier.

This module mirrors test/oracle/adopt/scorecard.go over the realistic
transcript domain (printable agent output). It is the parity-pinned Python
scorer the DSPy optimizer (optimize.py) wraps as its metric. The Go package
is the SINGLE SOURCE OF TRUTH; this is a faithful port, asserted
case-for-case against the SHARED golden corpus
(tools/dspy-tune/golden/parity_cases.json) by both test_parity.py here and
test/oracle/adopt/parity_test.go on the Go side.

Known boundary (immaterial, documented — WR-01-A): Python str.strip() also
strips the C0 ASCII separators U+001C-U+001F, which Go's strings.TrimSpace
(unicode.IsSpace) does not, so a first line beginning with one of those
control chars would classify differently. Such chars never begin a real LLM
transcript line, so the divergence is out-of-domain; the golden corpus pins
the realistic domain where Go and Python agree exactly. Do NOT "fix" this by
matching Python's broader whitespace set — that would introduce the opposite
divergence on the Unicode space separators Go's IsSpace accepts.

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
    backticks and re-strip; drop a leading "$ " THEN a leading "> " prompt
    (two UNCONDITIONAL SEQUENTIAL strips, mirroring Go's two back-to-back
    strings.TrimPrefix calls — NOT a mutually-exclusive if/elif); return the
    first surviving line. Returns "" if none.

    The second strip is a separate `if`, not `elif`: on a stacked prompt line
    like "$ > helix foo" Go strips BOTH prefixes ("$ " then "> ") to yield
    "helix foo". An if/elif here would strip only "$ " and leave "> helix foo",
    silently diverging from the Go truth (WR-01).
    """
    for raw in response.split("\n"):
        line = raw.strip()
        if line == "" or line.startswith("```"):
            continue
        line = line.strip("`").strip()
        if line.startswith("$ "):
            line = line[2:]
        if line.startswith("> "):  # second `if`, NOT `elif` — sequential like Go
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
