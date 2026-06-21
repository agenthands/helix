---
phase: 93-skill-md-nudge-repurpose-helix-setup-flip
plan: 04
subsystem: skill-validation
tags: [skill, behavioral-oracle, llm-test, token-cost, TEST-03, SKILL-04]
requires:
  - "93-01: embeddedSkillMD + installSkill (the SKILL.md asset under test)"
  - "test/oracle/llm harness: SkipWithoutAPIKey, AskSingleTurn, WriteTranscript, FormatToolList, StartRunner"
provides:
  - "cli.EmbeddedSkillBody() — exported single-source-of-truth accessor for the embedded SKILL.md body"
  - "cli.skillDescription() — frontmatter description accessor (idle-cost payload)"
  - "Non-gated SKILL-04 idle-cost bound proof (<=1536-char cap) + filled SKILL.md token-note"
  - "TEST-03 skill-vs-grep-baseline tool-selection oracle (build tag llm, key-gated)"
affects:
  - "internal/cli (skill.go accessors), test/oracle/llm (prompt.go + new oracle test)"
tech-stack:
  added: []
  patterns:
    - "stdlib-only frontmatter parse (no YAML dep) — zero-dep invariant held"
    - "build-tag llm + SkipWithoutAPIKey hermetic gating (never a vacuous pass)"
    - "baseline-vs-treatment A/B prompt comparison with aggregate (not per-task) assertion"
key-files:
  created:
    - test/oracle/llm/skill_trigger_test.go
    - test/oracle/llm/testdata/transcripts/skill-baseline-{0..7}.json
    - test/oracle/llm/testdata/transcripts/skill-with-{0..7}.json
    - test/oracle/llm/testdata/transcripts/skill04-blob-sizes.json
  modified:
    - internal/cli/skill.go
    - internal/cli/skill_test.go
    - internal/cli/skills/helix/SKILL.md
    - test/oracle/llm/prompt.go
decisions:
  - "Recorded the FormatToolList brief-description blob (2467 bytes / 39 tools) as the preloaded 'before' figure and labelled it honestly as a conservative lower bound (live tools/list also ships per-tool JSON input schemas not counted)."
  - "Aggregate assertion (majority helix + >=1 grep->helix shift) instead of per-task to tolerate single-call noise (D-19)."
metrics:
  duration_seconds: 310
  tasks_completed: 2
  files_created: 17
  files_modified: 4
  completed: 2026-06-21
status: complete
---

# Phase 93 Plan 04: Skill Empirical Validation + Token Rationale Summary

Extended the existing `test/oracle/llm/` behavioral harness with a skill-vs-grep-baseline tool-selection oracle (TEST-03) and backed the SKILL-04 idle-cost claim with a dependency-free non-gated proof plus a filled, measured token-note in SKILL.md.

## What was built

**Task 1 — SKILL-04 dependency-free idle-cost bound + filled token-note** (commit `1cdd6b4c`)
- Added `skillDescription() (string, error)` to `internal/cli/skill.go`: a stdlib-only `---`-fence frontmatter parser (single-line + `>-`/`>`/`|`/`|-` block scalars) returning the `description` (+ optional `when_to_use`). This is the idle-skill-cost payload — the only text Claude Code keeps in context until the skill triggers. No YAML dependency (`git diff go.mod` empty).
- Added three NON-gated tests (run in the default suite, no API key):
  - `TestSkillDescriptionAccessor` — accessor returns non-empty description.
  - `TestSkillIdleCostBound` — `len([]byte(description)) <= 1536` (the Claude Code listing cap; hermetic SKILL-04 proof). Measured actual = **599 bytes**.
  - `TestSkillTokenNoteFilled` — no `<N>`/`<M>` placeholders remain and a digit-bearing idle-cost figure is present.
- Filled the SKILL.md token-note with real measured numbers: idle **599 bytes** (≈150 tokens) vs preloaded MCP `tools/list` brief-description blob **2467 bytes / 39 tools** (≈617 tokens), with the 1536-char cap as the dependency-free upper bound.

