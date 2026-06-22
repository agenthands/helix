# Phase 95: Identity & Docs Rewrite + docgen Regen - Pattern Map

**Mapped:** 2026-06-22
**Files analyzed:** 9 (3 docs rewritten, 1 CLAUDE.md section added, 1 docgen modified, 1 docgen test updated, 1 Makefile target added, 1 CI step added, daemon imports cross-referenced)
**Analogs found:** 9 / 9 (all in-repo — zero external; this is a docs+tooling phase)

> Verification note: every file:line and excerpt below was re-read from live source this session (not copied from RESEARCH.md). RESEARCH.md's anchors were confirmed accurate; deltas vs. research are flagged inline.

## File Classification

| Change | Role | Data Flow | Closest Analog | Guidance | Match |
|--------|------|-----------|----------------|----------|-------|
| `Makefile` add `verify-docs` target | config (build) | transform/check | `Makefile:80` `verify-cligen` | mirror this | exact |
| `.github/workflows/go-test.yml` add `docgen --check` step | config (CI) | check-gate | `go-test.yml:102-106` cligen step | mirror this | exact |
| `cmd/docgen/main.go` re-key table to verbs | utility (codegen) | transform | `cmd/docgen/main.go:99-121` (self) + `internal/cli/verb.go:81` `VerbToolNames()` | modify this | exact |
| `cmd/docgen/main_test.go` verb-form asserts | test | check | `cmd/docgen/main_test.go:72-88` `TestToolTableContainsKnownTools` | rewrite this | exact |
| `cmd/docgen/main.go` blank-import reconcile | config (codegen) | — | `internal/daemon/imports.go:1-27` (reference set) | cross-ref + gate (do NOT force literal equality) | role-match |
| README.md tool table | docs (generated) | — | `cmd/docgen` output between markers | regen this (never hand-edit) | exact |
| README.md MCP-primary prose | docs | — | STATE.md:26 canonical Core Value | rewrite this | n/a |
| CLAUDE.md MCP-primary prose | docs | — | STATE.md:26 canonical Core Value | rewrite this | n/a |
| `.planning/PROJECT.md` MCP-primary prose | docs | — | STATE.md:26 canonical Core Value | rewrite this | n/a |
| CLAUDE.md NEW Helix-CLI routing section | docs | — | CLAUDE.md:194+ SMTC "Decision matrix" (FORMAT only) | mirror format / leave SMTC intact | exact |

---

## Pattern Assignments

### `Makefile` — add `verify-docs` (config, check-gate)

**Analog:** `Makefile:80` (`verify-cligen`) — MIRROR THIS exactly.

Live source (`Makefile:77-81`):
```makefile
docs: ## Regenerate tool and language tables in README.md
	$(GO) run ./cmd/docgen

verify-cligen: ## HARD-FAIL drift gate: internal/cli/verbs_gen.go must match the live tool registry (VERB-02)
	$(GO) run ./cmd/helix-cligen --check
```

**New target (paste sibling, same `$(GO)` convention + `## ` help comment):**
```makefile
verify-docs: ## HARD-FAIL drift gate: README.md tool table must match the live registry (DOCS-02)
	$(GO) run ./cmd/docgen --check
```
Note: `docs` (regen) already exists at line 77 — reuse it; only `verify-docs` (check) is new.

---

### `.github/workflows/go-test.yml` — add `docgen --check` step (config, CI gate)

**Analog:** `go-test.yml:102-106` (the cligen step) — MIRROR THIS. This is the load-bearing DOCS-02 deliverable; the gate does not exist in CI today (line 104 only mentions docgen in a comment).

Live source (`go-test.yml:99-106`):
```yaml
      - name: go test
        run: go test ./... -count=1

      - name: helix-cligen drift gate (VERB-02)
        # HARD-FAIL if internal/cli/verbs_gen.go is stale relative to the live
        # tool registry (an *Args edit without regen). Mirrors the docgen
        # discipline. Regenerate with: go run ./cmd/helix-cligen
        run: go run ./cmd/helix-cligen --check
```

**New step (insert as sibling, same name/comment/run shape):**
```yaml
      - name: docgen drift gate (DOCS-02)
        # HARD-FAIL if README.md tool table is stale relative to the live
        # tool registry (a tool registration without `make docs`). Closes the
        # v1.12 docgen-drift hole. Regenerate with: go run ./cmd/docgen
        run: go run ./cmd/docgen --check
```
Place it adjacent to the cligen step (after line 106) so the two drift gates read as a pair.

---

### `cmd/docgen/main.go` — re-key tool table to `helix <verb>` (utility, transform)

**Analog:** the function itself + the verb seam. The ONLY line that changes is the row format at `main.go:116`.

