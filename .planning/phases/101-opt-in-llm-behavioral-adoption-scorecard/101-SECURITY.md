---
phase: 101
slug: opt-in-llm-behavioral-adoption-scorecard
status: secured
threats_open: 0
threats_closed: 9
asvs_level: 1
created: 2026-06-23
---

# SECURITY.md — Phase 101: Opt-In LLM Behavioral Adoption Scorecard

**Audit date:** 2026-06-23
**ASVS Level:** 1
**block_on:** high
**Disposition:** SECURED — 9/9 threats closed (7 mitigate, 2 accept), 0 open.

Retroactive verification of the threat register declared across `101-01-PLAN.md`
and `101-02-PLAN.md`. Each `mitigate` threat was verified by locating the named
test/assertion/build-tag and confirming it is load-bearing (not a green-path-only
or vacuous assertion). Each `accept` threat was verified by confirming its
accepted-risk premise still holds against the implementation.

Implementation files were treated as READ-ONLY; nothing was patched.

---

## Threat Verification

| Threat ID | Category | Disposition | Status | Evidence |
|-----------|----------|-------------|--------|----------|
| T-101-01 | Repudiation (of correctness) | mitigate | CLOSED | `test/oracle/adopt/scorecard_test.go:68-88` `TestSabotagedSkillRevertAndFail` asserts `drop >= MaterialDrop` (`scorecard.go:30` `MaterialDrop = 0.4`), not `>= 0.0`. Package is build-tag-FREE (`scorecard.go:14`), so it runs in `go test ./...` — verified `go list ./test/oracle/adopt/...` resolves the package with NO tags. **Flip-test proof:** forcing the 6 sabotaged fixtures to a helix choice (drop → 0.0) makes the test FAIL with "scorecard measures nothing: 1.00 -> 1.00, drop 0.00 < MaterialDrop 0.40"; fixtures restored, green re-verified. The gate genuinely discriminates. |
| T-101-02 | Tampering (latent) | mitigate | CLOSED | `scorecard_test.go:119-125` `TestSabotageNonNoop` asserts `len(StripDecisionMatrix(cli.EmbeddedSkillBody())) < len(cli.EmbeddedSkillBody())` against the REAL embedded body (`internal/cli` imported at `scorecard_test.go:11`). `StripDecisionMatrix` (`scorecard.go:111-122`) returns the input UNCHANGED when the `## Decision matrix` anchor is absent, so a future heading rename silently no-ops the strip — and this test turns RED. Verified passing in the default suite. |
| T-101-03 | Spoofing (false adoption) | mitigate | CLOSED | `ClassifyChoice` (`scorecard.go:85-95`) keys on `strings.HasPrefix(FirstCommand(...), "helix ")` / fallback prefixes — never `strings.Contains` over the whole response. `TestFirstCommandNotSubstring` (`scorecard_test.go:49-62`) proves a prose mention of "helix" followed by a `grep` first command classifies as fallback (not choice), and a `grep` command merely containing the word "helix" is not a choice. Verified passing. |
| T-101-04 | Elevation (0/0 = pass) | mitigate | CLOSED | `Scorecard` (`scorecard.go:129-132`) returns a zero result + non-nil error when `len(buckets) < MinTasks` (`MinTasks = 5`, `scorecard.go:24`). `TestEmptyBucketRejected` (`scorecard_test.go:101-114`) asserts nil / empty-slice / one-element buckets all error, the result is the zero value, and `ChoiceRate != 1.0`. `loadFixtureBucket` (`scorecard_test.go:30-32`) also guards against a deletion shrinking a bucket below the floor. Verified passing. |
| T-101-05 | Information disclosure (committed fixtures) | accept | CLOSED (premise holds) | All 12 committed fixtures under `test/oracle/adopt/testdata/transcripts/` contain only synthetic command strings + plausible constants (`model: "fixture"`, `category: "adoption"`). Secret-pattern scan (`sk-`, `api[_-]?key`, `secret`, `token`, `password`, `bearer`, `ANTHROPIC`, `DEEPSEEK`, 32+ char blobs) over all 12 fixtures returned NO matches. Tests are hermetic — no key required (verified: the 5 tests pass with no env key, no `-tags`). |
| T-101-06 | Repudiation (vacuous pass) | mitigate | CLOSED | Judge `adoption` dimension wired end-to-end: `Score.Adoption` (`rubric.go:18`), `ValidateScoreValues` branch (`rubric.go:50-52`), `ComputeVerdict` `dims` entry `{"adoption", s.Adoption}` (`rubric.go:80`), `RubricPrompt` `### adoption` anchor with grep-finds-a-definition negative exemplar (`rubric.go:144-147`) + `"adoption"` JSON key (`rubric.go:150`) + "6 dimensions" prose (`rubric.go:112`), and `aggregate.go` `sums`/`dims` entries (`aggregate.go:47,66,129`). `TestAdoptionNegativeExemplarVerdict` (`adoption_exemplar_test.go:22-68`) proves `adoption=0.0` → `soft_fail` (one zero) and `adoption+tool_choice=0.0` → `fail`, rejects `adoption=0.7`, and asserts the rubric carries the anchor/exemplar/key/count. Verified passing under `-tags llmjudge`, no API key. |
| T-101-07 | Information disclosure (API key in logs/transcripts) | mitigate | CLOSED | `SkipWithoutAPIKey` (`client.go:50-63`) reads only env presence and never interpolates the key value (skip message is the literal "ANTHROPIC_API_KEY not set"). `TestAdoptionScorecardLive` calls it FIRST (`adoption_scorecard_test.go:36`). The `Transcript` struct (`transcript.go:18-26`) has no key field. **Empirical:** with keys unset the live leg SKIPs cleanly logging only the env-var name; and a scan of a freshly-written live transcript for the actual `DEEPSEEK_API_KEY` value found NO match — the key never lands in a written transcript. No new key handling added by this phase. |
| T-101-08 | Tampering (drift) | mitigate | CLOSED | The live leg builds buckets from raw responses and feeds them to the SAME pure `adopt.Scorecard` (`adoption_scorecard_test.go:114,116`), classifying via `adopt.ClassifyChoice` / `adopt.FirstCommand` (`adoption_scorecard_test.go:106-110`) — no re-implemented classifier. The detector lift was made real: `skill_trigger_test.go` deleted its local `firstCommandLine`/`mentionsHelix`/`mentionsGrepBaseline` and now delegates to `adopt.ClassifyChoice` (`skill_trigger_test.go:109,130`). Grep confirms zero local `func firstCommandLine`/`func mentionsHelix` definitions remain. Single source of truth — cannot drift. |
| T-101-SC | Tampering (npm/pip/cargo installs) | accept | CLOSED (premise holds) | `git diff` of `go.mod`/`go.sum` across the phase commit range (`73958b97~5..73958b97`) is EMPTY — zero new Go dependencies. The `adopt` package imports only stdlib (`fmt`, `strings`) in `scorecard.go`; the test imports only `testify/require` + `internal/cli` (already in the tree). No package-manager installs in either plan. |
| T-101-09 | Elevation (gate runs in CI) | mitigate | CLOSED | Both tagged legs carry build tags: `//go:build llm` (`adoption_scorecard_test.go:1`, `skill_trigger_test.go:1`) and `//go:build llmjudge` (`rubric.go:1`, `aggregate.go:1`, `adoption_exemplar_test.go:1`). `go list ./test/oracle/llm/... ./test/oracle/judge/...` with NO tags returns "matched no packages". `grep -rniE 'tags[ =].*(llm|llmjudge)'` over `Makefile` + `.github/workflows/` returns NONE. The live/judge legs can never block merge; the only oracle package in the default `go test ./...` enumeration is `test/oracle/adopt`. |

