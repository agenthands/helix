# Phase 105: SKILL.md Decision-Matrix Rewrite - Research

**Researched:** 2026-06-24
**Domain:** Hand-authored Agent Skill content (Markdown decision matrix) + Go embed/test seams
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
All implementation choices are at Claude's discretion — the discuss phase was skipped (`workflow.skip_discuss`). The binding spec is the ROADMAP phase goal + the four success criteria, which are reproduced as locked acceptance gates:

1. No decision-matrix row mixes a QUERY (read-state) verb with an ACTION (mutate-state) verb — query and action verbs occupy separate rows (resolves SKILL-ISSUE.md rows 68/70/71/72/73).
2. Every decision-matrix row carries explicit "Not this" guidance — no `—` placeholders remain (the concrete grep/sed/cat/find fallback each verb displaces is named, per the CLAUDE.md routing table as canonical source).
3. Every indexed-graph verb (`get-semantic-graph-status`, `explain-cluster`, `explain-symbol-deep`, `get-change-impact-graph`, `validate-graph-edge`, `find-related-symbols`, `get-semantic-context`) carries a "requires `index-semantic-graph` first" prerequisite note, and the matrix is grouped by capability.
4. The `## Decision matrix` heading (the `StripDecisionMatrix` anchor) is preserved, the rewritten `SKILL.md` stays under the SKILL-04 idle-cost (size) cap, and a SKILL.md↔`VerbToolNames()` cross-check test holds.

### Claude's Discretion
Exact row wording, capability grouping order, prerequisite-note phrasing, sub-heading vs inline grouping — all at Claude's discretion, guided by codebase conventions and the SKILL-ISSUE.md proposed-matrix blueprint.

### Deferred Ideas (OUT OF SCOPE)
None — discuss skipped. **Note:** `reference.md` corrections (the "Output"/"Use this, not that" fixes in SKILL-ISSUE.md §reference.md) were already shipped in **Phase 104** (REFGEN-01). Phase 105 touches **only the hand-authored `SKILL.md`** and its test seams. Do NOT hand-edit `reference.md` (it is generated and `--check`-gated).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SKILL-01 | No matrix row mixes a QUERY verb with an ACTION verb | §QUERY-vs-ACTION classification table (below) gives the per-verb read/mutate label for all 50 verbs; SKILL-ISSUE.md rows 68/70/71/72/73 are the exact offenders, with a ready split in §Proposed Matrix. |
| SKILL-02 | Every row carries "Not this" guidance — no `—` placeholders | §"Not this" Canonical Mapping enumerates a grep/sed/cat/find fallback for every verb, sourced from CLAUDE.md routing table; 6 placeholder rows identified by line number. |
| SKILL-03 | Indexed-graph verbs carry "requires `index-semantic-graph` first" note; matrix grouped by capability | §Indexed-Graph Prerequisite confirms all 7 verbs exist in `VerbToolNames()`; §Capability Grouping gives the grouping; §StripDecisionMatrix Constraint warns about heading placement. |
</phase_requirements>

## Summary

This is a **pure content-rewrite phase on one hand-authored file** (`internal/cli/skills/helix/SKILL.md`, 93 lines / 5,999 bytes) plus extension of one Go test seam. There is **no new code, no new dependency, no generator** involved. The analysis work is already done: `.planning/SKILL-ISSUE.md` (moved out of the embed dir in Phase 103) contains a complete, line-cited diagnosis AND a fully-drafted "after" matrix (lines 152–172), a row-count delta (37→44), and an authoritative QUERY/ACTION classification of all 50 verbs (lines 330–396). The planner should treat SKILL-ISSUE.md as the design spec and CLAUDE.md's "Helix CLI tool routing" table as the canonical "Not this" source.

The four acceptance gates map to concrete, already-discovered seams: the matrix split is mechanical (SKILL-ISSUE rows 68/70/71/72/73), the "Not this" fills come from CLAUDE.md, the indexed-graph prerequisite is a prose note added to 7 reader-verb rows, and the invariants are guarded by (a) the existing `TestSkillVerbMembershipDrift` cross-check, (b) the existing `TestSabotageNonNoop` / `TestSkillDescriptionCap` anchor+cap guards, and (c) a NEW anti-vacuity test the milestone constraint requires (a deliberately mixed/placeholder row must be REJECTED).

