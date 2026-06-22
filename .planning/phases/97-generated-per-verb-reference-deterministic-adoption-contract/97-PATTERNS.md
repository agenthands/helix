# Phase 97: Generated Per-Verb Reference + Deterministic Adoption Contract - Pattern Map

**Mapped:** 2026-06-22
**Files analyzed:** 8 (3 NEW, 5 MODIFY)
**Analogs found:** 8 / 8 (all exact in-repo)

This phase is almost entirely cloning of existing, drift-gated machinery. Every new
file has a strong in-tree analog. The only genuinely novel code is the `helix-refgen`
render body and the ADOPT-01 contract assertions; everything else mirrors
`cmd/docgen` / `cmd/helix-cligen` / `installSkill` / `nudge_test.go`.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `cmd/helix-refgen/main.go` (NEW) | generator (cmd/main) | transform (registry → file) + `--check` gate | `cmd/docgen/main.go` + `cmd/helix-cligen/main.go` | exact |
| `cmd/helix-refgen/main_test.go` (NEW) | test | unit + round-trip | `cmd/helix-cligen/*_test.go`, `nudge_test.go` harness | role-match |
| `internal/cli/skills/helix/reference.md` (NEW) | generated artifact | committed build artifact | `internal/cli/skills/helix/SKILL.md` + README docgen table | exact (sibling) |
| `internal/cli/skill.go` (MODIFY) | config/asset embed + installer | file-I/O (embed → atomic write) | itself (current `string` → `embed.FS`) | self |
| exported verb→flags accessor in `internal/cli` (NEW func, e.g. `verb.go`) | accessor | read-only catalog read | `VerbToolNames()` (`internal/cli/verb.go:81-88`) | exact |
| `internal/cli/reference_contract_test.go` (NEW) | test (adoption gate) | deterministic in-package assert | `TestHelixSymbolicTools_NoDrift` (`nudge_test.go:94-104`) | exact |
| `internal/cli/nudge_test.go` (MODIFY) | test (golden table) | request-response golden | `runNudgeCapture`/`parseAdvisory` (`nudge_test.go:276-356`) | exact (extend) |
| `internal/cli/skill_test.go` (MODIFY) | test | unit (install + idle-cost) | `TestSkillIdleCostBound` (`skill_test.go:141`) + install tests | exact (extend) |
| `Makefile` (MODIFY) | build config | make target | `verify-cligen`/`verify-docs` (`Makefile:80-85`) | exact |
| `.github/workflows/go-test.yml` (MODIFY) | CI config | CI drift-gate step | cligen/docgen gate steps (`:102-112`) | exact |
| `internal/cli/setup_clients.go` (POSSIBLY MODIFY) | install call site | — | `installSkill` call (`:158-163`) | self (likely no change needed) |

## Pattern Assignments

### `cmd/helix-refgen/main.go` (NEW — generator, transform + `--check`)

**Analog:** `cmd/docgen/main.go` (registry walk + `--check` + blank-import parity), `cmd/helix-cligen/main.go` (offline scan + `--check` over a single committed file — closest because refgen also writes ONE file, not a section-replace).

**Blank-import-parity block to copy VERBATIM** (`cmd/docgen/main.go:21-46`). This is load-bearing — MEMORY `helix-tool-docs-drift`: docgen once silently dropped tools because its imports diverged from the daemon's. Copy the whole block including the comment:
```go
// Blank imports trigger skill.Register() via init() (same as daemon/imports.go).
// ... blank-import parity rule comment ...
_ "github.com/agenthands/helix/internal/kernel/diag"
_ "github.com/agenthands/helix/internal/kernel/edit"
_ "github.com/agenthands/helix/internal/kernel/fileops"
_ "github.com/agenthands/helix/internal/kernel/health"
_ "github.com/agenthands/helix/internal/kernel/help"
_ "github.com/agenthands/helix/internal/kernel/symbols"
_ "github.com/agenthands/helix/internal/profile"
_ "github.com/agenthands/helix/internal/skill/memory"
_ "github.com/agenthands/helix/internal/skill/repomap"
_ "github.com/agenthands/helix/internal/skill/semantic"
_ "github.com/agenthands/helix/internal/skill/workflow"
"github.com/agenthands/helix/internal/skill"
```
NEVER blank-import `internal/semantic/extract/*` (D-02: double GrammarRegistry).

