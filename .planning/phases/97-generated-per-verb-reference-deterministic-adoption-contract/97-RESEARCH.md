# Phase 97: Generated Per-Verb Reference + Deterministic Adoption Contract - Research

**Researched:** 2026-06-22
**Domain:** Go-native code-generation (registry-walk → committed artifact + `--check` drift gate), `embed.FS` skill bundling, deterministic adoption-contract testing (anti-vacuity)
**Confidence:** HIGH — every integration point, signature, and landmine below was read directly from in-tree source on the live `docs/readme-langsupport-lineage` branch (50-verb registry confirmed).

## Summary

Phase 97 adds a **generated per-verb `reference.md`** (REF-01) shipped alongside the terse `SKILL.md` via an `embed.FS` switch (REF-02), gated by a `helix-refgen --check` drift gate wired into `make` + CI (REF-03), plus a **deterministic, merge-gating adoption-contract test** (ADOPT-01) proving the reference covers every frozen verb and that the nudge steers each standard-tool shape to the specific correct `helix` verb. This is an additive layer on a mature surface: the registry-walk + `--check` pattern already exists twice (`cmd/docgen`, `cmd/helix-cligen`); the install + path-containment machinery already exists (`internal/cli/skill.go`); the nudge classifier and its golden tests already exist (`internal/cli/nudge.go` + `nudge_test.go`). The single structural code change is `embeddedSkillMD string` → `embed.FS`.

**One critical correction to upstream research:** ARCHITECTURE.md Pattern 1 and the phase brief both assert the reference args section comes from `help.ExtractParamDocs(tool.InputSchema)` driven off `skill.ToolProviders() → tool.InputSchema`. **`mcp.ToolDef` (what `ToolProvider.Tools()` returns) does NOT have an `InputSchema` field** (verified: `internal/mcp/registry.go:9-17`). `InputSchema` is only available from a fully-built `*SerenaMCPServer` via `CollectToolSchemas()` (`internal/mcp/server.go:264-269`), which only the daemon constructs (`internal/daemon/daemon.go:610`). The generator must therefore recover per-verb args **either** by reusing the already-committed `verbSpecs` in `internal/cli/verbs_gen.go` (simplest — flags carry name/toolArg/kind/required/help) **or** by re-running cligen's offline AST scan (`cmd/helix-cligen/scan.go`). This is the single highest-risk planning decision in the phase.

**Primary recommendation:** Build `cmd/helix-refgen` mirroring `cmd/docgen`/`cmd/helix-cligen` (same blank-import set, same `--check` exit-1 diff gate, same `os.WriteFile` regen path). Source the per-verb arg data from `internal/cli`'s already-generated `verbSpecs` (zero new AST machinery, already drift-gated by cligen) rather than `ExtractParamDocs`. Source the **completeness authority** for ADOPT-01 from `cli.VerbToolNames()` (`internal/cli/verb.go:81`), never from refgen's own output. Switch `skill.go` to `//go:embed skills/helix/*` and walk the FS in `installSkill`, keeping `withinSkillRoot`/atomic-write intact and the SKILL-04 idle-cost assertion reading `SKILL.md` only.

## User Constraints (from CONTEXT.md)

### Locked Decisions
Discuss phase was skipped (`workflow.skip_discuss`). All implementation choices are at Claude's discretion, constrained by the carried research below.

### Claude's Discretion
- The per-verb reference MUST be generated from the tool registry (reuse `cmd/docgen` walk + `internal/kernel/help`), NOT hand-written — inherit the docgen blank-import-parity-with-daemon rule and a `--check` drift gate.
- The one structural code change: `internal/cli/skill.go` `embeddedSkillMD string` → `embed.FS` so `installSkill` ships `reference.md` alongside `SKILL.md`, preserving `withinSkillRoot` containment + atomic write, SKILL-04 idle-cost bound still asserted on `SKILL.md` only.
- The adoption-contract completeness list MUST be sourced from `internal/cli/verbs_gen.go` (`VerbToolNames()`) as the authority — NOT the generator's own output (anti-vacuity Pitfall 1).
- Anti-vacuity mandatory: ship a deliberate break-the-invariant → assert-RED test (delete a verb from reference → completeness gate RED; break a nudge-shape mapping → contract RED). Reject empty-bucket-as-pass. Key nudge detectors on the emitted command, not substring.

