// Package adopt is the hermetic, build-tag-FREE core of the Phase 101 LLM-behavioral
// adoption scorecard (ADOPT-02). Unlike the live harness in test/oracle/llm (which is
// //go:build llm || llmjudge and therefore invisible to `go test ./...`), this package
// carries NO build tag: its pure scorer + committed fixtures + anti-vacuity tests RUN in
// the default suite with no network and no API key. It is the SOLE authoritative
// non-vacuity proof for the scorecard (101-RESEARCH Pitfall 1).
//
// The first-emitted-command detector is LIFTED verbatim from the formerly
// //go:build llm helper firstCommandLine (test/oracle/llm/skill_trigger_test.go:33-48),
// re-homed here as the single source of truth so it can be exercised hermetically and
// re-imported by the tagged live leg (Plan 02). Classification keys on the FIRST command
// PREFIX, never strings.Contains over the whole response — the injected SKILL.md's own
// "helix" text must not inflate the choice rate (101-RESEARCH Pitfall 2).
package adopt

import (
	"fmt"
	"strings"
)

// MinTasks is the empty-bucket floor: a Scorecard over fewer than MinTasks transcripts
// is an ERROR, never choice_rate=1.0 (0/0 must not read as a pass — Phase 87 CR-01).
// Mirrors Phase 97 ADOPT-01b's `>= 5` case floor.
const MinTasks = 5

// MaterialDrop is the revert-and-fail threshold: stripping the SKILL.md decision matrix
// must drop choice_rate by at least this margin versus the intact skill. It is a
// defensible MATERIAL margin, deliberately NOT `>= 0.0` — a green-path-only gate is
// presumed broken (101-RESEARCH A1, T-101-01).
const MaterialDrop = 0.4

// Bucket is one classified interaction. Only the first emitted command in Response is
// load-bearing for classification; the rest of the transcript shape is irrelevant here.
type Bucket struct {
	Response string
}

// ScorecardResult is the aggregated two-metric scorecard over a task bucket. ChoiceRate
// and FallbackRate are complementary over the classified transcripts: on a bucket where
// every Response is either a helix choice or a standard-tool fallback they sum to 1.0.
type ScorecardResult struct {
	ChoiceRate   float64
	FallbackRate float64
	Choices      int
	Fallbacks    int
	Total        int
}

// FirstCommand returns the first non-empty command line of response with common
// command-line decoration stripped: surrounding code fences, surrounding backticks, and a
// leading "$ "/"> " shell prompt. It mirrors the "ONLY the single command line" contract
// so the classifier evaluates the command the model actually emitted, not surrounding
// prose. Lifted verbatim from firstCommandLine (test/oracle/llm/skill_trigger_test.go:33).
func FirstCommand(response string) string {
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

// fallbackPrefixes are the standard tools the helix skill is meant to displace. A first
// command starting with one of these is a FALLBACK. The trailing space matters: it keys on
// the invocation token, not a substring (e.g. "ls " not "ls" so "lsp" is not a fallback).
var fallbackPrefixes = []string{"grep ", "sed ", "cat ", "find ", "rg ", "ls "}

// ClassifyChoice classifies a response by the PREFIX of its FIRST emitted command:
// chose is true iff the first command starts with "helix "; fellBack is true iff it starts
// with a fallback tool prefix. A prose or unrecognized first line yields (false, false).
// This deliberately uses HasPrefix on FirstCommand, NOT strings.Contains over the whole
// response — the injected SKILL.md is full of the word "helix", and a Contains check would
// count that against the model (101-RESEARCH Pitfall 2, T-101-03).
func ClassifyChoice(response string) (chose, fellBack bool) {
	cmd := strings.ToLower(FirstCommand(response))
	chose = strings.HasPrefix(cmd, "helix ")
	for _, t := range fallbackPrefixes {
		if strings.HasPrefix(cmd, t) {
			fellBack = true
			break
		}
	}
	return
}

// decisionMatrixHeading is the literal heading that anchors the sabotage strip. The
// section runs from this heading to the next "\n## " heading. The literal is required
// here because it is the strip anchor; the non-noop guarantee is enforced by a test
// against the REAL embedded body (TestSabotageNonNoop), not by trusting this constant.
const decisionMatrixHeading = "## Decision matrix"

// StripDecisionMatrix returns skill with the "## Decision matrix" section removed (from the
// heading up to the next "## " heading, or to end-of-file if it is the last section). If the
// heading is absent it returns the input UNCHANGED — a defensive no-op only when the anchor
// is genuinely missing. Because that defensive path could silently re-introduce vacuity if a
// future SKILL.md restructure renames the heading, the non-noop guarantee is enforced
// externally by asserting len(StripDecisionMatrix(real)) < len(real) (T-101-02). Mirrors the
// Phase 97 referenceMissingVerbs pure-helper pattern: a deterministic text transform run on
// intact vs mutated bytes.
func StripDecisionMatrix(skill string) string {
	i := strings.Index(skill, decisionMatrixHeading)
	if i < 0 {
		return skill
	}
	rest := skill[i+len(decisionMatrixHeading):]
	j := strings.Index(rest, "\n## ")
	if j < 0 {
		return skill[:i]
	}
	return skill[:i] + rest[j:]
}

// Scorecard aggregates a task bucket into the two-metric ScorecardResult. It REJECTS an
// empty or sub-floor bucket with a non-nil error and a zero result: 0/0 must never read as
// choice_rate=1.0 (Phase 87 CR-01, T-101-04). Otherwise it classifies each bucket via
// ClassifyChoice and reports ChoiceRate=choices/total and FallbackRate=fallbacks/total over
// Total=len(buckets).
func Scorecard(buckets []Bucket) (ScorecardResult, error) {
	if len(buckets) < MinTasks {
		return ScorecardResult{}, fmt.Errorf("empty/under-sized task bucket: %d < %d", len(buckets), MinTasks)
	}
	var choices, fallbacks int
	for _, b := range buckets {
		chose, fellBack := ClassifyChoice(b.Response)
		if chose {
			choices++
		}
		if fellBack {
			fallbacks++
		}
	}
	total := len(buckets)
	return ScorecardResult{
		ChoiceRate:   float64(choices) / float64(total),
		FallbackRate: float64(fallbacks) / float64(total),
		Choices:      choices,
		Fallbacks:    fallbacks,
		Total:        total,
	}, nil
}
