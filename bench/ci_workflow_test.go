// Package benchci hosts the INFRA-04 hermetic acceptance proof for the CI
// cost-policy workflow (.github/workflows/bench.yml).
//
// This is a STRUCTURE test, not a live CI run: it parses the committed workflow
// YAML and asserts the budget-protecting invariants (the PR 5-minute hard cap,
// the no-secret hermetic PR job, least-privilege permissions, the
// schedule/dispatch-gated full job, and the absence of any LLM-judge reference).
// The LIVE CI execution remains inspection-gated per 89-VALIDATION.md.
//
// No external dependency is added: gopkg.in/yaml.v3 is already a direct module
// dependency (used by bench/cost and bench/runners).
package benchci

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// workflowPath resolves .github/workflows/bench.yml relative to this test's
// package directory (bench/), i.e. repo-root/.github/workflows/bench.yml.
const workflowPath = "../.github/workflows/bench.yml"

// workflow mirrors only the structure the INFRA-04 acceptance criteria pin.
// Unmodeled fields are ignored by yaml.v3, which is intentional: this test
// asserts the cost-policy invariants, not a byte-for-byte schema.
type workflow struct {
	Permissions struct {
		Contents string `yaml:"contents"`
	} `yaml:"permissions"`
	Jobs map[string]job `yaml:"jobs"`
}

type job struct {
	If             string `yaml:"if"`
	TimeoutMinutes int    `yaml:"timeout-minutes"`
	Steps          []struct {
		Name string `yaml:"name"`
		Run  string `yaml:"run"`
		Uses string `yaml:"uses"`
	} `yaml:"steps"`
}

func readWorkflow(t *testing.T) (raw []byte, wf workflow) {
	t.Helper()
	abs, err := filepath.Abs(workflowPath)
	if err != nil {
		t.Fatalf("resolve workflow path: %v", err)
	}
	raw, err = os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read %s: %v", abs, err)
	}
	// Assertion (1): the file is valid YAML.
	if err := yaml.Unmarshal(raw, &wf); err != nil {
		t.Fatalf("bench.yml is not valid YAML: %v", err)
	}
	return raw, wf
}

// runStep returns the single `run:` step command for a job, or "" if none.
func runStep(j job) string {
	for _, s := range j.Steps {
		if s.Run != "" {
			return strings.TrimSpace(s.Run)
		}
	}
	return ""
}

// TestBenchWorkflow is the INFRA-04 hermetic acceptance proof. It parses
// .github/workflows/bench.yml and asserts the cost-policy structure.
func TestBenchWorkflow(t *testing.T) {
	raw, wf := readWorkflow(t)

	// (4) least-privilege permissions: contents == read.
	if wf.Permissions.Contents != "read" {
		t.Errorf("permissions.contents = %q, want %q (least privilege, T-89-04-03)",
			wf.Permissions.Contents, "read")
	}

	// (2) bench-quick job exists with timeout-minutes: 5 (the HARD PR cap).
	quick, ok := wf.Jobs["bench-quick"]
	if !ok {
		t.Fatalf("workflow is missing the bench-quick PR job")
	}
	if quick.TimeoutMinutes != 5 {
		t.Errorf("bench-quick timeout-minutes = %d, want 5 (HARD PR cost cap, T-89-04-02)",
			quick.TimeoutMinutes)
	}
	// bench-quick must be PR-gated.
	if !strings.Contains(quick.If, "pull_request") {
		t.Errorf("bench-quick `if` = %q, want it gated on the pull_request event", quick.If)
	}
	// (3) its single run step invokes `make bench-quick`.
	if cmd := runStep(quick); cmd != "make bench-quick" {
		t.Errorf("bench-quick run step = %q, want %q", cmd, "make bench-quick")
	}

	// (5) bench-full job invokes `make bench` and is gated on schedule/dispatch.
	full, ok := wf.Jobs["bench-full"]
	if !ok {
		t.Fatalf("workflow is missing the bench-full job")
	}
	if cmd := runStep(full); cmd != "make bench" {
		t.Errorf("bench-full run step = %q, want %q", cmd, "make bench")
	}
	if !strings.Contains(full.If, "schedule") || !strings.Contains(full.If, "workflow_dispatch") {
		t.Errorf("bench-full `if` = %q, want it gated on schedule || workflow_dispatch (never pull_request)", full.If)
	}
	if strings.Contains(full.If, "pull_request") {
		t.Errorf("bench-full `if` = %q must NOT run on pull_request (expensive path is maintainer/schedule-gated)", full.If)
	}

	// (6) no LLM-judge reference anywhere in the file (EVAL-07 mirror).
	const judgeRef = "tool_behavior" + "_judge"
	if strings.Contains(string(raw), judgeRef) {
		t.Errorf("bench.yml references the LLM judge (%s); it must NEVER gate merges (EVAL-07)", judgeRef)
	}

	// No provider secret in the hermetic PR job: assert no ${{ secrets.* }}
	// expression appears in the bench-quick job body (T-89-04-01).
	for _, s := range quick.Steps {
		if strings.Contains(s.Run, "secrets.") || strings.Contains(s.Uses, "secrets.") {
			t.Errorf("bench-quick references a secret in step %q; the PR job must be hermetic (no provider secret, T-89-04-01)", s.Name)
		}
	}
}
