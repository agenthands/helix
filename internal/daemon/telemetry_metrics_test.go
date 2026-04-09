// Phase 11 METRIC-01: /metrics endpoint smoke test on the admin listener.
//
// Verifies:
//   - GET /metrics returns 200
//   - Content-Type is text/plain (Prometheus exposition format)
//   - Body contains the six serena_* metric families plus the Go + Process
//     runtime collectors (METRIC-04)
//   - /healthz and /readyz still work alongside /metrics (Phase 10 regression)
package daemon

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/postfix/serena/internal/config"
	"github.com/postfix/serena/internal/obs"
)

// adminDaemonWithMetrics builds a Daemon with the admin listener configured
// for a loopback random-port bind AND a populated obs.Provider so /metrics
// has a registry to expose.
func adminDaemonWithMetrics() *Daemon {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &Daemon{
		config: &config.SerenaConfig{
			Observability: config.ObservabilityConfig{AdminAddr: "127.0.0.1:0"},
		},
		logger: logger,
		obs:    obs.Noop(logger.Handler()),
	}
}

func TestAdmin_MetricsEndpoint(t *testing.T) {
	resetReady(t)
	d := adminDaemonWithMetrics()

	// Prime every vector so every family is present in Gather() output
	// (CounterVec / HistogramVec / GaugeVec with no labelled series yield
	// an empty family that promhttp drops from the exposition).
	m := d.obs.Metrics()
	m.ToolCalls.WithLabelValues("read_file", "claude-code", "read", "go", "success").Inc()
	m.ToolDuration.WithLabelValues("read_file", "claude-code", "read", "go").Observe(0.001)
	m.LSPoolWorkersSet("go", 1)
	m.LSPoolEviction("go", "idle")
	m.LSPoolCircuitStateSet("go", 0)
	m.LSPoolRestart("go")

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- d.listenAdmin(ctx) }()
	defer func() {
		cancel()
		select {
		case <-errCh:
		case <-time.After(5 * time.Second):
			t.Fatal("listenAdmin did not exit within 5s of ctx cancel")
		}
	}()

	addr := waitForAdminAddr(t, 2*time.Second)

	// Poll /metrics until it returns 200 (Serve loop may need a beat).
	var body string
	var contentType string
	var status int
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://" + addr + "/metrics")
		if err == nil {
			status = resp.StatusCode
			contentType = resp.Header.Get("Content-Type")
			b, _ := io.ReadAll(resp.Body)
			body = string(b)
			_ = resp.Body.Close()
			if status == http.StatusOK {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if status != http.StatusOK {
		t.Fatalf("/metrics status = %d, want 200", status)
	}
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("Content-Type = %q, want contains text/plain", contentType)
	}

	// Serena-owned families (METRIC-01).
	wantSerena := []string{
		"serena_tool_calls_total",
		"serena_tool_duration_seconds",
		"serena_lspool_workers",
		"serena_lspool_evictions_total",
		"serena_lspool_circuit_state",
		"serena_lspool_restarts_total",
	}
	for _, name := range wantSerena {
		if !strings.Contains(body, name) {
			t.Errorf("/metrics body missing %q", name)
		}
	}

	// Runtime collectors (METRIC-04).
	wantRuntime := []string{
		"go_goroutines",
		"process_resident_memory_bytes",
	}
	for _, name := range wantRuntime {
		if !strings.Contains(body, name) {
			t.Errorf("/metrics body missing runtime metric %q", name)
		}
	}

	// Specific sample observations we primed above should round-trip through
	// the exposition so we know the labels reach the scraper.
	if !strings.Contains(body, `tool_name="read_file"`) {
		t.Errorf("/metrics body missing primed sample for tool_name=read_file")
	}
}

// TestAdmin_MetricsRegressionHealthReadyz is the Phase 10 regression guard:
// /healthz and /readyz must keep working after /metrics joins the mux.
func TestAdmin_MetricsRegressionHealthReadyz(t *testing.T) {
	resetReady(t)
	ready.Store(1) // so /readyz returns 200
	d := adminDaemonWithMetrics()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- d.listenAdmin(ctx) }()
	defer func() {
		cancel()
		select {
		case <-errCh:
		case <-time.After(5 * time.Second):
			t.Fatal("listenAdmin did not exit within 5s of ctx cancel")
		}
	}()

	addr := waitForAdminAddr(t, 2*time.Second)

	// Probe all three endpoints.
	check := func(path string, wantStatus int) {
		t.Helper()
		var status int
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			resp, err := http.Get("http://" + addr + path)
			if err == nil {
				status = resp.StatusCode
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
				if status == wantStatus {
					return
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Errorf("%s status = %d, want %d", path, status, wantStatus)
	}
	check("/healthz", http.StatusOK)
	check("/readyz", http.StatusOK)
	check("/metrics", http.StatusOK)
}
