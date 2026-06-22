# Phase 95: Identity & Docs Rewrite + docgen Regen - Research

**Researched:** 2026-06-22
**Domain:** Documentation rewrite (identity reframing) + code-generated README table + drift-gate CI wiring (Go tooling)
**Confidence:** HIGH (entirely codebase-anchored; no external deps)

## Summary

Phase 95 is the FINAL v2.0 phase. It is a docs-and-tooling phase, not a runtime-behavior phase: rewrite Helix's identity from "MCP-primary" to "CLI-first" across three docs (README.md, CLAUDE.md, PROJECT.md), re-anchor the CLAUDE.md tool-routing guidance to `helix <verb>` names, and regenerate the auto-generated README tool table against the frozen 50-verb CLI surface behind a green drift gate. All three requirements (DOCS-01/02/03) are satisfiable with `Edit` on docs + a focused change to `cmd/docgen/main.go` + a new CI gate step.

Two findings dominate the plan. **(1) The docgen drift gate is NOT wired into CI** — `cmd/docgen --check` exists and is currently green locally, but `.github/workflows/go-test.yml` only gates `helix-cligen --check`; the sole `docgen` reference there is a comment. The v1.12 docgen-drift root cause (a missing blank import silently drifting the table) recurred precisely because nothing enforced regeneration in CI. DOCS-02's "drift gate is green" acceptance is best read as *add the missing CI step* mirroring the cligen gate. **(2) docgen's blank-import set diverges from the daemon's**, but in a way that currently produces the correct table — see the import-parity analysis below. The planner must reconcile this honestly rather than blindly "make them equal," because the daemon registers `health`/`help` via explicit `RegisterTools` calls (non-blank), not init().

The historic "53 tools" / "41+ tools" counts are stale. The live frozen surface is **50 callable verbs** (one per `verbSpecs` entry, == `cli.VerbToolNames()`), and the current README table renders **51 rows** because `analyze_blast_radius` is registered under two categories (`symbol-retrieval` and `symbols`) — 50 unique tools, 51 rows. The CLI-first docs must use 50, reconciled honestly.

**Primary recommendation:** (a) `Edit` the MCP-primary framing in README/CLAUDE.md/PROJECT.md to CLI-first while explicitly preserving "MCP-SDK/gRPC retained as internal daemon plumbing"; (b) re-key the README tool table to `helix <verb>` names by deriving `verb = strings.ReplaceAll(toolName, "_", "-")` (proven mechanical for all 50) or via a new exported `cli` seam; (c) wire a `docgen --check` step into `go-test.yml` and a `make verify-docs` target; (d) do NOT rewrite the CLAUDE.md "SMTC-first tool routing" section — it documents the EXTERNAL `mcp__smtc__*` dev server, a different product, and DOCS-03 does not target it.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Identity / Core Value framing | Docs (README, PROJECT.md) | CLAUDE.md | Pure prose; no code path. README is the public face; PROJECT.md is the planning source of truth; CLAUDE.md is the agent's working instructions. |
| Tool-routing guidance | CLAUDE.md | — | Agent-facing operating instructions live only in CLAUDE.md. (Note: the *Helix-own* routing matrix referenced by DOCS-03 — see Open Q1.) |
| Auto-generated tool table | `cmd/docgen` (build tooling) | README.md (output target) | Table is generated, "do not hand-edit." docgen reads the live registry via blank-import init() + `skill.ToolProviders()`. |
| Drift gate | CI (`.github/workflows/go-test.yml`) + Makefile | `cmd/docgen --check` | Enforcement tier — the gate must run in CI to actually prevent drift (the v1.12 lesson). |

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
Discuss was skipped (`workflow.skip_discuss`). All implementation choices are at Claude's discretion, bounded by the carried STATE constraints + requirements below:

