# Phase 101: Opt-In LLM-Behavioral Adoption Scorecard - Research

**Researched:** 2026-06-23
**Domain:** LLM-behavioral evaluation harness reuse (Go, build-tag-gated, anti-vacuity) + a hermetic pure-scorer split
**Confidence:** HIGH — every claim below is anchored to a direct in-tree source read at `file:line`.

## Summary

Phase 101 adds ADOPT-02: an opt-in, build-tag-gated LLM-behavioral scorecard that measures **helix-choice rate** and **standard-tool fallback rate**, complementing the deterministic ADOPT-01 merge gate already shipped in Phase 97. Almost all the live-LLM plumbing already exists in `test/oracle/llm/` (the v1.4 harness): a multi-provider Anthropic+DeepSeek client that SKIPs cleanly without a key, a normalized `Transcript` JSON shape, a `firstCommandLine`/`mentionsHelix`/`mentionsGrepBaseline` detector triad, the `SkillSystemPrompt(skillBody)` / `GrepBaselineSystemPrompt()` before/after conditions, and an `llmjudge` rubric package (`test/oracle/judge/`) that scores transcripts on a 0/0.5/1.0 scale with a `ComputeVerdict` pass/soft_fail/fail engine. The genuinely new work is small: a **two-metric scorecard** (choice + fallback, complementary), a **sabotaged-skill revert-and-fail self-test**, a **negative judge exemplar**, and an **empty-bucket floor** — all keyed on the first emitted command, not substring presence.

The single dominant design decision — and the reason this phase was research-flagged — is the **hermetic/live split**. The entire `test/oracle/llm` package is invisible to `go test ./...` because every file (including `client.go`, `prompt.go`, `transcript.go`) carries `//go:build llm || llmjudge` (verified: `go list ./test/oracle/llm/...` → "matched no packages"). That correctly keeps the live, nondeterministic, key-gated, never-blocks-merge layer out of CI. But it means **the anti-vacuity proof itself must NOT live in that package**, or it would never run. The recommendation is therefore a **pure, build-tag-FREE `helixadopt` scorer package** (parse transcript → `firstCommand` detector → choice/fallback classify → bucket scorecard) that imports nothing tag-gated, ships **recorded/synthetic fixture transcripts**, and is exercised by `go test ./...`. The sabotaged-skill revert-and-fail and the negative-exemplar tests run against those fixtures with **no network and no API key**. The live `//go:build llm` leg becomes a thin adapter that calls the same pure scorer on freshly captured transcripts.

**Primary recommendation:** Create a non-tag-gated `test/oracle/adopt/` (package `adopt`) holding the pure scorer + fixtures + hermetic anti-vacuity tests (runs in `go test ./...`); add ONE new `//go:build llm` file in `test/oracle/llm/` that captures live transcripts and feeds them to the same pure scorer; extend `test/oracle/judge/rubric.go` with an `adoption` dimension + negative exemplar under `//go:build llmjudge`. Reuse `firstCommandLine`'s logic (lift it into the pure package). Zero new Go deps.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Parse transcript → first emitted command | Pure scorer pkg (no build tag) | — | Deterministic string logic; must run hermetically in `go test ./...` to prove non-vacuity |
| Classify choice (helix) vs fallback (grep/sed/cat) | Pure scorer pkg | — | Same — keyed on first command, not substring |
| Bucket aggregation + choice_rate/fallback_rate | Pure scorer pkg | — | Empty-bucket rejection lives here; complementarity assertion is a unit test |
| Sabotaged-skill body generation (strip matrix) | Pure scorer pkg (helper) | — | Deterministic text transform; the revert-and-fail anchor runs hermetically |
| Recorded/synthetic fixture transcripts | Pure scorer pkg `testdata/` | — | The sole authoritative hermetic proof (no key, no network) |
| Live model interrogation (Anthropic/DeepSeek) | `//go:build llm` leg | reuses `test/oracle/llm/client.go` | Needs API key; opt-in; never blocks merge |
| Judge rubric scoring (negative exemplar) | `//go:build llmjudge` (`test/oracle/judge`) | reuses `RubricPrompt`/`ComputeVerdict` | Judge is itself an LLM call; gated separately like the existing two-stage flow |
| Loading real SKILL.md / reference / nudge | `internal/cli` accessors (no build tag) | — | `EmbeddedSkillBody()` / `EmbeddedReference()` are plain exported funcs, importable anywhere |

## User Constraints

> Phase 101 CONTEXT.md was auto-generated (discuss skipped via `workflow.skip_discuss`). All implementation choices are Claude's discretion, constrained by the carried research below.

