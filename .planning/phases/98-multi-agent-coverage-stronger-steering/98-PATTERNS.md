# Phase 98: Multi-Agent Coverage + Stronger Steering - Pattern Map

**Mapped:** 2026-06-22
**Files analyzed:** 7 (3 modify nudge/test, 2 new setup_agents + test, 1 modify setup_clients, 1 modify activate)
**Analogs found:** 7 / 7 (every primitive already exists in `internal/cli/`)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/cli/nudge.go` (MODIFY) | CLI hook / classifier | request-response (stdin JSON → stdout envelope) | self — broaden `isGrepReadTool` (nudge.go:429-441) | exact (in-file edit) |
| `internal/cli/nudge_test.go` (MODIFY) | test (golden) | unit | self — `nudgeShapeGoldenCases` (nudge_test.go:475-490) + silent loop (543-558) | exact (in-file edit) |
| `internal/cli/setup_agents.go` (NEW) | CLI setup / file writer | file-I/O (idempotent append) | `mergeHooksIntoSettings` (setup_hooks.go:77-123) + `installSkill` (skill.go:159-236) + `saveSessionStats` (nudge.go:231-254) | role-match (compose 3) |
| `internal/cli/setup_agents_test.go` (NEW) | test (golden round-trip) | unit | existing `internal/cli/*_test.go` (testify assert/require) | role-match |
| `internal/cli/setup_clients.go` (MODIFY) | CLI setup / registrar | event-driven (per-client dispatch) | `GeminiCLIRegistrar.Register` (setup_clients.go:285-287) + `clientRegistry()` (43-53) | exact (in-file edit) |
| `internal/cli/activate.go` (MODIFY) | CLI hook | request-response (stdout → agent context) | self — activate stdout write (activate.go:71) | exact (in-file edit) |

## Pattern Assignments

### `internal/cli/nudge.go` (CLI hook, STEER-01)

**Analog:** self. The gate to broaden is `isGrepReadTool`, NOT `classifyBashTarget` or `bashSteerMessage`.

**The exact gate to edit** (nudge.go:429-441):
```go
func isGrepReadTool(name string, input map[string]any) bool {
	switch name {
	case "Grep", "Read":
		return true
	case "Bash":
		cmd, _ := input["command"].(string)
		return strings.Contains(cmd, "grep") ||
			strings.Contains(cmd, "find") ||
			strings.Contains(cmd, "rg") ||
			strings.Contains(cmd, "ag")
	}
	return false
}
```
STEER-01 edit: add `sed`/`cat` recognition. **Per Open Question 1, anchor on `fields[0]` (the command token)** rather than `strings.Contains(cmd, "cat")` (which would false-match `concatenate.go`). This keeps parity with `classifyBashTarget`'s own `fields[0]` switch (nudge.go:344-345), which ALREADY includes `sed`/`cat`.

**The gate is reached from runNudge** (nudge.go:104-116) — only when `isGrepReadTool` is true does `steerMessage` run:
```go
	if isGrepReadTool(toolName, input.ToolInput) {
		stats.GrepReadCount++
		_ = saveSessionStats(statsPath, stats)
		if advisory, emit := steerMessage(toolName, input.ToolInput); emit {
			emitAdvisory(advisory)
		}
		return nil
	}
```

**The verb mapping is already correct and DEAD for Bash today** — `bashSteerMessage` (nudge.go:180-206) already maps:
- `cat` → `helix read-file` + `helix get-symbol-overview` (nudge.go:190-192)
- `sed` with `-i` → `helix replace-in-file` / `helix replace-symbol-body` (nudge.go:193-195)
- `sed` (no `-i`) → `helix read-file` (sed -n range) (nudge.go:196-197)

Do NOT edit `bashSteerMessage` or `classifyBashTarget` — they already handle sed/cat operands. The prose/log/config SILENCE is already enforced by `classifyBashTarget` (nudge.go:399-402, `nonCodeExtensions`/`nonCodeBasenames`) and `steerMessage`'s `!isCode → "", false` (nudge.go:169-171).

---

### `internal/cli/nudge_test.go` (test golden, STEER-01 / STEER-03 / DEFER-97-01)

**Analog:** self.

**Add firing rows to `nudgeShapeGoldenCases()`** (nudge_test.go:475-490). Current 5 cases each map a shape to a SPECIFIC verb (never a weak "helix" substring). Add:
- `{"bash-sed-i", "Bash", map[string]any{"command": ``sed -i 's/a/b/' pkg/s.go``}, "helix replace-in-file"}` (matches nudge.go:194)
- `{"bash-cat", "Bash", map[string]any{"command": ``cat internal/edit.go``}, "helix read-file"}` (matches nudge.go:191)

**Bump the empty-bucket floor** from `>= 5` to `>= 7` (nudge_test.go:506):
```go
	require.GreaterOrEqual(t, len(cases), 5,
		"golden must cover at least the five steering shapes; rejecting empty-bucket-as-pass")
```

**REMOVE the now-contradictory silent loop** (nudge_test.go:543-558). It currently asserts `bash-sed-i-silent` / `bash-cat-silent` produce NO advisory; once STEER-01 fires them, leaving it would FAIL. The block to delete:
```go
	for _, silent := range []struct{ name, cmd string }{
		{"bash-sed-i-silent", `sed -i 's/a/b/' pkg/s.go`},
		{"bash-cat-silent", `cat internal/edit.go`},
	} {
		// ... asserts assert.Falsef(ok, ...)
	}
```

**Keep / extend the negative control** (nudge_test.go:529-537, `negative-control-prose-grep`). Add STEER-03 broadened rows asserting SILENCE on the new shapes: `sed -i 's/x/y/' README.md`, `cat app.log`, `cat config.yaml`. Mirror the existing `t.Run("negative-control-...", ...)` shape that asserts `parseAdvisory` returns `ok == false`.

**Anti-vacuity revert proof** — `TestNudgeGoldenRevertFails` (nudge_test.go:561-567+) already proves the golden keys on the SPECIFIC verb (asserts a WRONG verb `helix rename-symbol` does NOT appear while `helix read-file` does). Extend it to a sed/cat shape per the validation map (STEER-01 anti-vacuity row).

**Also extend** (per RESEARCH Test Map): add sed/cat true-cases to `TestIsGrepReadTool` and the new sed/cat shapes to `TestNudgeAdvisory_AlwaysExitZero`'s input list.

---

### `internal/cli/setup_agents.go` (NEW — CLI setup file writer, AGENT-01/02/03)

**Analog 1 — idempotent sentinel-delimited append:** `mergeHooksIntoSettings` (setup_hooks.go:77-123). The filter-then-rewrite shape (strip prior Helix entries by sentinel, re-append) is the precedent. For markdown, the sentinel is `<!-- helix:begin -->` … `<!-- helix:end -->` (HTML comment, invisible in render) instead of `"helix_managed": true`:
```go
	// Filter out previous Helix entries.
	filtered := filterOutHelixEntries(existingMatchers)
	// Append new Helix entries.
	filtered = append(filtered, newMatchers...)
```
New helper signature (RESEARCH §Code Examples):
```go
const (
	helixBlockBegin = "<!-- helix:begin (managed by `helix setup`) -->"
	helixBlockEnd   = "<!-- helix:end -->"
)
// writeAgentInstructions: read → strip-between-sentinels → append block →
// 32 KiB cap (Codex AGENTS.md; trim the APPENDED block to a pointer, never user content) → atomic write.
func writeAgentInstructions(path, body string, maxBytes int) error
```

**Analog 2 — atomic temp+rename write:** `saveSessionStats` (nudge.go:231-254) is the minimal precedent:
```go
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("writing temp stats file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming stats file: %w", err)
	}
```
`installSkill` (skill.go:159-236) is the bundle-level two-pass version if multiple files must be staged atomically.

**Analog 3 — path containment (V12):** `withinSkillRoot` / `lexicalSkillRoot` / `containedIn` (skill.go:252-285). The `filepath.Rel` + `".."`-prefix guard:
```go
func containedIn(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
```
Adapt to the agent instruction-file destination (AGENTS.md/GEMINI.md dir), derive dest names from a fixed convention, never raw user input.

**Analog 4 — embedded reference accessor (AGENT-01):** `embeddedSkillBytes()` (skill.go:29-37) reads `skills/helix/SKILL.md` from `embeddedSkillFS` (skill.go:22, `//go:embed skills/helix/*`). Add a sibling `EmbeddedReference()` reading `skills/helix/reference.md`:
```go
func embeddedSkillBytes() string {
	b, err := embeddedSkillFS.ReadFile("skills/helix/SKILL.md")
	if err != nil {
		panic("embedded skills/helix/SKILL.md missing: " + err.Error())
	}
	return string(b)
}
```

**Analog 5 — Codex hooks.json (AGENT-03):** mirror `helixHookConfig` (setup_hooks.go:28-70) which builds the hook map via `map[string]any` + `json.MarshalIndent` (NEVER string-concat JSON). The Claude PreToolUse entry to mirror (setup_hooks.go:44-56):
```go
	"PreToolUse": {
		map[string]any{
			"matcher": "Grep|Read|Bash",
			"hooks": []any{
				map[string]any{
					"type":          "command",
					"command":       fmt.Sprintf("%s nudge", binaryPath),
					"timeout":       5,
					"helix_managed": true,
				},
			},
		},
	},
```
The Codex hook command is `<binaryPath> nudge` — the SAME engine (`runNudge`), no second output struct. **GATE the exact Codex hooks.json byte-shape (key names, command array-vs-string, file location) behind `checkpoint:human-verify` per Assumption A1 before pinning a golden.**

---

### `internal/cli/setup_agents_test.go` (NEW — test)

**Analog:** existing `internal/cli/*_test.go` patterns (testify `assert`/`require`, `t.TempDir()`). Required goldens (RESEARCH Test Map + Pitfall 4):
- (a) append into empty dir produces the block
- (b) pre-existing user content survives (round-trip)
- (c) re-run is idempotent — NO duplicate block (the anti-vacuity invariant; a duplicated block must FAIL)
- (d) 32 KiB cap with a representative near-cap user file keeps file ≤ 32*1024
- `TestGeminiNoHookArtifact` — assert NO Gemini `hooks.json` artifact is produced (locked anti-feature)

---

### `internal/cli/setup_clients.go` (MODIFY — registrar, AGENT-02/03)

**Analog — the teardown-only handler to flip:** `GeminiCLIRegistrar.Register` (setup_clients.go:285-287):
```go
func (r *GeminiCLIRegistrar) Register(cfg RegistrationConfig) error {
	return teardownOnlyRegister(r, cfg, "gemini-cli")
}
```
Flip gemini-cli / generic to ALSO call `writeAgentInstructions(...)` after teardown (keep the MCP-teardown). `teardownOnlyRegister` (setup_clients.go:94-102) stays the precedent for the teardown half.

**Analog — registry to extend:** `clientRegistry()` (setup_clients.go:43-53) returns 7 registrars; add `"codex": &CodexRegistrar{}`. New `CodexRegistrar` implements the `ClientRegistrar` interface (setup_clients.go:14-28): `Name`/`Description`/`Register`/`Unregister`/`teardownMCP`. Its `Register` writes AGENTS.md (terse block + pointer, NOT the full 32 KB reference) via `writeAgentInstructions` AND a Codex hooks.json via the AGENT-03 writer. Scope the flip to codex + gemini-cli + generic only (Assumption A5; leave vscode/jetbrains/opencode teardown-only).

**Also:** add `"codex"` to `ValidArgs` (setup.go:31).

---

### `internal/cli/activate.go` (MODIFY — SessionStart stdout, STEER-02)

**Analog:** self. The current SessionStart stdout write (activate.go:70-72):
```go
	// Output for Claude Code hook stdout (added to agent context)
	fmt.Fprintf(os.Stdout, "Helix workspace activated: %s (status: %s)\n", absPath, resp.Status)
	return nil
```
STEER-02 extends this to also emit the terse "use X not Y" matrix once per session, AFTER the activation line, behind a size cap (≤ ~2 KiB per Open Question 3; SKILL-04 uses a 1,536-char idle cap as the precedent). Source the matrix text from a SINGLE constant so it cannot drift from SKILL.md's table. Fail-open: a priming failure must not break the session (stdout best-effort, return path unchanged). New test `TestSessionStartPriming` asserts the cap + fail-open.

---

## Shared Patterns

### Idempotent managed-block write
**Source:** `mergeHooksIntoSettings` filter-then-rewrite (setup_hooks.go:91-106)
**Apply to:** `setup_agents.go` `writeAgentInstructions` (strip-between-sentinels → re-append)

### Atomic file write (temp + rename)
**Source:** `saveSessionStats` (nudge.go:243-251), `installSkill` two-pass (skill.go:179-235)
**Apply to:** every new instruction-file / hooks.json write in `setup_agents.go`

### JSON via encoding/json marshal, NEVER string concat (T-34-01)
**Source:** `emitAdvisory` (nudge.go:139-149), `helixHookConfig`+`json.MarshalIndent` (setup_hooks.go:28-70, 110)
**Apply to:** Codex hooks.json writer; the PreToolUse envelope is already correct (shared with Claude)

### Path containment (V12, defeats `..` traversal)
**Source:** `withinSkillRoot`/`containedIn` filepath.Rel guard (skill.go:252-285)
**Apply to:** instruction-file + hooks.json destinations in `setup_agents.go`

### Embedded asset accessor (one generated source)
**Source:** `embeddedSkillBytes()` reading `embeddedSkillFS` (skill.go:22, 29-37)
**Apply to:** new `EmbeddedReference()` reading `skills/helix/reference.md` (AGENT-01)

### Bash string read as DATA, never executed (V5 / T-93-05)
**Source:** `classifyBashTarget` bounded `strings.Fields` tokenization (nudge.go:337-425), proven by `TestClassifyBashTarget_PureDataNoExec`
**Apply to:** the STEER-01 `isGrepReadTool` broadening — token-anchor on `fields[0]`, no `os/exec`

### Advisory exit-0 / fail-open (locked anti-feature: no exit-2 deny)
**Source:** `runNudge` always returns nil + `emitAdvisory` never errors (nudge.go:64-121, 139-149)
**Apply to:** every broadened shape; Codex hook emits `additionalContext`, never `permissionDecision:"deny"`

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| (none) | — | — | Every primitive (idempotent block, atomic write, JSON envelope, path containment, embed accessor, data-only Bash tokenization) already exists in `internal/cli/`. The new code is wiring + one append helper, not new infrastructure. |

## Metadata

**Analog search scope:** `internal/cli/` (nudge.go, nudge_test.go, setup_hooks.go, setup_clients.go, skill.go, activate.go, setup.go)
**Files scanned:** 7
**Pattern extraction date:** 2026-06-22
