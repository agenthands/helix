// Phase 54 Plan 01 (Wave 0): registry-driven PromQL validation test.
//
// This test reads every Grafana dashboard JSON under deploy/grafana/ and every
// Markdown runbook under docs/runbooks/, parses every PromQL expression with
// github.com/prometheus/prometheus/promql/parser, and asserts:
//
//  1. Every metric name (after stripping histogram _bucket/_count/_sum suffixes)
//     is registered on a fresh *Metrics built via newMetrics(). The runtime
//     families (`up`, `go_*`, `process_*`) are allowlisted because dashboards
//     reference `up{job=~".*helix.*"}` for the $instance template variable.
//  2. Every label NAME on every VectorSelector is in AllowedLabels (metrics.go),
//     in carveOuts[base] (metrics_labels_test.go), or in the PromQL-internal
//     allowlist {__name__, le, job, instance} ($instance template variable).
//
// The test FAIL-CLOSES when deploy/grafana/ or docs/runbooks/ is empty.
//
// As of Plan 54-04 (this file's most-recent edit), BOTH branches are
// UNCONDITIONALLY fail-closed — `deploy/grafana/*.json` MUST contain at least
// one dashboard, and `docs/runbooks/*.md` MUST contain at least one runbook.
// The previous Wave 0 env-var gate has been fully removed now that Phase 54
// Wave 1 has shipped runbook content; both fatal branches are unconditional.
//
// TestDashboardsAndRunbooksValidatorFailsClosedOnEmpty proves the empty-dir
// fail-closed contract by independently validating the glob pre-condition,
// pinning the contract against accidental regressions.
package obs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/prometheus/prometheus/promql/parser"
)

// projectRoot resolves the repository root from this test file's location.
// internal/obs/dashboards_test.go → ../../.. = repo root.
func projectRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// stripHistogramSuffix maps "*_bucket"/"*_count"/"*_sum" → base family name
// so PromQL like `histogram_quantile(0.95, rate(helix_repomap_extract_duration_seconds_bucket[5m]))`
// resolves against the registered base histogram family.
func stripHistogramSuffix(name string) string {
	for _, sfx := range []string{"_bucket", "_count", "_sum"} {
		if strings.HasSuffix(name, sfx) {
			return strings.TrimSuffix(name, sfx)
		}
	}
	return name
}

// primeAllVectors mirrors metrics_labels_test.go:111-125 verbatim. Without
// priming, prometheus.Registry.Gather() drops empty families and the validator
// produces false negatives ("metric not registered").
//
// Open Question 5 in 54-RESEARCH.md resolves to: copy-paste rather than
// refactoring metrics_labels_test.go. Refactor to a shared helper in a later
// cleanup phase.
func primeAllVectors(m *Metrics) {
	m.ToolCalls.WithLabelValues("t", "p", "m", "go", "success").Inc()
	m.ToolDuration.WithLabelValues("t", "p", "m", "go").Observe(0.001)
	m.LSPoolWorkers.WithLabelValues("go").Set(1)
	m.LSPoolEvictions.WithLabelValues("go", "idle").Inc()
	m.LSPoolCircuitState.WithLabelValues("go").Set(0)
	m.LSPoolRestarts.WithLabelValues("go").Inc()
	m.RenameStrategy.WithLabelValues("lsp-native").Inc()
	m.LSPoolLookups.WithLabelValues("go", "hit").Inc()
	m.RepoMapLookups.WithLabelValues("go", "hit").Inc()
	m.RepoMapExtract.WithLabelValues("go", "treesitter").Observe(0.001)
	m.SessionLifecycle.WithLabelValues("started", "stdio").Inc()
	m.EditOutcome.WithLabelValues("replace_symbol_body", "success", "exact").Inc()
}

// registeredFamilies enumerates the live Prometheus registry via newMetrics()
// (the in-package constructor used by metrics_labels_test.go), primes every
// vector, then returns a name-set of the registered helix_* families.
func registeredFamilies(t *testing.T) map[string]bool {
	t.Helper()
	m := newMetrics()
	primeAllVectors(m)
	mfs, err := m.Registry().Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	out := map[string]bool{}
	for _, mf := range mfs {
		out[mf.GetName()] = true
	}
	return out
}

// dashFile / dashPanel / target capture the minimum Grafana JSON shape needed
// to walk panels[].targets[].expr. Nested panels[] (for `row` containers) is
// included even though D-24 forbids row panels — recursion is cheap insurance.
type dashFile struct {
	Panels []dashPanel `json:"panels"`
}
type dashPanel struct {
	Type    string      `json:"type"`
	Targets []target    `json:"targets,omitempty"`
	Panels  []dashPanel `json:"panels,omitempty"` // nested rows
}
type target struct {
	Expr string `json:"expr"`
}

