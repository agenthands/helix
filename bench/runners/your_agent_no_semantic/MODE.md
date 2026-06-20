---
mode: no_semantic
profile: bench-no-semantic
---

# no_semantic

The semantic-store-disabled ablation arm. Resolves to the `bench-no-semantic`
profile (internal/profile/profiles/bench-no-semantic.yaml). This arm exercises
Helix with its semantic-graph subsystem turned off at the kernel
(`disable_semantic_subsystem`, ABLATE-06) so the matrix can isolate the
contribution of the DuckDB semantic graph (cluster maps, change-impact
analysis, cross-symbol relations) versus the LSP symbol tools and the
tree-sitter RepoMap that survive without it.

The kernel gate is the koanf field **`semantic_index.bench_disabled`**
(`semantic.Config.BenchDisabled`; note the nested path — unlike the top-level
`disable_lsp_subsystem`). Its precedence is **CLI `--disable-semantic-subsystem`
> profile YAML `disable_semantic_subsystem` > default-off**: the
`bench-no-semantic.yaml` profile carries `disable_semantic_subsystem: true`, and
an explicit `--disable-semantic-subsystem` CLI flag (when set) overrides via
`overrides["semantic_index.bench_disabled"] = true`; absent both, the subsystem
stays enabled. The daemon resolves `effSemanticDisabled := cfg.SemanticIndex.BenchDisabled
|| activeProfile.DisableSemanticSubsystem` once at the composition root (Plan
81-04).

This is a **build-but-block** gate, not a build-skip: the DuckDB semantic store
is STILL built and could be queried, but every back-channel read consumer is
forced to `integ.NoopLookup{}` with a disabled `ConfigGate`, so the retrieval
source resolves to `tree_sitter` (not a degraded fallback). The independent
runtime proof of the gate is that an E2E `no_semantic` run reports
`helix_semantic_store_reads_total == 0`; the bench cell scrapes that counter from
the daemon's shutdown log line and **fails the cell hard** on any non-zero read
(Plan 81-05, D-05). Because the store exists and the assertion still measures
zero, the guarantee is non-vacuous.

The gate un-wires Phase 65's `SetSemanticLookup` strangler-fig for the full set
of semantic consumers. A future contributor MUST keep ALL of these routed
through the gate (or `integ.ChooseSource`) — ripping out the bypass for any one
of them would leak a semantic read onto the no_semantic arm and corrupt the
ablation measurement:

- `get_repo_map`
- `get_context`
- `find_related_symbols`
- `explain_symbol_deep`
- `validate_graph_edge`
- `analyze_blast_radius`
- `RankFiles`
- `ExpandFrom`

(Directory name `your_agent_no_semantic` is retained for forward-compatibility;
the resolved `mode` frontmatter value is `no_semantic`.)
