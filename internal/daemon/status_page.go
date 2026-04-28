// Phase 54 OBS-01: in-binary HTML status page mounted at GET / on the admin
// listener. Reads the SAME *prometheus.Registry already serving /metrics —
// no second registry, no caching, no JS frameworks, zero new external deps.
//
// Boundary preservation: this file imports prometheus/client_model (dto.*)
// to walk the gather result, but does NOT call any prometheus/client_golang
// registry constructors — the read side only. Constructors stay in internal/obs.
package daemon

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"sort"
	"time"

	dto "github.com/prometheus/client_model/go"
)

//go:embed status_page.html.tmpl
var indexTmplSrc string

// indexTmpl is parsed once at package init; template.Must panics if the
// embedded template is malformed (fail-fast for an embedded resource).
var indexTmpl = template.Must(template.New("status-page").Parse(indexTmplSrc))

// PageModel is the view model passed to the template. Populated by
// buildPageModel from a []*dto.MetricFamily snapshot.
type PageModel struct {
	Tools       []ToolRow
	Edits       []EditRow
	Workers     []WorkerRow
	Circuits    []CircuitRow
	Evictions   []EvictionRow
	Caches      []CacheRow
	Process     ProcessInfo
	GeneratedAt time.Time
}

// ToolRow is one tool's RED snapshot: total calls (across all outcomes) and
// p95 latency in seconds (bucket-quantized — see Pitfall #1).
type ToolRow struct {
	Name       string
	Calls      uint64
	P95Seconds float64
	HasP95     bool
}

// EditRow is one (tool, outcome) pair from serena_edit_outcome_total.
type EditRow struct {
	Tool    string
	Outcome string
	Count   uint64
}

// WorkerRow is one (language, worker count) pair from serena_lspool_workers.
type WorkerRow struct {
	Language string
	Count    int64
}

// CircuitRow is one (language, state) pair where state is the textual mapping
// of the gauge value: 0->closed, 1->half-open, 2->open.
type CircuitRow struct {
	Language string
	State    string
}

// EvictionRow is one (language, reason, count) tuple where count > 0.
type EvictionRow struct {
	Language string
	Reason   string
	Count    uint64
}

// CacheRow is the hit rate for one (family, language) pair.
// HasData is false when hits+misses == 0 (renders as "—").
type CacheRow struct {
	Family     string
	Language   string
	HitRatePct float64
	HasData    bool
}

// ProcessInfo holds RSS bytes (HasRSS=false on Windows per Pitfall #6) and
// goroutine count.
type ProcessInfo struct {
	RSSBytes   int64
	HasRSS     bool
	RSSHuman   string
	Goroutines int
}