### Deferred Ideas (OUT OF SCOPE)
None — discuss phase skipped. STEER-01/02/03, AGENT-01/02/03 (Phase 98), VENDOR/EDITBENCH/REPOEVAL/FUZZBENCH/BASELINE (Phases 99-102), ADOPT-02 LLM-behavioral scorecard (Phase 101) are all OUT OF SCOPE for Phase 97.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REF-01 | `cmd/helix-refgen` generates `reference.md` from the live registry covering every verb in `verbs_gen.go` with synopsis, args, output shape, worked example, "use this not that" guidance | Registry walk pattern: `cmd/docgen/main.go:109-135` (`skill.ToolProviders()` → `tp.Tools()`). Per-verb args source: `verbSpecs` in `internal/cli/verbs_gen.go` (name/toolArg/kind/required/help already present) OR cligen AST scan (`cmd/helix-cligen/scan.go`). NOTE: `tool.InputSchema` is NOT available from `ToolDef` (see Pitfall 1). |
| REF-02 | `skill.go` switches single embedded `string` → `embed.FS`; `installSkill` ships `reference.md` alongside `SKILL.md` atomically with path-containment | `internal/cli/skill.go:21-22` (`//go:embed skills/helix/SKILL.md` + `string`), `installSkill` `:131-156`, `withinSkillRoot` `:172-205`. Switch to `//go:embed skills/helix/*` + `embed.FS`, walk in `installSkill`. |
| REF-03 | `helix-refgen --check` merge-gating drift gate (make + CI), sourced from `verbs_gen.go` authority, inheriting docgen blank-import-parity rule | `--check` diff pattern: `cmd/docgen/main.go:79-86`, `cmd/helix-cligen/main.go:68-79`. Make targets: `Makefile:80-85` (`verify-cligen`/`verify-docs`). CI: `.github/workflows/go-test.yml:104-113`. |
| ADOPT-01 | Deterministic merge-gating test: (a) `reference ⊇ VerbToolNames()` completeness, (b) per-shape nudge-fires golden table mapping each standard-tool shape to the specific suggested verb — non-vacuous (revert-and-fail proven), keyed on emitted command, rejecting empty-bucket | Authority: `cli.VerbToolNames()` `internal/cli/verb.go:81-88`. Nudge mapping: `steerMessage`/`bashSteerMessage` `internal/cli/nudge.go:158-206`. Existing golden test scaffold: `nudge_test.go:334-422` (`runNudgeCapture`/`parseAdvisory`). Existing drift-gate precedent: `TestHelixSymbolicTools_NoDrift` `nudge_test.go:94-104`. |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Generate `reference.md` from registry | Build-time generator (`cmd/helix-refgen`) | — | Mirrors `cmd/docgen`/`cmd/helix-cligen`; blank-imports drive `init()` registration, same source as daemon |
| Embed + install skill bundle | CLI setup (`internal/cli/skill.go` + `setup_clients.go`) | — | Path-contained atomic install is an existing CLI concern; no daemon involvement |
| Steer standard tools → verbs (nudge) | CLI hook handler (`internal/cli/nudge.go`) | — | `helix nudge` is invoked by Claude Code PreToolUse; pure stdin→stdout, no daemon |
| Adoption-contract gate | Test tier (`internal/cli/*_test.go`) | CI/make | Deterministic, in-package test reading `VerbToolNames()` + reference bytes + nudge output |
| Verb authority list | Generated catalog (`internal/cli/verbs_gen.go` via `VerbToolNames()`) | — | The frozen 50-verb registry; the single source of truth for completeness |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `embed` | go 1.x (in tree) | Multi-file skill bundle | Zero-dep invariant (`git diff go.mod` must stay empty per milestone rule); already used pattern-adjacent |
| Go stdlib `flag` | in tree | `--check` / `--out` CLI flags on `helix-refgen` | Exact pattern in `cmd/docgen/main.go:57-59` and `cmd/helix-cligen/main.go:42-43` |
| `internal/skill` `ToolProviders()` | in tree | Registry walk source | `internal/skill/registry.go:47`; same call docgen + cligen use |
| `internal/cli` `VerbToolNames()` | in tree | Completeness authority | `internal/cli/verb.go:81-88`; sorted, fresh-copy, out-of-package safe |
| `internal/kernel/help` `FormatHelp`/`ExtractParamDocs` | in tree | Optional per-verb help rendering | `internal/kernel/help/help.go:21,71` — see Pitfall 1 caveat on `InputSchema` availability |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `golang.org/x/tools/go/packages` | in tree (already a dep) | AST scan of `*Args` structs for arg recovery | ONLY if reusing cligen's `scanToolArgs` directly (`cmd/helix-cligen/scan.go:51`) instead of `verbSpecs` |
| `github.com/stretchr/testify` | in tree | assert/require in contract tests | Already used throughout `nudge_test.go`, `skill_test.go` |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Sourcing args from `verbSpecs` (verbs_gen.go) | Building a `SerenaMCPServer` + `CollectToolSchemas()` to get real `InputSchema` | Server build pulls in the whole daemon graph; heavier, and `verbSpecs` is already drift-gated by cligen. Use `verbSpecs`. |
| New `cmd/helix-refgen` | Extending `cmd/docgen` to emit a 2nd artifact | Separate binary keeps the README generator single-purpose and gives `reference.md` its own `--check`; matches the cligen precedent (one generator per artifact). Recommended: new binary. |
| Generating into `internal/cli/skills/helix/reference.md` | A separate top-level docs dir | Must live under the embedded `skills/helix/` tree so `//go:embed skills/helix/*` picks it up and `installSkill` ships it. |

**Installation:** No new dependencies. (`go.mod` must stay unchanged — milestone zero-dep invariant.)

## Package Legitimacy Audit

Not applicable — Phase 97 installs **zero external packages**. All work uses Go stdlib (`embed`, `flag`) and in-tree packages already present (`internal/skill`, `internal/cli`, `internal/kernel/help`, `golang.org/x/tools` already in go.mod). The milestone invariant explicitly forbids new dependencies (REQUIREMENTS.md line 98, Out of Scope: "New Go dependencies").

## Architecture Patterns

### System Architecture Diagram

