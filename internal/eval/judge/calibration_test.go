package judge_test

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/eval/judge"
	"github.com/agenthands/helix/internal/eval/report"
	"github.com/agenthands/helix/internal/eval/score"
	"github.com/agenthands/helix/internal/eval/trace"
)

// TestCalibration is the offline judge-vs-heuristic calibration harness.
//
// It runs the LLM judge (against an httptest fake Anthropic server) and the
// heuristic scorer on the same 10 labeled fixtures, computes Cohen's κ and
// Pearson r between the two binary verdicts, and writes the result to
// eval/reports/calibration.json.
//
// The test does NOT make any live API calls (W2 constraint). It does NOT
// fail on low correlation — calibration is data, not a gate. It fails only
// if the harness wiring is broken: judge.JudgeFailed=true, NaN metrics,
// out-of-range metrics, or report-file write errors.
func TestCalibration(t *testing.T) {
	fixtures := judge.BuiltInLabeledFixtures()
	if len(fixtures) != 10 {
		t.Fatalf("BuiltInLabeledFixtures count = %d; want 10", len(fixtures))
	}

	// Fake Anthropic server: per-task canned response built from the fixture's
	// ExpectedJudgeScores. The handler reads the inbound prompt body, finds
	// the TaskID substring, and returns the matching fixture's scores. This
	// guarantees deterministic correlation regardless of model behavior.
	scoresByID := make(map[string]judge.Scores, len(fixtures))
	for _, f := range fixtures {
		scoresByID[f.TaskID] = f.ExpectedJudgeScores
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := readBody(r)
		var picked judge.Scores
		var pickedID string
		for id, s := range scoresByID {
			if strings.Contains(body, id) {
				picked = s
				pickedID = id
				break
			}
		}
		if pickedID == "" {
			// Default zeros if no match — should not happen in this test.
			picked = judge.Scores{}
		}
		scoreJSON, _ := json.Marshal(map[string]any{
			"scores": map[string]int{
				"right_tool":   picked.RightTool,
				"evidence":     picked.Evidence,
				"blast_radius": picked.BlastRadius,
				"recovery":     picked.Recovery,
			},
			"reasoning": "calibration fixture " + pickedID,
			"flags":     []string{},
		})
		resp := map[string]any{
			"id":          "msg_calib",
			"type":        "message",
			"role":        "assistant",
			"model":       "claude-sonnet-4-6",
			"stop_reason": "end_turn",
			"usage":       map[string]int{"input_tokens": 10, "output_tokens": 5},
			"content": []map[string]any{
				{"type": "text", "text": string(scoreJSON)},
			},
		}
		respBytes, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(respBytes)
	}))
	defer srv.Close()

	// Build inputs.
	inputs := make([]judge.Input, 0, len(fixtures))
	for _, f := range fixtures {
		inputs = append(inputs, judge.Input{
			TaskID:          f.TaskID,
			Mode:            f.Mode,
			TaskKind:        f.Kind,
			TaskDescription: "Calibration fixture " + f.TaskID,
			Trace: trace.MergedTrace{
				SchemaVersion: "1",
				TaskID:        f.TaskID,
				Mode:          f.Mode,
				RunID:         "calib-run",
				StartedAt:     time.Now(),
				EndedAt:       time.Now(),
				Outcome:       "success",
				Events:        f.Events,
			},
			Result: report.EvalResult{
				TaskID:    f.TaskID,
				Mode:      f.Mode,
				Success:   true,
				TestsPass: true,
				Outcome:   "success",
			},
		})
	}

	c := judge.NewClient(judge.Options{APIKey: "k", BaseURL: srv.URL})
	out := judge.Run(context.Background(), c, inputs, "claude-sonnet-4-6")
	if out.JudgeFailed {
		t.Fatalf("judge.Run reported JudgeFailed=true: %s", out.ErrorSummary)
	}
	if len(out.Tasks) != len(fixtures) {
		t.Fatalf("judge.Run returned %d entries; want %d", len(out.Tasks), len(fixtures))
	}

	// Heuristic verdicts via score.Apply on a generic rename rule set
	// (tool-name-only matching; no args_match constraint).
	rules := genericRenameRules(t)

	pairs := make([]calibPair, 0, len(fixtures))

	// Index judge output entries by TaskID.
	byID := make(map[string]judge.Entry, len(out.Tasks))
	for _, e := range out.Tasks {
		byID[e.TaskID] = e
	}

	for _, f := range fixtures {
		j, ok := byID[f.TaskID]
		if !ok {
			t.Fatalf("judge output missing TaskID %s", f.TaskID)
		}
		jSum := j.Scores.RightTool + j.Scores.Evidence + j.Scores.BlastRadius + j.Scores.Recovery
		jBinary := 0
		if jSum >= 2 { // half or more axes positive → "good"
			jBinary = 1
		}

		s := score.Apply(inputs[indexOf(fixtures, f.TaskID)].Trace, rules)
		hBinary := 0
		if s.Total > 0 {
			hBinary = 1
		}

		pairs = append(pairs, calibPair{
			judgeBinary: jBinary,
			heurBinary:  hBinary,
			judgeRaw:    float64(jSum),
			heurRaw:     float64(s.Total),
		})
	}

	// Cohen's κ over the 10 pairs.
	n := len(pairs)
	var agree, jYes, hYes int
	for _, p := range pairs {
		if p.judgeBinary == p.heurBinary {
			agree++
		}
		if p.judgeBinary == 1 {
			jYes++
		}
		if p.heurBinary == 1 {
			hYes++
		}
	}
	po := float64(agree) / float64(n)
	pjYes := float64(jYes) / float64(n)
	phYes := float64(hYes) / float64(n)
	pjNo, phNo := 1-pjYes, 1-phYes
	pe := pjYes*phYes + pjNo*phNo
	var kappa float64
	if pe == 1.0 {
		kappa = 0
	} else {
		kappa = (po - pe) / (1 - pe)
	}

	// Pearson r over raw sums (judgeRaw, heurRaw).
	pearsonR := pearson(pairs)

	// Sanity assertions.
	if math.IsNaN(kappa) {
		t.Fatalf("kappa is NaN")
	}
	if kappa < -1 || kappa > 1 {
		t.Fatalf("kappa = %v; want in [-1, 1]", kappa)
	}
	if math.IsNaN(pearsonR) {
		// Pearson can be NaN only if all values in one column are identical,
		// which we have engineered NOT to happen. Treat as harness failure.
		t.Fatalf("pearson r is NaN — fixtures appear to lack variance")
	}
	if pearsonR < -1 || pearsonR > 1 {
		t.Fatalf("pearson r = %v; want in [-1, 1]", pearsonR)
	}

	// Write eval/reports/calibration.json from the test source-file location.
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	reportsDir := filepath.Join(repoRoot, "eval", "reports")
	if err := os.MkdirAll(reportsDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", reportsDir, err)
	}
	calibPath := filepath.Join(reportsDir, "calibration.json")

	jVerdicts := make([]int, len(pairs))
	hVerdicts := make([]int, len(pairs))
	for i, p := range pairs {
		jVerdicts[i] = p.judgeBinary
		hVerdicts[i] = p.heurBinary
	}

	payload := map[string]any{
		"__readme":           "INFORMATIONAL — calibration metric, not a gate",
		"computed_at":        time.Now().UTC().Format(time.RFC3339),
		"n":                  n,
		"kappa":              kappa,
		"pearson_r":          pearsonR,
		"judge_verdicts":     jVerdicts,
		"heuristic_verdicts": hVerdicts,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if err := os.WriteFile(calibPath, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write %s: %v", calibPath, err)
	}

	// Round-trip: re-read and assert valid JSON with expected fields.
	rb, err := os.ReadFile(calibPath)
	if err != nil {
		t.Fatalf("read back %s: %v", calibPath, err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(rb, &parsed); err != nil {
		t.Fatalf("parse calibration.json: %v", err)
	}
	if _, ok := parsed["kappa"]; !ok {
		t.Errorf("calibration.json missing kappa")
	}
	if _, ok := parsed["pearson_r"]; !ok {
		t.Errorf("calibration.json missing pearson_r")
	}
}

// readBody slurps and restores r.Body as a string (for the test handler).
func readBody(r *http.Request) string {
	if r.Body == nil {
		return ""
	}
	defer r.Body.Close()
	buf := make([]byte, 0, 4096)
	chunk := make([]byte, 4096)
	for {
		n, err := r.Body.Read(chunk)
		if n > 0 {
			buf = append(buf, chunk[:n]...)
		}
		if err != nil {
			break
		}
	}
	return string(buf)
}

// indexOf returns the index of the fixture with the given TaskID.
func indexOf(fixtures []judge.LabeledFixture, id string) int {
	for i, f := range fixtures {
		if f.TaskID == id {
			return i
		}
	}
	return -1
}

// calibPair holds the per-fixture binary verdicts and raw sums used for
// Cohen's κ and Pearson r computations.
type calibPair struct {
	judgeBinary, heurBinary int
	judgeRaw, heurRaw       float64
}

// pearson computes Pearson's r over (judgeRaw, heurRaw) pairs.
// Returns NaN if either column has zero variance.
func pearson(pairs []calibPair) float64 {
	n := float64(len(pairs))
	if n == 0 {
		return math.NaN()
	}
	var sumX, sumY, sumXY, sumXX, sumYY float64
	for _, p := range pairs {
		x, y := p.judgeRaw, p.heurRaw
		sumX += x
		sumY += y
		sumXY += x * y
		sumXX += x * x
		sumYY += y * y
	}
	num := n*sumXY - sumX*sumY
	den := math.Sqrt((n*sumXX - sumX*sumX) * (n*sumYY - sumY*sumY))
	if den == 0 {
		return math.NaN()
	}
	return num / den
}

// genericRenameRules constructs a tool-name-only rules set for the rename
// family. We avoid args_match because the calibration fixtures use synthetic
// per-fixture symbols (Alpha, Beta, ...) that won't match any single static
// symbol literal. The rule shape mirrors eval/corpus/go-rename-public-001/expected_tools.yaml
// but with args_match removed.
func genericRenameRules(t *testing.T) score.Rules {
	t.Helper()
	tmp := t.TempDir()
	yamlSrc := `task_kind: rename

expect_sequence:
  - id: rename-after-references
    score: 1
    pattern:
      - tool: find_references
      - tool: rename_symbol

expect_set:
  - id: verified-after-edit
    score: 1
    tools:
      - rename_symbol
      - verify_edit

forbid_sequence:
  - id: rename-by-grep
    score: -1
    pattern:
      - tool: search_for_pattern
      - tool: replace_in_file
`
	path := filepath.Join(tmp, "rules.yaml")
	if err := os.WriteFile(path, []byte(yamlSrc), 0o644); err != nil {
		t.Fatalf("write rules.yaml: %v", err)
	}
	rules, err := score.LoadRules(path)
	if err != nil {
		t.Fatalf("LoadRules: %v", err)
	}
	return rules
}