The single non-obvious risk: **`## Decision matrix` is the LAST `## ` heading in the file**, so `StripDecisionMatrix` strips from the heading to EOF. Adding any new `## ` heading AFTER the matrix (e.g. a "## Prerequisites" section) would shorten the strip and risk weakening `TestSabotageNonNoop`. Keep capability grouping INSIDE the single matrix section (sub-rows or a `###`-free leading column), or place any new `## ` heading BEFORE the matrix.

**Primary recommendation:** Rewrite the `## Decision matrix` table in place using the SKILL-ISSUE.md "after" draft (lines 152–172) as the base, fill all 6 `—` cells from the CLAUDE.md routing table, add a one-line "requires `index-semantic-graph` first" note to the 7 graph-reader rows, group rows by capability with a `**Capability**` lead-column convention (no new `## ` headings after the matrix), keep the frontmatter description ≤1,536 bytes, and add one anti-vacuity test asserting a fabricated QUERY/ACTION-mixed or `—`-placeholder matrix is rejected.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Decision-matrix content | Hand-authored asset (`SKILL.md`) | — | SKILL.md is NOT generated; it is the idle-tier agent steering text, authored by hand. |
| Embed + ship to disk | `internal/cli/skill.go` (`go:embed`, `installSkill`) | — | Already implemented; closed-set allowlist `{SKILL.md, reference.md}` (Phase 103). No change needed. |
| Idle-cost (size) bound | `skillDescription()` + `TestSkillDescriptionCap`/`TestSkillIdleCostBound` | — | The cap is on the **frontmatter `description`** (≤1,536 bytes), NOT the body. Body length is unbounded. |
| Verb-name correctness | `TestSkillVerbMembershipDrift` (cross-check vs `VerbToolNames()`) | — | Already exists; pins every `\`helix <verb>\`` citation in the body to a real frozen verb. |
| Anchor preservation | `StripDecisionMatrix` + `TestSabotageNonNoop` (`test/oracle/adopt`) | adoption scorecard (`test/oracle/llm`) | The `## Decision matrix` literal is a strip anchor used by the adoption scorecard's sabotage arm. |
| Story consistency w/ reference.md | Phase 104 output (already shipped) | — | reference.md verb set/grouping is the consistency baseline; Phase 105 does not touch it. |

## Standard Stack

No libraries. This phase edits Markdown + Go tests with already-vendored stdlib (`strings`, `regexp`, `testing`). **Zero new Go deps** is a milestone invariant (`git diff go.mod` must stay empty).

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Hand-rewriting SKILL.md | Generating it from the registry (like reference.md) | OUT OF SCOPE — SKILL.md is deliberately hand-authored (terse idle-tier steering with editorial "Not this" judgment); generating it is a future idea, not this phase. The milestone keeps SKILL.md hand-authored, reference.md generated. |

**Installation:** none.

## Package Legitimacy Audit

Not applicable — this phase installs no external packages. (`git diff go.mod` stays empty; milestone-wide "zero new Go deps" constraint.)

## Architecture Patterns

### File-flow Diagram

```
                         (edit this file ONLY)
   internal/cli/skills/helix/SKILL.md  ── hand-authored, frontmatter + ## Decision matrix
            │
            │  //go:embed skills/helix/*   (internal/cli/skill.go)
            ▼
   embeddedSkillFS ──► embeddedSkillBytes() ──► EmbeddedSkillBody()  (SKILL.md ONLY)
            │                    │                       │
            │                    │                       └──► test/oracle/adopt.StripDecisionMatrix(body)
            │                    │                              └─ TestSabotageNonNoop: len(stripped) < len(body)
            │                    │                              └─ adoption scorecard sabotage arm (test/oracle/llm)
            │                    │
            │                    ├──► skillDescription() ──► TestSkillDescriptionCap / TestSkillIdleCostBound  (≤1536 B)
            │                    │
            │                    └──► helixVerbRe (`helix <kebab>`) ──► TestSkillVerbMembershipDrift
            │                              └─ kebab→snake ∈ cli.VerbToolNames()  (50 verbs)
            │
            └──► installSkill(targetDir)  ── ships {SKILL.md, reference.md} closed set (Phase 103; unchanged)

   cli.VerbToolNames()  ── 50 frozen verbs (generated verbs_gen.go) ── the cross-check oracle
   CLAUDE.md "Helix CLI tool routing" table ── canonical "Not this" grep/sed/cat/find mapping
   reference.md (generated, Phase 104) ── verb grouping/semantics consistency baseline (NOT edited)
```

