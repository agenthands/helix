---
mode: your_agent_full
profile: bench-full
---

# your_agent_full

The full-tool-surface bench mode. Resolves to the `bench-full` profile
(internal/profile/profiles/bench-full.yaml) — every Helix code-intelligence
tool available, every subsystem enabled.

This is the seed mode for Phase 77's first end-to-end smoke (D-05). The
remaining five ablation modes and the full ABLATE-01 `MODE.md` convention land
in Phase 80; the resolver that reads this frontmatter is table-driven so those
modes drop in as new `bench/runners/<mode>/MODE.md` directories with no Go
change.
