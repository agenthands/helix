# Phase 101: Opt-In LLM-Behavioral Adoption Scorecard - Pattern Map

**Mapped:** 2026-06-23
**Files analyzed:** 5 new/modified
**Analogs found:** 5 / 5 (all exact or strong in-tree analogs; this phase is ~80% reuse)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `test/oracle/adopt/scorecard.go` (NEW, NO build tag) | pure scorer / utility | transform (parse→classify→aggregate) | `test/oracle/llm/skill_trigger_test.go:35` (`firstCommandLine`/`mentionsHelix`/`mentionsGrepBaseline`) + `internal/cli/reference_contract_test.go:34` (`referenceMissingVerbs` pure-helper) | exact (logic lifted) |
| `test/oracle/adopt/scorecard_test.go` (NEW, NO build tag) | test (hermetic) | request-response over fixtures | `internal/cli/reference_contract_test.go` (revert-and-fail on intact vs mutated bytes) | exact |
| `test/oracle/adopt/testdata/transcripts/*.json` (NEW, committed) | fixture | file-I/O | `test/oracle/llm/transcript.go:18` (`Transcript` shape) + `skill-baseline-*.json` run artifacts | role-match (committed, not gitignored) |
| `test/oracle/llm/adoption_scorecard_test.go` (NEW, `//go:build llm`) | test (live, opt-in) | request-response (live API) | `test/oracle/llm/skill_trigger_test.go:76` (`TestSkillVsBaseline`) | exact |
| `test/oracle/judge/rubric.go` (MODIFY, `//go:build llmjudge`) | rubric / scorer | transform | `test/oracle/judge/rubric.go:104` (`RubricPrompt`) + `:62` (`ComputeVerdict`) | exact (extension in place) |

## The Load-Bearing Constraint (read first)

The pure scorer + ALL anti-vacuity tests live in a **build-tag-FREE** package `test/oracle/adopt` (package `adopt`) so they RUN in `go test ./...`. The entire `test/oracle/llm` package carries `//go:build llm || llmjudge` on EVERY file (verified at `transcript.go:1`, `client.go:1`, `skill_trigger_test.go:1`) — `go list ./test/oracle/llm/...` returns "matched no packages" without a tag. A scorer placed there would never run in CI (Pitfall 1, the meta-vacuity).

**Detector lift (duplicate vs shared — decided: LIFT/MOVE, single source of truth):**
`firstCommandLine`, `mentionsHelix`, `mentionsGrepBaseline` are currently `unexported` funcs in the `//go:build llm` file `skill_trigger_test.go:25-60`. They CANNOT be imported across the build tag. The plan: **move** the logic into `adopt` as exported `FirstCommand` / `ClassifyChoice`, then have the live leg (`skill_trigger_test.go` and the new `adoption_scorecard_test.go`) call `adopt.FirstCommand(...)` instead of the local copies. This keeps ONE implementation. (A thin local copy is the fallback only if rewiring `skill_trigger_test.go` is deemed out of phase scope — but lifting is cleaner and is the research recommendation.)

## Pattern Assignments

### `test/oracle/adopt/scorecard.go` (pure scorer, NO build tag)

**Analog A — detector triad to LIFT** from `test/oracle/llm/skill_trigger_test.go:35-60`:

```go
// firstCommandLine: strips fences, leading "$ "/"> " prompt, backticks; returns
// the FIRST non-empty line. LIFT verbatim into adopt as exported FirstCommand.
func firstCommandLine(response string) string {
	for _, raw := range strings.Split(response, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "```") {
			continue
		}
		line = strings.Trim(line, "`")
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "$ ")
		line = strings.TrimPrefix(line, "> ")
		return strings.TrimSpace(line)
	}
	return ""
}

