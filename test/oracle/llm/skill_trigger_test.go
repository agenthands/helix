//go:build llm

package llm

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/internal/cli"
	"github.com/agenthands/helix/test/harness"
	"github.com/agenthands/helix/test/oracle/adopt"
)

// The first-command detector triad (firstCommandLine / mentionsHelix /
// mentionsGrepBaseline) was LIFTED into the build-tag-FREE test/oracle/adopt
// package as the single source of truth (Plan 01: adopt.FirstCommand /
// adopt.ClassifyChoice). This live leg now delegates to those exported helpers
// rather than carrying a local copy, so there is exactly ONE classifier
// implementation and no drift between the hermetic scorer and the live oracle.
//
// adopt.ClassifyChoice keys BOTH chose ("helix " prefix) and fellBack
// (grep/sed/cat/find/rg/ls prefix) on the FIRST emitted command, so the prior
// `mentionsGrepBaseline(resp) && !mentionsHelix(resp)` baseline-chose-grep
// predicate collapses to ClassifyChoice's fellBack (a fallback-prefixed first
// command is, by construction, not a helix command). The prior fallback detector
// used a looser strings.Contains form; the lifted prefix classifier is the
// intentional, stricter single source of truth (101-PATTERNS Pitfall 2).

// TestSkillVsBaseline is the TEST-03 behavioral oracle: for each helix-appropriate
// code task it asks the subject model TWICE — once with a grep/sed/cat baseline
// system prompt (no SKILL.md body) and once with the helix Agent Skill body
// injected — and records whether tool selection shifts toward a `helix` verb.
//
// Honest gating: //go:build llm + SkipWithoutAPIKey(t) as the FIRST line. Without
// an API key the test SKIPs hermetically (never a vacuous pass); the assertion
// fires ONLY when keyed. The aggregate shift (not every individual task) is
// asserted to tolerate single-task noise (D-19 sequential calls + InterCallDelay).
//
// SKILL-04 side-measurement: when keyed it also records the full MCP tools/list
// blob size (FormatToolList, the preloaded "before" schema tax) versus the
// embedded SKILL.md description size (the on-demand "after" idle cost) into a
// transcript.
func TestSkillVsBaseline(t *testing.T) {
	SkipWithoutAPIKey(t)

	// The oracle measures SELECTION, not execution, so SkipLS is fine. The runner
	// is started purely to obtain the live tools/list blob for the SKILL-04
	// before/after sizes (the MCP head is alive until Phase 94).
	runner := harness.StartRunner(t, harness.RunnerOptions{SkipLS: true})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	client := NewClient()
	model := SubjectModel()

	// Single source of truth: load the embedded SKILL.md body (no inlined copy).
	skillBody := cli.EmbeddedSkillBody()
	require.NotEmpty(t, skillBody, "embedded SKILL.md body is empty")

	baselineSystem := GrepBaselineSystemPrompt()
	skillSystem := SkillSystemPrompt(skillBody)

	// SKILL-04 before/after blob sizes (recorded for the token-note correlation).
	if result, err := runner.Session.ListTools(ctx, &mcp.ListToolsParams{}); err == nil {
		preloadBlob := FormatToolList(result.Tools)
		WriteTranscript(t, &Transcript{
			ScenarioID: "skill04-blob-sizes",
			Category:   "selection",
			Model:      model,
			System:     "SKILL-04 idle-cost measurement",
			UserPrompt: "preloaded tools/list (FormatToolList) vs embedded SKILL.md body",
			Response: "preloaded_tools_list_bytes=" +
				itoa(len(preloadBlob)) + " tool_count=" + itoa(len(result.Tools)) +
				" skill_body_bytes=" + itoa(len(skillBody)),
			StopReason: "measurement",
		})
		t.Logf("SKILL-04: preloaded tools/list blob=%d bytes (%d tools), skill body=%d bytes",
			len(preloadBlob), len(result.Tools), len(skillBody))
	}

	tasks := SkillTaskDescriptions()

	var (
		baselineGrep int // baseline runs that chose grep/sed/cat
		skillHelix   int // skill runs that chose a helix verb
		shifted      int // tasks where baseline=grep AND skill=helix
		callIdx      int
	)

	for i, task := range tasks {
		task := task

		// Baseline condition.
		if callIdx > 0 {
			InterCallDelay()
		}
		callIdx++
		baseResp, baseStop, err := AskSingleTurn(ctx, client, model, baselineSystem, SkillTaskUserPrompt(task))
		require.NoError(t, err, "baseline AskSingleTurn failed for task %d", i)
		// Single source of truth: the first emitted command falling back to a
		// grep/sed/cat baseline tool (adopt.ClassifyChoice fellBack). A fallback
		// first command is, by construction, not a helix command.
		_, baseChoseGrep := adopt.ClassifyChoice(baseResp)
		if baseChoseGrep {
			baselineGrep++
		}
		WriteTranscript(t, &Transcript{
			ScenarioID: "skill-baseline-" + itoa(i),
			Category:   "selection",
			Model:      model,
			System:     baselineSystem,
			UserPrompt: SkillTaskUserPrompt(task),
			Response:   baseResp,
			StopReason: baseStop,
		})

		// Skill condition (same task).
		InterCallDelay()
		callIdx++
		skillResp, skillStop, err := AskSingleTurn(ctx, client, model, skillSystem, SkillTaskUserPrompt(task))
		require.NoError(t, err, "skill AskSingleTurn failed for task %d", i)
		// Single source of truth: the first emitted command is a helix verb
		// (adopt.ClassifyChoice chose).
		skillChoseHelix, _ := adopt.ClassifyChoice(skillResp)
		if skillChoseHelix {
			skillHelix++
		}
		if baseChoseGrep && skillChoseHelix {
			shifted++
		}
		WriteTranscript(t, &Transcript{
			ScenarioID: "skill-with-" + itoa(i),
			Category:   "selection",
			Model:      model,
			System:     skillSystem,
			UserPrompt: SkillTaskUserPrompt(task),
			Response:   skillResp,
			StopReason: skillStop,
		})

		t.Logf("task %d: baseline_grep=%v skill_helix=%v\n  baseline=%q\n  skill=%q",
			i, baseChoseGrep, skillChoseHelix, strings.TrimSpace(baseResp), strings.TrimSpace(skillResp))
	}

	n := len(tasks)
	t.Logf("AGGREGATE: baseline chose grep on %d/%d, skill chose helix on %d/%d, shifted %d/%d",
		baselineGrep, n, skillHelix, n, shifted, n)

	// Aggregate assertions (tolerant of single-task noise):
	// 1. The skill condition selects a helix verb on a majority of tasks.
	require.Greater(t, skillHelix*2, n,
		"skill condition shifted to a helix verb on only %d/%d tasks (expected majority)", skillHelix, n)
	// 2. The skill measurably shifts selection toward helix vs the baseline
	//    (at least one task moved grep -> helix; the comparison is the deliverable).
	require.Positive(t, shifted,
		"no task shifted from grep/sed/cat baseline to a helix verb under the skill (expected >=1)")
}

// itoa is a tiny stdlib-free int formatter to avoid importing strconv just for
// transcript strings.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
