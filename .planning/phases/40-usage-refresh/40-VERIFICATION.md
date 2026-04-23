---
phase: 40-usage-refresh
verified: 2026-04-23T14:30:00Z
status: gaps_found
score: 6/9 must-haves verified
overrides_applied: 0
gaps:
  - truth: "RepoMap tools documented with correct parameter names"
    status: failed
    reason: "USAGE.md documents get_repo_map parameter as max_tokens but actual parameter in source code (internal/skill/repomap/skill.go) is token_budget. Agents following this documentation will pass a parameter that gets silently ignored or triggers a smart-error suggestion."
    artifacts:
      - path: "USAGE.md"
        issue: "Lines 153 and 159: max_tokens should be token_budget"
    missing:
      - "Replace max_tokens with token_budget in RepoMap subsection text (line 153)"
      - "Replace get_repo_map(max_tokens=4096) with get_repo_map(token_budget=4096) in code example (line 159)"
  - truth: "fuzzy_edit code example uses correct named parameters"
    status: failed
    reason: "USAGE.md line 146 shows fuzzy_edit with 5 positional arguments but the actual tool (internal/kernel/fileops/tools.go) takes 3 named parameters: path, search, replacement. Ellipsis segments are embedded inside search/replacement strings, not passed as separate arguments. An agent following this example would produce an invalid tool call."
    artifacts:
      - path: "USAGE.md"
        issue: "Line 146: positional args instead of named params path, search, replacement"
    missing:
      - "Replace fuzzy_edit example with named-parameter form: fuzzy_edit(path=\"src/handler.go\", search=\"func HandleRequest(\\n...\\n)\", replacement=\"func HandleRequest(ctx context.Context,\\n...\\n)\")"
  - truth: "Troubleshooting section reflects current known issues including rust-analyzer rename"
    status: failed
    reason: "Roadmap SC-3 specifies rust-analyzer rename as a known issue to document. The research file (40-RESEARCH.md) incorrectly stated it was 'already documented' at USAGE.md lines 420-434, but no dedicated rust-analyzer rename troubleshooting entry exists in USAGE.md (before or after phase 40 changes). Only a generic mention of rust-analyzer as an install command exists."
    artifacts:
      - path: "USAGE.md"
        issue: "Missing dedicated rust-analyzer rename troubleshooting entry with Symptom/Cause/Fix pattern"
    missing:
      - "Add rust-analyzer rename troubleshooting entry documenting the known limitation where textDocument/rename returns 'No references found at position' in fresh workspaces"
---

# Phase 40: USAGE Refresh Verification Report

**Phase Goal:** USAGE.md comprehensively documents all features through v1.7 so users can discover and use every capability
**Verified:** 2026-04-23T14:30:00Z
**Status:** gaps_found
**Re-verification:** No -- initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Reader can discover fuzzy editing capabilities (4-strategy cascade, ellipsis support) in a Feature Guide section | FAILED | Section exists at line 139 with correct content, BUT code example (line 146) uses positional args instead of named params -- would produce invalid tool call (WR-02) |
| 2 | Reader can discover RepoMap tools (get_repo_map, get_context) with token budget parameters | FAILED | Section exists at line 149, BUT documents parameter as `max_tokens` when actual param is `token_budget` -- would cause silent failure (WR-01) |
| 3 | Reader can discover all v1.7 features: setup CLI, smart errors, progressive descriptions, lazy init, health monitoring | VERIFIED | All 5 v1.7 features documented in subsections at lines 163-215 with concept + example pattern |
| 4 | Tutorial 1 uses serena setup claude-code instead of manual JSON config | VERIFIED | Line 22 shows `serena setup claude-code`; no `mcpServers` JSON block remains |
| 5 | Feature Guide section is positioned between Quick Tutorials and Profiles and Modes | VERIFIED | Feature Guide at line 137, after Quick Tutorials (line 7), before Profiles and Modes (line 217) |
| 6 | Reader can find jdtls cold-start delay troubleshooting entry with timeout_index workaround | VERIFIED | Entry at line 493 with Symptom/Cause/Fix pattern, includes `timeout_index: 300` YAML example |
| 7 | Reader can find gopls/Go 1.25 benchmark constraint entry with version compatibility workaround | VERIFIED | Entry at line 511 with Symptom/Cause/Fix pattern, mentions v0.17.1 incompatibility and Go 1.24+ requirement |
| 8 | New troubleshooting entries follow the existing Symptom/Cause/Fix pattern | VERIFIED | Both entries use bold **Symptom:**/**Cause:**/**Fix:** labels matching existing entries |
| 9 | Troubleshooting section reflects current known issues including rust-analyzer rename | FAILED | jdtls and gopls entries present, but rust-analyzer rename entry missing entirely (roadmap SC-3 requires it) |