```
                  skill.ToolProviders()                cli.VerbToolNames()
                  (tool.Name, .Description,             (verbs_gen.go verbSpecs,
                   .HelpText, .BriefDescription)         50 frozen tool names)
                           │                                     │
                           │ blank-imports == daemon/imports.go  │ (authority)
                           ▼                                     │
            ┌──────────────────────────────┐                    │
            │  cmd/helix-refgen (NEW)       │                    │
            │   - walk providers            │                    │
            │   - per-verb args from        │                    │
            │     verbSpecs (NOT InputSchema)│                   │
            │   - render reference.md       │                    │
            │   - --check: exit 1 on diff   │                    │
            └──────────────┬───────────────┘                    │
                           │ writes (committed)                 │
                           ▼                                     │
   internal/cli/skills/helix/reference.md   internal/cli/skills/helix/SKILL.md (·unchanged)
                           │                          │          │
                           │   //go:embed skills/helix/*  (◆ string→embed.FS)
                           ▼                          ▼          │
            ┌──────────────────────────────────────────┐        │
            │ installSkill(targetDir)  (◆ walk FS)      │        │
            │  - withinSkillRoot guard (· unchanged)    │        │
            │  - atomic temp+rename per file (· pattern)│        │
            └──────────────┬───────────────────────────┘        │
                           ▼                                     │
        <.claude>/skills/helix/{SKILL.md, reference.md}          │
                                                                 │
   ┌─────────────────────────────────────────────────────────────────────────┐
   │  ADOPT-01 deterministic contract test (NEW, internal/cli/*_test.go)       │
   │   (a) referenceCovers(VerbToolNames())  ◄── authority, NOT refgen output  │
   │   (b) nudge-fires golden: shape → emitted `helix <verb>` (keyed on cmd)   │
   │   + revert-and-fail: drop a verb / break a mapping ⇒ RED                  │
   └─────────────────────────────────────────────────────────────────────────┘

   helix nudge (· internal/cli/nudge.go: steerMessage/bashSteerMessage, exit-0)
        ▲ asserted by the (b) golden table — read as DATA, mapping unchanged in P97
```

### Recommended Project Structure (delta only)

```
cmd/
└── helix-refgen/                ★ NEW — dedicated generator (mirror cmd/docgen)
    ├── main.go                      blank-imports == daemon/imports.go + docgen set
    └── main_test.go                 generator unit + --check round-trip

internal/cli/
├── skill.go                     ◆ embeddedSkillMD string → //go:embed skills/helix/* embed.FS
│                                   + EmbeddedSkillBody() still returns SKILL.md only
│                                   + skillDescription() reads SKILL.md entry from FS
│                                   + installSkill walks FS, writes each file atomically
├── skills/helix/
│   ├── SKILL.md                 · unchanged (terse idle tier)
│   └── reference.md             ★ NEW — generated per-verb reference (committed)
├── reference_contract_test.go   ★ NEW — reference ⊇ VerbToolNames() + revert-and-fail
└── nudge_test.go                ◆ add per-shape→verb golden table + revert-and-fail row

Makefile                         ◆ add verify-reference target (mirror verify-cligen)
.github/workflows/go-test.yml    ◆ add "helix-refgen drift gate" step (mirror cligen step)
```

### Pattern 1: Registry-as-source generation with a `--check` drift gate
**What:** A `cmd/*` generator blank-imports the daemon's tool-providing packages (firing `init()` registration), calls `skill.ToolProviders()`, renders a committed artifact, and `--check` exits 1 if the file would change.
**When to use:** `reference.md` (exactly this phase).
**Example (the exact `--check` shape to clone):**
```go
// Source: cmd/docgen/main.go:79-91 (and cmd/helix-cligen/main.go:68-83)
if *check {
    if result != string(content) {
        fmt.Fprintf(os.Stderr, "reference.md is out of date. Run 'go run ./cmd/helix-refgen' to regenerate.\n")
        os.Exit(1)
    }
    fmt.Println("reference.md is up to date.")
    return
}
if err := os.WriteFile(*out, []byte(result), 0644); err != nil { log.Fatalf(...) }
```
**Blank-import parity rule (load-bearing):** the generator's blank-import set must enumerate the same tool-providing packages as `internal/daemon/imports.go:15-24` (diag, edit, fileops, symbols, profile, skill/memory, skill/repomap, skill/semantic, skill/workflow) plus the non-blank health/help (docgen blank-imports `health`+`help` explicitly — `cmd/docgen/main.go:35-36`). Literal equality is NOT required; the `--check` gate is the real protection. Never blank-import `internal/semantic/extract/*` (double GrammarRegistry, D-02). MEMORY `helix-tool-docs-drift`: docgen once silently dropped tools because its imports diverged from the daemon's — copy the docgen/cligen import block verbatim.

### Pattern 2: `embed.FS` skill bundle + path-contained atomic install
**What:** Today `installSkill` (`skill.go:131`) writes ONE embedded string to `<targetDir>/SKILL.md` via temp+rename, guarded by `withinSkillRoot`. Switching to `embed.FS` means walking the FS and writing each entry.
**When to use:** Shipping `reference.md` alongside `SKILL.md` (REF-02).
**Example:**
```go
// Source: internal/cli/skill.go:21-22 (today) → proposed
//go:embed skills/helix/*
var embeddedSkillFS embed.FS

func installSkill(targetDir string) error {
    if !withinSkillRoot(targetDir) { /* unchanged guard, skill.go:132-134 */ }
    if err := os.MkdirAll(targetDir, 0755); err != nil { /* skill.go:136 */ }
    entries, _ := embeddedSkillFS.ReadDir("skills/helix")
    for _, e := range entries {
        data, _ := embeddedSkillFS.ReadFile("skills/helix/" + e.Name())
        // single-trailing-newline policy (skill.go:142-145), temp+rename (skill.go:147-153)
    }
}
```
**Ripple points (all small, all localized — verified):**
- `EmbeddedSkillBody()` (`skill.go:24-29`) must still return ONLY `SKILL.md` bytes (consumed by `test/oracle/llm/prompt.go:142` + `skill_trigger_test.go:91`). Read `skills/helix/SKILL.md` from the FS.
- `skillDescription()` (`skill.go:42-55`) and the frontmatter parsers (`skill.go:61-109`) operate on the SKILL.md string — feed them the `SKILL.md` FS entry, not the bundle.
- The SKILL-04 idle-cost test (`skill_test.go:103-148`, `TestSkillIdleCostBound`) reads `embeddedSkillMD` and asserts ≤1536 chars on the **description only** — keep it pointed at `SKILL.md`. `reference.md` is explicitly NOT bound by the 1536 cap (progressive disclosure: it loads on demand, not at idle).
- `skill_test.go` references `embeddedSkillMD` directly in ~12 places (lines 75,83,106,156,162,166,185,209,217,245 etc.) and `setup_test.go:590` — these need a `SKILL.md`-returning accessor or a renamed var. Plan a mechanical rename.