Current row emitter (`cmd/docgen/main.go:106-117`):
```go
	for _, tp := range providers {
		category := tp.Name()
		for _, tool := range tp.Tools() {
			desc := tool.Description
			// Truncate long descriptions at first sentence.
			if idx := strings.Index(desc, ". "); idx > 0 && idx < 120 {
				desc = desc[:idx+1]
			}
			// Escape pipes in description.
			desc = strings.ReplaceAll(desc, "|", "\\|")
			sb.WriteString(fmt.Sprintf("| `%s` | %s | %s |\n", tool.Name, category, desc))
		}
	}
```

**Re-key (proven mechanical: `verb == toolName.replace("_","-")` for all 50, zero mismatches):**
```go
			verb := strings.ReplaceAll(tool.Name, "_", "-")
			sb.WriteString(fmt.Sprintf("| `helix %s` | %s | %s |\n", verb, category, desc))
```
Header at `main.go:103` (`| Tool | Category | Description |`) may stay or become `| Verb | Category | Description |` — planner's call; either passes `--check` once regenerated.

**Authoritative-seam alternative (if a string transform is deemed too implicit):** `internal/cli/verb.go:81` exposes `cli.VerbToolNames()` (read-only, returns a fresh sorted copy of `verbSpecs` tool names). docgen could import `internal/cli` to validate/derive instead of the inline replace. Tradeoff: adds a `cli` import to docgen; the replace adds none. Recommend the replace (RESEARCH Alternatives table) unless verb groups/short-text are wanted in the table.

**`--check` mechanism (unchanged, already correct — `main.go:69-76`):** regenerates in memory, `os.Exit(1)` on diff, else "README.md is up to date." Do NOT touch this; it is what the new CI step invokes.

---

### `cmd/docgen/main_test.go` — verb-form assertions (test)

**Analog/target:** `cmd/docgen/main_test.go:72-88` `TestToolTableContainsKnownTools` — REWRITE the assertions from raw tool names to verb forms.

Current (`main_test.go:72-88`):
```go
func TestToolTableContainsKnownTools(t *testing.T) {
	table := generateToolTable()
	knownTools := []string{
		"go_to_definition",
		"replace_symbol_body",
		"read_file",
		"get_diagnostics",
		"write_memory",
		"onboard_project",
		"switch_mode",
	}
	for _, tool := range knownTools {
		if !strings.Contains(table, tool) {
			t.Errorf("tool table missing known tool %q", tool)
		}
	}
}
```

**Rewrite to verb forms (note: after re-keying, the table no longer contains underscored names, so these asserts WOULD FAIL — they must flip to kebab `helix <verb>`):**
```go
	knownVerbs := []string{
		"helix go-to-definition",
		"helix replace-symbol-body",
		"helix read-file",
		"helix get-diagnostics",
		"helix write-memory",
		"helix onboard-project",
		"helix switch-mode",
	}
```
**Leave intact:** `TestGenerateToolTable` (≥30 rows, `main_test.go:43-56`), `TestGenerateLanguageTable` (≥40 rows, `main_test.go:58-70`), and the three `TestReplaceSection*` (`main_test.go:8-41`) — all remain valid after re-keying (they count rows / test marker logic, not tool-name strings).

---

### `cmd/docgen/main.go` blank-imports ↔ `internal/daemon/imports.go` (config reconcile)

**Reference set:** `internal/daemon/imports.go:1-27`. Do NOT force literal list equality — the lists differ BY DESIGN and the tool SET still agrees (`--check` is the real protection).

docgen blank imports (`cmd/docgen/main.go:22-32`): diag, edit, fileops, **health**, **help**, symbols, profile, memory, repomap, semantic, workflow.

daemon blank imports (`internal/daemon/imports.go:5-14`): diag, edit, fileops, symbols, profile, memory, **guardrails**, repomap, semantic, workflow.

Divergence and why it is correct:
- `health` + `help`: blank in docgen; in the daemon they are NON-blank in `daemon.go` (their `RegisterTools` is called explicitly) — that non-blank import ALSO fires their `skill_adapter.go` `init()`. Both surfaces see `get_health`/`get_tool_help`. ✅ sets agree.
- `guardrails`: blank in the daemon only; `GuardrailsSkill.Tools()` returns nil → contributes ZERO rows. Its presence/absence in docgen is table-neutral. ✅ sets agree.

**D-02 HARD INVARIANT — do NOT cross-import the semantic-EXTRACT providers.** `imports.go:15-27` explicitly forbids blank-importing `internal/semantic/extract/*` (goextract/tsextract/pyextract): a blank import would let an `init()` spin a second `*treesitter.GrammarRegistry`, breaking the singleton (BUG-04 / EXTRACT-05). This applies to docgen too — never add extract providers to either file.