### Recommended Project Structure
No structural change. Files touched:
```
internal/cli/skills/helix/SKILL.md   # the rewrite (only content change)
internal/cli/skill_test.go           # extend: add anti-vacuity QUERY/ACTION + placeholder rejection test
```
(Optionally a small helper in `skill_test.go` to parse matrix rows; keep it in-package, stdlib-only.)

### Pattern 1: In-place matrix rewrite preserving the anchor
**What:** Keep `## Decision matrix` byte-exact as the section heading; rewrite the table body and following prose underneath it.
**When to use:** Always — the heading is a load-bearing strip anchor.
**Example (capability grouping inside one table, no new `## ` heading):**
```markdown
## Decision matrix

Grouped by capability. Verbs marked † require `helix index-semantic-graph` first.

| Capability | Question | Use this | Not this |
|---|---|---|---|
| Navigation | Where is symbol `X` defined? | `helix go-to-definition --path --line --column` | `grep "X"` |
| ... | ... | ... | ... |
| Semantic graph | Build / refresh the graph | `helix index-semantic-graph` / `helix refresh-semantic-graph` | manual indexing |
| Semantic graph | Symbols related to one symbol † | `helix find-related-symbols` | recursive grep |
```
(A leading `Capability` column groups by capability without introducing a new `## ` heading — preserving the StripDecisionMatrix EOF endpoint.)

### Anti-Patterns to Avoid
- **Adding a new `## ` heading after the matrix** (e.g. `## Prerequisites`): shortens `StripDecisionMatrix`'s strip, risks `TestSabotageNonNoop` regression and weakens the adoption-scorecard sabotage arm. Group inside the matrix or place new headings before it.
- **Renaming/altering `## Decision matrix`**: silently no-ops the strip → `TestSabotageNonNoop` goes RED (by design). Don't.
- **Hand-editing `reference.md`**: fails `helix-refgen --check`. Out of scope.
- **Leaving any `—` "Not this" cell**: violates SKILL-02; the new test must reject it.
- **Putting a QUERY and ACTION verb in the same row**: violates SKILL-01.
- **Bloating the frontmatter `description` past 1,536 bytes**: the cap is on the description, not the body — but don't accidentally lengthen the description while editing.
- **Citing a verb not in `VerbToolNames()`** (typo / dead verb): `TestSkillVerbMembershipDrift` goes RED.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Verb-name correctness | A hardcoded verb list in the test | `cli.VerbToolNames()` (existing) + existing `TestSkillVerbMembershipDrift` | Single source of truth; the drift gate already exists. |
| Anchor non-noop proof | A new strip implementation | Existing `StripDecisionMatrix` + `TestSabotageNonNoop` | Already covers anchor drift; just don't break it. |
| Size bound | A custom byte counter | Existing `TestSkillDescriptionCap` / `TestSkillIdleCostBound` | The 1,536-byte cap is already asserted hermetically (no API key). |
| "Not this" fallbacks | Inventing fresh grep/sed wording per verb | CLAUDE.md "Helix CLI tool routing" table (canonical) | Constraint #2 names CLAUDE.md as the canonical source — copy its mapping. |

**Key insight:** Almost every invariant already has a green-path test. The only NEW work the anti-vacuity milestone constraint demands is a *break-the-invariant* test (a deliberately mixed/placeholder matrix must be REJECTED), proving the gate isn't vacuous.

## Runtime State Inventory

Not a rename/refactor/migration phase — this is a content edit to a single embedded asset. No stored data, live-service config, OS-registered state, secrets, or build artifacts carry matrix content. The only "ship" path (`installSkill` → user `.claude/skills/helix/`) re-reads the embedded bytes at `helix setup` time, so a rebuilt binary fully propagates the change. **Nothing found in any category — verified by grep for matrix content outside SKILL.md (none) and by `installSkill` reading exclusively from `embeddedSkillFS`.**

## Common Pitfalls

### Pitfall 1: New `## ` heading after the matrix shortens the strip
**What goes wrong:** Adding `## Prerequisites` (or any `## `) below the table makes `StripDecisionMatrix` stop at that heading instead of EOF, so the matrix body is no longer fully stripped; the sabotage arm strips less than intended.
**Why it happens:** `StripDecisionMatrix` runs from `## Decision matrix` to the *next* `\n## ` (or EOF). Today the matrix is the last `## ` section.
**How to avoid:** Group by capability *inside* the table (lead column or `†` markers) or place any new heading BEFORE the matrix. Keep the matrix the terminal section.
**Warning signs:** `grep -n '^## ' SKILL.md` shows a heading after `## Decision matrix`.