---

## Unregistered Flags

None. Neither `101-01-SUMMARY.md` nor `101-02-SUMMARY.md` declares a `## Threat Flags`
section. No new agent-facing attack surface was introduced: the phase adds only test
code (one build-tag-FREE pure scorer package, two build-tag-gated test legs, and an
additive 6th judge-rubric dimension). No production / `cmd/helix` code path, no network
listener, no new external input boundary. The single trust boundary that crosses into
runtime code is the committed fixture JSON → `encoding/json` → typed struct in the
hermetic package, which carries only trusted in-repo synthetic data (T-101-05).

---

## Accepted Risks Log

| ID | Risk | Rationale | Verified |
|----|------|-----------|----------|
| T-101-05 | Committed fixtures could carry a leaked secret. | Fixtures are hand-authored synthetic command strings + plausible constants only; tests are hermetic (no key required). | Secret-pattern scan over all 12 fixtures: no matches. |
| T-101-SC | A package install could pull an unvetted dependency. | No package installs; the phase adds only test code over stdlib + in-tree imports. | `git diff go.mod go.sum` across the phase range is empty. |

---

## Verification Commands (re-runnable)

```
go test ./test/oracle/adopt/... -count=1            # 5 hermetic anti-vacuity tests, no key, no tags — GREEN
go list ./test/oracle/adopt/...                     # resolves WITHOUT -tags (in default suite)
go list ./test/oracle/llm/... ./test/oracle/judge/... # "matched no packages" (excluded from default suite)
go test -tags llmjudge ./test/oracle/judge/... -run TestAdoptionNegativeExemplarVerdict -count=1  # GREEN, no key
go test -tags llm ./test/oracle/llm/... -run TestAdoptionScorecardLive -count=1   # SKIPs cleanly with keys unset
grep -rniE 'tags[ =].*(llm|llmjudge)' Makefile .github/workflows/   # NONE
git diff 73958b97~5 73958b97 -- go.mod go.sum       # empty (zero new deps)
git check-ignore test/oracle/adopt/testdata/transcripts/intact-01.json  # exit 1 (committed, not gitignored)
```

**Adversarial flip-test (T-101-01 materiality):** temporarily setting all 6 sabotaged
fixtures to a helix choice (drop → 0.0) makes `TestSabotagedSkillRevertAndFail` FAIL with
"scorecard measures nothing" — confirming the gate discriminates and is not green-path-only.
Fixtures restored from git; green re-verified.