**Guidance (RESEARCH Option A + B):**
- Primary: the NEW `docgen --check` CI step is the real anti-drift guard (catches the v1.12 missing-import class regardless of list symmetry).
- Secondary: add cross-reference comments in BOTH files stating "tool-bearing init()-registered providers must appear in both; health/help are non-blank in the daemon by design; never add semantic/extract/* per D-02." Optionally a shared blank-import file, but mind D-02.

---

### README.md — tool table (docs, generated)

**Source:** `cmd/docgen` output between `<!-- BEGIN TOOLS -->` / `<!-- END TOOLS -->`.
**Guidance:** REGEN THIS via `make docs` after the docgen change — NEVER hand-edit between the markers (CLAUDE.md "do not hand-edit the tool table"; the new `--check` gate will trip on manual edits). Table currently renders 51 rows for 50 unique tools (`analyze_blast_radius` registered under two categories); that dual-row is honest registry output — leave it, do not dedupe in prose.

---

## Doc Prose Rewrites (DOCS-01) — rewrite this, with exact current text

**Canonical replacement Core Value to reuse** (STATE.md:26): *"The `helix` CLI is the only surface an agent touches — terse, `relpath:line:col`-anchored, zero schema-preload tax — driving the unchanged warm LSP/RepoMap kernel behind it, so agents use the toolset instead of falling back to grep/sed/cat."*

**MUST-stay-accurate framing** (do NOT claim MCP removed): *"Agents drive the `helix` CLI (via Bash); the CLI dials a warm daemon over internal gRPC. The MCP Go SDK and gRPC IPC are retained as internal daemon plumbing — no longer an agent-facing surface."* (STATE.md:82, 94-02).

### CLAUDE.md (current text verified live)
| Line | Current text | Action |
|------|--------------|--------|
| 18 | `**Helix** — The IDE for your coding agent. A Go-native code intelligence platform for MCP.` | drop "for MCP" → CLI-first identity |
| 20 | `Helix provides 53 MCP tools for semantic code retrieval, editing, and refactoring across 52 languages via LSP.` | retire "53 MCP tools" → "50 `helix` CLI verbs" (or count-free) |
| 24 | `**Core Value:** Rock-solid LSP-backed MCP runtime that … exposes semantic code operations as agent tools …` | replace with STATE.md:26 wording (CLI is agent surface; MCP runtime internal) |
| 94 | `- **Protocol:** MCP (Model Context Protocol) -- primary interface for all clients` | reframe: CLI is the agent interface; MCP SDK/gRPC retained internally |
| 179 | `- **Protocol**: MCP (Model Context Protocol) -- primary interface` | same reframe (Constraints section) |

**Leave intact** (accurate internal-MCP statements): CLAUDE.md:33/55/57/69 (describe `internal/mcp/` SDK, skill MCP tools, middleware) and the entire `## Code intelligence: SMTC-first tool routing` section (194+) — see next section.

### README.md (current text verified live)
| Line | Current text | Action |
|------|--------------|--------|
| 13 | `Code intelligence platform for MCP — 41+ tools across 52 languages.` | "platform for MCP" + stale "41+" → CLI-first + 50 verbs |
| 19 | `* It integrates with any client/LLM via the model context protocol (**MCP**).` | reframe: agents drive `helix` CLI |
| 43 | `Helix provides 41+ MCP tools for coding workflows, backed by real language servers.` | → "50 `helix` CLI verbs" / count-free |
| 47 | `Agents connect via the **model context protocol (MCP)** through:` | STALE/WRONG (heads deleted Phase 94) — rewrite to: agents invoke `helix <verb>` via Bash; daemon dial is internal gRPC |
| 110,122,134 | `"args": ["--mode=stdio"]` (and `--profile=…` variants) | STALE — `--mode=stdio` removed; rewrite config blocks to `helix setup` + CLI usage |
| 142-144 | `helix --mode=http --http-addr=…` / `…:8080/mcp` / `--serve` | STALE — `--mode=http` + `/mcp` removed (only `--mode=auto`); delete/rewrite |
| (table) | `<!-- BEGIN TOOLS -->`…`<!-- END TOOLS -->` | regen via docgen (above) |

> Delta vs RESEARCH: research cited README ~104-145/199/350-404/411/428. Live grep confirms the active stale flags at 110/122/134/142-144 and prose at 13/19/43/47. Planner should re-grep the 199/411/428 lines (internal-detail rewording, lower priority) at edit time.

