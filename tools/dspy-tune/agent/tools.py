"""Fixed-argv `helix <verb>` subprocess shim + curated OpenAI tool schemas.

DEV-TIME / OFFLINE ONLY. This module lives under tools/ and is NEVER linked
into the helix binary, `helix setup`, go.mod, or the default `go test ./...`
path. It is imported in-process by the dev-time ReAct agent (react.py) and,
later, by the Phase-108 GEPA task-success metric.

Security (T-107-01): `run_verb` builds a FIXED argv list and calls
`subprocess.run([...])` — never a shell, never string concatenation. The verb
name is constrained to the curated TOOL_SCHEMAS set, so a model cannot invoke
an arbitrary command; only `helix <curated-verb> <args...>` is ever spawned.

HARNESS-02a: `run-tests` verb calls pytest directly (not helix) to close the
feedback loop. Agent can now verify work by running tests.
"""

import json
import subprocess
from dataclasses import dataclass


@dataclass
class VerbResult:
    """The outcome of one `helix <verb>` invocation."""

    argv: list
    exit: int
    stdout: str
    stderr: str = ""


# Curated subset of the helix routing-table verbs (read + light-edit), kebab
# names verbatim from internal/cli/verbs_gen.go, each declared with its REAL flag
# surface. T-113 fix: helix verbs take NAMED --flags (e.g. `read-file --path X`),
# NOT positionals — the Phase-107 shim passed `location` positionally so EVERY
# verb failed with "required flag --path not set" (the fake-based hermetic test
# could not catch this; only a real-helix smoke did). Kept small (the CLAUDE.md
# read/edit subset) — widen only if task-success is recall-limited.
#
# _VERB_SPECS: verb -> (description, {param_name: (flag, json_type, required)}).
_VERB_SPECS = {
    "read-file": ("Read a file (optionally a line range).", {
        "path": ("--path", "string", True),
        "start_line": ("--start-line", "integer", False),
        "end_line": ("--end-line", "integer", False),
    }),
    "get-symbol-overview": ("List the symbols (outline) in a file.", {
        "path": ("--path", "string", True),
    }),
    "get-diagnostics": ("Get compiler/LSP diagnostics for a file.", {
        "path": ("--path", "string", True),
    }),
    "search-symbols": ("Find symbols by name across the repository.", {
        "query": ("--query", "string", True),
    }),
    "go-to-definition": ("Resolve where a symbol is defined.", {
        "path": ("--path", "string", True),
        "line": ("--line", "integer", True),
        "column": ("--column", "integer", True),
    }),
    "find-references": ("Find all references/callers of a symbol.", {
        "path": ("--path", "string", True),
        "line": ("--line", "integer", True),
        "column": ("--column", "integer", True),
    }),
    "replace-in-file": ("Replace a literal pattern with a replacement in a file.", {
        "path": ("--path", "string", True),
        "pattern": ("--pattern", "string", True),
        "replacement": ("--replacement", "string", True),
    }),
    "fuzzy-edit": ("Drift-tolerant edit: replace a (fuzzily matched) search block.", {
        "path": ("--path", "string", True),
        "search": ("--search", "string", True),
        "replacement": ("--replacement", "string", True),
    }),
    "insert-before-symbol": ("Insert content immediately before a named symbol.", {
        "path": ("--path", "string", True),
        "symbol_name": ("--symbol-name", "string", True),
        "content": ("--content", "string", True),
    }),
    "insert-after-symbol": ("Insert content immediately after a named symbol.", {
        "path": ("--path", "string", True),
        "symbol_name": ("--symbol-name", "string", True),
        "content": ("--content", "string", True),
    }),
    # HARNESS-02a: run-tests verb for feedback loop (calls pytest directly).
    "run-tests": ("Run tests for a file or directory.", {
        "path": ("--path", "string", True),
        "extra_args": ("--extra-args", "string", False),  # optional pytest args
    }),
}


def _schema(name, description, params):
    properties = {}
    required = []
    for pname, (flag, jtype, req) in params.items():
        properties[pname] = {"type": jtype, "description": f"value for {flag}"}
        if req:
            required.append(pname)
    return {
        "type": "function",
        "function": {
            "name": name,
            "description": description,
            "parameters": {
                "type": "object",
                "properties": properties,
                "required": required,
            },
        },
    }


TOOL_SCHEMAS = [_schema(n, d, p) for n, (d, p) in _VERB_SPECS.items()]

# The set of verb names the agent is allowed to spawn (defense-in-depth: even if
# a model fabricates a tool_call name, run_verb only ever runs `helix <verb>`).
VERB_NAMES = set(_VERB_SPECS)


def _argv_for(verb, args):
    """Build a fixed argv `helix <verb> --flag value ...` from decoded tool-call
    args, using the verb's REAL flag surface (_VERB_SPECS). Each declared param
    present in `args` becomes a `--flag value` pair (stable, spec order). Unknown
    keys are ignored (defense-in-depth); a missing required flag is left to helix
    to report so the agent observes the error and retries. Never shell-quoted."""
    argv = ["helix", verb]
    spec = _VERB_SPECS.get(verb)
    if not spec or not isinstance(args, dict):
        return argv
    _desc, params = spec
    for pname, (flag, _jtype, _req) in params.items():
        if pname in args and args[pname] is not None:
            argv.append(flag)
            argv.append(str(args[pname]))
    return argv


def _run_tests_verb(args, cwd, timeout=120):
    """HARNESS-02a: Run pytest directly for the test feedback loop.

    This verb bypasses helix and calls pytest via `uv run pytest` directly,
    allowing the agent to run tests and observe results.
    """
    path = args.get("path", ".")
    extra_args = args.get("extra_args", "")

    # Build argv: uv run pytest <path> --tb=short -q [extra_args]
    argv = ["uv", "run", "pytest", path, "--tb=short", "-q"]
    if extra_args:
        # Split extra_args by space to pass individual args
        argv.extend(extra_args.split())

    proc = subprocess.run(
        argv,
        cwd=cwd,
        capture_output=True,
        text=True,
        timeout=timeout,
    )
    return VerbResult(argv=argv, exit=proc.returncode, stdout=proc.stdout, stderr=proc.stderr)


def run_verb(call, cwd, timeout=60):
    """Run one model tool-call as `helix <verb> <args...>` via fixed-argv subprocess.

    `call` duck-types an OpenAI tool_call: `.function.name` (kebab verb) and
    `.function.arguments` (a JSON STRING). NEVER uses a shell.

    HARNESS-02a: `run-tests` verb dispatches to _run_tests_verb which calls
    pytest directly instead of going through helix.
    """
    verb = call.function.name
    try:
        args = json.loads(call.function.arguments or "{}")
    except json.JSONDecodeError as e:
        # LLM returned malformed JSON - return error instead of crashing
        return VerbResult(
            argv=[verb, f"<malformed-json: {e.msg}>"],
            exit=1,
            stdout="",
            stderr=f"JSON decode error in tool arguments: {e.msg}\nRaw: {call.function.arguments[:200]}"
        )

    # HARNESS-02a: Dispatch run-tests to pytest directly
    if verb == "run-tests":
        return _run_tests_verb(args, cwd, timeout=timeout)

    # All other verbs go through helix subprocess
    argv = _argv_for(verb, args)
    proc = subprocess.run(
        argv,
        cwd=cwd,
        capture_output=True,
        text=True,
        timeout=timeout,
    )
    return VerbResult(argv=argv, exit=proc.returncode, stdout=proc.stdout, stderr=proc.stderr)