**Task 2 — TEST-03 skill-vs-baseline behavioral oracle** (commit `9811473b`)
- Added `test/oracle/llm/skill_trigger_test.go` (`//go:build llm`, `SkipWithoutAPIKey(t)` first line) mirroring `selection_test.go`. For each of 8 helix-appropriate code tasks it asks the subject model twice — a grep/sed/cat baseline system prompt vs the same prompt with the SKILL.md body injected — and asserts the aggregate selection shift toward a `helix` verb (majority helix under the skill + ≥1 grep→helix shift). `WriteTranscript` records every outcome; `InterCallDelay()` between every call (D-19).
- Added `SkillSystemPrompt(skillBody)`, `GrepBaselineSystemPrompt()`, and 8 helix-verb task descriptions to `prompt.go`.
- Added exported `cli.EmbeddedSkillBody()` so the oracle loads the skill from the single embedded source (no inlined SKILL.md copy).
- When keyed, the test also records the SKILL-04 before/after blob sizes (`FormatToolList` preload vs embedded skill body) to a transcript.

## Gates: which RAN vs SKIPPED (honest)

| Gate | Status |
|------|--------|
| `go build ./...` | RAN — clean |
| `go vet ./...` (untagged) | RAN — clean |
| `go test ./internal/cli/... -count=1` (incl. non-gated SKILL-04 bound + no-placeholder token-note) | RAN — green |
| `go vet -tags llm ./test/oracle/llm/...` | RAN — clean (llm file compiles) |
| `go test -tags llm -run TestSkillVsBaseline ./test/oracle/llm/...` | **RAN (not skipped)** — a `DEEPSEEK_API_KEY` was present in the dev env, so the oracle executed against `deepseek-chat` |
| `git diff go.mod` / `git diff api/proto/` | RAN — both empty |

Honesty note on the llm gate: the harness is `SkipWithoutAPIKey`-gated and SKIPs hermetically in CI without a key. In this execution a DeepSeek key was set, so the test genuinely ran rather than skipping — the result below is a real keyed run, not a vacuous green. The non-gated SKILL-04 bound (`internal/cli`) holds regardless of any key.

## Empirical result (keyed run, deepseek-chat)

```
AGGREGATE: baseline chose grep on 8/8, skill chose helix on 8/8, shifted 8/8
SKILL-04: preloaded tools/list blob = 2467 bytes (39 tools), skill body (full) recorded; idle description = 599 bytes
```

Every one of the 8 code tasks moved from a grep/sed/cat/find/rg baseline command to a `helix <verb>` once the SKILL.md decision table was in context — e.g. baseline `rg -n 'Prompt'` → skill `helix search-symbols --query=Prompt`; baseline `sed -i …` → skill `helix replace-symbol-body --symbol-name=InterCallDelay`. Transcripts are committed under `test/oracle/llm/testdata/transcripts/skill-{baseline,with}-N.json`.

## Deviations from Plan

None — plan executed exactly as written. Two small in-spirit choices (both anticipated by the plan text):
- The plan said record the preloaded figure from "FormatToolList output … or record the char-count bound if no keyed run was performed." A keyed run WAS available, so the exact FormatToolList blob (2467 bytes / 39 tools) is recorded; it is labelled a conservative lower bound because the live `tools/list` also carries per-tool JSON input schemas that `FormatToolList` (name + brief description only) does not.
- Task 1 added its accessor (`skillDescription`) in production `skill.go` per the plan; the pre-existing `skill_test.go` already had a private `frontmatterValue` test helper from 93-01, left intact (no collision; distinct names).

## Threat surface

No new network endpoints, auth paths, or schema changes beyond the already-vetted behavioral-oracle → external LLM API boundary (T-93-10, mitigated by reusing `SkipWithoutAPIKey` which never logs the key). T-93-11 (vacuous-gate risk) is mitigated: SKILL-04 has a non-gated unit proof and TEST-03's skip is honest. No new packages (T-93-SC): `git diff go.mod` empty.

## Known Stubs

None.

## Self-Check: PASSED

- Files: `internal/cli/skill.go`, `internal/cli/skill_test.go`, `internal/cli/skills/helix/SKILL.md`, `test/oracle/llm/skill_trigger_test.go`, `test/oracle/llm/prompt.go` — all FOUND.
- Commits: `1cdd6b4c` (Task 1), `9811473b` (Task 2) — both present in `git log`.