**Score:** 6/9 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `USAGE.md` | Feature Guide section with 7 subsections plus updated Tutorial 1 | VERIFIED | 7 subsections present (Fuzzy Editing, RepoMap and Context, Setup CLI, Smart Errors, Progressive Descriptions, Lazy Workspace Init, Health Monitoring). Tutorial 1 updated. |
| `USAGE.md` | Two new troubleshooting entries for jdtls and gopls | VERIFIED | Both entries present with Symptom/Cause/Fix pattern |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| USAGE.md Feature Guide | USAGE.md Configuration Reference | cross-reference links to config keys | WIRED | Line 215: `See [Configuration Reference](#configuration-reference) for degradation.timeout_index` |
| USAGE.md jdtls troubleshooting | USAGE.md Configuration Reference | cross-reference to degradation.timeout_index | WIRED | Line 504: `timeout_index: 300` references config key documented in Configuration Reference |

### Data-Flow Trace (Level 4)

Not applicable -- documentation-only phase, no dynamic data rendering.

### Behavioral Spot-Checks

Step 7b: SKIPPED (documentation-only phase, no runnable entry points to test)

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| USAGE-01 | 40-01 | USAGE.md documents all v1.6 features (fuzzy editing, RepoMap/get_context, 23 tree-sitter grammars) | PARTIAL | All features documented but with factual inaccuracies: wrong parameter name (max_tokens vs token_budget) and incorrect code example (positional vs named args) |
| USAGE-02 | 40-01 | USAGE.md documents all v1.7 features (setup CLI, health tool, hooks, smart errors, progressive descriptions, lazy init) | SATISFIED | All 5 v1.7 features documented with concept + example pattern |
| USAGE-03 | 40-02 | USAGE.md troubleshooting section is current with known issues and workarounds | PARTIAL | jdtls and gopls entries added, but rust-analyzer rename entry missing (referenced in roadmap SC-3) |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| USAGE.md | 153 | Wrong parameter name `max_tokens` (actual: `token_budget`) | BLOCKER | Agent tool calls will fail or silently use default budget |
| USAGE.md | 159 | Code example uses `max_tokens` in tool call | BLOCKER | Same as above -- reinforces incorrect parameter name |
| USAGE.md | 146 | `fuzzy_edit` example uses 5 positional args instead of named params (path, search, replacement) | BLOCKER | Agent will produce invalid tool call following this example |
| USAGE.md | 141 | Strategy name "IndentFlex" vs actual "indentation_flexible" | INFO | Minor naming mismatch between docs and tool output (IN-01 from code review) |

### Human Verification Required

None -- all checks are verifiable programmatically for documentation content accuracy.

### Gaps Summary

Three gaps were found, two related to factual inaccuracies identified in the code review (WR-01, WR-02) and one missing content item from roadmap SC-3:

1. **Wrong parameter name in RepoMap documentation (WR-01):** USAGE.md documents `max_tokens` but the actual parameter is `token_budget`. This is a blocker because agents following the documentation will pass an unrecognized parameter, causing silent failure (tool uses default budget instead of specified value) or a smart-error suggestion.

2. **Invalid fuzzy_edit code example (WR-02):** The example shows 5 positional arguments but the tool takes 3 named parameters (`path`, `search`, `replacement`). Ellipsis segments belong inside the search/replacement strings, not as separate arguments. An agent copying this example will produce an invalid tool call.

3. **Missing rust-analyzer rename troubleshooting entry (SC-3):** Roadmap success criterion 3 explicitly lists "rust-analyzer rename" as a known issue that should have a troubleshooting entry. The research phase incorrectly concluded it was "already documented" but no such entry exists. The `40-PATTERNS.md` file documents the issue details (textDocument/rename returns "No references found at position" in fresh workspaces) but this was never added to USAGE.md.

All three gaps are actionable and can be addressed in a single remediation plan targeting USAGE.md.

---

_Verified: 2026-04-23T14:30:00Z_
_Verifier: Claude (gsd-verifier)_
