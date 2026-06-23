# Phase 101: Opt-In LLM-Behavioral Adoption Scorecard - Context

**Gathered:** 2026-06-23
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

An opt-in, build-tag-gated LLM-behavioral scorecard measures an agent's helix-choice rate and standard-tool fallback rate using the reused v1.4 `llm`/`llmjudge` harness, with built-in failing anchors (sabotaged-skill revert-and-fail + negative judge exemplar) so the score can demonstrably fail — and it never blocks merge.

**Requirements:** ADOPT-02

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — discuss phase was skipped per user setting. Use ROADMAP phase goal, success criteria, and codebase conventions to guide decisions.

Key constraints carried from research (SUMMARY.md / PITFALLS.md) + Phases 97/98:
- **Reuse the v1.4 harness at `test/oracle/llm/`** (client.go, prompt.go, transcript.go, the multi-provider Anthropic+DeepSeek client, judge scoring infra). Build-tag gated (`llm` / `llmjudge`) — it MUST NOT run in the default `go test ./...` suite and NEVER blocks merge (LLM nondeterminism). This is the ADOPT-02 layer that complements the deterministic ADOPT-01 merge gate from Phase 97.
- **The scorecard measures two metrics:** helix-choice rate (does the model pick `helix <verb>` for a code-semantic task) and standard-tool fallback rate (does it reach for grep/sed/cat/Read). They should be complementary on a known fixture.
- **Anti-vacuity is the dominant risk (research flag — this phase needs care):**
  - **Detector keys on the FIRST emitted command line** (the model's actual chosen action), NOT substring presence of "helix" anywhere in the transcript (Pitfall 1).
  - **Sabotaged-skill revert-and-fail self-test (MANDATORY, the hermetic anchor):** run the scorer against a skill body with the decision matrix STRIPPED and assert `choice_rate` drops materially vs the intact skill. If the score is identical with and without the skill, the test FAILS. This is the deliberate break-the-invariant proof — and it should be as HERMETIC as possible (deterministic given fixed/recorded transcripts), so the non-vacuity proof does not itself require a live API key.
  - **Negative judge exemplar:** the judge rubric includes an explicit response that runs `grep -r` to find a definition and scores 0 on adoption — so the rubric can return a FAILING score (it isn't trivially-always-pass).
  - **Reject empty/one-element task buckets** as a pass.
- **Hermetic vs live split:** the live LLM run needs API keys and is opt-in/never-gating. But the SCORER logic (parse transcript → first-command detector → choice/fallback classification → rubric) must be unit-testable HERMETICALLY against recorded/fixture transcripts so the sabotaged-skill revert-and-fail and the negative-exemplar tests run without a network/key. Mirror how the existing `test/oracle/llm` tests gate live vs fixture.
- Depends on Phase 97 (loads the skill body / reference) and Phase 98 (the deterministic contract + steering are green first). The scorecard loads the real embedded SKILL.md / reference and the nudge.
- Zero new deps (reuse anthropic-sdk-go already present). Respect existing build-tag conventions.

</decisions>

<code_context>
## Existing Code Insights

Codebase context will be gathered during plan-phase research. Relevant existing files: `test/oracle/llm/` (client.go multi-provider, prompt.go, transcript.go, selection_test.go, disambiguation_test.go, interpretation_test.go, skill_trigger_test.go, testdata/, doc.go) — the v1.4 behavioral harness + judge scoring; `internal/cli/skill.go` (EmbeddedSkillBody / EmbeddedReference from Phases 97/98); `internal/cli/nudge.go`; the build-tag taxonomy (`//go:build llm` / `llmjudge`) and how `test/harness/` gates them.

</code_context>

<specifics>
## Specific Ideas

No specific requirements beyond the ROADMAP success criteria — discuss phase skipped. Refer to ROADMAP Phase 101 description and its 3 success criteria.

</specifics>

<deferred>
## Deferred Ideas

None — discuss phase skipped.

</deferred>
