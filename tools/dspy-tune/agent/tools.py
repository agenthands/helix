"""Fixed-argv `helix <verb>` subprocess shim + curated OpenAI tool schemas.

DEV-TIME / OFFLINE ONLY. This module lives under tools/ and is NEVER linked
into the helix binary, `helix setup`, go.mod, or the default `go test ./...`
path. It is imported in-process by the dev-time ReAct agent (react.py) and,
later, by the Phase-108 GEPA task-success metric.

Security (T-107-01): `run_verb` builds a FIXED argv list and calls
`subprocess.run([...])` — never a shell, never string concatenation. The verb
name is constrained to the curated TOOL_SCHEMAS set, so a model cannot invoke
an arbitrary command; only `helix <curated-verb> <args...>` is ever spawned.
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
# names verbatim from internal/cli/verbs_gen.go. Kept intentionally small (the
# CLAUDE.md routing-table read/edit subset) — widen only if Phase-108
# task-success is recall-limited.
_READ_VERBS = {
    "go-to-definition": "Resolve where a symbol is defined (relpath:line:col).",
    "find-references": "Find all references/callers of a symbol.",
    "search-symbols": "Find symbols by name across the repository.",
    "get-symbol-overview": "List the symbols (outline) in a file.",
    "read-file": "Read a file (or a line range).",
    "get-diagnostics": "Get compiler/LSP diagnostics for a file.",
}
_EDIT_VERBS = {
    "replace-in-file": "Replace text in a file.",
    "fuzzy-edit": "Apply a drift-tolerant fuzzy text edit.",
    "insert-before-symbol": "Insert code before a symbol.",
    "insert-after-symbol": "Insert code after a symbol.",
}


def _schema(name, description):
    return {
        "type": "function",
        "function": {
            "name": name,
            "description": description,
            "parameters": {
                "type": "object",
                "properties": {
                    "location": {
                        "type": "string",
                        "description": "A relpath:line:col anchor, a relpath, or a symbol name, as the verb requires.",
                    },
                    "args": {
                        "type": "array",
                        "items": {"type": "string"},
                        "description": "Any additional positional arguments for the verb.",
                    },
                },
                "required": [],
            },
        },
    }


TOOL_SCHEMAS = [_schema(n, d) for n, d in {**_READ_VERBS, **_EDIT_VERBS}.items()]

# The set of verb names the agent is allowed to spawn (defense-in-depth: even if
# a model fabricates a tool_call name, run_verb only ever runs `helix <verb>`).
VERB_NAMES = {s["function"]["name"] for s in TOOL_SCHEMAS}


def _argv_for(verb, args):
    """Build a fixed argv for `helix <verb> ...` from decoded tool-call args.

    `location` (if present) is the leading positional; any `args` array follows.
    Everything is coerced to str and passed as a list element — never shell-quoted.
    """
    argv = ["helix", verb]
    if isinstance(args, dict):
        loc = args.get("location")
        if loc is not None:
            argv.append(str(loc))
        extra = args.get("args")
        if isinstance(extra, (list, tuple)):
            argv.extend(str(v) for v in extra)
        else:
            # Fallback: append remaining scalar values positionally, stably ordered.
            for k, v in args.items():
                if k in ("location", "args"):
                    continue
                argv.append(str(v))
    return argv


def run_verb(call, cwd, timeout=60):
    """Run one model tool-call as `helix <verb> <args...>` via fixed-argv subprocess.

    `call` duck-types an OpenAI tool_call: `.function.name` (kebab verb) and
    `.function.arguments` (a JSON STRING). NEVER uses a shell.
    """
    verb = call.function.name
    args = json.loads(call.function.arguments or "{}")
    argv = _argv_for(verb, args)
    proc = subprocess.run(
        argv,
        cwd=cwd,
        capture_output=True,
        text=True,
        timeout=timeout,
    )
    return VerbResult(argv=argv, exit=proc.returncode, stdout=proc.stdout, stderr=proc.stderr)