### Locked Decisions (from CONTEXT.md + REQUIREMENTS ADOPT-02 + STATE cross-cutting invariants)
- **Reuse the v1.4 harness at `test/oracle/llm/`** (client.go, prompt.go, transcript.go, judge infra). Build-tag gated (`llm` / `llmjudge`). MUST NOT run in default `go test ./...`. NEVER blocks merge (LLM nondeterminism).
- **Two metrics, complementary on a known fixture:** helix-choice rate + standard-tool fallback rate.
- **Anti-vacuity (the dominant risk):**
  - Detector keys on the **FIRST emitted command line**, not substring presence of "helix" anywhere.
  - **Sabotaged-skill revert-and-fail self-test (MANDATORY hermetic anchor):** strip the decision matrix → assert `choice_rate` drops materially vs the intact skill. Identical-with-and-without ⇒ FAIL.
  - **Negative judge exemplar:** a response that runs `grep -r` to find a definition scores 0 on adoption so the rubric can return a failing score.
  - **Reject empty/one-element task buckets** as a pass.
- **Hermetic vs live split:** the scorer logic must be unit-testable hermetically (no network/key); the live LLM run is opt-in/never-gating. Mirror how the existing `test/oracle/llm` tests gate live vs fixture.
- Depends on **Phase 97** (loads real embedded SKILL.md / reference) and **Phase 98** (deterministic contract + steering green). Loads the real embedded SKILL.md / reference / nudge.
- **Zero new deps** (reuse `anthropic-sdk-go` already present). Respect existing build-tag conventions.

### Claude's Discretion
- Package layout for the pure scorer; exact metric thresholds; fixture transcript count (above the empty-bucket floor); whether the sabotage helper strips by `## Decision matrix` heading or by table rows; whether the judge `adoption` dimension is added to the existing 5-dim `Score` or scored separately.

### Deferred Ideas (OUT OF SCOPE)
- None (discuss skipped). ADOPT-03 (continuous adoption telemetry) is a v2 requirement, explicitly out of this milestone.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| ADOPT-02 | Opt-in LLM-behavioral adoption scorecard (reuse v1.4 `llm`/`llmjudge` harness, build-tag gated, never blocks merge) measuring helix-choice rate + standard-tool fallback rate, with a sabotaged-skill revert-and-fail self-test and a negative judge exemplar so the score can actually fail. | Harness reuse map (`client.go`/`prompt.go`/`transcript.go`/`judge/*`); `firstCommandLine` detector at `skill_trigger_test.go:35`; `SkillSystemPrompt`/`GrepBaselineSystemPrompt` at `prompt.go:132-149`; `EmbeddedSkillBody()`/`EmbeddedReference()` at `skill.go:45,66`; `RubricPrompt`/`ComputeVerdict` at `rubric.go:62,104`; hermetic-split design (pure pkg, no build tag). |

## Standard Stack

### Core (all reused — ZERO new deps)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/anthropics/anthropic-sdk-go` | v1.35.0 | Live subject + judge model calls (Anthropic path) | Already in `go.mod:` (verified); the harness `client.go` uses it |
| `github.com/openai/openai-go` | v1.12.0 | DeepSeek OpenAI-compatible path | Already a dep; `askDeepSeek` (client.go:131) routes through it |
| `github.com/stretchr/testify` | v1.11.1 | `require`-based assertions | Used by every existing oracle test |
| stdlib `encoding/json`, `os`, `strings`, `testing` | — | Transcript marshal, fixture I/O, pure scorer | The pure scorer needs nothing else |

### Supporting (in-tree, reused)
| Component | Path | Purpose | When to Use |
|-----------|------|---------|-------------|
| `Transcript` struct + read/write | `test/oracle/llm/transcript.go:18,40,61` | Normalized recorded interaction (ScenarioID, System, UserPrompt, Response, StopReason) | Fixture shape for hermetic transcripts; live capture leg writes these |
| `firstCommandLine` / `mentionsHelix` / `mentionsGrepBaseline` | `test/oracle/llm/skill_trigger_test.go:35,25,52` | First-emitted-command detector triad | LIFT into the pure pkg (they are currently `//go:build llm`, so not importable hermetically) |
| `SkillSystemPrompt` / `GrepBaselineSystemPrompt` / `SkillTaskDescriptions` | `test/oracle/llm/prompt.go:143,132,114` | before/after system prompts + 8 code tasks | Live leg only (these are tag-gated); the sabotaged prompt is a new variant |
| `EmbeddedSkillBody()` / `EmbeddedReference()` | `internal/cli/skill.go:45,66` | Real embedded SKILL.md / reference.md bytes (NOT tag-gated) | Load the intact skill; sabotage = transform of these bytes |
| Judge `RubricPrompt` / `Score` / `ComputeVerdict` / `ScoreTranscript` | `test/oracle/judge/rubric.go:104,12,62`, `scorer.go:22` | LLM-judge scoring with 0/0.5/1.0 anchors + pass/soft_fail/fail | Add an `adoption` dimension + negative exemplar (llmjudge tag) |
| `SkipWithoutAPIKey` / `Provider` / `SubjectModel` / `AskSingleTurn` / `InterCallDelay` | `test/oracle/llm/client.go:50,37,73,94,160` | Clean key-gated skip + provider routing + rate-limit delay | Live leg only |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Pure non-tagged `adopt` package | A new `//go:build llm` test in `test/oracle/llm` | REJECTED: the whole package is excluded from `go test ./...` (verified), so the anti-vacuity proof would never run in CI — the exact vacuity this phase exists to kill |
| Lift `firstCommandLine` into pure pkg | `import` it from `test/oracle/llm` | Impossible: that file is `//go:build llm`; importing it forces the build tag and re-hides the scorer. Copy/move the ~15-line helper into the pure pkg (single source of truth there). |
| New `adoption` dimension on `Score` | Separate `AdoptionScore` struct | Either works; extending `Score` reuses `ComputeVerdict` but changes the 5-dim averages — a separate score keeps the existing aggregate intact. Discretion. |

