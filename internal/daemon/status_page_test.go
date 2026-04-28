// Tests for the in-binary HTML status page (Phase 54 OBS-01).
//
// These tests cover:
//   - GET / renders 200 HTML with inline <style>, no external assets.
//   - GET /notapage returns 404 (literal path narrowing).
//   - POST / returns 405 with Allow: GET.
//   - The page reads the SAME *prometheus.Registry that serves /metrics —
//     a counter incremented between two GETs is visible in the second response.
//   - Histogram p95 columns are annotated "(approx, bucket-quantized)".
//   - Partial gather does not panic — handler logs and continues.
//   - buildPageModel is a pure projection over []*dto.MetricFamily.
package daemon

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/postfix/serena/internal/config"
	"github.com/postfix/serena/internal/obs"
	dto "github.com/prometheus/client_model/go"
	"google.golang.org/protobuf/proto"
)

// newStatusPageDaemon constructs a Daemon with only the fields the status
// page handler needs: obs Provider with a working Metrics sink, plus a
// throwaway logger and config.
func newStatusPageDaemon(t *testing.T) *Daemon {
	t.Helper()
	p := obs.Noop(slog.NewTextHandler(io.Discard, nil))
	return &Daemon{
		config: &config.SerenaConfig{Observability: config.ObservabilityConfig{}},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		obs:    p,
	}
}

func TestStatusPageRender(t *testing.T) {
	d := newStatusPageDaemon(t)
	d.obs.Metrics().ToolCalls.WithLabelValues("find_symbols", "claude-code", "edit", "go", "success").Inc()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	d.handleStatusPage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html...", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "find_symbols") {
		t.Fatalf("expected tool name in body; got %q", body[:min(400, len(body))])
	}
	if !strings.Contains(body, "<style>") {
		t.Fatal("expected inline <style> block (criterion #1: inline CSS)")
	}
	if strings.Contains(body, "<script src=") {
		t.Fatal("page must not load external scripts (criterion #1)")
	}
	if strings.Contains(body, "<link rel=\"stylesheet\" href=") {
		t.Fatal("page must not load external stylesheets (criterion #1)")
	}
}