// mentionsHelix: PREFIX on the chosen command, NOT Contains anywhere (WR-93-05).
func mentionsHelix(response string) bool {
	cmd := firstCommandLine(response)
	return strings.HasPrefix(strings.ToLower(cmd), "helix ")
}
```

NOTE on the fallback detector: the existing `mentionsGrepBaseline` (skill_trigger_test.go:52-59) uses `strings.Contains(lower, tool)` — substring, NOT prefix. For the adoption scorer, classify fallback on the **FirstCommand prefix** (`HasPrefix(cmd, "grep ")`, etc.) per Pitfall 2, not the loose `Contains` form. Tool set to mirror: `{"grep ", "sed ", "cat ", "find ", "rg ", "ls "}` (skill_trigger_test.go:54).

**Analog B — pure-helper revert-and-fail factoring** from `internal/cli/reference_contract_test.go:34-46`:

```go
// referenceMissingVerbs is factored out so the revert-and-fail sibling runs the
// IDENTICAL logic against a mutated copy. Mirror this shape: one pure function,
// run on intact bytes AND on mutated (sabotaged) bytes.
func referenceMissingVerbs(refBytes string, authority []string) []string { ... }
```

**Core pattern — `StripDecisionMatrix`** (new helper). SKILL.md anchors the matrix with the literal heading at `internal/cli/skills/helix/SKILL.md:35` (`## Decision matrix`, followed by a table; next `## ` heading terminates the section). Load the real body via `cli.EmbeddedSkillBody()` (`internal/cli/skill.go:45`). Strip by heading index → next `\n## `. MUST be guarded by a `len(out) < len(in)` test against the real embedded body to prevent a silent no-op after a future SKILL.md restructure.

**Empty-bucket floor** — mirror `reference_contract_test.go`'s `require.Equal(t, 50, len(authority))` size anchor and the Phase 97 ADOPT-01b `>= 5` floor: `Scorecard(buckets)` returns an `error` (never `choice_rate=1.0`) when `len(buckets) < MinTasks`.

---

### `test/oracle/adopt/scorecard_test.go` (hermetic tests, NO build tag)

**Analog:** `internal/cli/reference_contract_test.go` — pure helper run on intact vs mutated bytes, with a size anchor.

Tests to write (from RESEARCH Test Map): `TestFirstCommandNotSubstring`, `TestSabotagedSkillRevertAndFail` (assert `ChoiceRate(intact) - ChoiceRate(sabotaged) >= MaterialDrop` where MaterialDrop ~0.4, NOT `>= 0.0`), `TestComplementary` (`require.InDelta(t, 1.0, ChoiceRate+FallbackRate, 1e-9)`), `TestEmptyBucketRejected` (asserts the error), `TestSabotageNonNoop` (`len(StripDecisionMatrix(EmbeddedSkillBody())) < len(EmbeddedSkillBody())`). Uses `testify/require`.

---

### `test/oracle/adopt/testdata/transcripts/*.json` (committed fixtures)

**Analog:** `Transcript` struct at `test/oracle/llm/transcript.go:18-27`:

```go
type Transcript struct {
	ScenarioID string `json:"scenario_id"`
	Category   string `json:"category"` // "selection", "disambiguation", "interpretation"
	Model      string `json:"model"`
	System     string `json:"system"`
	UserPrompt string `json:"user_prompt"`
	Response   string `json:"response"`
	StopReason string `json:"stop_reason"`
	SelfJudged bool   `json:"self_judged,omitempty"`
}
```

The `Response` field is the load-bearing one for classification. Intact fixtures: `Response` starts with `helix <verb>` (e.g. `helix go-to-definition ...`). Sabotaged fixtures: `Response` starts with `grep -rn 'func NewClient' .` (the real recorded shape lives in the gitignored `skill-baseline-0.json`). Negative fixture: a grep-first response for the judge exemplar.

**CRITICAL fixture-location divergence from the analog:** the live `test/oracle/llm/testdata/transcripts/` dir is **`.gitignored`** (`*` + `!.gitignore` + `!.gitkeep`) — those are run artifacts (WR-93-04). The `adopt` fixtures are the OPPOSITE: intentionally COMMITTED synthetic/recorded fixtures, the authoritative hermetic spec. Place them under `test/oracle/adopt/testdata/transcripts/` with NO `*`-ignore. The pure pkg reads ONLY its own committed dir; the live leg writes ONLY the gitignored dir. Count must exceed `MinTasks`.

---

### `test/oracle/llm/adoption_scorecard_test.go` (NEW, `//go:build llm`)

**Analog:** `test/oracle/llm/skill_trigger_test.go:76` (`TestSkillVsBaseline`) — copy the gating + capture loop.

Build-tag header + gate (from skill_trigger_test.go:1, :77, and prompt.go):