// handleStatusPage serves the in-binary HTML metrics page at GET /.
// Non-GET methods return 405; non-"/" paths return 404. The handler renders
// to a bytes.Buffer first (Pitfall #5) so a template error returns a clean
// 500 instead of a half-written response.
func (d *Daemon) handleStatusPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Pitfall #2: ServeMux "/" is a catch-all; narrow to the literal root.
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	families, err := d.obs.Metrics().Registry().Gather()
	if err != nil {
		// Pitfall #3: partial gather is valid; log and render whatever
		// families we got. This mirrors promhttp.ContinueOnError.
		d.logger.Warn("status page: partial gather", "err", err)
	}

	model := buildPageModel(families, time.Now())

	var buf bytes.Buffer
	if err := indexTmpl.Execute(&buf, model); err != nil {
		d.logger.Error("status page: template execute", "err", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(buf.Bytes())
}

// buildPageModel projects a []*dto.MetricFamily snapshot into a PageModel.
// Pure function: no IPC, no I/O — testable with hand-built fixtures.
func buildPageModel(families []*dto.MetricFamily, now time.Time) PageModel {
	model := PageModel{GeneratedAt: now}

	// Two-pass aggregation: the tool-call counter family and the tool-duration
	// histogram family are independent but produce rows in the same Tools
	// table. Collect by name first, merge at the end.
	toolCalls := map[string]uint64{}
	toolP95 := map[string]float64{}
	toolHasP95 := map[string]bool{}

	for _, f := range families {
		switch f.GetName() {
		case "serena_tool_calls_total":
			for _, m := range f.GetMetric() {
				name := extractLabelValue(m, "tool_name")
				if name == "" {
					continue
				}
				toolCalls[name] += uint64(m.GetCounter().GetValue())
			}
		case "serena_tool_duration_seconds":
			for _, m := range f.GetMetric() {
				name := extractLabelValue(m, "tool_name")
				if name == "" {
					continue
				}
				p95 := p95FromHistogram(m.GetHistogram())
				toolP95[name] = p95
				toolHasP95[name] = !math.IsInf(p95, 0) && m.GetHistogram().GetSampleCount() > 0
			}
		case "serena_edit_outcome_total":
			for _, m := range f.GetMetric() {
				model.Edits = append(model.Edits, EditRow{
					Tool:    extractLabelValue(m, "tool"),
					Outcome: extractLabelValue(m, "outcome"),
					Count:   uint64(m.GetCounter().GetValue()),
				})
			}
		case "serena_lspool_workers":
			for _, m := range f.GetMetric() {
				model.Workers = append(model.Workers, WorkerRow{
					Language: extractLabelValue(m, "language"),
					Count:    int64(m.GetGauge().GetValue()),
				})
			}
		case "serena_lspool_circuit_state":
			for _, m := range f.GetMetric() {
				model.Circuits = append(model.Circuits, CircuitRow{
					Language: extractLabelValue(m, "language"),
					State:    circuitStateName(m.GetGauge().GetValue()),
				})
			}
		case "serena_lspool_evictions_total":
			for _, m := range f.GetMetric() {
				count := uint64(m.GetCounter().GetValue())
				if count == 0 {
					continue
				}
				model.Evictions = append(model.Evictions, EvictionRow{
					Language: extractLabelValue(m, "language"),
					Reason:   extractLabelValue(m, "reason"),
					Count:    count,
				})
			}
		case "serena_lspool_cache_total", "serena_repomap_cache_total":
			model.Caches = append(model.Caches, computeHitRates(f.GetName(), f)...)
		case "process_resident_memory_bytes":
			for _, m := range f.GetMetric() {
				v := int64(m.GetGauge().GetValue())
				model.Process.RSSBytes = v
				model.Process.HasRSS = true
				model.Process.RSSHuman = humanBytes(v)
			}
		case "go_goroutines":
			for _, m := range f.GetMetric() {
				model.Process.Goroutines = int(m.GetGauge().GetValue())
			}
		}
	}

	// Merge tool calls + p95 into Tools. Take the union of tool names so a
	// tool with calls but no duration observations still renders, and vice
	// versa.
	names := map[string]struct{}{}
	for n := range toolCalls {
		names[n] = struct{}{}
	}
	for n := range toolP95 {
		names[n] = struct{}{}
	}
	sortedNames := make([]string, 0, len(names))
	for n := range names {
		sortedNames = append(sortedNames, n)
	}
	sort.Strings(sortedNames)
	for _, n := range sortedNames {
		model.Tools = append(model.Tools, ToolRow{
			Name:       n,
			Calls:      toolCalls[n],
			P95Seconds: toolP95[n],
			HasP95:     toolHasP95[n],
		})
	}

	// Stable orderings on label-keyed slices for deterministic output.
	sort.Slice(model.Edits, func(i, j int) bool {
		if model.Edits[i].Tool != model.Edits[j].Tool {
			return model.Edits[i].Tool < model.Edits[j].Tool
		}
		return model.Edits[i].Outcome < model.Edits[j].Outcome
	})
	sort.Slice(model.Workers, func(i, j int) bool { return model.Workers[i].Language < model.Workers[j].Language })
	sort.Slice(model.Circuits, func(i, j int) bool { return model.Circuits[i].Language < model.Circuits[j].Language })
	sort.Slice(model.Evictions, func(i, j int) bool {
		if model.Evictions[i].Language != model.Evictions[j].Language {
			return model.Evictions[i].Language < model.Evictions[j].Language
		}
		return model.Evictions[i].Reason < model.Evictions[j].Reason
	})
	sort.Slice(model.Caches, func(i, j int) bool {
		if model.Caches[i].Family != model.Caches[j].Family {
			return model.Caches[i].Family < model.Caches[j].Family
		}
		return model.Caches[i].Language < model.Caches[j].Language
	})

	return model
}

// extractLabelValue scans a Metric's label pairs for the named label and
// returns its value (empty string when absent).
func extractLabelValue(m *dto.Metric, name string) string {
	for _, lp := range m.GetLabel() {
		if lp.GetName() == name {
			return lp.GetValue()
		}
	}
	return ""
}

// p95FromHistogram returns the upper bound of the first bucket whose
// cumulative count reaches 95% of the total. Returns +Inf when no bucket
// crosses the threshold (typical when the +Inf bucket is omitted from the
// dto serialization). Bucket-quantized — annotated on the page (Pitfall #1).
func p95FromHistogram(h *dto.Histogram) float64 {
	if h == nil {
		return 0
	}
	total := h.GetSampleCount()
	if total == 0 {
		return 0
	}
	target := 0.95 * float64(total)
	for _, b := range h.GetBucket() {
		if float64(b.GetCumulativeCount()) >= target {
			return b.GetUpperBound()
		}
	}
	return math.Inf(+1)
}

// circuitStateName maps the circuit gauge value (0/1/2) to the textual
// state. Other values render as the raw integer for diagnostic visibility.
func circuitStateName(v float64) string {
	switch int(v) {
	case 0:
		return "closed"
	case 1:
		return "half-open"
	case 2:
		return "open"
	default:
		return fmt.Sprintf("%d", int(v))
	}
}

// computeHitRates groups counter rows by language and computes hit rate per
// (family, language). HasData=false when hits+misses == 0.
func computeHitRates(family string, mf *dto.MetricFamily) []CacheRow {
	type acc struct{ hits, misses float64 }
	byLang := map[string]*acc{}
	for _, m := range mf.GetMetric() {
		lang := extractLabelValue(m, "language")
		result := extractLabelValue(m, "result")
		v := m.GetCounter().GetValue()
		a, ok := byLang[lang]
		if !ok {
			a = &acc{}
			byLang[lang] = a
		}
		switch result {
		case "hit":
			a.hits += v
		case "miss":
			a.misses += v
		}
	}
	rows := make([]CacheRow, 0, len(byLang))
	for lang, a := range byLang {
		denom := a.hits + a.misses
		row := CacheRow{Family: family, Language: lang}
		if denom > 0 {
			row.HitRatePct = 100.0 * a.hits / denom
			row.HasData = true
		}
		rows = append(rows, row)
	}
	return rows
}

// humanBytes formats a byte count with binary units (KiB/MiB/GiB).
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	suffix := []string{"KiB", "MiB", "GiB", "TiB"}[exp]
	return fmt.Sprintf("%.1f %s", float64(n)/float64(div), suffix)
}
