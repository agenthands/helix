# Phase 25 Context: Fuzzy Edit Engine

**Phase:** 25 — Fuzzy Edit Engine
**Milestone:** v1.6 — Context Intelligence & Resilient Editing
**Discussed:** 2026-04-15
**Requirements:** FUZZ-01, FUZZ-02, FUZZ-03, FUZZ-07, FUZZ-08

## Domain Boundary

Build a **self-contained fuzzy text matching/replacement library** that downstream phase 26 will wire into `replace_symbol_body`, `replace_content`, and a new standalone `fuzzy_edit` MCP tool. **No new MCP tools land in this phase.** No tool registration, no MCP wiring, no changes to existing edit/fileops callers — those are Phase 26's job. Phase 25 ends when the engine is implemented, unit-tested, and importable.

## Decisions

### 1. Package shape & API

- **Location:** `internal/fuzzy/` — standalone package, zero deps on `internal/kernel/edit/` or `internal/kernel/fileops/`. Both Phase 26 callers import it cleanly without crossing each other.
- **Primary entry point:** `func Match(source, search string, opts Options) (*Result, error)`
  - `Result` carries: `Strategy` (enum), `Score` (float64), `StartByte`, `EndByte`, `MatchedText` (the original region), `ReplacementText` (after indentation reflow — caller-supplied replacement passed via `Options`).
  - Engine is **pure** — no file I/O. Caller reads file, calls `Match`, decides whether to write. Enables dry-run, composition, and clean unit tests.
- **Options struct fields (initial):**
  - `Replacement string` — what to substitute (engine reflows indentation onto this)
  - `AllowEllipsis bool` — opt-in to ellipsis segmentation
  - (Future, deferred to FUZZ-09) `MinScore float64` — configurable threshold

### 2. Matching algorithm