### Pattern 3: PreToolUse advisory envelope, exit-0 (unchanged in P97, asserted by ADOPT-01b)
**What:** `nudge.go` emits `{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":"…"}}` at exit 0 (`nudge.go:123-149`); `steerMessage`/`bashSteerMessage` map a tool/command shape to the advisory text (`nudge.go:158-206`).
**When to use:** ADOPT-01(b) asserts this existing mapping per-shape. Phase 97 does NOT broaden the classifier (that is STEER-01, Phase 98) — it only adds the golden contract over the current behavior.
**Trade-off:** The advisory text today names verbs in prose (e.g. "`helix search-symbols`"). The ADOPT-01(b) detector must key on the **emitted command token** (`helix <verb>`), not on substring "helix" — `nudge_test.go:344` currently asserts `Contains(..., "helix")`, which is exactly the weak assertion Pitfall 1 warns against. The new golden must assert the SPECIFIC verb per shape.

### Anti-Patterns to Avoid
- **Hand-writing `reference.md`** — drifts from the frozen 50-verb registry; the `--check` gate and ADOPT-01(a) become a stale duplicate. Generate it. (PITFALLS Pitfall 8, Anti-Pattern 1.)
- **Sourcing ADOPT-01(a) completeness from refgen's own output** — tautological set-compares-to-itself; deleting a verb can never turn it RED. Source from `VerbToolNames()`. (PITFALLS Pitfall 1.)
- **`Contains(output, "helix")` as the nudge assertion** — matches the prompt/skill text, not the chosen command. Key on the emitted `helix <verb>`. (PITFALLS Pitfall 1.)
- **Empty-bucket-as-pass** — a contract that iterates an empty shape set and reports 100% coverage. Assert a non-zero floor and fail on any zero bucket. (PITFALLS Pitfall 1, Phase 87 CR-01.)
- **Binding `reference.md` to the 1536-char SKILL-04 cap** — that cap is the idle-listing bound on the SKILL.md *description* only. (PITFALLS Pitfall 6, UX "fat reference".)

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Per-verb arg list | A new schema parser | `verbSpecs` in `internal/cli/verbs_gen.go` (name/toolArg/kind/required/help) | Already generated + drift-gated by cligen; `InputSchema` is NOT on `ToolDef` |
| `--check` diff gate | A bespoke comparison | `os.ReadFile` + string-compare + `os.Exit(1)` per `cmd/docgen/main.go:79-86` | Identical pattern shipped twice already |
| Registry enumeration | Reflecting on packages | `skill.ToolProviders()` → `tp.Tools()` (`cmd/docgen/main.go:109-118`) | Same source the daemon, docgen, cligen all read |
| Completeness authority | A second verb list | `cli.VerbToolNames()` (`internal/cli/verb.go:81`) | The frozen registry; out-of-package safe (returns a copy) |
| Atomic contained file write | A new writer | `installSkill`'s temp+rename + `withinSkillRoot` (`skill.go:131-205`) | Already security-reviewed (T-93-01, path-traversal guard) |
| Nudge capture in tests | A new harness | `runNudgeCapture`/`parseAdvisory` (`nudge_test.go:276-332`) | Already redirects stdin/stdout, isolates session-stats |

**Key insight:** Phase 97 is almost entirely *gluing existing, drift-gated machinery* — the only genuinely new code is the `helix-refgen` renderer body and the ADOPT-01 contract test. Everything else is a clone of `cmd/docgen` / `cmd/helix-cligen` / `installSkill` / `nudge_test.go`.

## Runtime State Inventory

> Phase 97 is greenfield-additive (a new generator, a new generated file, an embed-shape change, new tests). It introduces no rename of any stored key, service config, OS registration, secret, or installed artifact.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — no datastore key/collection touched | none |
| Live service config | None — no external service config carries a P97 string | none |
| OS-registered state | None — no Task Scheduler / pm2 / systemd registration changes | none |
| Secrets/env vars | None — no secret or env-var name introduced or renamed | none |
| Build artifacts | The committed `reference.md` is a NEW build artifact (generated, like `verbs_gen.go`/README tables). It must be regenerated and committed whenever a verb/description changes; the `--check` gate enforces freshness. No stale-artifact cleanup needed (the file does not exist yet). | Commit generated `reference.md`; wire `--check` |

**Note on `skill_test.go` references:** the `embeddedSkillMD` → `embed.FS` switch is a *code* rename within `internal/cli`, not a runtime-state migration. ~13 in-package test references (`skill_test.go`, `setup_test.go:590`) and 2 out-of-package consumers (`test/oracle/llm/{prompt.go:142,skill_trigger_test.go:91}` via `EmbeddedSkillBody()`) must compile against the new shape — a mechanical edit, verified by `go build ./...`.