- **DOCS-01:** No doc may claim MCP as the primary agent interface. CLI-first framing must be consistent across README, CLAUDE.md, PROJECT.md (Core Value + the Constraints "Protocol: MCP — primary interface" line). The CLI (`helix <verb>`) is the agent surface; **MCP-SDK/gRPC remain as INTERNAL daemon plumbing — do NOT claim MCP is removed entirely; it is retained internally; only the agent-facing heads were deleted in Phase 94.**
- **DOCS-02:** The CLAUDE.md tool-routing matrix must cite `helix` CLI verbs end-to-end instead of MCP tool names (where that matrix refers to Helix's OWN tools). Preserve meaning; re-anchor to the real frozen verb names from `internal/cli/verbs_gen.go`. *(Note: REQUIREMENTS.md numbers this as DOCS-03; see Open Q2 for the ID-swap.)*
- **DOCS-03 (docgen drift gate — heed the v1.12 docgen-drift lesson):** Regenerate the auto-generated tool table against the frozen CLI surface; `cmd/docgen` enumerates verbs; docgen's blank imports MUST stay EQUAL to the daemon's (v1.12 root cause: `cmd/docgen` missing a blank import). Three-way parity: registry ↔ CLI ↔ docgen. The drift gate must be green. *(Note: REQUIREMENTS.md numbers this as DOCS-02; see Open Q2.)*
- Honor zero-proto / no-new-deps where applicable.

### Claude's Discretion
All implementation choices (CONTEXT.md: "All implementation choices at Claude's discretion — discuss skipped").

### Deferred Ideas (OUT OF SCOPE)
None — this is the final v2.0 phase. Milestone audit/complete/cleanup follow after.
</user_constraints>

<phase_requirements>
## Phase Requirements

> Requirement TEXT below is verbatim from `.planning/REQUIREMENTS.md` (the canonical source). The CONTEXT.md DOCS-02/03 numbering is swapped relative to REQUIREMENTS.md — both are listed; see Open Q2. The planner MUST satisfy all three behaviors regardless of which ID label is used.

| ID (REQUIREMENTS.md) | Description | Research Support |
|----|-------------|------------------|
| DOCS-01 | Identity rewrite — README, CLAUDE.md, PROJECT.md ("Core Value"; Constraints "Protocol: MCP — primary interface") rewritten to CLI-first. *Acceptance:* no doc claims MCP as the primary agent interface; CLI-first framing consistent. | Exact file:line locations of every MCP-primary claim enumerated in `## MCP-Primary Claims Inventory` below. |
| DOCS-02 | Auto-generated tool table regenerated against the CLI surface (`cmd/docgen` enumerates verbs; docgen blank-imports stay == the daemon's). *Acceptance:* generated table lists `helix` verbs and the docgen drift gate is green. | docgen mechanism + import-parity analysis + the missing-CI-gate finding in `## docgen Mechanism` below. |
| DOCS-03 | CLAUDE.md "tool routing" guidance updated to reference `helix <verb>` instead of MCP tool names. *Acceptance:* the routing matrix cites CLI verbs end-to-end. | The Helix-own-vs-SMTC distinction in `## CLAUDE.md Tool-Routing` below + the kebab↔tool mapping. |
</phase_requirements>

## Standard Stack

This phase ships **zero new dependencies** (honors no-new-deps). The "stack" is the existing in-repo tooling.

### Core
| Component | Location | Purpose | Why Standard |
|-----------|----------|---------|--------------|
| `cmd/docgen` | `cmd/docgen/main.go` | Generates the README tool + language tables between `<!-- BEGIN/END TOOLS -->` markers; `--check` mode for drift | Existing project tool; "do not hand-edit" the table per CLAUDE.md [VERIFIED: codebase] |
| `cli.VerbToolNames()` | `internal/cli/verb.go:81` | Exported read-only seam returning the 50 underlying tool names from `verbSpecs` | The canonical frozen-surface enumerator (used by 91-03 tests, render_policy coverage) [VERIFIED: codebase] |
| `verbSpecs` | `internal/cli/verbs_gen.go` | Generated kebab-verb → tool catalog (50 entries); `// DO NOT EDIT` | The frozen CLI surface; regenerated by `cmd/helix-cligen` [VERIFIED: codebase] |
| `skill.ToolProviders()` | `internal/skill/` | Returns all init()-registered providers; docgen iterates these to build the table | How docgen currently enumerates tools [VERIFIED: cmd/docgen/main.go:100] |

### Supporting
| Component | Location | Purpose | When to Use |
|-----------|----------|---------|-------------|
| `make docs` | `Makefile:77` | `go run ./cmd/docgen` — regenerate README | Run after any registry/verb change to refresh the table |
| `make verify-cligen` | `Makefile:80` | `helix-cligen --check` drift gate (VERB-02) | The PATTERN to mirror for a new `verify-docs` docgen gate |
| `go-test.yml` | `.github/workflows/go-test.yml:102` | CI step running `helix-cligen --check` | The PATTERN to mirror — add a sibling `docgen --check` step |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Deriving verb from `toolName` in docgen (`strings.ReplaceAll(t, "_", "-")`) | Add an exported `cli.VerbCatalog()` seam returning (verb, tool, short, group) | The derive approach is proven mechanical for all 50 verbs (see verification) and adds no `cli`-package surface; a new seam is cleaner/more explicit but is more code. Recommend the derive approach UNLESS the planner wants docgen to show verb groups/short text — then add the seam. |
| New `make verify-docs` + CI step | Fold docgen --check into existing cligen step | Separate step gives a clearer failure message ("README out of date") and mirrors the established cligen pattern. Recommend separate. |

**Installation:** None — no packages installed. (Package Legitimacy Audit omitted: this phase installs no external packages.)

## docgen Mechanism (DOCS-02 — the load-bearing requirement)

### How it works today
`cmd/docgen/main.go` [VERIFIED: read in full]:
1. Blank-imports 11 packages (lines 22–32) to fire `init()` → `skill.Register()`.
2. `generateToolTable()` (line 99) iterates `skill.ToolProviders()`; for each provider, emits one row per `tp.Tools()` entry: `` | `<tool.Name>` | <provider.Name()> | <truncated desc> | ``.
3. `generateLanguageTable()` (line 124) reads `langregistry.Entries()`.
4. `replaceSection()` (line 86) swaps content between `<!-- BEGIN TOOLS -->`/`<!-- END TOOLS -->` (and the LANGUAGES markers) in README.md.
5. `--check` (line 69): regenerate in memory, exit 1 if it differs from disk; else print "README.md is up to date." [VERIFIED: ran `go run ./cmd/docgen --check` → "README.md is up to date." exit 0]

### Current table shape (what must change for DOCS-02)
- Header: `| Tool | Category | Description |` — keyed by **MCP tool name** (`go_to_definition`), NOT verb.
- 51 rows for 50 unique tools: `analyze_blast_radius` appears twice (categories `symbol-retrieval` AND `symbols`) [VERIFIED: README.md:399-400]. This is a registry duplicate (registered by both the symbols kernel provider and a symbols-adapter), surfaced honestly by docgen.
- DOCS-02 wants the table to **list `helix` verbs**. The minimal change: emit the verb name instead of (or alongside) the tool name. Since `verb == strings.ReplaceAll(toolName, "_", "-")` for ALL 50 verbs [VERIFIED: scripted check over verbs_gen.go — zero mismatches], docgen can render `` `helix go-to-definition` `` mechanically.

### blank-import parity analysis (the v1.12 lesson — read carefully)
The CONTEXT/STATE constraint says "docgen's blank imports MUST stay EQUAL to the daemon's." The literal import lists are NOT equal today, but the resulting tool SET is correct. Exact divergence [VERIFIED: diffed `cmd/docgen/main.go` imports vs `internal/daemon/imports.go`]:

| Package | docgen blank-import | daemon blank-import | daemon registers via |
|---------|:------------------:|:-------------------:|----------------------|
| `internal/kernel/diag` | ✅ | ✅ | init() |
| `internal/kernel/edit` | ✅ | ✅ | init() |
| `internal/kernel/fileops` | ✅ | ✅ | init() |
| `internal/kernel/symbols` | ✅ | ✅ | init() |
| `internal/profile` | ✅ | ✅ | init() |
| `internal/skill/memory` | ✅ | ✅ | init() |
| `internal/skill/repomap` | ✅ | ✅ | init() |
| `internal/skill/semantic` | ✅ | ✅ | init() (the v1.12 fix) |
| `internal/skill/workflow` | ✅ | ✅ | init() |
| `internal/kernel/health` | ✅ (blank) | ❌ | **non-blank** in `daemon.go:30` + explicit `health.RegisterTools` (daemon.go:691) |
| `internal/kernel/help` | ✅ (blank) | ❌ | **non-blank** in `daemon.go:31` + explicit `help.RegisterTools` (daemon.go:692) |
| `internal/skill/guardrails` | ❌ | ✅ (blank) | init() — but `GuardrailsSkill.Tools()` returns `nil` (skill.go:33) → contributes **zero** rows |

**Interpretation (HIGH confidence):**
- `health` + `help`: BOTH register via `skill.Register()` in their `skill_adapter.go` `init()` [VERIFIED: health/skill_adapter.go:13, help/skill_adapter.go:13]. docgen reaches them by blank import; the daemon reaches them because `daemon.go` imports them non-blank (for `RegisterTools` wiring) which ALSO fires their init(). So both surfaces see `get_health` + `get_tool_help`. ✅ Tool sets agree.
- `guardrails`: registers via init() but exposes **no tools** (`Tools()` → nil). Its presence/absence in docgen changes NOTHING in the table. ✅ Tool sets agree.

**Therefore the table is currently CORRECT and `--check` is green.** The risk the v1.12 lesson warns about is *future* drift: if a new tool-bearing provider is added to the daemon but not blank-imported into docgen (exactly the `internal/skill/semantic` miss), the table silently drifts. The robust fix the planner should weigh:
- **Option A (recommended):** Wire `docgen --check` into CI (currently MISSING — see below) so ANY drift fails the build, regardless of import-list symmetry. This is the real protection; literal import-list equality is a fragile proxy.
- **Option B (also do):** Add a comment in BOTH `imports.go` and `cmd/docgen/main.go` cross-referencing each other and stating the rule "tool-bearing providers must appear in both." Optionally extract a shared `imports_shared` blank-import file imported by both, so they CANNOT diverge. (Mind the daemon's D-02 invariant: do NOT blank-import the semantic *extract* providers — see imports.go:15-27.)

### CRITICAL FINDING: docgen drift gate is NOT in CI
[VERIFIED: grep of `.github/workflows/`] — `go-test.yml:106` runs `go run ./cmd/helix-cligen --check`, but there is **no** `go run ./cmd/docgen --check` step anywhere in CI. The only `docgen` token in the workflow is a COMMENT at line 104 ("Mirrors the docgen discipline"). DOCS-02's acceptance "the docgen drift gate is green" cannot be satisfied by a gate that doesn't run. **The planner MUST add a `docgen --check` CI step** (sibling to the cligen step) and a `make verify-docs` target (sibling to `verify-cligen`). This is the single most important deliverable for DOCS-02 and directly closes the v1.12 hole.

### Three-way parity (registry ↔ CLI ↔ docgen)
- registry → CLI: enforced today by `helix-cligen --check` (VERB-02 gate, CI line 106).
- registry → docgen: enforced by `docgen --check` ONCE wired (currently local-only, ungated).
- CLI → docgen: implied if both derive from the registry AND docgen keys on verbs derived from tool names. A render_policy coverage test (`internal/cli/render_policy_test.go`) already asserts every `verbSpecs` entry has a render class — a similar "every verb appears in README" assertion could be added, but the docgen --check gate plus cligen gate already transitively cover it.

## MCP-Primary Claims Inventory (DOCS-01)

> Exact file:line locations of every claim framing MCP as the primary/agent interface, so the planner can target precise `Edit`s. Current text quoted. **Preserve accurate internal-MCP statements** (SDK/gRPC retained); only retire *primary-agent-interface* framing. All [VERIFIED: codebase grep + Read].

### README.md (most stale — also carries Phase-94-deleted config)
| Line | Current text (abridged) | Issue |
|------|-------------------------|-------|
| 3-5 | "Helix is the code editor for your LLM." | Identity OK-ish; ensure CLI-first hero is consistent. |
| 13 | "Code intelligence platform for MCP — 41+ tools across 52 languages." | "platform for MCP" + stale "41+"; → CLI-first + 50 verbs. |
| 19 | "It integrates with any client/LLM via the model context protocol (**MCP**)." | MCP-as-integration-surface claim. Reframe: agents drive the `helix` CLI. |
| 43 | "Helix provides 41+ MCP tools for coding workflows…" | Stale count + MCP-tool framing → "50 `helix` CLI verbs". |
| 46-49 | "Agents connect via the **model context protocol (MCP)** through: stdio… Streamable HTTP…" | **STALE & WRONG** — both heads DELETED in Phase 94. Must be rewritten to: agents invoke `helix <verb>` via Bash; daemon connection is internal gRPC. |
| 104-145 | Manual-config JSON blocks using `"args": ["--mode=stdio"]`, `--mode=http`, `/mcp` endpoint | **STALE & WRONG** — `--mode=stdio`/`--mode=http` removed (only `--mode=auto` remains; legacy modes error per STATE 94-02). Rewrite to `helix setup` + CLI usage. |
| 199 | "isolated from MCP traffic" (admin listener) | Acceptable as internal detail; reword to "daemon traffic" for accuracy. |
| 350-404 | The auto-generated tool table (`| Tool | Category | …`) | DOCS-02 target — regenerate to verbs. |
| 363 | `get_tool_help` desc: "documentation for any MCP tool…" | Generated text; comes from tool Description — reframe at source if desired. |
| 411 | "MCP Runtime (stdio/HTTP transports, …)" | STALE transports; internal runtime retained but stdio/HTTP heads gone. |
| 428 | "symbol-level MCP operations over LSP" (Serena inspiration) | Historical/inspiration context — acceptable, but verify it doesn't imply Helix's *agent* surface is MCP. |

### CLAUDE.md
| Line | Current text | Issue |
|------|--------------|-------|
| 18 | "**Helix** — The IDE for your coding agent. A Go-native code intelligence platform **for MCP**." | "platform for MCP" → CLI-first identity. |
| 20 | "Helix provides **53 MCP tools** for semantic code retrieval…" | Stale count (53) + MCP-tool framing → "50 `helix` CLI verbs". |
| 24 | "**Core Value:** Rock-solid LSP-backed **MCP runtime** that … exposes semantic code operations as **agent tools**…" | Core-Value MCP-primary claim → CLI-first reframe (CLI is the agent surface; MCP runtime is internal). |
| 94 | "- **Protocol:** MCP (Model Context Protocol) -- **primary interface for all clients**" | THE explicit primary-interface line (Technology Stack section). Reframe: CLI is the agent interface; MCP SDK/gRPC retained internally. |
| 179 | "- **Protocol**: MCP (Model Context Protocol) -- **primary interface**" | THE explicit primary-interface line (Constraints section). Same reframe. |

### PROJECT.md (.planning/PROJECT.md)
| Line | Current text | Issue |
|------|--------------|-------|
| 5 | "A Go-native code intelligence platform **for MCP**: … **41+ callable MCP tools** …" | "platform for MCP" + stale count → CLI-first. |
| 9 | "**Core Value:** Rock-solid LSP-backed **MCP runtime** that … exposes semantic code operations **as tools**." | Core-Value MCP-primary → CLI-first. (NOTE: the canonical v2.0 Core Value already exists in STATE.md line 26 and PROJECT.md milestone section line 175 — reuse that wording.) |
| 222 | "- **Protocol**: MCP (Model Context Protocol) — **primary interface**" | THE explicit primary-interface line (Constraints). Same reframe. |

### What MUST stay accurate (do NOT over-claim MCP removal)
- MCP-SDK, gRPC IPC (`StreamMCP` wire), the 5 middlewares, and all tool handlers are **RETAINED behind the wire** [VERIFIED: STATE.md:82, 94-02 decision line 133]. Phase 94 deleted ONLY the agent-facing heads (stdio forwarder + Streamable-HTTP `/mcp`).
- The CLI→daemon path is internal gRPC over a unix socket (RETIRE-04 added opt-in loopback TCP).
- Suggested accurate framing: *"Agents drive the `helix` CLI (via Bash); the CLI dials a warm daemon over internal gRPC. The MCP Go SDK and gRPC IPC are retained as internal daemon plumbing — they are no longer an agent-facing surface."*
- The canonical v2.0 Core Value to reuse [VERIFIED: STATE.md:26]: *"The `helix` CLI is the only surface an agent touches — terse, `relpath:line:col`-anchored, zero schema-preload tax — driving the unchanged warm LSP/RepoMap kernel behind it, so agents use the toolset instead of falling back to grep/sed/cat."*

## CLAUDE.md Tool-Routing (DOCS-03)

**The decisive distinction (HIGH confidence) — get this right or the planner will wrongly rewrite an external product's guidance:**

CLAUDE.md contains exactly ONE large routing matrix: the **"## Code intelligence: SMTC-first tool routing"** section (lines 194–260) [VERIFIED: read in full]. Every row references `mcp__smtc__*` (e.g., `mcp__smtc__goto_definition`). **These are NOT Helix's own tools** — `smtc` ("Show Me The Code") is a *separate, external* MCP dev server (per the user's MEMORY.md and the project's own CLAUDE.md note at lines 234-247: "This repo (Helix, Go-native) → no security capability available… The security rows exist… for cross-project portability of this guidance, not for use against this codebase.").

Implications for DOCS-03:
- DOCS-03 says re-anchor the routing matrix where it "refers to **Helix's OWN tools**." The SMTC matrix refers to SMTC's tools, which have NO 1:1 Helix-verb equivalent (e.g., `mcp__smtc__find_taint_sinks`, `get_cfg`, `get_ir`, taint tracing — Helix ships none of these). **Rewriting the SMTC section to `helix` verbs would be factually wrong** and would break the dev's working tool-routing for this repo.
- **There is currently NO Helix-own tool-routing matrix in CLAUDE.md.** So DOCS-03 has two honest readings (see Open Q1):
  - (a) ADD a new "Helix CLI tool routing" section that maps code questions → `helix <verb>` (Helix's actual product surface), leaving the SMTC section intact (it's a dev-environment concern, orthogonal to the product). This best satisfies "the routing matrix cites CLI verbs end-to-end" by creating the matrix it describes.
  - (b) Treat DOCS-03 as already-satisfied-by-omission (no Helix-own MCP-tool routing exists to rewrite) and only ensure no Helix-own tool *names* appear MCP-style elsewhere. Weaker; likely fails the "cites CLI verbs end-to-end" acceptance because no such matrix exists.
- **Recommendation:** (a) — author a concise Helix-CLI routing/decision section using the real 50 verbs, and add a one-line clarifier that the SMTC section is about the external dev MCP server (NOT Helix's product), so a future reader/agent never conflates them.

### Verb names for the routing section (the 50 frozen verbs, kebab form)
All derive mechanically as `verb = toolName.replace("_","-")` [VERIFIED: zero mismatches across verbs_gen.go]. Grouped by `groupID`:
- **navigation:** `go-to-definition`, `find-references`, `find-implementations`, `get-hover-info`, `get-call-hierarchy`, `get-type-hierarchy`, `get-symbol-overview`, `search-symbols`, `analyze-blast-radius`
- **fileops:** `read-file`, `create-file`, `list-directory`, `find-files`, `search-in-files`, `replace-in-file`, `fuzzy-edit`
- **edit:** `replace-symbol-body`, `insert-before-symbol`, `insert-after-symbol`, `rename-symbol`, `safe-delete-symbol`, `verify-edit`
- **diagnostics:** `get-diagnostics`, `get-code-actions`, `format-code`
- **repomap:** `get-repo-map`, `get-context`, `get-semantic-context`, `get-semantic-graph-status`, `index-semantic-graph`, `refresh-semantic-graph`, `find-related-symbols`, `explain-symbol-deep`, `validate-graph-edge`, `get-cluster-map`, `explain-cluster`, `get-change-impact-graph`
- **memory:** `read-memory`, `write-memory`, `edit-memory`, `delete-memory`, `rename-memory`, `list-memories`, `search-memories`, `onboard-project`, `prepare-for-new-conversation`, `switch-mode`, `get-token-budget`, `get-health`, `get-tool-help`

(50 total. `groupID` is the `verbSpecs` capability group — useful for organizing the routing section.)

## Architecture Patterns

### Recommended approach (data flow)
```
internal/cli/verbs_gen.go  ──(helix-cligen, frozen)──>  50 verbSpecs entries
        │                                                        │
        │ VerbToolNames() / derive verb=toolName.replace(_,-)    │
        ▼                                                        ▼
cmd/docgen/main.go ──reads skill.ToolProviders() (init registry)──> generateToolTable()
        │                                                        │
        │ replaceSection(BEGIN/END TOOLS)                        │
        ▼                                                        ▼
   README.md tool table (verb-keyed)  <──  docgen --check (CI gate, NEW)  ──> green/fail
```

### Pattern 1: Mirror the cligen drift gate
**What:** The project already has the exact pattern for a hard-fail drift gate.
**When to use:** For the new docgen gate (DOCS-02).
**Example:**
```makefile
# Source: Makefile:80 (verify-cligen) — mirror for docs
verify-docs: ## HARD-FAIL drift gate: README.md tool table must match the live registry
	$(GO) run ./cmd/docgen --check
```
```yaml
# Source: .github/workflows/go-test.yml:102-106 (cligen step) — add sibling step
- name: docgen drift gate (DOCS-02)
  run: go run ./cmd/docgen --check
```

### Pattern 2: Edit generated text at the source, not in README
**What:** Tool descriptions in the table come from `tool.Description` (the tool's registered description). The string "documentation for any MCP tool" (README:363) originates from the tool's own Description.
**When to use:** If DOCS-01 requires removing "MCP" from generated rows, edit the description in the tool's registration, then regen — never hand-edit README between the markers.

### Anti-Patterns to Avoid
- **Hand-editing the README tool table** between `<!-- BEGIN/END TOOLS -->` — it WILL be overwritten by `make docs` and WILL trip the (new) `--check` gate. [VERIFIED: CLAUDE.md "do not hand-edit the tool table"]
- **Rewriting the SMTC-first routing section to helix verbs** — it documents a DIFFERENT product (see DOCS-03 analysis). Factually wrong.
- **Claiming "MCP removed"** — only the agent-facing heads are gone; SDK/gRPC retained internally.
- **Forcing literal blank-import equality** by adding `health`/`help` blank-imports to the daemon or removing them from docgen without understanding the non-blank registration path — risks an unused-import compile error or a registration regression. Reconcile via the CI gate + cross-ref comments instead.
- **Using a hardcoded count (50/51/53)** in prose that will drift. Prefer phrasing like "the full verb inventory is auto-generated in README.md" (CLAUDE.md already does this).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Tool table generation | A manual markdown table | `cmd/docgen` (regen via `make docs`) | Auto-generated; drift-gated; single source of truth |
| Verb enumeration | A hand-kept verb list | `cli.VerbToolNames()` / `verbSpecs` derive | Frozen surface; already the canonical seam |
| Drift detection | An ad-hoc diff script | `docgen --check` + CI step (mirror cligen) | Established project pattern; one obvious failure message |

**Key insight:** Everything DOCS-02 needs is generation + a gate. The verb surface is already frozen and code-generated; the only NEW code is (1) keying the table on verbs and (2) wiring the gate into CI.

## Runtime State Inventory

> This is a docs/tooling phase, NOT a rename/migration. Included briefly because DOCS-01 touches "MCP-primary" strings that may have stored/registered analogues.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — no datastore keys reference "MCP-primary" framing. | None. |
| Live service config | None — Phase 93-03 already flipped `helix setup` to skill+hooks install across 7 clients (STATE:132); no MCP-server registration remains as the agent path. README's *documentation* of old `--mode=stdio` config is stale text (code edit only). | Rewrite stale README config blocks (README:104-145) — prose only. |
| OS-registered state | None. | None. |
| Secrets/env vars | None — `HELIX_*` names unchanged; no MCP-named env vars to retire. | None. |
| Build artifacts | None — docgen writes README.md in-place; no stale generated artifact carries old framing once regenerated. | Run `make docs` once after edits. |

**Canonical question — after docs are edited, what still claims MCP-primary?** Only the regenerated tool table (handled by `make docs`) and any tool *Description* strings containing "MCP" (README:363 source). Verify with a final grep for "primary interface" / "MCP tool" across the three docs.

## Common Pitfalls

### Pitfall 1: docgen --check green locally but no CI gate (the v1.12 recurrence)
**What goes wrong:** A future registry change drifts the README table; nothing fails because the gate isn't in CI.
**Why it happens:** `docgen --check` exists but `.github/workflows/go-test.yml` never invokes it (only cligen). [VERIFIED]
**How to avoid:** Add the `docgen --check` CI step + `make verify-docs` THIS phase. This IS the DOCS-02 deliverable.
**Warning signs:** PRs that change tool registrations merge with a stale README.

### Pitfall 2: Over-claiming MCP removal
**What goes wrong:** Docs say "MCP removed/gone," contradicting the retained internal SDK/gRPC.
**Why it happens:** Conflating "agent-facing MCP heads deleted" (true) with "MCP gone" (false).
**How to avoid:** Use the precise framing in "What MUST stay accurate" above.
**Warning signs:** Any doc sentence asserting MCP no longer exists in Helix.

### Pitfall 3: Rewriting the external SMTC routing matrix
**What goes wrong:** The dev's repo-local code-routing guidance is corrupted to nonexistent helix verbs (Helix has no taint/CFG/IR tools).
**Why it happens:** "tool routing" in CLAUDE.md visually looks like the DOCS-03 target.
**How to avoid:** Treat SMTC section as out-of-scope external-tooling; satisfy DOCS-03 by adding a Helix-own CLI routing section (Open Q1 recommendation (a)).
**Warning signs:** `mcp__smtc__*` rows being changed to `helix ...`.

### Pitfall 4: Stale counts (53 / 41+ / 51)
**What goes wrong:** Prose hardcodes a wrong number.
**Why it happens:** Multiple historical counts across docs (53 in CLAUDE.md:20, 41+ in README/PROJECT, 51 table rows, 50 verbs).
**How to avoid:** The frozen truth is **50 callable verbs** (== `len(verbSpecs)` == `VerbToolNames()`); README table renders 51 rows due to the `analyze_blast_radius` dual-category registration. Prefer count-free phrasing or cite 50 with the dup explained. Reconcile honestly per the STATE constraint "VERB-01 acceptance is generated-count == live-registry-count enumerated by name, not a hardcoded number" (STATE:84).
**Warning signs:** "53 MCP tools" surviving anywhere.

### Pitfall 5: ui-plan-gate false-positive
**What goes wrong:** Plan-phase mis-triggers UI handling on the word "interface."
**Why it happens:** "MCP as primary *interface*" reads as UI to the gate (CONTEXT.md:14).
**How to avoid:** Plan with `--skip-ui` (already noted in CONTEXT).

## Code Examples

### Deriving the verb name in docgen (verified mechanical)
```go
// Source: verified over internal/cli/verbs_gen.go — verb == toolName with _→-
//   for all 50 entries (zero mismatches).
verb := strings.ReplaceAll(tool.Name, "_", "-")
// Render: | `helix go-to-definition` | navigation | <desc> |
```
> If the planner prefers an authoritative seam over a string transform, add to `internal/cli/verb.go`:
> ```go
> // VerbCatalog returns (kebabVerb, toolName, short, groupID) for each verb.
> func VerbCatalog() []VerbInfo { /* iterate verbSpecs */ }
> ```
> Then docgen imports `internal/cli` and renders from it. Tradeoff captured in Alternatives table.

### The drift-check invocation (already implemented)
```go
// Source: cmd/docgen/main.go:69-76
if *check {
    if result != string(content) {
        fmt.Fprintf(os.Stderr, "README.md is out of date. Run 'go run ./cmd/docgen' to regenerate.\n")
        os.Exit(1)
    }
    fmt.Println("README.md is up to date.")
    return
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Agent connects via MCP (stdio/HTTP heads) | Agent drives `helix <verb>` CLI; daemon dial is internal gRPC | Phase 94 (94-02, STATE:133) | Docs must stop describing stdio/HTTP MCP as the agent surface |
| MCP tool-name table (`go_to_definition`) | `helix <verb>` verb table | Phase 95 (this phase, DOCS-02) | docgen re-keyed to verbs |
| 53/41+ tool counts | 50 frozen verbs | Phase 91 (verbs_gen frozen) | Honest count reconciliation |
| docgen --check local-only | docgen --check as a CI gate | Phase 95 (this phase) | Closes the v1.12 drift hole |

**Deprecated/outdated:**
- `helix --mode=stdio` / `--mode=http` / `/mcp` endpoint: REMOVED (only `--mode=auto` remains; legacy modes error, STATE 94-02). README manual-config blocks (104-145) document removed flags — must be rewritten.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | DOCS-03 is best satisfied by ADDING a Helix-own CLI routing section (not rewriting the SMTC section) | CLAUDE.md Tool-Routing / Open Q1 | If the intent was literally "rewrite an existing Helix routing matrix," but none exists, the planner picks the wrong target. Low risk — analysis shows no Helix-own matrix exists; recommendation is conservative and additive. |
| A2 | The CONTEXT.md DOCS-02/03 numbering swap vs REQUIREMENTS.md is a labeling artifact, not two different requirement sets | Open Q2 | If treated as 4 distinct behaviors, scope inflates. Low risk — both sources describe the same two behaviors (table-regen + routing-rewrite). |
| A3 | "docgen blank-imports == daemon's" is satisfiable via the CI gate + cross-ref comments rather than literal import-list equality | docgen Mechanism | If a reviewer insists on literal equality, extra refactor (shared imports file) needed. Captured as Option B. |

## Open Questions

1. **DOCS-03 target: add a Helix-own CLI routing matrix, or is it satisfied by omission?**
   - What we know: CLAUDE.md's only routing matrix is the EXTERNAL `mcp__smtc__*` section; no Helix-own MCP-tool routing matrix exists to "rewrite."
   - What's unclear: whether the requirement author expected a Helix routing matrix to already exist.
   - Recommendation: ADD a concise "Helix CLI tool routing" section using the 50 real verbs + a one-line clarifier that the SMTC section is a separate external dev server. This makes "the routing matrix cites CLI verbs end-to-end" literally true. Do NOT touch the SMTC rows.

2. **DOCS-02 vs DOCS-03 numbering swap (CONTEXT.md ↔ REQUIREMENTS.md).**
   - What we know: REQUIREMENTS.md: DOCS-02 = table regen, DOCS-03 = routing rewrite. CONTEXT.md: DOCS-02 = routing rewrite, DOCS-03 = drift gate. The two BEHAVIORS are identical; only the ID labels are swapped.
   - Recommendation: Plan against REQUIREMENTS.md IDs (the canonical traceability source: REQUIREMENTS.md:70-72, 132-134) and note the CONTEXT label swap in the plan so the traceability matrix stays consistent.

3. **Blank-import parity: enforce literally or via gate?**
   - What we know: lists differ but tool sets agree; `--check` is the real protection; literal equality is fragile (health/help are non-blank in the daemon by design).
   - Recommendation: Primary = wire `docgen --check` into CI. Secondary = cross-reference comments in both files (and optionally a shared blank-import file), explicitly excluding the daemon's D-02 semantic-extract providers.

## Environment Availability

> This phase has no external dependencies (Go toolchain + in-repo tooling only).

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | `go run ./cmd/docgen`, tests | ✓ | host | — |
| `cmd/docgen` | DOCS-02 | ✓ (builds + `--check` green) | in-repo | — |
| `cmd/helix-cligen` | verb surface regen/gate | ✓ | in-repo | — |

**Missing dependencies with no fallback:** None.
**Missing dependencies with fallback:** None.

## Validation Architecture

> nyquist_validation is enabled (config.json workflow.nyquist_validation = true).

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go standard `testing` |
| Config file | none (Go modules) |
| Quick run command | `go test ./cmd/docgen/ ./internal/cli/ -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DOCS-02 | README table regenerated; verb-keyed; drift-free | unit + gate | `go run ./cmd/docgen --check` (exit 0) | ✅ (gate logic exists; CI step ❌ Wave 0) |
| DOCS-02 | Table lists `helix` verbs (not raw tool names) | unit | new assert in `cmd/docgen/main_test.go`: `strings.Contains(table, "helix go-to-definition")` (replace `TestToolTableContainsKnownTools` raw-tool asserts) | ⚠️ existing test asserts raw `go_to_definition` (main_test.go:74-87) — must update |
| DOCS-02 | docgen ↔ registry parity persists | gate | `go test ./cmd/docgen/ -run TestGenerateToolTable` + CI `docgen --check` | ✅ test / ❌ CI step Wave 0 |
| DOCS-01 | no doc claims MCP as primary agent interface | grep gate (manual or scripted) | `! grep -rn "primary interface" README.md CLAUDE.md .planning/PROJECT.md` | ❌ Wave 0 (optional scripted check) |
| DOCS-03 | routing matrix cites CLI verbs end-to-end | grep gate | `grep -n "helix go-to-definition" CLAUDE.md` (presence of Helix-CLI routing section) | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./cmd/docgen/ ./internal/cli/ -count=1` + `go run ./cmd/docgen --check`
- **Per wave merge:** `go vet ./... && go test ./... -count=1`
- **Phase gate:** `make docs` clean (no diff), `go run ./cmd/docgen --check` green, new CI step present, full suite green before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] CI step `go run ./cmd/docgen --check` in `.github/workflows/go-test.yml` (mirror cligen step at line 102-106) — covers DOCS-02 drift gate
- [ ] `make verify-docs` target in Makefile (mirror `verify-cligen` at line 80)
- [ ] Update `cmd/docgen/main_test.go` `TestToolTableContainsKnownTools` (line 72-88) — currently asserts raw tool names; must assert verb forms after re-keying
- [ ] (Optional) a scripted DOCS-01 grep guard ("primary interface" absent from the three docs) — could be a warn-only CI step or a `make` target

*Note: existing `TestGenerateToolTable` (≥30 rows) and `TestGenerateLanguageTable` (≥40 rows) remain valid after re-keying.*

## Security Domain

> security_enforcement is absent in config (= enabled by default). This is a docs/tooling phase with no runtime code path, no input handling, no external deps, and no auth/crypto/data surface. ASVS categories are not materially engaged.

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | no | docgen reads in-repo source only; no untrusted input |
| V6 Cryptography | no | — |

**Note:** The only "security-adjacent" concern is documentation *accuracy* about the retained internal MCP/gRPC plumbing and the loopback-gated TCP bind (RETIRE-04) — docs must not misstate the network exposure (default unix socket; non-loopback TCP opt-in/gated). No threat patterns introduced.

## Sources

### Primary (HIGH confidence — codebase, this session)
- `cmd/docgen/main.go` (read in full) — generation + `--check` mechanism, blank imports
- `cmd/docgen/main_test.go` (read in full) — existing test assertions to update
- `internal/daemon/imports.go` + `internal/daemon/daemon.go:24-31,646-696` — daemon blank vs non-blank registration; guardrails/health/help wiring
- `internal/cli/verbs_gen.go` (read in full) + `internal/cli/verb.go:76-101` — 50 frozen verbs, `VerbToolNames()`
- `internal/cli/render_policy.go` — verb→tool keying precedent (untracked new file from Phase 92 prep)
- `internal/skill/guardrails/skill.go:21,33` — `Tools()` returns nil
- `.github/workflows/go-test.yml:93-106` — cligen gate present, docgen gate ABSENT
- `Makefile:74-87` — `docs` / `verify-cligen` targets
- `README.md` (1-149, 300-429), `CLAUDE.md` (18-24, 94, 179, 194-260), `.planning/PROJECT.md` (1-9, 219-223), `.planning/STATE.md`, `.planning/ROADMAP.md:163-177`, `.planning/REQUIREMENTS.md:60-78,132-134` — claim inventory + IDs
- Scripted verification: 50 verbs, `verb==toolName.replace(_,-)` for all (zero mismatches); `docgen --check` exit 0; README table = 51 rows / 50 unique tools

### Secondary (MEDIUM) — none
### Tertiary (LOW) — none

## Metadata

**Confidence breakdown:**
- docgen mechanism + drift-gate gap: HIGH — read source, ran `--check`, grepped CI
- MCP-primary claim inventory: HIGH — exact file:line via grep + Read
- DOCS-03 SMTC-vs-Helix distinction: HIGH — read the section + project's own clarifier note + user MEMORY
- Verb enumeration / kebab mapping: HIGH — scripted over generated source, zero mismatches
- Import-parity reconciliation: HIGH — diffed both files, confirmed registration paths

**Research date:** 2026-06-22
**Valid until:** 2026-07-22 (stable; the only volatility is the registry/verb set, which is frozen for v2.0)