- **Unit:** **Line-based throughout** all 4 strategies. Aider-style. Search and source split on `\n`; comparisons happen line-by-line.
- **The 4-strategy cascade:**
  1. **Exact** — byte-for-byte line equality
  2. **Whitespace-normalized** — `strings.TrimSpace` each line before compare (collapses internal whitespace differences only at line boundaries; do NOT collapse internal spaces inside lines unless tests show it's needed)
  3. **Indentation-flexible** — strip leading whitespace only (`strings.TrimLeft(line, " \t")`); internal whitespace must match
  4. **Fail-with-diff** — return `serr.InvalidArgs` with a unified-diff-style payload showing what was searched vs nearest candidate
- **`similarity_score` reporting (FUZZ-02):** discrete tier values
  - exact → `1.0`
  - whitespace-normalized → `0.95`
  - indentation-flexible → `0.85`
  - fail → `0.0`
  - No real similarity computation needed. Simple, deterministic, agent-readable.
- **`match_strategy` field:** string enum: `"exact" | "whitespace_normalized" | "indentation_flexible" | "failed"`.

### 3. Ellipsis placeholder semantics (FUZZ-08)

- **Marker:** literal `...` (three ASCII dots) on its **own line**, optionally preceded by whitespace (the leading ws is ignored).
- **Segmentation:** the `...` line splits the search block into **segments**. Each segment must match in order in the source, with arbitrary lines allowed between segments. Multiple `...` lines in a single search block are supported (N segments separated by N-1 ellipses).
- **Anchor requirement:** every segment must contain at least 1 non-empty anchor line. Empty segments (e.g., search starting or ending with `...`) are rejected with a validation error.
- **Replacement application:** when a search block uses ellipsis, the replacement block must use the SAME segmentation pattern (same number of `...` markers in matching positions). Each replacement segment substitutes its corresponding source region; the `...`-bridged unchanged content is preserved verbatim.
- **Inline `...` is NOT a marker** — it is treated as literal text (so Go variadic args, Python `Ellipsis`, etc. survive).

### 4. Ambiguity detection (FUZZ-07)

- When a strategy finds **N > 1 matches** in the source, the engine **does not pick one** — it returns `serr.InvalidArgs` immediately at that strategy level (does not cascade to looser strategies).
- **Error payload:** structured detail listing match locations as **1-indexed line numbers**, capped at **5**. If more than 5 matches exist, append `"...and N more"` to the detail. Message format:
  ```
  ambiguous match: search block matches N locations (lines 12, 47, 89, 134, 201, ...and 3 more)
  ```
- Agent's recovery path: add more context lines to the search block to disambiguate.

### 5. Indentation preservation (FUZZ-03)

- **Algorithm — common-prefix dedent + reapply (Aider-style):**
  1. Compute the **longest common leading-whitespace prefix** across all non-empty lines of the search block. Call this `searchPrefix`.
  2. Strip `searchPrefix` from every line of the search block (that's what gets matched against the source after the chosen strategy's normalization).
  3. Strip the **same number of leading characters** from every line of the replacement block (assume the agent indented replacement and search consistently).
  4. Read the **first matched line's leading whitespace** from the source — call this `sourcePrefix`.
  5. Prefix every line of the dedented replacement with `sourcePrefix` and write it back.
- **Empty/whitespace-only lines** in the replacement: leave empty (no prefix added) to avoid trailing-whitespace lint failures.
- **Tabs vs spaces:**
  - Treat **tab = 1 character**, no expansion. Comparisons and prefix-length calculations operate on raw bytes.
  - The **indentation-flexible strategy** strips ALL leading whitespace, so a search block using spaces will still match a source region using tabs (and vice versa).
  - The **reapply step uses the matched region's actual leading bytes** verbatim — no tab-width assumptions, no normalization. If the source uses tabs, the replacement gets tabs; if spaces, spaces.

## Out of Scope for Phase 25

These belong to **Phase 26** and must NOT leak into Phase 25's plans:

- Standalone `fuzzy_edit` MCP tool registration (FUZZ-04)
- Wiring fallback into `replace_symbol_body` (FUZZ-05)
- Wiring fallback into `replace_content` (FUZZ-06)
- Any changes to `internal/kernel/edit/replace.go` or `internal/kernel/fileops/replace.go`
- Any changes to MCP tool registration in the daemon

These are **future requirements**, not Phase 25:

- Configurable `MinScore` per call (FUZZ-09 — listed as Future in REQUIREMENTS.md)

## Deferred Ideas

None raised during discussion.

## Canonical Refs

- `.planning/ROADMAP.md` — Phase 25 success criteria
- `.planning/REQUIREMENTS.md` — FUZZ-01..03, FUZZ-07, FUZZ-08
- `.planning/PROJECT.md` — Core value & constraints
- `internal/kernel/edit/replace.go` — current `ReplaceBody`/`ReplaceBodyWithPlan` (Phase 26 will integrate here)
- `internal/kernel/fileops/replace.go` — current `ReplaceInFile` (Phase 26 will integrate here)
- `internal/errors/` — typed error package; engine returns `serr.InvalidArgs` for ambiguity and fail-with-diff (per v1.5 conventions)

External references for the researcher:

- Aider's `coders/editblock_coder.py` and `search_replace.py` — canonical reference for the line-based 4-strategy cascade and common-prefix dedent algorithm. The researcher should read these to confirm the algorithm details match what we've decided.

## Notes for Researcher

- Confirm Aider's exact whitespace-normalization rules (per-line `TrimSpace` vs intra-line collapse) and document any deviation we should make.
- Surface any Go-native libraries for line-level diff that could power the fail-with-diff payload (e.g., `github.com/sergi/go-diff` or `github.com/pmezard/go-difflib`) — note license and dep weight; we prefer stdlib if a hand-rolled diff is sufficient.
- Verify there is no existing fuzzy/diff utility in `internal/` that we'd be duplicating (`grep -r fuzzy internal/` returned nothing at discuss time).

## Notes for Planner

- Engine is pure, so plans should land as: (1) types & options, (2) line splitter + ellipsis segmenter, (3) the 4 strategies as separate functions, (4) cascade orchestrator, (5) indentation reflow, (6) error/diff formatting, (7) unit tests covering every strategy, ambiguity, ellipsis, and indent edge case. Each plan should be small enough for a single atomic commit.
- Tests must hit every success criterion in ROADMAP.md Phase 25 explicitly — strategy reporting, indent inheritance, ambiguity refusal, ellipsis anchoring.
- No integration tests in this phase — those land in Phase 26 once the engine has callers.
