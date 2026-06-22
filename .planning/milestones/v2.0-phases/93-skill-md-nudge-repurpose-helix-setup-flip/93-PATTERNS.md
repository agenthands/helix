# Phase 93: SKILL.md + Nudge Repurpose + `helix setup` Flip - Pattern Map

**Mapped:** 2026-06-21
**Files analyzed:** 9 (3 new code, 1 new asset, 4 modified, 1 new test + 1 modified test)
**Analogs found:** 9 / 9 (all anchored to real files in this repo)

This phase is ~80% wiring of machinery that already exists and is tested. Every new/changed file has a concrete in-repo analog; the SKILL.md decision-table format mirrors `CLAUDE.md`'s SMTC matrix. No new deps, no proto churn (carried Phase 90/92 invariant).

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/cli/skill.go` (NEW) | config/asset writer | file-I/O | `internal/eval/judge/client.go` (embed→string) + `internal/cli/setup_hooks.go` (atomic write) | exact (embed) / role-match (write) |
| `internal/cli/skills/helix/SKILL.md` (NEW) | static asset (markdown) | — (embedded data) | `internal/kernel/help/docs/*.md` + `CLAUDE.md` "Decision matrix" | format-match |
| `internal/cli/nudge.go` (MODIFIED) | CLI / PreToolUse hook handler | event-driven (stdin JSON) | itself (repurpose `runNudge`/`isGrepReadTool`) | exact (in-place) |
| `internal/cli/setup_clients.go` (MODIFIED) | CLI client registrars | request-response (config mutation) | itself (`removeFromJSONConfig`, per-client `Unregister`) | exact (in-place) |
| `internal/cli/setup.go` (MODIFIED) | CLI orchestrator | request-response | itself (`runSetup` flow) | exact (in-place) |
| `internal/cli/skill_test.go` (NEW) | unit test | — | `internal/cli/verbs_gen_test.go` (`VerbToolNames()` membership) | role-match |
| `internal/cli/nudge_test.go` (MODIFIED) | unit test | — | existing nudge tests | exact |
| `internal/cli/setup_test.go` (MODIFIED) | unit test | — | existing setup tests | exact |
| `test/oracle/llm/skill_trigger_test.go` (NEW) | behavioral test | request-response (LLM API) | `test/oracle/llm/selection_test.go` | exact |
| `test/oracle/llm/prompt.go` (MODIFIED) | test fixture/prompt builder | — | itself (`SelectionSystemPrompt`) | exact |

## Pattern Assignments

### `internal/cli/skill.go` (NEW — config/asset writer, file-I/O)

**Analog A (embed→string):** `internal/eval/judge/client.go:21-22`

```go
//go:embed prompts/rubric.md
var embeddedRubric string
```

Convention to replicate: single-file markdown asset embedded as a `string` var (not `embed.FS`). The blank `_ "embed"` import is needed when no other `embed` symbol is referenced (see `client.go:10`). SKILL.md is one file, so the string form (not the `embed.FS` form of `internal/kernel/help/embed.go:5-6`) is the closer match. Place the asset at `internal/cli/skills/helix/SKILL.md` and embed with `//go:embed skills/helix/SKILL.md` (path is relative to the embedding `.go` file).

**Analog B (atomic, idempotent, dir-creating disk write):** `internal/cli/nudge.go:130-155` (`saveSessionStats`) and `internal/cli/setup_hooks.go:115-122`

```go
// nudge.go:140-152 — MkdirAll 0755, write temp 0644, rename (atomic, no corruption)
if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil { ... }
tmpPath := path + ".tmp"
if err := os.WriteFile(tmpPath, append(data, '\n'), 0644); err != nil { ... }
if err := os.Rename(tmpPath, path); err != nil { ... }
```

Convention to replicate for `installSkill(targetDir)`: `MkdirAll(dir, 0755)` then atomic temp+rename write at `0644`. Resolve+contain the target path before write (mirror the `filepath.Rel`+`..`-prefix guard cited from 92-02 `readSnippetLine`) to prevent traversal from a crafted client/project dir (Security V12).

---

### `internal/cli/skills/helix/SKILL.md` (NEW — static asset, markdown)

**Format analog:** `CLAUDE.md` "## Code intelligence: SMTC-first tool routing" → "### Decision matrix" (the `| Question | Use this | Not this |` table).

The exact 3-column table shape Phase 93 must produce is already authored in this repo's `CLAUDE.md` for SMTC. Replicate that header and row style, but cite the **frozen `helix` kebab verbs** (below) instead of `mcp__smtc__*`, and the **frozen terse output** `relpath:line:col<TAB>payload`.

**Frozen verb names to cite** (source: `internal/cli/verbs_gen.go` `verbSpecs` keys; accessor `internal/cli/verb.go:81 func VerbToolNames()`; group via the `groupID:` field, e.g. `verbs_gen.go:14 groupID: "navigation"`):

- navigation: `go-to-definition` `find-references` `search-symbols` `get-hover-info` `find-implementations` `get-call-hierarchy` `get-type-hierarchy` `get-symbol-overview` `analyze-blast-radius`
- edit: `replace-symbol-body` `rename-symbol` `safe-delete-symbol` `insert-before-symbol` `insert-after-symbol` `verify-edit`
- fileops: `read-file` `create-file` `find-files` `list-directory` `search-in-files` `replace-in-file` `fuzzy-edit`
- diagnostics: `get-diagnostics` `get-code-actions` `format-code`
- repomap: `get-repo-map` `get-context` `get-semantic-context` `get-cluster-map` `explain-cluster` `explain-symbol-deep` `find-related-symbols` `get-change-impact-graph` `get-semantic-graph-status` `index-semantic-graph` `refresh-semantic-graph` `validate-graph-edge`
- memory: `read-memory` `write-memory` `list-memories` `search-memories` `rename-memory` `edit-memory` `delete-memory` `onboard-project` `prepare-for-new-conversation` `switch-mode` `get-token-budget` `get-health` `get-tool-help`

Convention to replicate: cite verbs by capability group matching `groupID`, do NOT hardcode a verb count in prose (VERB-01 lesson). Frontmatter (`name` / `description` / `allowed-tools: Bash(helix:*)`) is hand-authored YAML inside the markdown — no parser dep at runtime (binary writes verbatim; validation is a test concern). Keep description+when_to_use ≤1,536 chars (idle-cost upper bound, SKILL-04).

---

### `internal/cli/nudge.go` (MODIFIED — PreToolUse hook handler, event-driven)

**Analog: itself.** The lifecycle is correct; Phase 93 changes the *message* and *trigger logic*, not the contract.

**Exit-0 / safe-decode contract to preserve** (`nudge.go:52-58`):

```go
func runNudge(cmd *cobra.Command, _ []string) error {
	var input hookInput
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		return nil // stdin fails → silent, exit 0
	}
```

`runNudge` already returns `nil` on every path (advisory only, D-11) and decodes stdin as data via `json.Decoder` — never executes the command (T-36-01). The new `classifyBashTarget` MUST also treat `input.ToolInput["command"]` as data only (tokenize, never `exec`).

**Repurpose target 1 — the emitted message** (`nudge.go:99`, currently a generic 5-call-threshold `fmt.Println`):

```go
fmt.Println("Tip: Helix provides find_symbol and get_symbols_overview ...")
```

Change to per-call advisory `helix <verb>` substitution. Surface as structured `hookSpecificOutput.additionalContext` JSON on stdout (confirm exact key against the installed CC hooks schema in Wave 0; the existing `hookInput` doc comment at `nudge.go:14-15` already references `code.claude.com/docs/en/hooks`). Keep exit 0 regardless (Pitfall 4).

**Repurpose target 2 — `isGrepReadTool`** (`nudge.go:184-198`):

```go
case "Bash":
	cmd, _ := input["command"].(string)
	return strings.Contains(cmd, "grep") || strings.Contains(cmd, "find") || ...
```

This `strings.Contains` cannot tell a `.go` target from a `.log`/`.md` target (Pitfall 2). Add `classifyBashTarget(cmd) -> (isCode bool, ok bool)`: tokenize, find file/path operands, check extensions against a code-extension allowlist (reuse the spirit of `langregistry.ByExtension`). Fail open — if parsing fails, no code target found, or target is `.md/.log/.txt/.json/.yaml/Dockerfile`, emit nothing and exit 0. Only suggest on a positively-identified code target.

**Steer table (Bash shape → frozen verb)** — embed in the nudge:

| Bash shape | Suggest |
|---|---|
| `grep <pat> <code>` | `helix search-in-files --pattern=<pat>` |
| `grep "func X"/symbol-decl` | `helix search-symbols --query=<name>` |
| `grep -r "Foo("` | `helix find-references` / `helix get-call-hierarchy` |
| `find . -name '*.go'` | `helix find-files --pattern='**/*.go'` |
| `cat <code>` | `helix read-file --path=<file>` (or `helix get-symbol-overview`) |
| `sed -n 'N,Mp' f` | `helix read-file --path=<file>` |
| `sed -i 's/../../' f` | `helix replace-in-file` / `helix replace-symbol-body` |