## Common Pitfalls

### Pitfall 1: `tool.InputSchema` does not exist on `ToolDef` (the upstream-research error)
**What goes wrong:** Following ARCHITECTURE.md Pattern 1 / the phase brief literally — `help.ExtractParamDocs(tool.InputSchema)` over `skill.ToolProviders()` — fails to compile. `mcp.ToolDef` (`internal/mcp/registry.go:9-17`) has only `Name`, `Description`, `BriefDescription`, `HelpText`, `RegisterFn`. There is no `InputSchema`.
**Why it happens:** `InputSchema` lives on the SDK's `*mcpsdk.Tool`, surfaced only after registration via `SerenaMCPServer.CollectToolSchemas()` (`server.go:264-269`), which the daemon builds at `daemon.go:610`. `get_tool_help` gets schemas that way (`tools.go:68`), but that requires a live server.
**How to avoid:** Source per-verb args from the already-committed `verbSpecs` (`internal/cli/verbs_gen.go`) — each `verbFlag` carries `name`, `toolArg`, `kind`, `required`, `help` (`internal/cli/verb.go:44-54`). This is in-package (refgen would import `internal/cli` or, cleaner, refgen lives where it can read the catalog) and already drift-gated by `helix-cligen --check`. Use `tool.Description`/`tool.HelpText` from the provider walk for the synopsis/"use-this-not-that" prose. If real JSON-schema types are wanted, reuse cligen's offline AST scan (`cmd/helix-cligen/scan.go:51` `scanToolArgs`) — do NOT build a server.
**Warning signs:** A compile error `tool.InputSchema undefined`; or a plan task that constructs a `SerenaMCPServer` inside a generator.

### Pitfall 2: ADOPT-01(a) tautology — completeness sourced from refgen output
**What goes wrong:** The completeness test reads the verb list from the same generator that produced `reference.md`, so it compares a set to itself and can never go RED.
**Why it happens:** It's the path of least resistance — refgen already knows the verbs.
**How to avoid:** Source the authority from `cli.VerbToolNames()` (`verb.go:81`). Assert `for each name in VerbToolNames(): reference.md mentions the kebab verb (strings.ReplaceAll(name,"_","-"))`. Ship a revert-and-fail test: delete one verb's section from a copy of `reference.md` → assert the gate goes RED. The precedent is `TestHelixSymbolicTools_NoDrift` (`nudge_test.go:94-104`) which already cross-checks a hard-coded map against `VerbToolNames()`.
**Warning signs:** The completeness test still passes after you delete a verb section from the reference.

### Pitfall 3: Nudge golden keyed on "helix" substring, not the specific verb
**What goes wrong:** ADOPT-01(b) asserts `Contains(advisory, "helix")` (the current `nudge_test.go:344` shape) — passes even if the wrong verb is suggested, and would pass on the prompt's own text.
**Why it happens:** Copying the existing weak assertion.
**How to avoid:** A golden table mapping each shape to the SPECIFIC expected verb token, e.g. `grep "func X" main.go` → must contain `helix search-symbols`; `sed -i ... .go` → must contain `helix replace-in-file` or `helix fuzzy-edit`; `find . -name '*.go'` → `helix find-files`; `cat x.go` → `helix read-file`; `grep -r ... .go` → `helix find-references`. Map verbatim from `bashSteerMessage` (`nudge.go:180-206`). Include a negative-control row (`grep TODO README.md` → no advisory) — already covered by `TestNudgeAdvisory_BashNonCodeGrep_Silent` (`nudge_test.go:348-356`); fold it into the golden. Add a revert-and-fail row: break one mapping → assert the golden goes RED.
**Warning signs:** Any `!= ""` or `Contains(..., "helix")` in the ADOPT-01 suite.

### Pitfall 4: SKILL-04 idle-cost regression from the embed switch
**What goes wrong:** After the `embed.FS` switch, `skillDescription()`/`TestSkillIdleCostBound` accidentally read the bundle (SKILL.md + reference.md) and the 1536-char assertion explodes — or, worse, is silently repointed at `reference.md`.
**Why it happens:** The functions previously took the single `embeddedSkillMD` string; the switch must thread the SKILL.md-only bytes through.
**How to avoid:** Keep `skillDescription()` reading `skills/helix/SKILL.md` specifically; keep `TestSkillIdleCostBound` (`skill_test.go:141-148`) asserting on `SKILL.md`'s frontmatter description only. Do NOT count `reference.md` toward the idle bound (it is the on-demand tier). The measured idle cost today is 599 bytes (`SKILL.md` token-note, line ~155).
**Warning signs:** `TestSkillIdleCostBound` fails after the embed switch; or a description accessor that returns reference content.

### Pitfall 5: refgen blank-import set drifts from the daemon
**What goes wrong:** refgen imports a different provider set than `internal/daemon/imports.go`, so `reference.md` covers the wrong tools — the exact `helix-tool-docs-drift` MEMORY defect (docgen silently lost tools).
**Why it happens:** Copy-paste omission of one blank import.
**How to avoid:** Copy the blank-import block verbatim from `cmd/docgen/main.go:32-42` (which already includes health+help). The `--check` gate plus ADOPT-01(a) (sourced from `VerbToolNames()`, the true 50) catch divergence: if refgen covers 49, ADOPT-01(a) goes RED.
**Warning signs:** `reference.md` has a different verb count than `len(VerbToolNames())` (50).