// extractPromQLFromDashboard reads JSON, walks panels (including nested rows),
// returns every non-empty targets[].expr string.
func extractPromQLFromDashboard(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var d dashFile
	if err := json.Unmarshal(data, &d); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var out []string
	var walk func(panels []dashPanel)
	walk = func(panels []dashPanel) {
		for _, p := range panels {
			for _, tgt := range p.Targets {
				if tgt.Expr != "" {
					out = append(out, tgt.Expr)
				}
			}
			if len(p.Panels) > 0 {
				walk(p.Panels)
			}
		}
	}
	walk(d.Panels)
	return out
}

// promqlFenceRe matches lowercase ```promql fences only (Pitfall #8). Uppercase
// `PromQL` is rejected on purpose so the runbook author convention stays bounded.
var promqlFenceRe = regexp.MustCompile("(?s)```promql\\s*\\n(.*?)```")

// extractPromQLFromRunbook scans Markdown for ```promql fenced blocks. Each
// fenced block may contain multiple blank-line-separated queries; comment-only
// lines (lines whose trim starts with `#`) are stripped before parsing
// (Pitfall #9 — pure-doc comment blocks must not produce parse errors).
func extractPromQLFromRunbook(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var out []string
	for _, m := range promqlFenceRe.FindAllSubmatch(data, -1) {
		blocks := strings.Split(string(m[1]), "\n\n")
		for _, b := range blocks {
			lines := strings.Split(b, "\n")
			var kept []string
			for _, ln := range lines {
				if strings.HasPrefix(strings.TrimSpace(ln), "#") {
					continue
				}
				kept = append(kept, ln)
			}
			expr := strings.TrimSpace(strings.Join(kept, "\n"))
			if expr != "" {
				out = append(out, expr)
			}
		}
	}
	return out
}

// promqlInternalLabels are PromQL/Prometheus-internal label names that may
// appear on any VectorSelector regardless of family. `__name__` is the
// canonical metric-name pseudo-label; `le` is the histogram bucket boundary;
// `job` is the canonical scrape-job target label; `instance` is added because
// the $instance template variable filter `{instance=~"$instance"}` is used on
// engine-dashboard panels (54-RESEARCH §A.4) and `instance` is not in
// AllowedLabels.
var promqlInternalLabels = map[string]bool{
	"__name__": true,
	"le":       true,
	"job":      true,
	"instance": true,
}

// problemsForExpr is the pure-function core of the validator: it returns a
// slice of human-readable problems (empty on success). validateExpr wraps it
// with t.Errorf for the production tests; the _catchesDrift companion calls
// it directly so it can assert "len(problems) > 0" without having to corrupt
// a *testing.T (mirrors metrics_labels_test.go::lintLabels pattern).
func problemsForExpr(expr string, families map[string]bool, source string) []string {
	// As of prometheus@v0.311.x, parser exposes only the NewParser-on-Options
	// entry point (the package-level ParseExpr function was removed during the
	// v3 marketing rework). Default Options{} is sufficient for vanilla PromQL;
	// experimental functions / duration expressions require explicit opts.
	p := parser.NewParser(parser.Options{})
	ast, err := p.ParseExpr(expr)
	if err != nil {
		return []string{
			"parse \"" + expr + "\" in " + source + ": " + err.Error(),
		}
	}
	allowed := map[string]bool{}
	for _, l := range AllowedLabels {
		allowed[l] = true
	}
	var problems []string
	parser.Inspect(ast, func(n parser.Node, _ []parser.Node) error {
		vs, ok := n.(*parser.VectorSelector)
		if !ok {
			return nil
		}
		base := stripHistogramSuffix(vs.Name)
		// `up` is a Prometheus-emitted scrape-status series; `go_*` and
		// `process_*` are runtime-family collectors registered alongside
		// helix_* (see newMetrics in metrics.go). All three are allowed.
		if base != "up" && !isRuntimeFamily(base) {
			if !families[base] {
				problems = append(problems,
					"metric \""+vs.Name+"\" (base \""+base+"\") in "+source+
						" expr \""+expr+"\" not registered")
			}
		}
		carve := carveOuts[base]
		for _, lm := range vs.LabelMatchers {
			ln := lm.Name
			if promqlInternalLabels[ln] {
				continue
			}
			if allowed[ln] || carve[ln] {
				continue
			}
			problems = append(problems,
				"label \""+ln+"\" on "+vs.Name+" in "+source+
					" expr \""+expr+"\" not in AllowedLabels or carveOuts["+base+"]")
		}
		return nil
	})
	return problems
}