**`--check` gate pattern** — `cmd/helix-cligen/main.go:68-84` is the closest shape (single committed file, reads existing first). The `reference.md` file does NOT exist yet on first run, so handle a missing-file `os.ReadFile` error gracefully (do NOT `log.Fatalf` on first generate; only `--check` against a missing file is a fail):
```go
out := flag.String("out", "internal/cli/skills/helix/reference.md", "path to reference.md")
check := flag.Bool("check", false, "check mode: exit 1 if reference.md would change")
flag.Parse()

rendered := renderReference()  // walk skill.ToolProviders(); args from verb accessor
if *check {
    existing, err := os.ReadFile(*out)
    if err != nil { log.Fatalf("reading %s: %v", *out, err) }   // cligen:69-72 shape
    if rendered != string(existing) {
        fmt.Fprintf(os.Stderr, "reference.md is out of date. Run 'go run ./cmd/helix-refgen' to regenerate.\n")
        os.Exit(1)
    }
    fmt.Println("reference.md is up to date.")
    return
}
if err := os.WriteFile(*out, []byte(rendered), 0644); err != nil { log.Fatalf("writing %s: %v", *out, err) }
fmt.Printf("Updated %s\n", *out)
```

**Registry-walk source** — `cmd/docgen/main.go:110-131`:
```go
providers := skill.ToolProviders()
for _, tp := range providers {
    category := tp.Name()
    for _, tool := range tp.Tools() {
        // tool.Description, tool.HelpText, tool.BriefDescription — NO tool.InputSchema (Pitfall 1)
        verb := strings.ReplaceAll(tool.Name, "_", "-")  // mechanical kebab mapping, docgen:129
        ...
    }
}
```

**Markdown escaping** (`cmd/docgen/main.go:124-125`) — escape pipes when rendering tool Descriptions into markdown tables (Security V12 / markup-injection mitigation):
```go
desc = strings.ReplaceAll(desc, "|", "\\|")
```

**Per-verb args source (CRITICAL — Pitfall 1):** `tool.InputSchema` does NOT exist on `mcp.ToolDef`. Do NOT call `help.ExtractParamDocs(tool.InputSchema)` and do NOT build a `SerenaMCPServer`. Source args from the new exported `internal/cli` accessor (below) backed by `verbSpecs` — each `verbFlag` carries `name`/`toolArg`/`kind`/`required`/`help` (`internal/cli/verb.go:44-54`, sample data `verbs_gen.go:11-19`).

---

### NEW exported verb→flags accessor in `internal/cli` (e.g. `VerbSpecsForDocs()`)

**Analog:** `VerbToolNames()` (`internal/cli/verb.go:81-88`) — mirror its fresh-copy discipline exactly:
```go
func VerbToolNames() []string {
    names := make([]string, 0, len(verbSpecs))
    for _, spec := range verbSpecs {
        names = append(names, spec.toolName)
    }
    sort.Strings(names)
    return names  // fresh copy — mutating it does not affect verbSpecs
}
```
The new accessor must return verb→(toolName, groupID, flags) for `helix-refgen` to render the args section, returning copies of the flag slices so callers cannot mutate `verbSpecs`. `verbFlag`/`verbSpec` are package-private (`verb.go:44-74`), so refgen cannot read `verbSpecs` directly — the accessor must export a doc-facing view (e.g. a small exported struct with `Name`/`ToolArg`/`Kind`/`Required`/`Help` string fields). Keep it read-only; mirror the `VerbToolNames` godoc note about the out-of-package seam.

---

### `internal/cli/skill.go` (MODIFY — `string` → `embed.FS`)

**Self-analog: the current shape is the thing being modified.** Preserve `withinSkillRoot` / `lexicalSkillRoot` / `containedIn` (`skill.go:172-205`) and the atomic temp+rename UNCHANGED.

**Current embed declaration to change** (`skill.go:21-22`):
```go
//go:embed skills/helix/SKILL.md
var embeddedSkillMD string
```
→ switch to `//go:embed skills/helix/*` + `var embeddedSkillFS embed.FS` (add `"embed"` non-underscore import). Provide a SKILL.md-only accessor (e.g. `embeddedSkillBytes()` reading `skills/helix/SKILL.md` from the FS) so the ~13 in-package `embeddedSkillMD` references stay a mechanical rename.