**Installation:** None — every package is already in `go.mod`. (`go.mod` confirmed: `anthropic-sdk-go v1.35.0`, `openai-go v1.12.0`, `testify v1.11.1`.)

## Package Legitimacy Audit

> No external packages are installed by this phase (ZERO new deps — locked decision). All dependencies are already vendored in `go.mod`. No `npm`/`pip`/`cargo` legitimacy check applies (pure Go, existing deps).

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
                       ┌────────────────────────────────────────────────┐
                       │  internal/cli (NO build tag — importable)        │
                       │  EmbeddedSkillBody()  EmbeddedReference()         │
                       └───────────────┬──────────────────────────────────┘
                                       │ real skill bytes
                                       ▼
   ┌──────────────────────────── HERMETIC (go test ./...) ───────────────────────────┐
   │  test/oracle/adopt   (package adopt — NO //go:build tag)                         │
   │                                                                                  │
   │   firstCommand(resp) ──► classify(cmd) ─┬─► CHOICE  (helix <verb>)               │
   │        (lifted)                          └─► FALLBACK (grep/sed/cat/find/Read)    │
   │                                                                                  │
   │   Scorecard(buckets) ──► choice_rate, fallback_rate  (complementary; empty-floor)│
   │                                                                                  │
   │   sabotage(skillBytes) ──► strips "## Decision matrix" section                   │
   │                                                                                  │
   │   testdata/transcripts/{intact-*.json, sabotaged-*.json, negative-*.json}        │
   │                                                                                  │
   │   TestRevertAndFail:   score(intact) - score(sabotaged) >= materialDrop          │
   │   TestComplementary:   choice_rate + fallback_rate covers every classified case  │
   │   TestEmptyBucketRejected: zero-element bucket is an error, NOT choice_rate=1.0   │
   └──────────────────────────────────────────────────────────────────────────────────┘
                                       ▲ same pure scorer
                                       │
   ┌─────────────── LIVE / OPT-IN (//go:build llm — excluded from go test ./...) ──────┐
   │  test/oracle/llm/adoption_scorecard_test.go  (NEW)                                │
   │   SkipWithoutAPIKey(t)  ──► AskSingleTurn(intact skill) + AskSingleTurn(sabotaged)│
   │   ──► WriteTranscript(...) ──► adopt.Scorecard(...)  (reuses pure scorer)          │
   └──────────────────────────────────────────────────────────────────────────────────┘
                                       ▲ transcripts
                                       │
   ┌─────────────── JUDGE (//go:build llmjudge — excluded from go test ./...) ─────────┐
   │  test/oracle/judge/rubric.go (EXTEND)                                             │
   │   RubricPrompt + adoption dimension + NEGATIVE EXEMPLAR                            │
   │   ("a response running `grep -r` to find a def scores 0 on adoption")             │
   │   ScoreTranscript ──► ComputeVerdict (can now return fail on a grep response)      │
   └──────────────────────────────────────────────────────────────────────────────────┘
```

### Recommended Project Structure
```
test/oracle/
├── adopt/                          # NEW — package adopt, NO build tag (runs in go test ./...)
│   ├── scorecard.go                # firstCommand, classify, Scorecard, sabotage helpers (pure)
│   ├── scorecard_test.go           # hermetic: revert-and-fail, complementary, empty-bucket
│   └── testdata/transcripts/       # COMMITTED recorded/synthetic fixtures (intact/sabotaged/negative)
├── llm/                            # EXISTING — //go:build llm || llmjudge
│   └── adoption_scorecard_test.go  # NEW — //go:build llm; live capture leg, feeds adopt.Scorecard
└── judge/                          # EXISTING — //go:build llmjudge
    └── rubric.go                   # EXTEND — adoption dimension + negative exemplar
```

### Pattern 1: Pure scorer, build-tag-free, fixture-driven (the hermetic anchor)
**What:** A package with NO `//go:build` line so it compiles and runs under `go test ./...`. It depends only on stdlib + `internal/cli` (which is tag-free). It reads COMMITTED fixture transcripts and computes the scorecard deterministically.
**When to use:** For every non-vacuity assertion (revert-and-fail, complementarity, empty-bucket). This is the SOLE authoritative proof; the live leg is informational.
**Example:**
```go
// Source: lifted from test/oracle/llm/skill_trigger_test.go:35-48 (firstCommandLine),
// moved into the NEW build-tag-free package so it runs in `go test ./...`.
package adopt

// FirstCommand returns the first non-empty command line with shell/fence
// decoration stripped (mirrors the "ONLY the single command line" contract).
func FirstCommand(response string) string {
	for _, raw := range strings.Split(response, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "```") {
			continue
		}
		line = strings.TrimSpace(strings.Trim(line, "`"))
		line = strings.TrimPrefix(line, "$ ")
		line = strings.TrimPrefix(line, "> ")
		return strings.TrimSpace(line)
	}
	return ""
}