## Code Examples

### Generator main skeleton (clone of docgen)
```go
// Source: cmd/docgen/main.go:56-92 + cmd/helix-cligen/main.go:41-83
func main() {
    out := flag.String("out", "internal/cli/skills/helix/reference.md", "path to reference.md")
    check := flag.Bool("check", false, "check mode: exit 1 if reference.md would change")
    flag.Parse()

    rendered := renderReference()            // walks skill.ToolProviders(), reads verbSpecs for args
    existing, err := os.ReadFile(*out)
    // ... (on first run, *out may not exist; handle gracefully) ...
    if *check {
        if rendered != string(existing) {
            fmt.Fprintf(os.Stderr, "reference.md is out of date. Run 'go run ./cmd/helix-refgen'.\n")
            os.Exit(1)
        }
        fmt.Println("reference.md is up to date."); return
    }
    if err := os.WriteFile(*out, []byte(rendered), 0644); err != nil { log.Fatalf("%v", err) }
}
```

### Completeness assertion (ADOPT-01a, authority = VerbToolNames)
```go
// Source pattern: nudge_test.go:94-104 (TestHelixSymbolicTools_NoDrift)
func TestReferenceCoversEveryVerb(t *testing.T) {
    ref := readEmbeddedReference(t) // skills/helix/reference.md bytes
    for _, toolName := range cli.VerbToolNames() {        // 50 frozen verbs = authority
        verb := strings.ReplaceAll(toolName, "_", "-")    // mechanical mapping, cmd/docgen/main.go:129
        assert.Containsf(t, ref, verb, "reference.md missing section for verb %q", verb)
    }
}
// revert-and-fail sibling: strip one verb's section from a copy → assert the check fails.
```