---

### `internal/cli/setup_clients.go` (MODIFIED — client registrars, config mutation)

**Analog: itself.** Add a focused MCP-only teardown and call it from each `Register` before the skill write. Do NOT reuse `Unregister` wholesale — it also strips hooks (Pitfall 3).

**Idempotent removal primitive to reuse** (`setup_clients.go:97-125 removeFromJSONConfig`):

```go
func removeFromJSONConfig(path, key, serverName string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) { return nil } // missing file → no-op
		...
	}
	servers, ok := existing[key].(map[string]any)
	if !ok { return nil } // missing key → no-op
	delete(servers, serverName)
	...
}
```

Already idempotent (missing file → nil, missing key → nil). New `teardownPriorMCP(cfg)` calls the per-client `(path, key, "helix")` triple already encoded in each `Unregister`:

- claude-code: `claude mcp remove helix --scope <scope>` (CLI, `setup_clients.go:233`) with `.mcp.json mcpServers` direct-write fallback
- vscode: `removeFromJSONConfig(path, "servers", "helix")` (`:464`)
- jetbrains: `("mcpServers")` `.junie/mcp/mcp.json` (`:511`)
- claude-desktop: `("mcpServers")` platform path (`:559`)
- gemini-cli: `("mcpServers")` + `ensureDisabled()` enablement (`:412, :326`)
- opencode: `("mcp")` `opencode.json` (`:628`)
- generic: stdout/file (`:688`)

