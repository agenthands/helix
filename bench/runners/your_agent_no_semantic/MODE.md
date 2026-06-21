---
mode: your_agent_no_semantic
profile: bench-no-semantic
---

# your_agent_no_semantic

The semantic-store-disabled ablation arm. Resolves to the `bench-no-semantic`
profile (internal/profile/profiles/bench-no-semantic.yaml). This arm excludes
the 10 semantic-store (DuckDB) tools so the matrix can isolate the
contribution of the semantic graph (`get_semantic_context`,
`explain_symbol_deep`, `find_related_symbols`, the cluster maps, and
change-impact analysis) versus the LSP symbol tools and the tree-sitter
RepoMap that remain available.

## Deferred guarantee — `ablation_status: guarantee_pending_phase_81`

This arm runs and emits a **real** result row, but that row is marked
**partial** via `ablation_status: guarantee_pending_phase_81`. The reason is
that the kernel-level `disable_semantic_subsystem` flag — the zero-DuckDB
guarantee, ABLATE-06 — does **not** land until Phase 81. Until that flag
ships, the `bench-no-semantic` profile is tool-filter-only (Phase 76
D-11/D-12): the 10 semantic-store tools are excluded from the agent's
surface, but the daemon may still read the semantic store via back-channel
paths (`get_repo_map` / `get_context` degrade to tree-sitter rather than
being hard-disabled at the kernel).

Therefore the row this arm produces is **not yet a clean no_semantic
measurement**. The `guarantee_pending_phase_81` status flags it as such for
downstream consumers: the number is honest about what it measures today (a
tool-filtered surface, not a kernel-disabled subsystem), and the clean
zero-DuckDB measurement arrives once Phase 81 wires the kernel flag with its
config-gate E2E test.

(Directory name `your_agent_no_semantic` is chosen per the CONTEXT
Claude's-discretion note for Phase 81 forward-compatibility.)