### Nudge per-shape golden (ADOPT-01b, keyed on emitted command)
```go
// Source: nudge_test.go:334-356 (runNudgeCapture/parseAdvisory) + nudge.go:180-206 (bashSteerMessage)
cases := []struct{ cmd, wantVerb string }{
    {`grep "func Foo" main.go`,       "helix search-symbols"},
    {`grep -r "Bar(" internal/x.go`,  "helix find-references"},
    {`sed -i 's/a/b/' pkg/s.go`,      "helix replace-in-file"},
    {`cat internal/edit.go`,          "helix read-file"},
    {`find . -name '*.go'`,           "helix find-files"},
}
for _, tc := range cases {
    out, err := runNudgeCapture(t, hookInput{ToolName: "Bash", ToolInput: map[string]any{"command": tc.cmd}})
    require.NoError(t, err)                              // exit-0 contract preserved
    adv, ok := parseAdvisory(t, out); require.True(t, ok)
    assert.Containsf(t, adv.HookSpecificOutput.AdditionalContext, tc.wantVerb,
        "shape %q must steer to %q, got %q", tc.cmd, tc.wantVerb, adv.HookSpecificOutput.AdditionalContext)
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Preloaded MCP `tools/list` schema blob (~2467 B across 39 tools) | On-demand terse `SKILL.md` (599 B idle) | v2.0 (Phase 93) | P97 adds the on-demand `reference.md` tier below SKILL.md — progressive disclosure, idle cost unchanged |
| Single embedded `string` skill | `embed.FS` multi-file bundle | THIS phase (REF-02) | Enables shipping reference.md + SKILL.md atomically |
| Hand-checked verb coverage | `VerbToolNames()`-authoritative drift gate | THIS phase (ADOPT-01) | Completeness becomes a merge gate |

**Deprecated/outdated:** The phase brief's `help.ExtractParamDocs(tool.InputSchema)`-from-`ToolProviders()` recipe is unbuildable as written (see Pitfall 1) — `ExtractParamDocs` is still valid but needs a schema source the providers don't expose.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Sourcing per-verb args from `verbSpecs` (verbs_gen.go) is acceptable in lieu of live `InputSchema` | Standard Stack / Pitfall 1 | If the planner wants full JSON-schema types (enums, descriptions) in the reference, `verbSpecs` lacks enum values and rich descriptions — would need cligen's AST scan or a server build. `verbSpecs` carries name/toolArg/kind/required/help, which covers REF-01's "args" requirement adequately. |
| A2 | A new `cmd/helix-refgen` binary is preferred over extending `cmd/docgen` | Alternatives Considered | Both satisfy REF-01/03; a separate binary is the cleaner precedent (cligen is separate from docgen). Low risk — either works. |
| A3 | `reference.md` is exempt from the 1536-char SKILL-04 cap | Pitfall 4 | The cap is explicitly the idle-listing bound on the SKILL.md description; reference.md is the on-demand tier. Confirmed by PITFALLS Pitfall 6 (progressive-disclosure contract). Low risk. |
| A4 | Phase 97 does NOT broaden the nudge classifier | Pattern 3 | Classifier broadening is STEER-01 (Phase 98). ADOPT-01(b) asserts the *current* mapping. If the planner reads ADOPT-01 as requiring new shapes, that would overlap Phase 98 — confirmed out of scope by traceability table (STEER-01 → Phase 98). |

## Open Questions

1. **Where does `cmd/helix-refgen` read `verbSpecs` from?**
   - What we know: `verbSpecs` is package-private to `internal/cli` (`verbs_gen.go:10`); `VerbToolNames()` is the only exported accessor and it returns names only, not flags.
   - What's unclear: To render the args section, refgen needs the flag detail. Options: (a) export a richer accessor from `internal/cli` (e.g. `VerbSpecsForDocs()`), (b) have refgen re-run cligen's AST scan, (c) keep the args section minimal (synopsis + worked example from `Description`/`HelpText`, args names only from a new accessor).
   - Recommendation: Add a small exported read-only accessor in `internal/cli` returning verb→(flags) for docs, mirroring `VerbToolNames()`'s fresh-copy discipline. Cleanest, no AST machinery, stays drift-gated by cligen.

2. **Does the reference render need real argument *descriptions* / enum values?**
   - What we know: REF-01 says "args" (names/types/required is the minimum). `verbSpecs` flag `help` carries the per-flag help string already.
   - What's unclear: Whether the worked example + "use this not that" must be hand-authored per verb or templated. Generated-from-registry forbids hand-authoring; a deterministic template keyed on group/verb is the safe interpretation.
   - Recommendation: Template the synopsis from `Description`, args from `verbSpecs` flags, and a generic worked example per `groupID` (navigation/edit/fileops/...). Keep it 100% generated so `--check` is meaningful.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | build, generate, test | ✓ (repo builds) | in tree | — |
| `golang.org/x/tools/go/packages` | optional AST-scan arg source | ✓ (already a dep, used by cligen) | go.mod | use `verbSpecs` instead |

No external services, network, or binaries required. Phase 97 is pure code + generated-file + test.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` + `github.com/stretchr/testify` (assert/require) |
| Config file | none — standard `go test` |
| Quick run command | `go test ./internal/cli/... ./cmd/helix-refgen/... -count=1` |
| Full suite command | `go test ./... -count=1` (plus `make vet`) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REF-01 | refgen renders a section for every verb | unit | `go test ./cmd/helix-refgen/... -run TestRenderCoversAllVerbs -count=1` | ❌ Wave 0 |
| REF-02 | `embed.FS` switch; `installSkill` writes BOTH files atomically + path-contained | unit | `go test ./internal/cli/... -run 'TestInstallSkill' -count=1` | ◐ extend `skill_test.go:234-313` |
| REF-02 | `EmbeddedSkillBody()` still returns SKILL.md only (no reference bleed) | unit | `go test ./internal/cli/... -run TestEmbeddedSkillBody -count=1` | ❌ Wave 0 |
| REF-02 | SKILL-04 idle-cost still ≤1536 on SKILL.md after switch | unit | `go test ./internal/cli/... -run TestSkillIdleCostBound -count=1` | ✅ `skill_test.go:141` |
| REF-03 | `helix-refgen --check` exits 1 on a stale/hand-edited reference | unit + make | `go run ./cmd/helix-refgen --check` ; `make verify-reference` | ❌ Wave 0 |
| ADOPT-01a | `reference ⊇ VerbToolNames()` (authority = registry) | unit | `go test ./internal/cli/... -run TestReferenceCoversEveryVerb -count=1` | ❌ Wave 0 |
| ADOPT-01a | revert-and-fail: drop a verb → completeness RED | unit (adversarial) | `go test ./internal/cli/... -run TestReferenceCompletenessRevertFails -count=1` | ❌ Wave 0 |
| ADOPT-01b | per-shape nudge → SPECIFIC verb golden, keyed on emitted command | unit | `go test ./internal/cli/... -run TestNudgeShapeGolden -count=1` | ◐ extend `nudge_test.go` |
| ADOPT-01b | negative control: `grep TODO README.md` does NOT fire | unit | `go test ./internal/cli/... -run TestNudgeAdvisory_BashNonCodeGrep_Silent -count=1` | ✅ `nudge_test.go:348` |
| ADOPT-01b | revert-and-fail: break a mapping → golden RED | unit (adversarial) | `go test ./internal/cli/... -run TestNudgeGoldenRevertFails -count=1` | ❌ Wave 0 |
| ADOPT-01b | empty-bucket guard: assert non-zero shape count | unit | folded into `TestNudgeShapeGolden` (`require.NotEmpty(cases)` + per-shape assert) | ❌ Wave 0 |

### Anti-Vacuity Architecture (mandatory — the v1.12 four-CRITICAL lesson)
Two revert-and-fail tests are non-negotiable for this phase:
1. **Completeness revert-and-fail (ADOPT-01a):** take the embedded `reference.md` bytes, delete one verb's section in-memory, run the completeness check against that mutated copy, assert it reports the missing verb / fails. This proves the gate is sourced from the registry (`VerbToolNames()`) and is NOT a set-compared-to-itself tautology. Pattern precedent: the project's repeated "revert-and-fail to prove non-vacuity" (PITFALLS, Phase 89 CR-01).
2. **Nudge-golden revert-and-fail (ADOPT-01b):** assert that a deliberately-wrong expected verb (e.g. expecting `cat x.go` → `helix rename-symbol`) makes the golden assertion fail. This proves the golden keys on the SPECIFIC verb, not on `Contains("helix")`.

**Empty-bucket rejection:** the nudge golden's case slice must be asserted non-empty AND every standard-tool shape class (grep / grep-r / sed-i / cat / find) must have ≥1 case — `require.GreaterOrEqual(len(cases), 5)` with a comment naming the 5 shapes. A 0/0 "all pass" is the Phase 87 CR-01 defect.

