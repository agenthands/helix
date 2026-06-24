"""Aider-polyglot task-success corpus materializer (dev-time only).

DEV-TIME / OFFLINE ONLY — lives under tools/, NEVER linked into the helix
binary, `helix setup`, go.mod, or the default `go test ./...` path.

Materializes the GEPA reward corpus consumed by `optimize.py::_load_aider_corpus`
from the vendored, pinned `Aider-AI/polyglot-benchmark` clone (the same pin the
Go leaf `bench/datasets/aider-polyglot/pin.go` uses). Each exercise becomes one
JSON descriptor:

    {task, language, task_dir, gold_src, gold_tests}

where `task` is the exercise's `.docs/instructions.md` prompt, `task_dir` ==
`gold_src` is the pristine (agent-unwritable) exercise dir the sandbox copies
from, and `gold_tests` mirrors `.meta/config.json` files.test verbatim
(restore_gold_tests overwrites these from the pristine source before grading).

CORPUS-01 / TUNE-FUT-01: grow the corpus so `optimize.split_corpus`'s sequestered
held-out split clears `val_size > 50`. `--check` exits non-zero if the requested
build produced < 101 descriptors — a fail-not-skip gate (a requested build that
silently came up short is a hard error, not a green vacuous pass).

Provenance: a `corpus_manifest.json` records the pinned sha + per-language counts.
Licensing: Exercism polyglot fixtures are MIT — see the existing
`bench/datasets/aider-polyglot/LICENSE-AUDIT.md`; this script vendors NOTHING into
the tree (the corpus dir is git-ignored), it only materializes descriptors that
point at the local clone cache.
"""

import argparse
import json
import os
import re
import subprocess
import sys

# Pinned sha — MUST match bench/datasets/aider-polyglot/pin.go PinnedSHA.
PINNED_SHA = "7e0611e77b54e2dea774cdc0aa00cf9f7ed6144f"
REPO_URL = "https://github.com/Aider-AI/polyglot-benchmark"

# CORPUS-01 floor: the sequestered held-out split must exceed VAL_SIZE_GATE (50);
# with a 3-way split the whole corpus needs comfortably more than that. 101 is the
# minimum that lets split_corpus carve a >50 held-out split AND keep train+val.
MIN_TASKS = 101

_HEX40 = re.compile(r"^[0-9a-f]{40}$")


def _cache_root():
    base = os.environ.get("HELIX_CACHE_DIR") or os.path.join(
        os.path.expanduser("~"), ".cache", "helix"
    )
    return os.path.join(base, "aider-polyglot")


def clone_or_reuse(sha=PINNED_SHA):
    """Shallow-clone the pinned repo into the cache (reuse if HEAD already == sha).
    Returns the repo root. Network only on a cold cache."""
    if not _HEX40.match(sha):
        raise ValueError(f"refusing non-hex-sha40 pin: {sha!r}")
    dest = os.path.join(_cache_root(), sha[:8])
    head = os.path.join(dest, ".git")
    if os.path.isdir(head):
        cur = subprocess.run(
            ["git", "-C", dest, "rev-parse", "HEAD"],
            capture_output=True, text=True,
        ).stdout.strip()
        if cur == sha:
            return dest
    os.makedirs(dest, exist_ok=True)
    # depth-1 default-branch clone; HEAD is asserted == pin below (the pinned sha
    # is the repo's current HEAD per pin.go / clone_test.go).
    subprocess.run(["git", "clone", "--depth", "1", REPO_URL, dest], check=True)
    cur = subprocess.run(
        ["git", "-C", dest, "rev-parse", "HEAD"], capture_output=True, text=True
    ).stdout.strip()
    if cur != sha:
        raise RuntimeError(
            f"clone HEAD {cur} != pinned {sha}; the upstream default branch moved — "
            f"re-pin PINNED_SHA (and bench/datasets/aider-polyglot/pin.go) deliberately."
        )
    return dest


def _instructions_text(ex_dir):
    """The agent-facing task prompt: .docs/instructions.md (+ optional
    instructions.append.md), falling back to introduction.md. Hard-error if none."""
    docs = os.path.join(ex_dir, ".docs")
    parts = []
    for name in ("introduction.md", "instructions.md", "instructions.append.md"):
        p = os.path.join(docs, name)
        if os.path.isfile(p):
            with open(p, encoding="utf-8") as fh:
                parts.append(fh.read().strip())
    text = "\n\n".join(p for p in parts if p)
    if not text:
        raise FileNotFoundError(f"no .docs instructions for exercise {ex_dir}")
    return text