### Pitfall 2: Confusing the size cap target (body vs description)
**What goes wrong:** Planner assumes the whole 5,999-byte body is capped at 1,536 and panics about headroom for +7 rows.
**Why it happens:** SKILL-04 is phrased as an "idle-cost cap," which sounds body-wide.
**How to avoid:** The cap is on the **frontmatter `description`** only (`skillDescription()`, `TestSkillDescriptionCap`, const `cap1536 = 1536`). Current description ≈ 599 bytes — ~937 bytes of headroom. Adding +7 body rows does NOT touch the description. The body is unbounded by any test today. (SKILL-ISSUE.md §Token Count estimates the description grows to ~650 bytes only if the description text itself is expanded — avoid that.)
**Warning signs:** `TestSkillDescriptionCap` / `TestSkillIdleCostBound` RED — means you edited the frontmatter description, not the matrix.

### Pitfall 3: `get-tool-help` has no natural grep fallback
**What goes wrong:** Six rows currently use `—`; one of them (row 75, `get-tool-help`) has no real grep/sed/cat displacement.
**Why it happens:** `get-tool-help` is a meta verb (returns Helix's own docs); there's no shell tool it replaces.
**How to avoid:** SKILL-02 forbids `—`. Supply a *named* non-tool fallback per the CLAUDE.md pattern: CLAUDE.md's product table uses literal "Not this" entries like `guess`, `infer by reading`. For `get-tool-help` use a concrete phrase (e.g. `guessing a verb's args` / `infer by reading`). The gate is "no `—`," not "must be grep/sed/cat" — a named human-fallback satisfies it.
**Warning signs:** A `| —` cell survives the rewrite (the new test must catch it).

### Pitfall 4: reference.md does NOT carry a literal prerequisite note
**What goes wrong:** Planner tries to make SKILL.md "consistent" with reference.md by copying a prerequisite sentence that isn't in reference.md.
**Why it happens:** Success criterion #3 adds the prerequisite note to SKILL.md; reference.md (Phase 104) describes verbs per-entry without an explicit "requires index-semantic-graph first" line.
**How to avoid:** The "one story" consistency is about **verb names, grouping, and semantics** matching — NOT byte-identical prereq prose. The prerequisite note is a SKILL.md-only addition. Don't edit reference.md to match.
**Warning signs:** A diff touching `reference.md`.

## Code Examples

### The matrix verb-citation regex the drift test uses (already in `skill_test.go`)
```go
// Source: internal/cli/skill_test.go:174
var helixVerbRe = regexp.MustCompile("`helix ([a-z][a-z0-9-]+)")
// every captured kebab verb, mapped kebab->snake, must be in cli.VerbToolNames()
```
Implication: write each verb as a backtick-fenced `` `helix some-verb` `` so the drift gate sees it. The matrix already cites all 50 verbs this way.

### The anchor strip (must keep `## Decision matrix` byte-exact)
```go
// Source: test/oracle/adopt/scorecard.go:101
const decisionMatrixHeading = "## Decision matrix"   // load-bearing literal
// StripDecisionMatrix: from heading to next "\n## " or EOF
```

### The size-cap assertion (cap is on the description, not the body)
```go
// Source: internal/cli/skill_test.go:118
const cap1536 = 1536
// total = len(description) [+ len(when_to_use) if present]; must be <= 1536
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| reference.md per-verb "Output"/"Use this, not that" wrong (group-collapse) | Fixed via per-verb generator override | Phase 104 (REFGEN-01) | reference.md is now correct → Phase 105 can align SKILL.md to a correct baseline. |
| `SKILL-ISSUE.md` inside embed dir (leaked into binary + every `helix setup`) | Moved to `.planning/SKILL-ISSUE.md`; closed-set bundle allowlist | Phase 103 (BUNDLE-01) | SKILL-ISSUE.md is now a planning doc, not shipped; it is the design spec for this phase. |
| Matrix mixes QUERY+ACTION, 6 `—` rows, no graph prereqs | This phase's rewrite | Phase 105 | The deliverable. |

**Deprecated/outdated:**
- The old SKILL-ISSUE.md "Files to Update" row for `cmd/helix-refgen/render.go` and the reference.md fix tables (§185–278) are **already done in Phase 104** — ignore them for Phase 105.

## Detailed Findings (grounded in the actual files)

### A. Current matrix structure
- `## Decision matrix` at SKILL.md line 35; table is lines 37–75 (header + separator + **36 data rows**; SKILL-ISSUE counts 37 incl header).
- Columns today: `Question | Use this | Not this`.
- All **50** frozen verbs are cited (confirmed: 50 unique `` `helix <verb>` `` citations == `len(VerbToolNames())==50`).
- Trailing content after the table (lines 77–93): a capability-grouping prose sentence + the SKILL-04 token-note HTML comment. **No `## ` heading follows the matrix** → `StripDecisionMatrix` currently strips to EOF.

### B. The 6 rows with `—` placeholders (SKILL-02 targets)
| Line | Row | Verbs | Needs |
|------|-----|-------|-------|
| 68 | Semantic graph status / build | `get-semantic-graph-status` / `index-semantic-graph` / `refresh-semantic-graph` | SPLIT (status=QUERY vs index/refresh=ACTION) **and** fill "Not this" |
| 69 | Validate a graph edge | `validate-graph-edge` | fill (e.g. `manual trace`) |
| 71 | Search / rename / edit / delete memory | `search-memories` / `rename-memory` / `edit-memory` / `delete-memory` | SPLIT (search=QUERY vs rename/edit/delete=ACTION) **and** fill |
| 72 | Onboard a project / new conversation | `onboard-project` / `prepare-for-new-conversation` | SPLIT (different purposes) **and** fill |
| 73 | Switch profile mode / token budget | `switch-mode` / `get-token-budget` | SPLIT (switch=ACTION vs budget=QUERY) **and** fill |
| 75 | Full tool help | `get-tool-help` | fill (named human-fallback; no grep equivalent — see Pitfall 3) |

### C. The 5 QUERY/ACTION-mixed rows (SKILL-01 targets) — exact, from SKILL-ISSUE.md
| Current row | Mix | Split into |
|-------------|-----|-----------|
| 68 | `get-semantic-graph-status`(Q) + `index-semantic-graph`/`refresh-semantic-graph`(A) | status row (Q) / build-refresh row (A) |
| 70 | `read-memory`/`list-memories`(Q) + `write-memory`(A) | read row (Q) / write row (A) |
| 71 | `search-memories`(Q) + `rename`/`edit`/`delete`-memory(A) | search row (Q) / mutate row (A) |
| 72 | `onboard-project`(Q) + `prepare-for-new-conversation`(A) | onboard row / handoff row |
| 73 | `switch-mode`(A) + `get-token-budget`(Q) | switch row (A) / budget row (Q) |

Result: 36→43 data rows (SKILL-ISSUE phrases it as 37→44 incl header). +7 rows.

### D. QUERY-vs-ACTION classification for ALL 50 verbs (authoritative, from SKILL-ISSUE.md §330–396)
**QUERY (read-state):** go-to-definition, find-references, find-implementations, get-type-hierarchy, get-hover-info, get-symbol-overview, analyze-blast-radius, search-symbols, get-call-hierarchy, find-files, list-directory, read-file, search-in-files, get-diagnostics, get-code-actions, get-repo-map, get-context, get-semantic-context, get-cluster-map, explain-cluster, explain-symbol-deep, find-related-symbols, get-change-impact-graph, get-semantic-graph-status, validate-graph-edge, read-memory, list-memories, search-memories, onboard-project (returns analysis), get-token-budget, get-health, get-tool-help.
**ACTION (mutate-state):** create-file, replace-in-file, rename-symbol, replace-symbol-body, fuzzy-edit, insert-before-symbol, insert-after-symbol, safe-delete-symbol, verify-edit, format-code, index-semantic-graph, refresh-semantic-graph, write-memory, edit-memory, rename-memory, delete-memory, prepare-for-new-conversation (generates handoff), switch-mode.

> Edge calls the planner should ratify: `verify-edit` is listed under Edit/ACTION in SKILL-ISSUE.md but is read-only in spirit (it confirms an edit applied) — it currently shares no row with a mutating verb, so it's not an SKILL-01 offender either way. `onboard-project`=QUERY and `prepare-for-new-conversation`=ACTION must NOT share a row (row 72 split). `format-code`=ACTION but currently sits alone, so not an SKILL-01 offender. Only rows 68/70/71/72/73 are the binding offenders.

### E. Indexed-graph prerequisite (SKILL-03) — confirmed in catalog
All 7 reader verbs exist in `VerbToolNames()` (verified): `get-semantic-graph-status`, `explain-cluster`, `explain-symbol-deep`, `get-change-impact-graph`, `validate-graph-edge`, `find-related-symbols`, `get-semantic-context`. (`get-cluster-map` is also a graph reader and should carry the note too; `index-semantic-graph`/`refresh-semantic-graph` are the builders and do NOT need the note.)
- **Builder verb confirmed:** `index-semantic-graph` (kebab) / `index_semantic_graph` (snake) is in the catalog; reference.md synopsis: "Build or refresh a committed semantic snapshot." Prerequisite phrasing should reference `index-semantic-graph` by exact kebab name (so the drift gate also sees a real verb citation).
- **Recommended note wording (Claude's discretion):** a single marker — e.g. a `†` symbol in the row with one legend line "† requires `helix index-semantic-graph` first" — keeps the table terse and adds only ~1 verb citation. Inline per-row "(requires `helix index-semantic-graph` first)" is also acceptable but more verbose.

### F. "Not this" canonical mapping (SKILL-02) — from CLAUDE.md "Helix CLI tool routing"
The CLAUDE.md product table (in CLAUDE.md, section "Helix CLI tool routing") is the canonical source. Direct mappings to copy:
| Verb(s) | CLAUDE.md "Not this" |
|---------|----------------------|
| go-to-definition | `Grep "X"` |
| find-references | `grep -r "X"` |
| get-call-hierarchy | manual grep chain |
| find-implementations | `grep "implements"` |
| get-type-hierarchy | read + reason |
| get-symbol-overview | `Read <file>` |
| search-symbols | `grep "func X"` |
| get-hover-info | infer by reading |
| analyze-blast-radius | manual trace |
| rename-symbol | `sed` |
| replace-symbol-body | line-number edit |
| insert-before-symbol / insert-after-symbol | regex edit |
| safe-delete-symbol | `sed -d` |
| fuzzy-edit | brittle exact patch |
| replace-in-file | `sed -i` |
| search-in-files | `grep -r` |
| find-files | `find` |
| read-file | `cat` |
| get-diagnostics | parse build output |
| get-code-actions | manual fix |
| format-code | hand-format |
| get-repo-map | read many files |
| get-context / get-semantic-context | several reads / grep chain |
| read-memory / write-memory / search-memories | ad-hoc notes |
For verbs NOT in the CLAUDE.md table (the graph/memory-mutate/workflow/session/meta verbs), use the SKILL-ISSUE.md "after" draft's named fallbacks (lines 159–171): `manual indexing`, `manual trace`, `scratch files`, `ad-hoc notes`, `grep notes`, `manual file edits`, `manual exploration`, `ad-hoc summaries`, `manual config edit`, `mental math`, and for `get-tool-help` a named human-fallback (`guess` / `infer by reading`).

### G. Proposed matrix (ready to adapt) — SKILL-ISSUE.md lines 152–172
SKILL-ISSUE.md already drafts the "after" table for the affected tail rows (semantic-graph through full-tool-help) with splits + "Not this" filled. The planner should adopt that draft and (a) add the indexed-graph prerequisite marker, (b) optionally add a `Capability` lead column for explicit grouping, (c) verify no `—` survives.

## Validation Architecture

> Nyquist validation is enabled for this phase.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` (stdlib) |
| Config file | none — `go test` |
| Quick run command | `go test ./internal/cli/ -run 'Skill'` |
| Full suite command | `go test ./...` (plus `go vet ./...`, `gofmt -w .` per CLAUDE.md) |

### Existing seams (already green — must STAY green)
| Test | File | Asserts |
|------|------|---------|
| `TestSkillVerbMembershipDrift` | `internal/cli/skill_test.go:179` | every `` `helix <verb>` `` in body → kebab→snake ∈ `VerbToolNames()` (the SKILL.md↔VerbToolNames cross-check named in success criterion #4) |
| `TestSkillDescriptionCap` / `TestSkillIdleCostBound` | `internal/cli/skill_test.go:105,141` | frontmatter description ≤ 1,536 bytes (SKILL-04 cap) |
| `TestSabotageNonNoop` | `test/oracle/adopt/scorecard_test.go:119` | `len(StripDecisionMatrix(body)) < len(body)` → proves `## Decision matrix` anchor present + non-trivial |
| `TestSkillTokenNoteFilled` / `TestSkillTokenNotePresent` | `internal/cli/skill_test.go:155,216` | token-note line present with a real digit (no `<N>/<M>`) |
| `TestSkillNoVerbCountLiteral` | `internal/cli/skill_test.go:207` | no hardcoded `5[03] verbs/tools` literal |
| `TestInstallSkillWritesBundle` etc. | `internal/cli/skill_test.go:320` | bundle ships `{SKILL.md, reference.md}` (Phase 103) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SKILL-01 | no matrix row mixes QUERY+ACTION | unit (NEW) | `go test ./internal/cli/ -run TestSkillMatrixNoQueryActionMix` | ❌ Wave 0 |
| SKILL-01 | a fabricated mixed row is REJECTED (anti-vacuity) | unit (NEW) | same test, negative arm on a synthetic mixed table | ❌ Wave 0 |
| SKILL-02 | no `—` "Not this" cell remains | unit (NEW) | `go test ./internal/cli/ -run TestSkillMatrixNoEmptyNotThis` | ❌ Wave 0 |
| SKILL-02 | a fabricated `—` row is REJECTED (anti-vacuity) | unit (NEW) | same test, negative arm on a synthetic `—` table | ❌ Wave 0 |
| SKILL-03 | each of the 7+1 graph-reader rows carries the `index-semantic-graph` prereq | unit (NEW) | `go test ./internal/cli/ -run TestSkillMatrixGraphPrereq` | ❌ Wave 0 |
| #4 | anchor preserved | unit (existing) | `go test ./test/oracle/adopt/ -run TestSabotageNonNoop` | ✅ |
| #4 | size cap held | unit (existing) | `go test ./internal/cli/ -run TestSkillDescriptionCap` | ✅ |
| #4 | SKILL↔VerbToolNames cross-check | unit (existing) | `go test ./internal/cli/ -run TestSkillVerbMembershipDrift` | ✅ |

### How to test (seam design for the NEW tests)
- Parse the `## Decision matrix` table from `embeddedSkillBytes()` with a small in-package helper: locate the heading, take rows beginning `|`, split on `|`, trim. Stdlib only.
- **SKILL-01 (no mix):** classify each verb in a row's "Use this" cell against a QUERY/ACTION map (encode the §D classification as a small in-test `map[string]bool`/two sets, keyed to `VerbToolNames()` so it can't drift). A row containing both a QUERY verb and an ACTION verb fails. **Negative arm:** feed the same checker a hand-built table with a known mixed row and assert it returns an error (proves the gate bites — milestone anti-vacuity requirement).
- **SKILL-02 (no `—`):** assert no matrix data row's last cell is `—`/empty. **Negative arm:** feed a synthetic `| q | v | — |` row, assert rejection.
- **SKILL-03 (prereq present):** for each of the 8 graph-reader verbs, assert its row text contains the prerequisite marker/legend referencing `index-semantic-graph`. **Negative arm:** a synthetic graph-reader row missing the marker is rejected.
- Keep the QUERY/ACTION classification map asserted complete against `VerbToolNames()` (every frozen verb is classified exactly once) so the map can't silently fall behind a future verb add.

### Sampling Rate
- **Per task commit:** `go test ./internal/cli/ -run 'Skill' && go test ./test/oracle/adopt/ -run TestSabotageNonNoop`
- **Per wave merge:** `go test ./... && go vet ./...`
- **Phase gate:** full suite green + `gofmt` clean before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `internal/cli/skill_test.go` — add `TestSkillMatrixNoQueryActionMix` (+negative arm), `TestSkillMatrixNoEmptyNotThis` (+negative arm), `TestSkillMatrixGraphPrereq` (+negative arm), and a matrix-row parse helper. Reuse `cli.VerbToolNames()` for the classification-completeness assertion.
- [ ] No framework install needed (stdlib `testing`).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | build + test | ✓ | (host) | — |
| `gofmt` / `go vet` | CLAUDE.md gates | ✓ | (host) | — |

No external services, no LSP, no network. Purely a content + unit-test change.

## Security Domain

`security_enforcement` is effectively N/A for this phase: it is a Markdown content edit + Go unit tests with no input handling, no network, no auth, no crypto, no new attack surface. The existing path-traversal containment on `installSkill` (`withinSkillRoot`, V12) is unchanged and out of scope. No ASVS category applies to a hand-authored steering-text edit.

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V5 Input Validation | no | N/A — no runtime input; matrix is static text |
| V12 File handling | unchanged | existing `installSkill` containment (not modified) |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The "one story" consistency with reference.md means matching verb names/grouping/semantics, NOT byte-identical prerequisite prose (reference.md has no literal prereq note). | Pitfall 4 | Low — if planner wanted literal parity it would require editing generated reference.md, which is explicitly out of scope. |
| A2 | A named human-fallback (e.g. `guess`/`infer by reading`) satisfies SKILL-02's "no `—`" for meta verbs like `get-tool-help` that displace no shell tool. | Pitfall 3 | Low — criterion text says "no `—` placeholders remain… the concrete grep/sed/cat/find fallback each verb displaces is named"; CLAUDE.md itself uses `guess`/`infer by reading` for such verbs. Planner may add a `checkpoint:human-verify` if stricter reading intended. |
| A3 | `get-cluster-map` should also carry the prerequisite note (it's a graph reader) though it's not in the 7 enumerated in criterion #3. | §E | Low — additive; including it cannot fail the gate, omitting it is also defensible since it's not enumerated. |
| A4 | Body length is unbounded by tests; only the frontmatter description is capped — so +7 rows is safe headroom-wise. | Pitfall 2 | Low — verified: `cap1536` asserted only on `skillDescription()`. |

**These are LOW-risk assumptions consistent with the codebase; none require user confirmation before planning, but A2 is the one a strict reviewer might tighten.**

## Open Questions (RESOLVED)

1. **Grouping presentation: lead `Capability` column vs `###` sub-headings vs `†` markers?**
   - What we know: criterion #3 requires "grouped by capability"; current file uses a trailing prose sentence listing groups.
   - What's unclear: which presentation the reviewer prefers.
   - Recommendation: use a leading `Capability` column (or capability sub-rows) INSIDE the single matrix table — explicit grouping with NO new `## ` heading after the matrix (preserves the StripDecisionMatrix EOF endpoint). Avoid `###` headings between table chunks if they'd split the table; a single table with a Capability column is safest.
   - **RESOLVED:** lead `Capability` column inside the single table, with `†` markers on the indexed-graph rows; NO new `## ` heading after the matrix (preserves the StripDecisionMatrix EOF endpoint). Adopted by 105-PATTERNS.md and 105-01-PLAN.md.

## Sources

### Primary (HIGH confidence — read directly from the repo this session)
- `internal/cli/skills/helix/SKILL.md` — current matrix, 36 data rows, 6 `—` rows (68/69/71/72/73/75), anchor at line 35, description ≈599 B, no `## ` heading after matrix.
- `.planning/SKILL-ISSUE.md` — diagnosis (rows 68/70/71/72/73), "after" draft (152–172), full 50-verb QUERY/ACTION classification (330–396), 37→44 row delta.
- `internal/cli/skill.go` — `go:embed`, `embeddedSkillBytes()`, `skillDescription()`, closed-set `bundleFiles`, `installSkill`.
- `internal/cli/skill_test.go` — `TestSkillVerbMembershipDrift` (cross-check), `TestSkillDescriptionCap`/`TestSkillIdleCostBound` (cap1536), `helixVerbRe`.
- `test/oracle/adopt/scorecard.go` + `scorecard_test.go` — `StripDecisionMatrix`, `decisionMatrixHeading` literal, `TestSabotageNonNoop`.
- `cli.VerbToolNames()` — verified `len==50` via in-package probe; all 7 (+`get-cluster-map`, `index-semantic-graph`, `refresh-semantic-graph`) graph verbs present.
- `internal/cli/skills/helix/reference.md` — verb synopses/grouping baseline; `index-semantic-graph` synopsis "Build or refresh a committed semantic snapshot"; no literal prereq note.
- `CLAUDE.md` "Helix CLI tool routing" table — canonical "Not this" mapping.
- `.planning/REQUIREMENTS.md`, `.planning/STATE.md` — SKILL-01/02/03 scope, v2.2 constraints, anti-vacuity + StripDecisionMatrix/SKILL-04 mandates.

### Secondary / Tertiary
- None — every claim is grounded in repo files read this session.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no deps; verified zero-new-dep constraint.
- Architecture: HIGH — all seams read directly (embed, tests, anchor).
- Pitfalls: HIGH — anchor/cap/heading mechanics verified against source.
- Classification: HIGH — taken verbatim from SKILL-ISSUE.md, cross-checked against `VerbToolNames()`.

**Research date:** 2026-06-24
**Valid until:** 2026-07-24 (stable; content phase, no fast-moving externals). Re-verify only if `VerbToolNames()` count changes or the matrix/anchor is touched by another phase first.