### .planning/PROJECT.md (current text verified live)
| Line | Current text | Action |
|------|--------------|--------|
| 5 | `A Go-native code intelligence platform for MCP: … 41+ callable MCP tools …` | "platform for MCP" + stale count → CLI-first |
| 9 | `Rock-solid LSP-backed MCP runtime that … exposes semantic code operations as tools.` | replace with STATE.md:26 Core Value |
| 222 | `- **Protocol**: MCP (Model Context Protocol) — primary interface` | reframe (Constraints) |

**Leave intact:** PROJECT.md historical changelog/milestone lines (15/29/35/71/83/145/203/207/290) — these are dated historical records of shipped MCP work, not present-tense primary-interface claims. Do NOT rewrite history; only retire present-tense identity/Core-Value/Constraints claims.

---

## CLAUDE.md NEW Helix-CLI routing section (DOCS-02-routing / REQUIREMENTS DOCS-03)

**Format analog:** the existing `## Code intelligence: SMTC-first tool routing` "Decision matrix" table at `CLAUDE.md:194-260`. MIRROR ITS FORMAT (a `| Question | Use this | Not this |` decision table). **LEAVE THE SMTC SECTION ENTIRELY INTACT** — `mcp__smtc__*` is a SEPARATE EXTERNAL dev MCP server (per user MEMORY + the project's own clarifier at CLAUDE.md:234-247), NOT Helix's product. Rewriting it to `helix` verbs would be factually wrong (Helix ships no taint/CFG/IR tools).

**Action:** ADD a new concise "Helix CLI tool routing" section (Open Q1 recommendation (a)) using the 50 frozen kebab verbs, plus a one-line clarifier that the SMTC section below it is an external dev server. This is what makes "the routing matrix cites CLI verbs end-to-end" literally true (no Helix-own matrix exists today to rewrite).

SMTC format to mirror (`CLAUDE.md:196` intro + decision-matrix table):
```
For code-aware operations, prefer SMTC MCP tools over `Bash` / `Grep` / `Read`. …
| Question | Use this | Not this |
|---|---|---|
| Where is symbol `X` defined? | `mcp__smtc__goto_definition` | `Grep "X"` |
```

New Helix section shape (use real verbs from `internal/cli/verbs_gen.go`, kebab = `toolName.replace("_","-")`):
```
## Helix CLI tool routing (this product's agent surface)
Agents drive Helix via the `helix <verb>` CLI (Bash), not MCP. Prefer these over grep/sed/cat.
| Question | Use this | Not this |
|---|---|---|
| Where is symbol X defined? | `helix go-to-definition` | `Grep "X"` |
| Who references X? | `helix find-references` | `grep -r` |
| File outline | `helix get-symbol-overview` | `Read <file>` |
| Rename across files | `helix rename-symbol` | sed |
| Blast radius of a change | `helix analyze-blast-radius` | manual trace |
(NOTE: the `## SMTC-first tool routing` section below describes an EXTERNAL dev MCP
 server (show-me-the-code), NOT Helix's own tools — do not conflate the two.)
```
The 50 verbs (grouped by `groupID`, all derived mechanically) are enumerated in RESEARCH.md §"Verb names for the routing section". Use the real frozen names; do NOT invent verbs.

---

## Shared Patterns

### Drift-gate (mirror the cligen pattern twice)
**Source:** `Makefile:80` + `go-test.yml:102-106`.
**Apply to:** the new `make verify-docs` target AND the new CI `docgen --check` step. Same `$(GO) run ./cmd/<tool> --check` shape, same HARD-FAIL semantics, same "regenerate with: go run ./cmd/<tool>" comment style.

### Edit generated text at the source, never in README
**Source:** `cmd/docgen/main.go:108-116` (descriptions come from `tool.Description`).
**Apply to:** any "MCP" string inside a generated table row (e.g. `get_tool_help` desc) — edit the tool's registered Description, then `make docs`; never hand-edit between `<!-- BEGIN/END TOOLS -->`.

### Frozen-surface enumeration
**Source:** `internal/cli/verb.go:81` `VerbToolNames()` + `internal/cli/verbs_gen.go` (`// DO NOT EDIT`).
**Apply to:** docgen verb derivation and the CLAUDE.md routing verbs — both must cite real `verbSpecs` names, never a hand-kept list.

## No Analog Found

None. Every change has a precise in-repo analog (cligen gate, the docgen function itself, the SMTC table format, STATE.md Core Value). This is the expected shape for a docs+tooling phase that explicitly mirrors an established pattern.

## Metadata

**Analog search scope:** `Makefile`, `.github/workflows/go-test.yml`, `cmd/docgen/`, `internal/daemon/imports.go`, `internal/cli/verb.go`, `README.md`, `CLAUDE.md`, `.planning/PROJECT.md`, `.planning/STATE.md`
**Files scanned:** 9 (all read or grepped live this session)
**Pattern extraction date:** 2026-06-22