**Critical: do NOT call `removeHooksFromSettings`** during teardown — Phase 93 wants hooks present. `Unregister`'s hook-removal is at `setup_clients.go:251-261` (`ClaudeCodeRegistrar.Unregister`); the MCP-only teardown must omit it.

**Hook + skill install ordering** in each `Register`: (1) `teardownPriorMCP`, (2) `installSkill(<client skills dir>)`, (3) `mergeHooksIntoSettings` (already idempotent — `setup_hooks.go:100-105` filters prior `helix_managed` then re-adds). The PreToolUse matcher already covers the targets: `setup_hooks.go:45 "matcher": "Grep|Read|Bash"`.

**Per-client skill consumer caveat (Open Q2):** only Claude-family clients (claude-code, claude-desktop) consume Agent Skills. For non-Claude clients the flip may mean MCP-teardown-only. Decide per client in planning; SKILL-02 acceptance is concretely specified for claude-code.

---

### `internal/cli/setup.go` (MODIFIED — orchestrator)

**Analog: itself** (`runSetup`, `setup.go:42-141`). The Register flow at `:107` already calls `registrar.Register(cfg)`; skill install wires *inside* each registrar's `Register` (not a separate orchestration step), so `runSetup` needs minimal change. `RegistrationConfig` (`setup_clients.go:26-34`) already carries `BinaryPath`/`Global`/`DryRun`/`ProjectDir`/`NoHooks`/`Printer` — sufficient; no new field needed unless a `--no-skill` opt-out is added (mirror the existing `NoHooks` flag at `setup.go:36, :82`).

---

### `internal/cli/skill_test.go` (NEW — unit test)

**Analog:** `internal/cli/verbs_gen_test.go:80-104` (`TestVerbToolNames_SortedSetEqual`).

Convention to replicate: assert every verb named in SKILL.md is a member of `cli.VerbToolNames()` (defined `internal/cli/verb.go:81`) — this catches docgen/cligen drift (Pitfall 5) in the default suite. Also assert: embedded `embeddedSkillMD` non-empty, frontmatter YAML parses (use already-vendored test libs, no new parser dep), token-note present, idempotent write.

---

### `test/oracle/llm/skill_trigger_test.go` (NEW — behavioral test)

**Analog:** `test/oracle/llm/selection_test.go:20-64` (`TestSelection`).