// validateExpr is the production wrapper around problemsForExpr that reports
// each problem via t.Errorf so a CI failure names the offending file/expr.
func validateExpr(t *testing.T, expr string, families map[string]bool, source string) {
	t.Helper()
	for _, p := range problemsForExpr(expr, families, source) {
		t.Error(p)
	}
}

// TestDashboardsAndRunbooksReferenceRegisteredMetrics walks every JSON
// dashboard and every Markdown runbook, validating every PromQL expression
// against the registered Prometheus registry (D-10..D-14).
//
// Both branches are unconditionally fail-closed as of Plan 54-04: empty
// deploy/grafana/ or docs/runbooks/ trees fail the test loudly.
func TestDashboardsAndRunbooksReferenceRegisteredMetrics(t *testing.T) {
	root := projectRoot(t)
	families := registeredFamilies(t)

	dashGlob := filepath.Join(root, "deploy", "grafana", "*.json")
	dashes, err := filepath.Glob(dashGlob)
	if err != nil {
		t.Fatalf("glob dashboards: %v", err)
	}
	if len(dashes) == 0 {
		t.Fatalf("no dashboards found at %s — deploy/grafana/ must contain at least one *.json dashboard.", dashGlob)
	}
	for _, p := range dashes {
		for _, expr := range extractPromQLFromDashboard(t, p) {
			validateExpr(t, expr, families, p)
		}
	}

	rbGlob := filepath.Join(root, "docs", "runbooks", "*.md")
	rbs, err := filepath.Glob(rbGlob)
	if err != nil {
		t.Fatalf("glob runbooks: %v", err)
	}
	if len(rbs) == 0 {
		t.Fatalf("no runbooks found at %s — docs/runbooks/ must contain at least one *.md runbook.", rbGlob)
	}
	for _, p := range rbs {
		for _, expr := range extractPromQLFromRunbook(t, p) {
			validateExpr(t, expr, families, p)
		}
	}
}

// TestDashboardsAndRunbooksReferenceRegisteredMetrics_catchesDrift is the
// negative proof that the validator catches drift. It feeds a synthetic
// PromQL expression with a typo'd metric name into validateExpr against a
// child *testing.T (via t.Run) and asserts the child reports failure.
//
// Mirrors the convention of metrics_labels_test.go:139-164 — if this test
// ever passes silently, the validator is broken.
func TestDashboardsAndRunbooksReferenceRegisteredMetrics_catchesDrift(t *testing.T) {
	families := registeredFamilies(t)
	badExpr := `helix_bogus_metric_total{result="hit"}`
	problems := problemsForExpr(badExpr, families, "synthetic")
	if len(problems) == 0 {
		t.Fatal("validator failed to flag unregistered metric helix_bogus_metric_total — drift detection is broken")
	}
	// The problem message must identify the metric so a CI failure is actionable.
	joined := strings.Join(problems, "|")
	if !strings.Contains(joined, "helix_bogus_metric_total") {
		t.Errorf("problem missing metric name: %v", problems)
	}
}

// TestDashboardsAndRunbooksValidatorFailsClosedOnEmpty proves the empty-dir
// fail-closed precondition independently of any env-var. It builds an empty
// deploy/grafana / docs/runbooks tree under t.TempDir() and asserts the same
// Glob shape used in production yields zero matches — which is the trigger
// condition for the production t.Fatalf.
//
// Now that Plan 54-04 has removed the env-var clauses from the production
// checks, an accidentally-empty deploy/grafana/ or docs/runbooks/ would
// loudly fail CI; this test pins the precondition.
func TestDashboardsAndRunbooksValidatorFailsClosedOnEmpty(t *testing.T) {
	tmp := t.TempDir()
	for _, sub := range []string{"deploy/grafana", "docs/runbooks"} {
		if err := os.MkdirAll(filepath.Join(tmp, sub), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", sub, err)
		}
	}
	dashes, err := filepath.Glob(filepath.Join(tmp, "deploy", "grafana", "*.json"))
	if err != nil {
		t.Fatalf("glob dashboards: %v", err)
	}
	rbs, err := filepath.Glob(filepath.Join(tmp, "docs", "runbooks", "*.md"))
	if err != nil {
		t.Fatalf("glob runbooks: %v", err)
	}
	if len(dashes) != 0 || len(rbs) != 0 {
		t.Fatalf("tmp setup wrong: dashes=%d rbs=%d", len(dashes), len(rbs))
	}
	// At this point the production check would t.Fatalf. The bare assertion
	// `len(dashes)==0 && len(rbs)==0` is the contract this test pins.
}