```go
//go:build llm

package llm
// ...
func TestAdoptionScorecardLive(t *testing.T) {
	SkipWithoutAPIKey(t)              // FIRST line — hermetic skip without a key (client.go:50)
	client := NewClient()             // client.go:67
	model := SubjectModel()           // client.go:73
	body := cli.EmbeddedSkillBody()   // skill.go:45 — real skill
	sabotaged := adopt.StripDecisionMatrix(body)
	for i, task := range SkillTaskDescriptions() {     // prompt.go:127
		InterCallDelay()                               // client.go:160
		resp, stop, err := AskSingleTurn(ctx, client, model,
			SkillSystemPrompt(body), SkillTaskUserPrompt(task)) // prompt.go:143/152, client.go:94
		require.NoError(t, err)
		WriteTranscript(t, &Transcript{...Response: resp, StopReason: stop...}) // transcript.go:40 (gitignored dir)
		// ... same for SkillSystemPrompt(sabotaged) ...
	}
	// feed captured transcripts to the SAME pure scorer:
	scIntact, _ := adopt.Scorecard(intactBuckets)
	scSab, _ := adopt.Scorecard(sabotagedBuckets)
}
```

The live leg is INFORMATIONAL (never blocks merge). The pure leg is the authoritative proof.

---

### `test/oracle/judge/rubric.go` (MODIFY, `//go:build llmjudge`)

**Analog:** the existing 5-dimension rubric IS the pattern. Two edit sites:

**1. `RubricPrompt()` at `rubric.go:104-141`** — add a 6th dimension block (mirror the existing `### tool_choice` anchor style at rubric.go:114-117) with a negative exemplar:

```
### adoption
- 0.0: chose a standard tool (grep/sed/cat/find) as the FIRST command for a
       code-symbol question — e.g. `grep -r "func NewClient"` to find a definition
- 0.5: chose a helix verb but a suboptimal one
- 1.0: chose the correct `helix <verb>` as the first command
```
Also extend the JSON response template at rubric.go:139-140 to include `"adoption"`.

**2. `Score` struct (rubric.go:12-23), `ValidateScoreValues` (:32-53), `ComputeVerdict` dims slice (:67-76)** — add the `Adoption float64` field + its validate branch + its `{"adoption", s.Adoption}` entry. `ComputeVerdict` ALREADY maps a zero to fail/soft_fail (rubric.go:90-98: `zeros >= 2 || Total < 2.5 → fail`; `zeros == 1 → soft_fail`), so a grep response scoring `adoption=0.0` flows through to a failing verdict with no threshold change. Per Open Question 2, also extend `aggregate.go` sums (the 5 dims are listed explicitly there) if the 6th dimension is added to the shared aggregate; OR score adoption via a dedicated scenario filter (discretion — the negative-exemplar fixture is load-bearing either way).

## Shared Patterns

### Single source of truth for the real skill body
**Source:** `internal/cli/skill.go:45` `EmbeddedSkillBody()` (and `:66` `EmbeddedReference()`) — plain exported funcs, NO build tag, importable anywhere.
**Apply to:** `adopt.StripDecisionMatrix` input, the live leg's intact prompt. Never re-embed or inline the markdown (skill_trigger_test.go:90-92 already enforces "no inlined copy").

### Build-tag taxonomy
**Source:** `client.go:1` / `transcript.go:1` = `//go:build llm || llmjudge`; `skill_trigger_test.go:1` = `//go:build llm`; `rubric.go:1` = `//go:build llmjudge`.
**Apply to:** the NEW live leg gets `//go:build llm`; the judge edit stays `//go:build llmjudge`; the `adopt` package gets NO tag.

### Key-gated clean skip
**Source:** `client.go:50` `SkipWithoutAPIKey(t)` — never logs the key (T-21-01), routes per provider (`Provider()` client.go:37).
**Apply to:** FIRST line of the live `TestAdoptionScorecardLive`.

### Pure-helper revert-and-fail
**Source:** `internal/cli/reference_contract_test.go:34` `referenceMissingVerbs` + size anchor `require.Equal(t, 50, len(authority))`.
**Apply to:** `adopt.Scorecard` run on intact vs sabotaged buckets; the `MaterialDrop` and `MinTasks` floors.

## No Analog Found

None. Every file has a strong in-tree analog. The only NEW shapes are the `Bucket`/`Scorecard` aggregation types and the `StripDecisionMatrix` text transform — both trivial stdlib `strings`/arithmetic, modeled on the Phase 97 pure-helper pattern.

## Metadata

**Analog search scope:** `test/oracle/llm/`, `test/oracle/judge/`, `internal/cli/` (skill + contract tests)
**Files scanned:** skill_trigger_test.go, transcript.go, client.go, prompt.go, rubric.go, skill.go, SKILL.md, reference_contract_test.go, testdata/transcripts/.gitignore
**Pattern extraction date:** 2026-06-23
