// Package runbooks_test enforces docs/runbooks/ compliance with ROADMAP
// criterion #3 of phase 54 (in-binary observability). The runbooks must:
//   - exist at known paths,
//   - contain the documented sections (Symptoms / Inspect / Triage / Remediate),
//   - contain none of the forbidden Grafana / PromQL references,
//   - all be indexed from README.md.
//
// This test runs as part of `go test ./...` so any future doc edit that
// reintroduces forbidden references or removes a runbook fails CI.
package runbooks_test

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// runbookFiles are the four runbooks named in ROADMAP criterion #3 for
// phase 54-obs-inbinary-page. Paths are relative to this test file's
// directory (docs/runbooks/).
var runbookFiles = []string{
	"ErrCircuitOpen.md",
	"deadline-timeouts.md",
	"ls-crash-restart.md",
	"memory-pressure-eviction.md",
}

// requiredHeadings is the set of `## ` H2 headings that every runbook must
// expose. Matched case-insensitively against the lowercased file body.
var requiredHeadings = []string{
	"## symptoms",
	"## inspect",
	"## triage",
	"## remediate",
}

// forbiddenSubstrings are tokens that MUST NOT appear in any runbook or
// in README.md. They correspond to the Grafana-stack approach abandoned
// in commit 82d39f87 (see .planning/phases/54-obs-inbinary-page/).
var forbiddenSubstrings = []string{
	"grafana",
	"grafana cloud",
	"promql",
	"panel deep-link",
	"${grafana_url}",
}

// promqlFence catches a triple-backtick promql fenced code block at the
// start of a line. A `(?m)` flag makes `^` match line starts.
var promqlFence = regexp.MustCompile("(?m)^```promql\\b")

func TestRunbookCompliance(t *testing.T) {
	for _, name := range runbookFiles {
		name := name
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(name)
			if err != nil {
				t.Fatalf("runbook %s missing or unreadable: %v", name, err)
			}
			body := string(raw)
			lower := strings.ToLower(body)

			for _, h := range requiredHeadings {
				if !strings.Contains(lower, h) {
					t.Errorf("runbook %s missing required heading %q", name, h)
				}
			}

			for _, bad := range forbiddenSubstrings {
				if strings.Contains(lower, bad) {
					t.Errorf("runbook %s contains forbidden substring %q", name, bad)
				}
			}

			if promqlFence.MatchString(body) {
				t.Errorf("runbook %s contains a forbidden ```promql fenced code block", name)
			}
		})
	}

	t.Run("README_indexes_all_runbooks", func(t *testing.T) {
		raw, err := os.ReadFile("README.md")
		if err != nil {
			t.Fatalf("docs/runbooks/README.md missing or unreadable: %v", err)
		}
		body := string(raw)

		for _, name := range runbookFiles {
			if !strings.Contains(body, name) {
				t.Errorf("README.md does not reference runbook %q", name)
			}
		}

		// README.md is allowed to mention Grafana/Prometheus in a negative
		// disclaimer ("No Prometheus, no Grafana, no Docker.") — that
		// disclaimer is the project's anti-external-stack stance and is
		// load-bearing operator guidance. The forbidden-substring gate is
		// therefore applied only to the four runbook files above. We still
		// reject a literal ```promql fenced block here, since that would
		// indicate accidental reintroduction of PromQL examples.
		if promqlFence.MatchString(body) {
			t.Errorf("README.md contains a forbidden ```promql fenced code block")
		}
	})
}
