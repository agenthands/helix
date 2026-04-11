# Phase 16: Core Documentation Update - Context

**Gathered:** 2026-04-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Update README, CHANGELOG, and CONTRIBUTING to accurately reflect Serena's current Go-native state and v1.2 capabilities (observability, metrics, tracing, graceful degradation, benchmarks). No new features — documentation catchup only.

</domain>

<decisions>
## Implementation Decisions

### README Scope & Structure
- **D-01:** Add a new dedicated "Production & Observability" section covering metrics, tracing, admin listener, and graceful degradation
- **D-02:** Position this section before install/config, right after "How Serena Works" — production features are a selling point
- **D-03:** Admin endpoints (/healthz, /readyz, /metrics, /debug/pprof) get a brief mention in the new README section as feature highlights; detailed config belongs in USAGE.md (Phase 17)

### CONTRIBUTING Modernization
- **D-04:** Replace Python content entirely — CONTRIBUTING becomes Go-only. No legacy Python instructions (legacy/ is reference-only, not actively developed)
- **D-05:** Full walkthrough level of detail — commands, context, plus how the integration test harness works, how to add new MCP tools, how to add language support, how benchmark CI gates work

### CHANGELOG Completeness
- **D-06:** Keep thematic grouping (not phase-by-phase). Fill gaps for Phase 14 (Documentation) and Phase 15 (Benchmark Gate Hardening) that are currently missing from the v1.2 entry
- **D-07:** No v1.3 stub — add v1.3 entry only when the full milestone ships (after Phase 17)

### Auto-generated Content
- **D-08:** Regenerate tool and language tables from current state (re-run generation scripts or rebuild from code)

### Claude's Discretion
- README narrative flow and section ordering beyond the "Production & Observability" placement
- CONTRIBUTING section ordering and headings
- CHANGELOG wording for gap-fill entries

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing Documentation (update targets)
- `README.md` — Current README (240 lines), has auto-generated tool/language tables from Phase 14
- `CONTRIBUTING.md` — Current CONTRIBUTING (42 lines), mostly legacy Python instructions
- `CHANGELOG.md` — Current CHANGELOG (86 lines), has v1.2 thematic entry with gaps

### Codebase Maps (context for accurate docs)
- `.planning/codebase/ARCHITECTURE.md` — 4-layer architecture details for README accuracy
- `.planning/codebase/STACK.md` — Technology stack for README/CONTRIBUTING accuracy
- `.planning/codebase/CONVENTIONS.md` — Code patterns for CONTRIBUTING walkthrough
- `.planning/codebase/TESTING.md` — Test infrastructure for CONTRIBUTING integration test docs
- `.planning/codebase/STRUCTURE.md` — Directory layout for CONTRIBUTING orientation

### Milestone References (for CHANGELOG gap-fill)
- `.planning/milestones/v1.2-ROADMAP.md` — All 7 phases (9-15) for verifying CHANGELOG completeness

### Source of Truth (for auto-generated tables)
- `internal/langregistry/` — 52-language registry for language table regeneration
- `internal/mcp/` — MCP tool registry for tool table regeneration

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- README already has solid structure (logo, tagline, "How Serena Works", tool/language tables, install, config)
- CHANGELOG has working thematic grouping format for v1.2

### Established Patterns
- Auto-generated tables in README (tool count, language support) — generation approach from Phase 14
- CHANGELOG uses thematic sections (Benchmark Harness, Observability, Tracing, Graceful Degradation)

### Integration Points
- `internal/obs/` — observability package for README production features section
- `internal/kernel/lspool/` — circuit breaker, graceful degradation for README mention
- `test/` — integration test harness for CONTRIBUTING walkthrough
- `test/bench/` — benchmark harness for CONTRIBUTING walkthrough
- `.github/workflows/` — CI workflows for CONTRIBUTING benchmark gate docs

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches within the decisions above.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 16-core-documentation-update*
*Context gathered: 2026-04-11*
