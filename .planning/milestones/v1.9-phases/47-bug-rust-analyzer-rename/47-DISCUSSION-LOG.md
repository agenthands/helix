# Phase 47: bug-rust-analyzer-rename - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-24
**Phase:** 47-bug-rust-analyzer-rename
**Areas discussed:** Fix strategy, Workaround mechanism, Failure UX, Test + verification shape

---

## Fix strategy

### Q: What's the primary strategy for this phase?

| Option | Description | Selected |
|--------|-------------|----------|
| Hybrid: fix + fallback | Attempt real fix (indexing readiness wait) first; fall back to workaround on failure. Matches roadmap's "fix OR workaround" wording. | ✓ |
| Fix-only: chase readiness signal | Invest only in real fix. No safety net if it doesn't pan out. | |
| Workaround-only | Skip the fix; ship client-side rename as QuirkAdapter override + USAGE.md doc. | |

**User's choice:** Hybrid: fix + fallback

### Q: Is RCA in-scope?

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, RCA is required | Reproduce failure in a harness, capture LSP wire traces, document trigger. Roadmap SC#4 requires this. | ✓ |
| No, jump to workaround | Treat as upstream bug; don't isolate further. | |

**User's choice:** Yes, RCA is required

---

## Workaround mechanism

### Q: What fallback ships if native fix is insufficient?

| Option | Description | Selected |
|--------|-------------|----------|
| Client-side rename via references | QuirkAdapter calls `textDocument/references`, collects ranges, applies text edits. Works because references already succeed. Documented semantic limits. | ✓ |
| Retry with indexing-readiness wait | Block rename until rust-analyzer signals indexing complete. May still hit upstream bug. | |
| Hover+references warmup, then rename | Trigger resolver via hover+references before rename. No evidence this fixes the specific error. | |
| Structured error only | Don't attempt workaround rename; return error pointing to fuzzy_edit. Honest but ships no rename capability. | |

**User's choice:** Client-side rename via references

### Q: Where does the workaround live?

| Option | Description | Selected |
|--------|-------------|----------|
| QuirkAdapter RenameOverride hook | Add optional method to QuirkAdapter; rename.go checks for it. Centralizes quirks. | ✓ |
| Branch inside rename.go by language | Detect language inline, call Rust helper directly. Simpler but bleeds quirk out of adapter layer. | |

**User's choice:** QuirkAdapter RenameOverride hook

### Q: Is a semantic-accuracy limitation acceptable?

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, document the limits | Client-side rename covers common case; document cross-crate / macro gaps in USAGE.md and QuirkAdapter doc. Reference BUG-DEFER-02. | ✓ |
| No — must match rust-analyzer exactly | Refuse to ship workaround unless it replicates cross-crate analysis. Effectively forces fix-only. | |

**User's choice:** Yes, document the limits

---

## Failure UX

### Q: How should rename_symbol respond when Rust rename ultimately fails?

| Option | Description | Selected |
|--------|-------------|----------|
| Structured error + fuzzy_edit suggestion | Typed error via serr with agent-actionable pointer to fuzzy_edit / replace_symbol_body / search_in_files. | ✓ |
| Plain error with docs pointer | Generic error referencing USAGE.md. Less actionable for agents. | |
| Degraded success: partial edits + warning | Apply found edits; warn about incompleteness. Half-rename can break code. | |

**User's choice:** Structured error + fuzzy_edit suggestion

### Q: Should the strategy used be surfaced in the result?

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — strategy tag in result | Return `strategy: "lsp-native" \| "rust-client-side"` mirroring fuzzy.Match pattern; emit bounded-label metric counter. | ✓ |
| No — success is success | Keep result shape unchanged; hide implementation. | |

**User's choice:** Yes — strategy tag in result

---

## Test + verification shape

### Q: How should the passing rename test be structured?

| Option | Description | Selected |
|--------|-------------|----------|
| Unskip existing test + strategy assertion | Remove `t.Skip` at rust_test.go:120, keep helper→renamed_helper case, assert strategy tag. Minimal delta. | ✓ |
| Unskip + new semantic-coverage cases | Also add multi-file / trait-method rename cases. More thorough, more flake risk. | |
| Replace existing with deterministic harness | Scrap current test; build new fixture designed to reproduce the exact trigger. Heavier. | |

**User's choice:** Unskip existing test + strategy assertion

### Q: Regression guardrail for hover / references / search?

| Option | Description | Selected |
|--------|-------------|----------|
| Existing TestSymbols_RustFixture coverage is enough | Keep existing hover / references / find_implementations assertions green. | ✓ |
| Add explicit before/after rename check | Run hover+references before AND after rename to prove no state corruption. | |

**User's choice:** Existing TestSymbols_RustFixture coverage is enough

### Q: Does shipping require hosted-CI green too?

| Option | Description | Selected |
|--------|-------------|----------|
| Local green is the gate; CI tracked separately | Hosted-runner rust-analyzer availability owned by TOOL-01 / Phase 50. | ✓ |
| Must be green on hosted CI too | Blocks on Phase 50 toolchain work; entangles phases. | |

**User's choice:** Local green is the gate; CI runner tracked separately

---

## Claude's Discretion

- Exact readiness signal used for the native-fix attempt (`$/progress` vs. `rust-analyzer/analyzerStatus` vs. re-issuing `prepareRename`) — guided by RCA.
- Exact serr error code name and message wording.
- Shape of the strategy metric counter (label set, name) under the current telemetry layer.

## Deferred Ideas

- Submitting an upstream rust-analyzer patch — owned by BUG-DEFER-02.
- Full semantic parity (cross-crate / macro-expansion) with native rust-analyzer rename.
- Generalizing `RenameOverride` to other LSes preemptively.