### Sampling Rate
- **Per task commit:** `go test ./internal/cli/... ./cmd/helix-refgen/... -count=1 && go run ./cmd/helix-refgen --check`
- **Per wave merge:** `make vet && go test ./... -count=1`
- **Phase gate:** full suite green + `make verify-reference` green + `go run ./cmd/helix-refgen --check` clean before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `cmd/helix-refgen/main.go` + `main_test.go` — covers REF-01, REF-03 (generator + `--check` round-trip)
- [ ] `internal/cli/reference_contract_test.go` — covers ADOPT-01a (completeness + revert-and-fail)
- [ ] Extend `internal/cli/nudge_test.go` — covers ADOPT-01b (per-shape golden + revert-and-fail + empty-bucket floor)
- [ ] Extend `internal/cli/skill_test.go` — covers REF-02 (install writes BOTH files; `EmbeddedSkillBody` returns SKILL.md only)
- [ ] `Makefile` `verify-reference` target + `.github/workflows/go-test.yml` drift-gate step — covers REF-03 CI wiring
- [ ] An exported read-only `internal/cli` accessor for verb→flags (Open Question 1) — needed if refgen renders the args section from `verbSpecs`

## Security Domain

> `security_enforcement` posture: this phase touches file-writing (skill install) and a code generator. ASVS V5 (Input Validation) and V12 (File/Resources) are the relevant categories; the existing path-traversal guard is the control.

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | yes | refgen reads only in-tree registry data (no untrusted input); nudge already parses stdin as DATA never exec (`nudge.go:65-70`, T-36-01) |
| V6 Cryptography | no | — |
| V12 File/Resources | yes | `withinSkillRoot`/`lexicalSkillRoot`/`containedIn` path-traversal guard (`skill.go:172-205`) — MUST remain intact across the `embed.FS` switch; the new multi-file write loop must still refuse escaping targets |

### Known Threat Patterns for this stack
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Skill install path traversal (`../../etc`) | Tampering | `withinSkillRoot` filepath.Rel guard (`skill.go:172`), unchanged; new write-loop writes only `<targetDir>/<entry>` for entries enumerated from the embedded FS (no user-controlled names) |
| Partial/corrupt skill file on concurrent setup | Tampering | atomic temp+rename per file (`skill.go:147-153` pattern), preserved per-file in the bundle loop |
| Nudge command injection via crafted Bash string | Tampering/Elevation | classifier tokenizes as DATA, never `os/exec` (`nudge.go:337` `classifyBashTarget`, T-93-05); unchanged in P97 |
| Generated `reference.md` injecting markup from a tool Description | Tampering | escape pipes / control chars when rendering Descriptions into markdown (docgen escapes pipes, `cmd/docgen/main.go:125`) — mirror that |

## Sources

### Primary (HIGH confidence — direct in-tree read)
- `internal/cli/skill.go:1-229` — embed shape, `installSkill`, `withinSkillRoot`, `skillDescription`, frontmatter parsers
- `internal/cli/nudge.go:1-442` — `steerMessage`, `bashSteerMessage`, `classifyBashTarget`, `emitAdvisory`, exit-0 contract, code/non-code extension tables
- `internal/cli/verb.go:44-106` — `verbFlag`/`verbSpec`/`VerbToolNames()`
- `internal/cli/verbs_gen.go:1-50+` — generated `verbSpecs` catalog (50 verbs, `toolName`/flags)
- `internal/cli/nudge_test.go:1-443` — `runNudgeCapture`/`parseAdvisory`, `TestHelixSymbolicTools_NoDrift`, classify-target goldens
- `internal/cli/skill_test.go` (grepped) — `TestSkillIdleCostBound` (`:141`), `embeddedSkillMD` references, install tests
- `internal/cli/setup_clients.go:130-176` — `installSkill` call site (ClaudeCodeRegistrar)
- `cmd/docgen/main.go:1-175` — registry walk + `--check` + blank-import parity rule + pipe-escape
- `cmd/helix-cligen/main.go:41-83` + `scan.go:14-68` — `--check` diff, AST `*Args` scan (`argInfo`/`argField`)
- `internal/kernel/help/help.go:21-102` — `ExtractParamDocs`/`FormatHelp` signatures
- `internal/kernel/help/tools.go:48-89` — how `get_tool_help` gets InputSchema (via `server.CollectToolSchemas()`)
- `internal/mcp/registry.go:9-17` — `ToolDef` fields (NO InputSchema)
- `internal/mcp/server.go:264-269` — `CollectToolSchemas()` (schema source requires built server)
- `internal/skill/skill.go:33-36`, `registry.go:47` — `ToolProvider` iface / `ToolProviders()`
- `internal/daemon/imports.go:15-24` — daemon blank-import set (parity authority)
- `Makefile:80-85,318-325` — `verify-cligen`/`verify-docs`/`verify-licenses` target shapes
- `.github/workflows/go-test.yml:104-113` — cligen/docgen drift-gate CI steps
- `internal/cli/skills/helix/SKILL.md` — terse skill body, decision matrix, 599 B idle-cost token note

### Secondary (this cycle's milestone research, HIGH)
- `.planning/research/ARCHITECTURE.md` (Patterns 1-2, build order) — corrected on `InputSchema` (Pitfall 1)
- `.planning/research/PITFALLS.md` (Pitfalls 1, 6, 8 — anti-vacuity, steering over-reach, scope/overlap)

### Project memory (HIGH)
- `helix-tool-docs-drift` — docgen blank-imports must == daemon's or generated docs silently drop tools

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every package/function read from source; zero new deps confirmed
- Architecture: HIGH — generator/embed/install/test seams all read directly; one upstream error corrected (InputSchema)
- Pitfalls: HIGH — anchored to named v1.12 CRITICALs + verified source signatures

**Research date:** 2026-06-22
**Valid until:** 2026-07-22 (stable; the registry is frozen at 50 verbs, the generator patterns are mature)
