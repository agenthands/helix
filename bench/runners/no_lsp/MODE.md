---
mode: no_lsp
profile: bench-no-lsp
---

# no_lsp

The LSP-disabled ablation arm. Resolves to the `bench-no-lsp` profile
(internal/profile/profiles/bench-no-lsp.yaml). This arm exercises Helix with
its LSP subsystem turned off (`disable_lsp_subsystem`, Phase 76 ABLATE-05) so
the matrix can isolate the contribution of LSP-backed symbol resolution
(go-to-definition, find-references, cross-file rename) versus the tree-sitter
RepoMap and fuzzy-edit paths that survive without a language server.

The `bench-no-lsp` profile drops the symbol-retrieval and diagnostics skills
(skill-selection, Phase 76 D-09); the kernel-level LSP disable flag is
consumed by the daemon, so LSP-dependent tools fail closed rather than
silently degrading.