**`installSkill` to extend** (`skill.go:131-156`) — keep the guard + MkdirAll, then walk the FS and write EACH entry with the existing per-file temp+rename + single-trailing-newline policy:
```go
func installSkill(targetDir string) error {
    if !withinSkillRoot(targetDir) {                          // UNCHANGED guard (:132-134)
        return fmt.Errorf("refusing skill install: target %q is outside the skills/helix root", targetDir)
    }
    if err := os.MkdirAll(targetDir, 0755); err != nil {     // UNCHANGED (:136)
        return fmt.Errorf("creating skill directory: %w", err)
    }
    entries, err := embeddedSkillFS.ReadDir("skills/helix")
    if err != nil { return fmt.Errorf("reading embedded skill dir: %w", err) }
    for _, e := range entries {
        data, err := embeddedSkillFS.ReadFile("skills/helix/" + e.Name())
        if err != nil { return fmt.Errorf("reading embedded %s: %w", e.Name(), err) }
        if len(data) == 0 || data[len(data)-1] != '\n' {     // single-trailing-newline policy (:143-145)
            data = append(data, '\n')
        }
        dst := filepath.Join(targetDir, e.Name())            // entry name from embed FS only — not user-controlled
        tmp := dst + ".tmp"
        if err := os.WriteFile(tmp, data, 0644); err != nil { return fmt.Errorf("writing temp skill file: %w", err) }
        if err := os.Rename(tmp, dst); err != nil { return fmt.Errorf("renaming skill file: %w", err) }
    }
    return nil
}
```
Note: entry names come ONLY from the embedded FS (never user input), so the multi-file loop keeps the T-93-01 path-traversal posture; `withinSkillRoot` still gates the targetDir.

**Ripple points (all small, all localized — verified in RESEARCH):**
- `EmbeddedSkillBody()` (`skill.go:24-29`) must STILL return ONLY `SKILL.md` bytes (consumed by `test/oracle/llm/prompt.go:142` + `skill_trigger_test.go:91`). Read `skills/helix/SKILL.md` from the FS.
- `skillDescription()` (`skill.go:42-55`) and the frontmatter parsers (`skill.go:61-109`) operate on the SKILL.md string — feed them the SKILL.md FS entry, NOT the bundle.
- `uninstallSkill` (`skill.go:213-227`) currently removes only `SKILL.md`; decide whether to also remove `reference.md` (recommend: remove all embedded entry names so uninstall stays clean).

---

### `internal/cli/skill_test.go` (MODIFY — keep SKILL-04 pointed at SKILL.md only)

**Analog: `TestSkillIdleCostBound`** (`skill_test.go:141-150`) — the 1536-char cap must STILL read SKILL.md description only, NOT the bundle, and `reference.md` is explicitly EXEMPT from the cap:
```go
func TestSkillIdleCostBound(t *testing.T) {
    desc, err := skillDescription()      // SKILL.md frontmatter only
    if err != nil { t.Fatalf("skillDescription() error: %v", err) }
    const cap1536 = 1536
    if n := len([]byte(desc)); n > cap1536 {
        t.Errorf("idle skill cost = %d bytes, exceeds Claude Code listing cap %d", n, cap1536)
    }
}
```
`embeddedSkillMD` is referenced directly in `skill_test.go` at ~lines 75, 83, 106, 156, 162, 166, 185, 209, 217, 245 and `setup_test.go:590` — these need a `SKILL.md`-returning accessor or a renamed var (mechanical, verify with `go build ./...`). Add a NEW `TestInstallSkill`-extension asserting BOTH `SKILL.md` AND `reference.md` land in `targetDir` (extend existing install test `skill_test.go:234-313`), plus `TestEmbeddedSkillBody` proving no reference.md bleed into the body.

---

### `internal/cli/reference_contract_test.go` (NEW — ADOPT-01a completeness, authority = `VerbToolNames()`)