def make_descriptor(ex_dir, language):
    """Build one `_load_aider_corpus`-shaped descriptor for an exercise dir.

    gold_src == task_dir == the pristine exercise dir (sandbox copies FROM it and
    restore_gold_tests reads pristine tests FROM it — the agent never edits it).
    gold_tests mirrors .meta/config.json files.test verbatim.
    """
    cfg_path = os.path.join(ex_dir, ".meta", "config.json")
    with open(cfg_path, encoding="utf-8") as fh:
        cfg = json.load(fh)
    files = cfg.get("files", {})
    solution = files.get("solution", [])
    test = files.get("test", [])
    if not solution or not test:
        raise ValueError(f"{cfg_path} missing files.solution or files.test")
    return {
        "task": _instructions_text(ex_dir),
        "language": language,
        "task_dir": ex_dir,
        "gold_src": ex_dir,
        "gold_tests": list(test),
    }


def enumerate_exercises(repo_root, track):
    """Yield (exercise_dir, language) for every practice exercise in a track that
    has a readable .meta/config.json with solution+test. Missing/invalid configs
    are skipped with a warning (never silently counted)."""
    practice = os.path.join(repo_root, track, "exercises", "practice")
    out = []
    if not os.path.isdir(practice):
        return out
    for name in sorted(os.listdir(practice)):
        ex_dir = os.path.join(practice, name)
        cfg = os.path.join(ex_dir, ".meta", "config.json")
        if not os.path.isfile(cfg):
            continue
        try:
            with open(cfg, encoding="utf-8") as fh:
                c = json.load(fh)
            f = c.get("files", {})
            if not f.get("solution") or not f.get("test"):
                print(f"  skip {track}/{name}: no solution/test in config", file=sys.stderr)
                continue
        except (OSError, json.JSONDecodeError) as e:
            print(f"  skip {track}/{name}: bad config ({e})", file=sys.stderr)
            continue
        out.append((ex_dir, track))
    return out


def build(tracks, out_dir, sha=PINNED_SHA):
    """Materialize descriptors for the given tracks into out_dir. Returns
    (total, per_language)."""
    repo = clone_or_reuse(sha)
    os.makedirs(out_dir, exist_ok=True)
    per_language = {}
    total = 0
    for track in tracks:
        exs = enumerate_exercises(repo, track)
        for ex_dir, lang in exs:
            try:
                d = make_descriptor(ex_dir, lang)
            except (OSError, ValueError, FileNotFoundError) as e:
                print(f"  skip {lang}/{os.path.basename(ex_dir)}: {e}", file=sys.stderr)
                continue
            name = f"{lang}__{os.path.basename(ex_dir)}.json"
            with open(os.path.join(out_dir, name), "w", encoding="utf-8") as fh:
                json.dump(d, fh, indent=2)
            per_language[lang] = per_language.get(lang, 0) + 1
            total += 1
    manifest = {
        "pinned_sha": sha,
        "repo": REPO_URL,
        "tracks": list(tracks),
        "total": total,
        "per_language": per_language,
        "license": "Exercism polyglot fixtures are MIT — see "
        "bench/datasets/aider-polyglot/LICENSE-AUDIT.md",
    }
    with open(os.path.join(out_dir, "corpus_manifest.json"), "w", encoding="utf-8") as fh:
        json.dump(manifest, fh, indent=2)
    return total, per_language


def main(argv=None):
    ap = argparse.ArgumentParser(description="Materialize the Aider task-success corpus.")
    ap.add_argument("--tracks", default="go,python,rust",
                    help="comma-separated language tracks (default: go,python,rust)")
    ap.add_argument("--out", default=os.path.join(os.path.dirname(os.path.abspath(__file__)), "corpus"),
                    help="output corpus dir (git-ignored; default tools/dspy-tune/corpus/)")
    ap.add_argument("--check", action="store_true",
                    help="exit non-zero if the materialized corpus has < %d tasks "
                         "(fail-not-skip)" % MIN_TASKS)
    args = ap.parse_args(argv)
    tracks = [t.strip() for t in args.tracks.split(",") if t.strip()]
    total, per_language = build(tracks, args.out)
    print(f"materialized {total} descriptors into {args.out}")
    print(f"per_language: {per_language}")
    if args.check and total < MIN_TASKS:
        print(
            f"FAIL: corpus has {total} < {MIN_TASKS} tasks — the held-out split "
            f"cannot clear val_size > 50. Add more tracks or exercises.",
            file=sys.stderr,
        )
        return 1
    print(f"OK: {total} >= {MIN_TASKS}" if args.check else "(no --check gate)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
