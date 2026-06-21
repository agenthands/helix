package swebench

import (
	"bufio"
	"strings"

	"github.com/agenthands/helix/bench/evaluators"
)

// differential.go computes the gold-vs-agent diff-overlap signal (SC#4): a PURE
// text transform over the `diff --git a/<f> b/<f>` headers of two unified diffs.
// Unlike patch_validator it touches NO working tree — it parses only the header
// lines to recover each patch's touched-file set, then reports
//
//	diff_overlap = |gold_files ∩ agent_files| / |gold_files|
//
// the fraction of the gold patch's files the agent also touched. The gold patch
// (the upstream instance row's reference fix) is the DENOMINATOR; an empty/nil
// gold (or a gold with no parseable diff headers) is an undefined denominator and
// is reported as (nil, *MetricError) — never a fabricated 0 or 1 (clone of
// regression_checker.RegressionRate's null+MetricError discipline,
// regression_checker.go:59-65).
//
// # Run-all-tests override (Open Q2/A5)
//
// The run-all-tests override is supplied by the UTBoost-augmented dataset itself:
// its broader FAIL_TO_PASS sets exercise the patch against MORE than just the
// PR-modified tests, which is what the verified gate's condition (b) consumes.
// differential reports the complementary gold-vs-agent FILE overlap signal — it
// answers "did the agent edit where the gold fix edited", not "which tests ran".

// differentialGraderName is stamped into every MetricError this file emits (clone
// of the completion_gate/regression_checker graderName const pattern).
const differentialGraderName = "swebench_differential"

// diffOverlapMetric is the metric name this transform produces.
const diffOverlapMetric = "diff_overlap"

// touchedFiles parses the set of files a unified-diff patch touches from its
// `diff --git a/<path> b/<path>` header lines. It returns a deduped set keyed on
// the post-image (b/) path. A patch with no such header lines yields an empty set.
// It reads stdlib-only (bufio over a string reader), mirroring the pure-text
// discipline — no git, no working tree.
func touchedFiles(patch string) map[string]struct{} {
	files := make(map[string]struct{})
	sc := bufio.NewScanner(strings.NewReader(patch))
	// Allow long header/content lines (a single diff line can exceed the 64KB
	// default token cap on large patches).
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	const prefix = "diff --git "
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		// Header form: "diff --git a/<path> b/<path>". Recover the b/<path> token.
		rest := strings.TrimPrefix(line, prefix)
		fields := strings.Fields(rest)
		if len(fields) < 2 {
			continue
		}
		bToken := fields[len(fields)-1]
		path := strings.TrimPrefix(bToken, "b/")
		if path == "" {
			continue
		}
		files[path] = struct{}{}
	}
	return files
}

// DiffOverlap returns the gold-vs-agent diff-overlap fraction |gold ∩ agent| /
// |gold| over the two patches' touched-file sets. When the gold patch has no
// parseable touched files (empty/nil text, or no `diff --git` headers) the
// denominator is undefined and DiffOverlap returns (nil, *MetricError) — never a
// fabricated 0 or 1. Otherwise it returns (overlap, nil) with overlap in [0,1].
// The agent patch contributing zero overlap is a legitimate 0.0, NOT an error.
func DiffOverlap(goldPatch, agentPatch string) (*float64, *evaluators.MetricError) {
	gold := touchedFiles(goldPatch)
	if len(gold) == 0 {
		return nil, &evaluators.MetricError{
			Metric: diffOverlapMetric,
			Grader: differentialGraderName,
			Reason: "gold patch has no parseable diff --git headers (undefined denominator)",
		}
	}

	agent := touchedFiles(agentPatch)
	inter := 0
	for f := range gold {
		if _, ok := agent[f]; ok {
			inter++
		}
	}

	overlap := float64(inter) / float64(len(gold))
	return &overlap, nil
}