```go
//go:build llm
func TestSelection(t *testing.T) {
	SkipWithoutAPIKey(t)                 // client.go:50 — hermetic skip, never logs key
	...
	response, stopReason, err := AskSingleTurn(ctx, client, model, system, user) // client.go:94
	require.NoError(t, err, ...)
	require.True(t, strings.Contains(strings.ToLower(response), strings.ToLower(tool.Name)), ...)
	WriteTranscript(t, &Transcript{...})  // transcript.go:31
}
```

Convention to replicate exactly: `//go:build llm` tag, `SkipWithoutAPIKey(t)` first line, `AskSingleTurn` for the single-turn call, `WriteTranscript` for the record, `InterCallDelay()` between calls. The new test runs the same code task twice — baseline (system prompt WITHOUT SKILL.md) vs skill (system prompt WITH SKILL.md body) — and asserts the skill run shifts the chosen tool toward a `helix` verb vs grep/sed/cat. For SKILL-04 the "before" full-MCP-schema blob is still obtainable via `runner.Session.ListTools` + `FormatToolList` (`selection_test.go:28, :34`) since the MCP head is alive until Phase 94.

---

### `test/oracle/llm/prompt.go` (MODIFIED — prompt builder)

**Analog: itself** (`SelectionSystemPrompt`, `prompt.go:97-103`).

```go
func SelectionSystemPrompt(toolList string) string {
	return fmt.Sprintf(`You are an expert at selecting the right MCP tool ... %s ...`, toolList)
}
```

Convention to replicate: add a `SkillSystemPrompt(skillBody string)` (or a `baseline`/`skill` variant pair) and helix-verb task descriptions, mirroring the `fmt.Sprintf` prompt-template style and `FormatToolList` helper (`prompt.go:127`).

## Shared Patterns

### Idempotent JSON config mutation (never string-concat)
**Source:** `internal/cli/setup_clients.go:62-93` (`mergeJSONConfig`) and `:97-125` (`removeFromJSONConfig`)
**Apply to:** `teardownPriorMCP` in every registrar; any new config write.
Read JSON → mutate map → `json.MarshalIndent` → `MkdirAll(0755)` → `WriteFile(0644)`. Missing file/key → no-op (T-34-01/05).

### Hook entry idempotency (helix_managed tagging)
**Source:** `internal/cli/setup_hooks.go:77-123` (`mergeHooksIntoSettings`) + `:177-214` (`filterOutHelixEntries`/`isHelixManaged`)
**Apply to:** the (3) hook-merge step of each `Register`. Already filters prior `"helix_managed": true` then re-adds; re-runs converge. Preserves user hooks.

### Safe hook-stdin parsing (data, never exec)
**Source:** `internal/cli/nudge.go:54-55` (`json.NewDecoder(os.Stdin).Decode`)
**Apply to:** the new `classifyBashTarget` — tokenize the command string as data; never shell out (T-36-01, Security V5).

### Atomic disk write
**Source:** `internal/cli/nudge.go:144-152` (temp + rename)
**Apply to:** `installSkill` SKILL.md write (Security V12 — config-corruption mitigation).

### Behavioral oracle gating
**Source:** `test/oracle/llm/client.go:50-60` (`SkipWithoutAPIKey`), `:94` (`AskSingleTurn`), `transcript.go:31` (`WriteTranscript`)
**Apply to:** `skill_trigger_test.go`. `//go:build llm`, key-gated hermetic skip, multi-provider (Anthropic + DeepSeek), transcript record.

### Embed-then-write a markdown asset
**Source:** `internal/eval/judge/client.go:21-22` (`//go:embed prompts/rubric.md` → `string`)
**Apply to:** `skill.go` (`embeddedSkillMD`). String form over `embed.FS` since SKILL.md is one file.

## No Analog Found

None. Every new/changed file maps to an in-repo analog. The only genuinely new *prose/logic* is: (a) the SKILL.md content, (b) `classifyBashTarget`, (c) the skill-vs-baseline comparison test — each of which copies the structure/contract of an existing file even where the content is new.

## Metadata

**Analog search scope:** `internal/cli/`, `internal/kernel/help/`, `internal/eval/judge/`, `test/oracle/llm/`
**Files scanned:** nudge.go, setup.go, setup_clients.go, setup_hooks.go, verbs_gen.go, verb.go, verbs_gen_test.go, help/embed.go, judge/client.go, oracle/llm/{client,prompt,selection_test,transcript}.go, CLAUDE.md
**Pattern extraction date:** 2026-06-21
**Constraints honored:** zero new deps, zero proto churn, SMTC-first (no java-security activation — Go project)