func ClassifyChoice(response string) (chose, fellBack bool) {
	cmd := strings.ToLower(FirstCommand(response))
	chose = strings.HasPrefix(cmd, "helix ")
	for _, t := range []string{"grep ", "sed ", "cat ", "find ", "rg ", "ls "} {
		if strings.HasPrefix(cmd, t) { // PREFIX on the chosen command, not Contains anywhere
			fellBack = true
		}
	}
	return
}
```

### Pattern 2: Sabotage by section-strip (deterministic, mirrors Phase 97 revert-and-fail)
**What:** Produce a sabotaged skill body by deleting the `## Decision matrix` section from the real `EmbeddedSkillBody()`. Phase 97's `referenceMissingVerbs` proved the revert-and-fail pattern by factoring the gate into a pure helper run against intact vs mutated bytes (97-02-SUMMARY.md). Mirror it exactly.
**When to use:** The sabotaged-skill self-test. The intact and sabotaged bodies are fed to the SAME pure `Scorecard`; the test asserts a material drop.
**Example:**
```go
// Source: pattern from internal/cli/reference_contract_test.go (Phase 97 ADOPT-01a):
//   factor the transform into a pure helper, run identical scoring on intact vs mutated bytes.
// SKILL.md anchors the matrix with a literal "## Decision matrix" heading
// (verified internal/cli/skills/helix/SKILL.md:39).
func StripDecisionMatrix(skill string) string {
	const h = "## Decision matrix"
	i := strings.Index(skill, h)
	if i < 0 { return skill } // fixture guards against silent no-op (see warning sign)
	j := strings.Index(skill[i+len(h):], "\n## ")
	if j < 0 { return skill[:i] }
	return skill[:i] + skill[i+len(h)+j:]
}
```
> NOTE: assert in a test that `StripDecisionMatrix` actually shortened the body (`len(out) < len(in)`), or a future SKILL.md restructure silently makes the sabotage a no-op — re-introducing vacuity.

### Pattern 3: Empty-bucket rejection (Phase 87 CR-01 lesson, in code)
**What:** A scorecard over an empty (or sub-floor) task set must be an ERROR, never `choice_rate = 1.0` (0/0 treated as success). Phase 97's ADOPT-01b used `require.GreaterOrEqual(len(cases), 5)`; replicate the floor here.
**Example:**
```go
func Scorecard(buckets []Bucket) (Scorecard, error) {
	if len(buckets) < MinTasks { // MinTasks >= a defensible floor (e.g. 5)
		return Scorecard{}, fmt.Errorf("empty/under-sized task bucket: %d < %d", len(buckets), MinTasks)
	}
	// ... choice_rate = choices/total; fallback_rate = fallbacks/total
}
```

### Anti-Patterns to Avoid
- **Placing the scorer in `test/oracle/llm`:** the package is `//go:build llm || llmjudge`-only; `go test ./...` skips it entirely (verified). The anti-vacuity proof would never run.
- **`strings.Contains(response, "helix")`:** counts the injected SKILL.md's own text (which is FULL of the word "helix") as adoption. Always key on `FirstCommand(...)` prefix. (Pitfall 1; WR-93-05.)
- **A judge rubric with no negative anchor:** add the grep-scores-0 exemplar to `RubricPrompt`, or `ComputeVerdict` can never return `fail` for an adoption transcript.
- **0/0 = pass:** an empty bucket reporting `choice_rate=1.0` is exactly the Phase 87 CR-01 CRITICAL.
- **Committing live transcripts as goldens:** transcripts are run artifacts (`.gitignored` at `test/oracle/llm/testdata/transcripts/.gitignore`). The pure pkg's fixtures are DIFFERENT — they are hand-authored/recorded SYNTHETIC fixtures, committed on purpose as the hermetic spec. Keep them in `test/oracle/adopt/testdata/`, NOT the gitignored live dir.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| First-command extraction | A new regex parser | Lift `firstCommandLine` (skill_trigger_test.go:35) | Already handles fences, `$ `/`> ` prompts, backticks; battle-tested |
| Multi-provider model call + key skip | A new HTTP client | `AskSingleTurn` + `SkipWithoutAPIKey` (client.go:94,50) | Handles Anthropic+DeepSeek routing, MaxTokens, rate-limit delay, never logs the key (T-21-01) |
| Transcript shape + read/write | A new struct | `Transcript` + `ReadTranscript`/`WriteTranscript` (transcript.go) | Normalized, non-deterministic fields already stripped |
| Judge scoring + verdict thresholds | A new scorer | `ScoreTranscript` + `ComputeVerdict` (scorer.go:22, rubric.go:62) | 0/0.5/1.0 validation, retry-once on invalid, pass/soft_fail/fail thresholds |
| Loading the real skill/reference | Re-embedding the markdown | `EmbeddedSkillBody()`/`EmbeddedReference()` (skill.go:45,66) | Single source of truth; no drift from the `--check`-gated asset |
| before/after prompts | New prompt strings | `SkillSystemPrompt`/`GrepBaselineSystemPrompt` (prompt.go:143,132) | The sabotaged prompt is a 1-line variant of `SkillSystemPrompt(StripDecisionMatrix(body))` |