**Analog: `TestHelixSymbolicTools_NoDrift`** (`nudge_test.go:94-104`) — the existing precedent for cross-checking against `VerbToolNames()` as authority:
```go
func TestHelixSymbolicTools_NoDrift(t *testing.T) {
    registered := make(map[string]bool)
    for _, name := range VerbToolNames() {
        registered[name] = true
    }
    for toolName := range helixSymbolicTools {
        assert.Truef(t, registered[toolName], "...stale name...", toolName)
    }
}
```
For ADOPT-01a, invert: iterate `VerbToolNames()` (the 50 frozen authority — NOT refgen's own output, Pitfall 2) and assert the embedded `reference.md` mentions each kebab verb:
```go
func TestReferenceCoversEveryVerb(t *testing.T) {
    ref := readEmbeddedReference(t)   // skills/helix/reference.md bytes via the embed.FS
    for _, toolName := range VerbToolNames() {
        verb := strings.ReplaceAll(toolName, "_", "-")   // docgen:129 mechanical mapping
        assert.Containsf(t, ref, verb, "reference.md missing section for verb %q", verb)
    }
}
```
**MANDATORY revert-and-fail sibling** (`TestReferenceCompletenessRevertFails`): take the embedded `reference.md` bytes, strip ONE verb's section in-memory, run the completeness check against the mutated copy, assert it reports the missing verb. Proves the gate is registry-sourced, not a set-compared-to-itself tautology.

---

### `internal/cli/nudge_test.go` (MODIFY — ADOPT-01b per-shape golden, keyed on emitted command)

**Harness analog: `runNudgeCapture` / `parseAdvisory`** (`nudge_test.go:276-332`) — reuse verbatim (redirects stdin/stdout, isolates session-stats per `t.TempDir()`):
```go
out, err := runNudgeCapture(t, hookInput{ToolName: "Bash", ToolInput: map[string]any{"command": tc.cmd}})
require.NoError(t, err)                                   // exit-0 contract preserved
adv, ok := parseAdvisory(t, out); require.True(t, ok)
```

**Weak assertion to STRENGTHEN** (`nudge_test.go:344` — the exact Pitfall 3 defect):
```go
assert.Contains(t, adv.HookSpecificOutput.AdditionalContext, "helix",   // ← too weak; matches prompt text
    "advisory should name a helix verb")
```
Replace with a golden table mapping each shape to the SPECIFIC expected `helix <verb>` token. Map the expected verbs VERBATIM from `bashSteerMessage` (`nudge.go:180-206`) and `steerMessage` (`nudge.go:158-174`):

| Shape | Emitted token to assert | `nudge.go` source line |
|-------|-------------------------|------------------------|
| `grep "func Foo" main.go` (Bash) | `helix search-symbols` | `:203-204` |
| `grep -r "Bar(" internal/x.go` | `helix find-references` | `:199-201` |
| `sed -i 's/a/b/' pkg/s.go` | `helix replace-in-file` | `:194-195` |
| `cat internal/edit.go` | `helix read-file` | `:191-192` |
| `find . -name '*.go'` | `helix find-files` | `:189` |

**Empty-bucket guard (Phase 87 CR-01 defect):** `require.GreaterOrEqual(t, len(cases), 5)` with a comment naming the 5 shapes; assert per-shape, never an empty-iteration pass.

**Negative control** — already exists, fold into the golden: `TestNudgeAdvisory_BashNonCodeGrep_Silent` (`nudge_test.go:348-356`), `grep TODO README.md` → no advisory.

**MANDATORY revert-and-fail** (`TestNudgeGoldenRevertFails`): assert that a deliberately-wrong expected verb (e.g. `cat x.go` → expect `helix rename-symbol`) makes the golden FAIL. Proves the golden keys on the SPECIFIC verb, not on `Contains("helix")`.

---

### `Makefile` (MODIFY — add `verify-reference`)

**Analog: `verify-cligen` / `verify-docs`** (`Makefile:80-85`) — mirror exactly:
```makefile
verify-cligen: ## HARD-FAIL drift gate: internal/cli/verbs_gen.go must match the live tool registry (VERB-02)
	$(GO) run ./cmd/helix-cligen --check

verify-docs: ## HARD-FAIL drift gate: README.md tool table must match the live registry (DOCS-02)
	$(GO) run ./cmd/docgen --check
```
Add:
```makefile
verify-reference: ## HARD-FAIL drift gate: internal/cli/skills/helix/reference.md must match the live registry (REF-03)
	$(GO) run ./cmd/helix-refgen --check
```
Also add a `reference:` regen target mirroring `docs:` (`Makefile:77-78`).

---

### `.github/workflows/go-test.yml` (MODIFY — add refgen drift-gate step)

**Analog: cligen/docgen gate steps** (`.github/workflows/go-test.yml:102-112`):
```yaml
- name: helix-cligen drift gate (VERB-02)
  run: go run ./cmd/helix-cligen --check
- name: docgen drift gate (DOCS-02)
  run: go run ./cmd/docgen --check
```
Add a sibling step `helix-refgen drift gate (REF-03)` → `run: go run ./cmd/helix-refgen --check`.

---

### `internal/cli/setup_clients.go` (LIKELY NO CHANGE)

**Self-analog: the install call site** (`setup_clients.go:158-163`):
```go
if !cfg.NoSkill {
    if err := installSkill(skillDir); err != nil {
        return fmt.Errorf("installing skill: %w", err)
    }
    cfg.Printer.Success("installed Helix skill to %s/SKILL.md", skillDir)
}
```
`installSkill` already takes only `skillDir` and now walks the FS internally — the call site needs NO change. The DryRun/Success messages (`:144`, `:162`) say "SKILL.md"; optionally update copy to "skill bundle (SKILL.md + reference.md)" for accuracy, but this is cosmetic, not load-bearing.

## Shared Patterns

### Registry-as-source + `--check` drift gate
**Source:** `cmd/docgen/main.go:56-92`, `cmd/helix-cligen/main.go:41-84`
**Apply to:** `cmd/helix-refgen/main.go`
The committed-artifact + `os.ReadFile`/string-compare/`os.Exit(1)` triad shipped twice already. Clone it; never hand-roll a comparison.

### Blank-import parity with the daemon
**Source:** `cmd/docgen/main.go:21-46` ≡ `internal/daemon/imports.go:15-24`
**Apply to:** `cmd/helix-refgen/main.go`
Copy the docgen block VERBATIM (it already includes health+help). The `--check` gate + ADOPT-01a (sourced from `VerbToolNames()`'s 50) catch any divergence. NEVER blank-import `internal/semantic/extract/*` (D-02).

### Authority = `VerbToolNames()`, never the generator's own output
**Source:** `internal/cli/verb.go:81-88`
**Apply to:** `reference_contract_test.go` (ADOPT-01a) and `cmd/helix-refgen` arg-recovery accessor
The 50 frozen verbs are the single source of truth for completeness. Source per-verb args from `verbSpecs` via a fresh-copy accessor; source the completeness floor from `VerbToolNames()`.

### Path-contained atomic file write
**Source:** `internal/cli/skill.go:131-205` (`withinSkillRoot` + temp+rename)
**Apply to:** the new multi-file `installSkill` loop
Security-reviewed (T-93-01); entry names from the embed FS are never user-controlled. Keep the guard and per-file atomic write intact.

### Nudge capture in tests
**Source:** `runNudgeCapture`/`parseAdvisory` (`nudge_test.go:276-332`)
**Apply to:** the ADOPT-01b golden in `nudge_test.go`
Already redirects stdin/stdout and isolates per-test session-stats; do not write a new harness.

### Anti-vacuity (revert-and-fail, keyed on emitted command, non-empty bucket)
**Source:** PITFALLS Pitfalls 1-3; precedent `TestHelixSymbolicTools_NoDrift`
**Apply to:** BOTH new/extended test files
Two non-negotiable revert-and-fail tests (completeness + nudge golden); key nudge detectors on the emitted `helix <verb>` token (not `Contains("helix")`); assert `len(cases) >= 5` covering grep/grep-r/sed-i/cat/find.

## No Analog Found

None. Every file in this phase has a strong in-tree analog (this is a glue-existing-machinery phase).

## Metadata

**Analog search scope:** `cmd/docgen/`, `cmd/helix-cligen/`, `internal/cli/` (skill.go, verb.go, verbs_gen.go, nudge.go, nudge_test.go, skill_test.go, setup_clients.go), `Makefile`, `.github/workflows/go-test.yml`
**Files scanned:** 11
**Pattern extraction date:** 2026-06-22
