# Phase 16: Core Documentation Update - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-11
**Phase:** 16-core-documentation-update
**Areas discussed:** README scope & structure, CONTRIBUTING modernization, CHANGELOG completeness, Auto-generated content

---

## README Scope & Structure

### Q1: How should v1.2 capabilities be added?

| Option | Description | Selected |
|--------|-------------|----------|
| Surgical inserts | Add to existing sections, minimal structural change | |
| New dedicated section | Add "Production & Observability" section with subsections | ✓ |
| You decide | Claude picks the approach | |

**User's choice:** New dedicated section
**Notes:** None

### Q2: Where should the new section go?

| Option | Description | Selected |
|--------|-------------|----------|
| After install/config | Users see how to get running first | |
| Before install, after "How Serena Works" | Production features as selling point | ✓ |
| You decide | Claude picks placement | |

**User's choice:** Before install, after "How Serena Works"
**Notes:** None

---

## CONTRIBUTING Modernization

### Q1: How should Python content be handled?

| Option | Description | Selected |
|--------|-------------|----------|
| Replace entirely | Go-only CONTRIBUTING, Python legacy not needed | ✓ |
| Keep Python in collapsed section | Go front and center, Python in details block | |
| You decide | Claude picks approach | |

**User's choice:** Replace entirely
**Notes:** None

### Q2: What level of detail for dev workflow?

| Option | Description | Selected |
|--------|-------------|----------|
| Commands only | Quick-reference table | |
| Commands + context | Commands plus brief explanations | |
| Full walkthrough | Commands, context, integration tests, adding tools/languages, benchmark CI | ✓ |

**User's choice:** Full walkthrough
**Notes:** None

---

## CHANGELOG Completeness

### Q1: How should the CHANGELOG be organized?

| Option | Description | Selected |
|--------|-------------|----------|
| Keep thematic grouping | Fill gaps for Phase 14 and 15, readers care about features | ✓ |
| Phase-by-phase | Each of 7 phases gets own subsection | |
| You decide | Claude picks approach | |

**User's choice:** Keep thematic grouping, fill gaps
**Notes:** None

### Q2: Should CHANGELOG get a v1.3 stub?

| Option | Description | Selected |
|--------|-------------|----------|
| Add v1.3 stub now | Signals work in progress | |
| Wait until v1.3 ships | Add full entry when both phases done | ✓ |
| You decide | Claude picks timing | |

**User's choice:** Wait until v1.3 ships
**Notes:** None

---

## Auto-generated Content

### Q1: How should auto-generated tables be handled?

| Option | Description | Selected |
|--------|-------------|----------|
| Regenerate from current state | Re-run generation to ensure accuracy | ✓ |
| Verify only | Spot-check, update only if wrong | |
| You decide | Claude picks approach | |

**User's choice:** Regenerate from current state
**Notes:** None

### Q2: Should README document admin endpoints?

| Option | Description | Selected |
|--------|-------------|----------|
| Mention in README | Brief list in Production & Observability section, details in USAGE.md | ✓ |
| USAGE.md only | README stays focused on MCP tools | |
| You decide | Claude picks approach | |

**User's choice:** Mention in README, details in USAGE.md
**Notes:** None

---

## Claude's Discretion

- README narrative flow and section ordering beyond "Production & Observability" placement
- CONTRIBUTING section ordering and headings
- CHANGELOG wording for gap-fill entries

## Deferred Ideas

None — discussion stayed within phase scope.