**Key insight:** The v1.4 harness was built precisely for this shape (TEST-03's `TestSkillVsBaseline` already measures a baseline-grep→skill-helix shift). Phase 101 is ~80% reuse + a hermetic re-home of the detector + a sabotage helper + a negative judge exemplar.

## Runtime State Inventory

> Not a rename/refactor/migration phase. This phase ADDS test code + fixtures only. No stored data, live-service config, OS-registered state, secrets, or build artifacts are renamed or migrated.
>
> **One adjacent caution (not runtime state, but a drift surface):** `StripDecisionMatrix` keys on the literal `## Decision matrix` heading in `internal/cli/skills/helix/SKILL.md:39`. If a future phase restructures SKILL.md, the sabotage could silently no-op. Mitigation: the `len(out) < len(in)` guard test (Pattern 2). **None found** in the other four categories — verified by direct read of the phase scope (test-only additions).

## Common Pitfalls

### Pitfall 1: The scorer lives where `go test ./...` can't see it (the meta-vacuity)
**What goes wrong:** The natural home for an LLM scorer is `test/oracle/llm`, but that whole package is `//go:build llm || llmjudge`. `go test ./...` reports it as "matched no packages" (verified). A revert-and-fail test placed there NEVER runs in CI — the gate is presumed broken (the v1.12 meta-lesson: four vacuous-pass CRITICALs all shipped because the adversarial test never ran on the default path).
**Why it happens:** The build tag is correct for the LIVE leg (key-gated, nondeterministic, never blocks merge). Authors assume the whole feature must share the tag.
**How to avoid:** Put the PURE scorer + fixtures + anti-vacuity tests in a build-tag-FREE package (`test/oracle/adopt`). Only the live capture leg and the judge extension carry build tags.
**Warning signs:** `go test ./...` passes in seconds and you cannot name a non-tagged test that ran the sabotaged-skill assertion.

### Pitfall 2: Substring "helix" counts the prompt's own text
**What goes wrong:** `SkillSystemPrompt` injects the full SKILL.md, which contains "helix" dozens of times. `strings.Contains(response, "helix")` is true even when the model emitted `grep`.
**How to avoid:** Classify on `FirstCommand(response)` PREFIX (`HasPrefix(cmd, "helix ")`), exactly as `mentionsHelix` does (skill_trigger_test.go:25-28).
**Warning signs:** choice_rate is suspiciously high and identical with and without the skill.

### Pitfall 3: Score identical with and without the skill (measures nothing)
**What goes wrong:** If the sabotaged and intact bodies produce the same `choice_rate`, the scorecard proves nothing.
**How to avoid:** The hermetic revert-and-fail uses RECORDED fixtures designed so the sabotaged transcripts emit grep and the intact transcripts emit helix — the test asserts `choiceRate(intact) - choiceRate(sabotaged) >= materialDrop`. (This is hermetic because the fixtures are committed, not live.)
**Warning signs:** the material-drop threshold is `>= 0.0`, or the sabotaged fixtures are copies of the intact ones.

### Pitfall 4: Empty/one-element bucket reported as a pass
**What goes wrong:** 0/0 → `choice_rate = 1.0`. Phase 87 CR-01 verbatim.
**How to avoid:** `Scorecard` returns an error below `MinTasks`; a `TestEmptyBucketRejected` asserts the error (not a 1.0).

### Pitfall 5: Negative judge exemplar missing → rubric can't fail
**What goes wrong:** The judge rubric has only positive anchors, so a grep response gets rubber-stamped.
**How to avoid:** Add an explicit `adoption` dimension to `RubricPrompt` with the anchor "0.0: chose grep/sed/cat/find for a code-symbol question (e.g. `grep -r 'func X'` to find a definition)" and a fixture transcript proving the judge returns 0/`fail` on it. `ComputeVerdict` already maps a zero to `fail`/`soft_fail` (rubric.go:90-98).

## Code Examples

### Hermetic revert-and-fail self-test (the phase's heart)
```go
// Source: pattern derived from internal/cli/reference_contract_test.go (Phase 97
// TestReferenceCompletenessRevertFails) — pure helper run on intact vs mutated bytes.
//go:build !ignore  // (no build tag at all — runs in `go test ./...`)
package adopt

func TestSabotagedSkillRevertAndFail(t *testing.T) {
	intact := loadFixtureBucket(t, "intact")       // committed transcripts: model chose helix verbs
	sabotaged := loadFixtureBucket(t, "sabotaged")  // committed transcripts: matrix stripped -> model grepped

	scIntact, err := Scorecard(intact)
	require.NoError(t, err)
	scSab, err := Scorecard(sabotaged)
	require.NoError(t, err)

	drop := scIntact.ChoiceRate - scSab.ChoiceRate
	require.GreaterOrEqual(t, drop, MaterialDrop, // e.g. 0.4 — NOT >= 0.0
		"choice_rate did not drop when the decision matrix was stripped (%.2f -> %.2f): scorecard measures nothing",
		scIntact.ChoiceRate, scSab.ChoiceRate)

	// complementarity guard: every classified transcript is either a choice or a fallback
	require.InDelta(t, 1.0, scIntact.ChoiceRate+scIntact.FallbackRate, 1e-9)
}
```

### Live capture leg (opt-in, reuses the pure scorer)
```go
// Source: structure from test/oracle/llm/skill_trigger_test.go:76 (TestSkillVsBaseline).
//go:build llm
package llm

func TestAdoptionScorecardLive(t *testing.T) {
	SkipWithoutAPIKey(t)            // FIRST line — hermetic skip without a key
	client := NewClient()
	model := SubjectModel()
	body := cli.EmbeddedSkillBody() // real skill
	sabotaged := adopt.StripDecisionMatrix(body)
	// ... AskSingleTurn(SkillSystemPrompt(body)) and AskSingleTurn(SkillSystemPrompt(sabotaged))
	//     per SkillTaskDescriptions(); WriteTranscript each; then adopt.Scorecard(...) on both.
}
```

### Negative judge exemplar (llmjudge)
```go
// Source: extend test/oracle/judge/rubric.go RubricPrompt() (rubric.go:104).
// Add to the rubric:
//   ### adoption
//   - 0.0: chose a standard tool (grep/sed/cat/find) as the FIRST command for a
//          code-symbol question — e.g. `grep -r "func NewClient"` to find a definition
//   - 0.5: chose a helix verb but a suboptimal one
//   - 1.0: chose the correct `helix <verb>` as the first command
// A committed fixture transcript whose Response is `grep -rn 'func NewClient' .`
// MUST score adoption=0.0 -> ComputeVerdict marks it fail/soft_fail.
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| TEST-03 `TestSkillVsBaseline` (live-only, aggregate shift) | + hermetic pure scorer with committed fixtures | Phase 101 | Anti-vacuity proof runs in `go test ./...`, not just when keyed |
| Single embedded SKILL.md `string` | `embed.FS` bundle (SKILL.md + reference.md) | Phase 97 (REF-02) | `EmbeddedReference()` now available alongside `EmbeddedSkillBody()` |
| Substring `mentionsHelix` concern | `firstCommandLine` prefix detector | Phase 93 (WR-93-05) | First-command keying is already the in-tree norm — lift it, don't reinvent |
| 5-dimension judge `Score` | + `adoption` dimension w/ negative exemplar | Phase 101 | Rubric can return a failing adoption score |

**Deprecated/outdated:**
- Committing transcripts to git: removed at WR-93-04 (`test/oracle/llm/testdata/transcripts/.gitignore` ignores `*`). The pure scorer's fixtures are a SEPARATE, intentionally-committed set under `test/oracle/adopt/testdata/`.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | A material-drop threshold around 0.4 is "material" for the revert-and-fail | Code Examples | LOW — the planner/author picks the exact floor; any value `> 0.0` with a real gap satisfies the invariant. Tune against the committed fixtures. |
| A2 | Extending the existing `Score` struct vs a separate adoption score is acceptable either way | Standard Stack alternatives | LOW — both compile; separate keeps the 5-dim aggregate averages unchanged. Discretion. |
| A3 | `MinTasks` floor of ~5 (mirroring Phase 97 ADOPT-01b) is defensible | Pattern 3 | LOW — the number is a planning choice; the invariant (reject empty/one-element) is what matters. |

**If this table is empty:** it is not — but every assumed item is a tunable threshold, not a structural claim. All structural claims (file paths, signatures, build tags, exclusion from `go test ./...`) are VERIFIED by direct read/`go list`.

## Open Questions

1. **Should the live leg capture into the gitignored `test/oracle/llm/testdata/transcripts/` or a new dir?**
   - What we know: live transcripts are gitignored run artifacts (WR-93-04). The pure pkg's fixtures must be committed.
   - What's unclear: whether the live leg should also write to the pure pkg's `testdata/` (it should NOT — that would mutate the committed hermetic spec).
   - Recommendation: live leg writes to the existing gitignored dir (reuse `WriteTranscript`); the pure pkg reads only its own committed `test/oracle/adopt/testdata/transcripts/`.

2. **Add the `adoption` dimension to the existing judge run, or a dedicated judge test?**
   - What we know: `TestJudge` (judge_test.go:19) scores ALL transcripts on 5 dims; adding a 6th dimension changes `aggregate.go` sums.
   - Recommendation: add `adoption` to `Score` + `ComputeVerdict` + `Aggregate` sums (judge_test.go + aggregate.go:41-64 list the 5 dims explicitly — extend all three), OR gate the adoption rubric behind a dedicated scenario filter. Discretion; the negative-exemplar fixture is the load-bearing part either way.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All | ✓ | per `go.mod` | — |
| `anthropic-sdk-go` | live leg (`//go:build llm`) | ✓ | v1.35.0 (go.mod) | — |
| `openai-go` (DeepSeek) | live leg | ✓ | v1.12.0 (go.mod) | — |
| `ANTHROPIC_API_KEY` / `DEEPSEEK_API_KEY` | live leg ONLY | ✗ (not in CI; not required) | — | `SkipWithoutAPIKey` skips cleanly; **the hermetic pure scorer needs NO key** and is the authoritative proof |

**Missing dependencies with no fallback:** none — the hermetic layer (the only CI-running layer) has zero external deps.
**Missing dependencies with fallback:** API keys — absent in CI by design; the live leg skips, the pure leg proves non-vacuity without them.

## Validation Architecture

> `workflow.nyquist_validation` is `true` in `.planning/config.json` — section included.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` + `testify/require` (v1.11.1) |
| Config file | none (standard `go test`) |
| Quick run command | `go test ./test/oracle/adopt/...` (hermetic, no key, runs in default suite) |
| Full suite command | `go test ./...` (includes the hermetic scorer; EXCLUDES `//go:build llm`/`llmjudge` by construction) |
| Live (opt-in) command | `ANTHROPIC_API_KEY=… go test -tags=llm ./test/oracle/llm/... -run AdoptionScorecard` then `-tags=llmjudge ./test/oracle/judge/...` |

### Phase Requirements → Test Map
| Req | Behavior | Test Type | Automated Command | File Exists? |
|-----|----------|-----------|-------------------|-------------|
| ADOPT-02 | First-emitted-command detector (not substring) | unit (hermetic) | `go test ./test/oracle/adopt/ -run TestFirstCommandNotSubstring` | ❌ Wave 0 |
| ADOPT-02 | Sabotaged-skill revert-and-fail: choice_rate drops when matrix stripped | unit (hermetic) | `go test ./test/oracle/adopt/ -run TestSabotagedSkillRevertAndFail` | ❌ Wave 0 |
| ADOPT-02 | choice_rate + fallback_rate complementary on a known fixture | unit (hermetic) | `go test ./test/oracle/adopt/ -run TestComplementary` | ❌ Wave 0 |
| ADOPT-02 | Empty/one-element bucket rejected (not choice_rate=1.0) | unit (hermetic) | `go test ./test/oracle/adopt/ -run TestEmptyBucketRejected` | ❌ Wave 0 |
| ADOPT-02 | `StripDecisionMatrix` actually shortens the real skill (no silent no-op) | unit (hermetic) | `go test ./test/oracle/adopt/ -run TestSabotageNonNoop` | ❌ Wave 0 |
| ADOPT-02 | Negative judge exemplar: grep response scores adoption=0 → fail | unit (llmjudge, hermetic on a fixture transcript IF judge is mockable; else opt-in) | `go test -tags=llmjudge ./test/oracle/judge/ -run AdoptionNegativeExemplar` | ❌ Wave 0 |
| ADOPT-02 | Scorecard NOT run by default suite's LLM leg / never blocks merge | meta-assert | `go list ./test/oracle/llm/...` returns no packages without `-tags`; live test carries `//go:build llm` | ✅ (verified now) |
| ADOPT-02 | Live capture leg skips cleanly without a key | gated | `go test -tags=llm ./test/oracle/llm/ -run AdoptionScorecardLive` (SKIPs) | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./test/oracle/adopt/...` (hermetic, < 1s, no key)
- **Per wave merge:** `go test ./... && go vet ./...` (the hermetic scorer is in the default suite; never blocks on the LLM legs)
- **Phase gate:** full hermetic suite green; `git diff go.mod` empty (zero new deps); live leg manually exercised by maintainer (informational, never gating).

### Wave 0 Gaps
- [ ] `test/oracle/adopt/scorecard.go` — `FirstCommand`, `ClassifyChoice`, `Scorecard`, `StripDecisionMatrix`, `Bucket`/`Scorecard` types (pure, no build tag) — covers ADOPT-02 detector + metrics
- [ ] `test/oracle/adopt/scorecard_test.go` — revert-and-fail, complementary, empty-bucket, sabotage-non-noop, first-command-not-substring
- [ ] `test/oracle/adopt/testdata/transcripts/{intact-*.json, sabotaged-*.json, negative-grep-*.json}` — committed synthetic/recorded fixtures (above the `MinTasks` floor)
- [ ] `test/oracle/llm/adoption_scorecard_test.go` — `//go:build llm` live capture leg feeding `adopt.Scorecard`
- [ ] `test/oracle/judge/rubric.go` — extend `RubricPrompt` with the `adoption` dimension + negative exemplar; extend `Score`/`ComputeVerdict`/`Aggregate` if the 6th dimension is added
- [ ] Decision: lift (move) `firstCommandLine`/`mentionsHelix`/`mentionsGrepBaseline` from `skill_trigger_test.go` into `adopt` and re-import in the live leg (single source of truth), OR keep a thin copy. (Lifting is cleaner; `skill_trigger_test.go` can then call `adopt.FirstCommand`.)

## Security Domain

> `security_enforcement` is not explicitly `false` in config; included per protocol. This phase adds test-only Go code with no new network surface, no new auth, no persistence, no user input parsing beyond reading committed fixture JSON.

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | API key is read by the EXISTING `client.go` via env, never logged (T-21-01, client.go:48-49); no new auth |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | yes (minor) | Fixture transcripts are committed JSON parsed via `encoding/json` into the typed `Transcript`; no untrusted external input |
| V6 Cryptography | no | — |

### Known Threat Patterns for this phase
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| API key leakage in logs/transcripts | Information disclosure | Reuse `SkipWithoutAPIKey` (never logs the key value, client.go:48-49); transcripts strip non-deterministic fields (transcript.go:18) |
| Vacuous gate shipped (the real risk) | (process) Repudiation of correctness | Hermetic revert-and-fail in the default suite; negative judge exemplar; empty-bucket rejection — all anchored to named v1.12 CRITICALs |
| Sabotage helper silent no-op after SKILL.md restructure | Tampering (latent) | `TestSabotageNonNoop` asserts `len(stripped) < len(intact)` |

## Sources

### Primary (HIGH confidence — direct in-tree read)
- `test/oracle/llm/client.go` (multi-provider client, `SkipWithoutAPIKey:50`, `AskSingleTurn:94`, `Provider:37`, `SubjectModel:73`, key envs `:17-34`) — all `//go:build llm || llmjudge`
- `test/oracle/llm/transcript.go` (`Transcript:18`, `WriteTranscript:40`, `ReadTranscript:61`, `TranscriptDir:74`)
- `test/oracle/llm/prompt.go` (`SkillSystemPrompt:143`, `GrepBaselineSystemPrompt:132`, `SkillTaskDescriptions:114`)
- `test/oracle/llm/skill_trigger_test.go` (`firstCommandLine:35`, `mentionsHelix:25`, `mentionsGrepBaseline:52`, `TestSkillVsBaseline:76`) — `//go:build llm`
- `test/oracle/llm/{selection,disambiguation,interpretation}_test.go` (scoring/gating patterns)
- `test/oracle/judge/{rubric,scorer,judge_test,aggregate}.go` (`RubricPrompt:104`, `Score:12`, `ComputeVerdict:62`, `ScoreTranscript:22`, `Aggregate:30`) — `//go:build llmjudge`
- `internal/cli/skill.go` (`embed.FS:21`, `EmbeddedSkillBody:45`, `EmbeddedReference:66` — NO build tag)
- `internal/cli/skills/helix/SKILL.md` (`## Decision matrix:39` heading — the sabotage anchor)
- `internal/cli/reference_contract_test.go` + `97-02-SUMMARY.md` (Phase 97 revert-and-fail pure-helper pattern, empty-bucket floor)
- `test/oracle/llm/testdata/transcripts/.gitignore` (transcripts are gitignored run artifacts) + `skill-baseline-0.json` (real recorded shape: `grep -rn 'func NewClient' ...`)
- `go.mod` (`anthropic-sdk-go v1.35.0`, `openai-go v1.12.0`, `testify v1.11.1` — zero new deps)
- **Verified by tooling:** `go list ./test/oracle/llm/...` → "matched no packages" (no tags); `go list -tags llm ./test/oracle/llm/...` → resolves — proving the package is excluded from `go test ./...`. `grep` confirmed NO `tags=llm`/`llmjudge` reference in `Makefile` or `.github/workflows/` (the scorecard does not run in CI).
- `.planning/{REQUIREMENTS.md (ADOPT-02), STATE.md (cross-cutting invariants), research/SUMMARY.md (Phase P2-1), research/PITFALLS.md (Pitfall 1)}`

### Secondary (MEDIUM confidence)
- Material-drop threshold (~0.4) and `MinTasks` floor (~5) — derived from Phase 97 ADOPT-01b's `>= 5` floor; exact values are tunable planning choices (A1, A3).

## Metadata

**Confidence breakdown:**
- Standard stack (reuse map): HIGH — every signature read at `file:line`; zero new deps confirmed against `go.mod`
- Architecture (hermetic/live split): HIGH — the `go list` exclusion proof is the load-bearing fact and was verified by tooling this session
- Pitfalls: HIGH — anchored to named v1.12 CRITICALs (Phase 86 substring, Phase 87 empty-bucket, Phase 89 revert-and-fail) and the in-tree `firstCommandLine` norm

**Research date:** 2026-06-23
**Valid until:** ~2026-07-23 (stable; the only churn risk is a SKILL.md restructure moving the `## Decision matrix` heading — guarded by `TestSabotageNonNoop`)