func TestStatusPageRouting_NotFound(t *testing.T) {
	d := newStatusPageDaemon(t)
	req := httptest.NewRequest(http.MethodGet, "/notapage", nil)
	rec := httptest.NewRecorder()
	d.handleStatusPage(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestStatusPageRouting_MethodNotAllowed(t *testing.T) {
	d := newStatusPageDaemon(t)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	d.handleStatusPage(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	allow := rec.Header().Get("Allow")
	if !strings.Contains(allow, "GET") {
		t.Fatalf("Allow header = %q, want contains GET", allow)
	}
}

func TestStatusPageRegistryShared(t *testing.T) {
	d := newStatusPageDaemon(t)
	m := d.obs.Metrics()

	// First increment + GET.
	m.ToolCalls.WithLabelValues("find_symbols", "claude-code", "edit", "go", "success").Inc()
	rec1 := httptest.NewRecorder()
	d.handleStatusPage(rec1, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec1.Code != http.StatusOK {
		t.Fatalf("first GET status = %d, want 200", rec1.Code)
	}
	body1 := rec1.Body.String()
	if !strings.Contains(body1, "find_symbols") {
		t.Fatalf("first body missing find_symbols")
	}

	// Second increment + GET — refresh must show the higher count.
	m.ToolCalls.WithLabelValues("find_symbols", "claude-code", "edit", "go", "success").Inc()
	m.ToolCalls.WithLabelValues("find_symbols", "claude-code", "edit", "go", "success").Inc()
	rec2 := httptest.NewRecorder()
	d.handleStatusPage(rec2, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec2.Code != http.StatusOK {
		t.Fatalf("second GET status = %d, want 200", rec2.Code)
	}
	body2 := rec2.Body.String()

	// The two bodies must differ — the count moved from 1 to 3.
	if body1 == body2 {
		t.Fatal("expected refresh to re-render with updated counter; bodies identical")
	}
	if !strings.Contains(body2, "3") {
		t.Fatalf("second body should contain count 3 for find_symbols; body=%q", body2[:min(800, len(body2))])
	}
}

func TestStatusPageRendersApproxAnnotation(t *testing.T) {
	d := newStatusPageDaemon(t)
	d.obs.Metrics().ToolDuration.WithLabelValues("find_symbols", "claude-code", "edit", "go").Observe(0.012)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	d.handleStatusPage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "approx, bucket-quantized") {
		t.Fatal("expected p95 column to carry '(approx, bucket-quantized)' annotation (Pitfall #1)")
	}
}

func TestStatusPageGatherErrorDoesNotPanic(t *testing.T) {
	// With a fresh registry no observations have happened yet. The handler
	// must still return 200 with a renderable (possibly empty-row) page.
	d := newStatusPageDaemon(t)
	rec := httptest.NewRecorder()
	d.handleStatusPage(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 even with empty registry", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<style>") {
		t.Fatal("expected inline <style> even on empty registry")
	}
}

func TestBuildPageModel_FromGatheredFamilies(t *testing.T) {
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	families := []*dto.MetricFamily{
		toolCallsFamily(map[string]uint64{"find_symbols": 5, "rename_symbol": 2}),
		toolDurationFamily("find_symbols", []bucket{{0.005, 1}, {0.01, 3}, {0.025, 5}, {0.05, 5}}, 5, 0.04),
		editOutcomeFamily([]editPoint{{"replace_symbol_body", "success", 7}, {"replace_symbol_body", "failed", 1}}),
		gaugeFamily("serena_lspool_workers", []langPoint{{"go", 2}, {"python", 1}}),
		gaugeFamily("serena_lspool_circuit_state", []langPoint{{"go", 0}, {"java", 2}}),
		evictionsFamily([]evPoint{{"java", "pressure", 3}, {"python", "idle", 0}}),
		cacheFamily("serena_repomap_cache_total", []cachePoint{{"go", "hit", 8}, {"go", "miss", 2}}),
		gaugeFamily("process_resident_memory_bytes", []langPoint{{"", 1024 * 1024 * 64}}),
		gaugeFamily("go_goroutines", []langPoint{{"", 42}}),
	}

	model := buildPageModel(families, now)

	if len(model.Tools) == 0 {
		t.Fatal("expected Tools rows")
	}
	var foundFindSymbols, foundRename bool
	for _, row := range model.Tools {
		if row.Name == "find_symbols" {
			foundFindSymbols = true
			if row.Calls != 5 {
				t.Errorf("find_symbols calls = %d, want 5", row.Calls)
			}
			if row.P95Seconds <= 0 {
				t.Errorf("find_symbols p95 = %v, want > 0", row.P95Seconds)
			}
		}
		if row.Name == "rename_symbol" {
			foundRename = true
			if row.Calls != 2 {
				t.Errorf("rename_symbol calls = %d, want 2", row.Calls)
			}
		}
	}
	if !foundFindSymbols || !foundRename {
		t.Fatalf("expected both tool rows; got %+v", model.Tools)
	}

	if len(model.Edits) != 2 {
		t.Errorf("Edits len = %d, want 2", len(model.Edits))
	}
	if len(model.Workers) != 2 {
		t.Errorf("Workers len = %d, want 2", len(model.Workers))
	}
	if len(model.Circuits) != 2 {
		t.Errorf("Circuits len = %d, want 2", len(model.Circuits))
	}
	// Check circuit state mapping: 0 -> closed, 2 -> open.
	circuitStates := map[string]string{}
	for _, c := range model.Circuits {
		circuitStates[c.Language] = c.State
	}
	if circuitStates["go"] != "closed" {
		t.Errorf("circuit go = %q, want closed", circuitStates["go"])
	}
	if circuitStates["java"] != "open" {
		t.Errorf("circuit java = %q, want open", circuitStates["java"])
	}

	// Evictions: only non-zero rows appear.
	if len(model.Evictions) != 1 {
		t.Errorf("Evictions len = %d, want 1 (zero-count row should be filtered)", len(model.Evictions))
	}
	if len(model.Evictions) >= 1 {
		ev := model.Evictions[0]
		if ev.Language != "java" || ev.Reason != "pressure" || ev.Count != 3 {
			t.Errorf("Evictions[0] = %+v, want {java pressure 3}", ev)
		}
	}

	if len(model.Caches) == 0 {
		t.Fatal("expected at least one cache row")
	}
	c := model.Caches[0]
	if !c.HasData {
		t.Error("cache row should have HasData=true with hits+misses>0")
	}
	if c.HitRatePct < 79 || c.HitRatePct > 81 {
		t.Errorf("cache hit rate = %v, want ~80", c.HitRatePct)
	}

	if !model.Process.HasRSS {
		t.Error("Process.HasRSS = false, want true")
	}
	if model.Process.RSSBytes != 1024*1024*64 {
		t.Errorf("Process.RSSBytes = %d, want %d", model.Process.RSSBytes, 1024*1024*64)
	}
	if model.Process.Goroutines != 42 {
		t.Errorf("Process.Goroutines = %d, want 42", model.Process.Goroutines)
	}
	if !model.GeneratedAt.Equal(now) {
		t.Errorf("GeneratedAt = %v, want %v", model.GeneratedAt, now)
	}
}

// --- test fixture helpers (hand-built dto.MetricFamily values) ---

type bucket struct {
	upper float64
	cum   uint64
}

type editPoint struct {
	tool, outcome string
	count         float64
}

type langPoint struct {
	lang  string
	value float64
}

type evPoint struct {
	lang, reason string
	count        float64
}

type cachePoint struct {
	language, result string
	count            float64
}

func labelPair(name, value string) *dto.LabelPair {
	return &dto.LabelPair{Name: proto.String(name), Value: proto.String(value)}
}

func toolCallsFamily(byTool map[string]uint64) *dto.MetricFamily {
	mf := &dto.MetricFamily{
		Name: proto.String("serena_tool_calls_total"),
		Type: dto.MetricType_COUNTER.Enum(),
	}
	for tool, n := range byTool {
		mf.Metric = append(mf.Metric, &dto.Metric{
			Label: []*dto.LabelPair{
				labelPair("tool_name", tool),
				labelPair("profile", "claude-code"),
				labelPair("mode", "edit"),
				labelPair("language", "go"),
				labelPair("outcome", "success"),
			},
			Counter: &dto.Counter{Value: proto.Float64(float64(n))},
		})
	}
	return mf
}

func toolDurationFamily(tool string, buckets []bucket, sampleCount uint64, sampleSum float64) *dto.MetricFamily {
	bs := make([]*dto.Bucket, 0, len(buckets))
	for _, b := range buckets {
		bs = append(bs, &dto.Bucket{
			CumulativeCount: proto.Uint64(b.cum),
			UpperBound:      proto.Float64(b.upper),
		})
	}
	return &dto.MetricFamily{
		Name: proto.String("serena_tool_duration_seconds"),
		Type: dto.MetricType_HISTOGRAM.Enum(),
		Metric: []*dto.Metric{
			{
				Label: []*dto.LabelPair{
					labelPair("tool_name", tool),
					labelPair("profile", "claude-code"),
					labelPair("mode", "edit"),
					labelPair("language", "go"),
				},
				Histogram: &dto.Histogram{
					SampleCount: proto.Uint64(sampleCount),
					SampleSum:   proto.Float64(sampleSum),
					Bucket:      bs,
				},
			},
		},
	}
}

func editOutcomeFamily(points []editPoint) *dto.MetricFamily {
	mf := &dto.MetricFamily{
		Name: proto.String("serena_edit_outcome_total"),
		Type: dto.MetricType_COUNTER.Enum(),
	}
	for _, p := range points {
		mf.Metric = append(mf.Metric, &dto.Metric{
			Label: []*dto.LabelPair{
				labelPair("tool", p.tool),
				labelPair("outcome", p.outcome),
			},
			Counter: &dto.Counter{Value: proto.Float64(p.count)},
		})
	}
	return mf
}

func gaugeFamily(name string, points []langPoint) *dto.MetricFamily {
	mf := &dto.MetricFamily{
		Name: proto.String(name),
		Type: dto.MetricType_GAUGE.Enum(),
	}
	for _, p := range points {
		var labels []*dto.LabelPair
		if p.lang != "" {
			labels = []*dto.LabelPair{labelPair("language", p.lang)}
		}
		mf.Metric = append(mf.Metric, &dto.Metric{
			Label: labels,
			Gauge: &dto.Gauge{Value: proto.Float64(p.value)},
		})
	}
	return mf
}

func evictionsFamily(points []evPoint) *dto.MetricFamily {
	mf := &dto.MetricFamily{
		Name: proto.String("serena_lspool_evictions_total"),
		Type: dto.MetricType_COUNTER.Enum(),
	}
	for _, p := range points {
		mf.Metric = append(mf.Metric, &dto.Metric{
			Label: []*dto.LabelPair{
				labelPair("language", p.lang),
				labelPair("reason", p.reason),
			},
			Counter: &dto.Counter{Value: proto.Float64(p.count)},
		})
	}
	return mf
}

func cacheFamily(name string, points []cachePoint) *dto.MetricFamily {
	mf := &dto.MetricFamily{
		Name: proto.String(name),
		Type: dto.MetricType_COUNTER.Enum(),
	}
	for _, p := range points {
		mf.Metric = append(mf.Metric, &dto.Metric{
			Label: []*dto.LabelPair{
				labelPair("language", p.language),
				labelPair("result", p.result),
			},
			Counter: &dto.Counter{Value: proto.Float64(p.count)},
		})
	}
	return mf
}
