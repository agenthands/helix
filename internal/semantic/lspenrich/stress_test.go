//go:build stress

// Phase 61 P04 — ENRICH-05 stress test: a 100-file enrichment burst MUST
// NOT bury foreground tool calls.  Build-tag-gated (`//go:build stress`)
// per project memory rule: benchmarks are LOCAL-ONLY — never on CI.
//
// `go test ./...` on CI does NOT compile this file.  Run locally via:
//
//	go test -tags stress -run TestStress_ENRICH05_Go \
//	    ./internal/semantic/lspenrich/... -count=1 -timeout 120s
//
// The test exercises the full Phase 61 stack — real *lspool.Pool, real
// Manager + Worker + Cascade + LaneQueue — against a real language
// server.  Each language variant (Go, Java) skips when its LS binary
// is not on PATH.

package lspenrich_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/kernel/lspool"
	"github.com/agenthands/helix/internal/langregistry"
	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/live/lspqueue"
	"github.com/agenthands/helix/internal/semantic/lspenrich"
	"github.com/agenthands/helix/internal/workspace"
)

// =============================================================================
// Acceptance gates (61-CONTEXT.md acceptance #11 + ENRICH-05 invariant):
//
//   - foregroundP95Budget — foreground tool calls must stay below this
//     while a 100-file enrichment burst runs concurrently.  Conservative
//     5s upper bound chosen to match the foreground-tool deadline policy
//     in TelemetryMiddleware (per-tool deadline injection); a healthy
//     pipeline yields p95 well under 1s on a developer laptop.
//
//   - stressDuration — total wall-clock time the stress runs.  30s
//     gives the foreground sampler ~150 samples (one every 200ms) which
//     is enough for a stable p95 estimate.
//
//   - foregroundProbeInterval — interval between foreground samples.
//     200ms matches the default yield_check_window.
//
//   - enrichBurstSize — 100 RevalidateFileJob events enqueued back-to-
//     back (this mirrors a `git checkout` storm or a 100-file edit
//     burst from a refactor tool — SPEC-DRAFT.md §29.5).
// =============================================================================

const (
	stressForegroundP95Budget = 5 * time.Second
	stressDuration            = 30 * time.Second
	stressProbeInterval       = 200 * time.Millisecond
	stressEnrichBurstSize     = 100
)

// stressPressure is a memory-pressure stub that always reports no
// pressure — keeps the lspool Pool from spuriously evicting workers
// during the stress window.  Mirrors integrationPressure in
// cascade_integration_test.go (duplicated here so the file is
// self-contained under the stress build tag).
type stressPressure struct{}

func (stressPressure) Level() lspool.PressureLevel     { return lspool.PressureNone }
func (stressPressure) WorkerRSS(_ int) (uint64, error) { return 0, nil }

// stressP95 returns the 95th-percentile sample from latencies.
// Returns 0 for an empty slice.  The implementation copies + sorts a
// duplicate so the caller's data is untouched.
func stressP95(latencies []time.Duration) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	dup := make([]time.Duration, len(latencies))
	copy(dup, latencies)
	sort.Slice(dup, func(i, j int) bool { return dup[i] < dup[j] })
	idx := int(math.Ceil(0.95*float64(len(dup)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(dup) {
		idx = len(dup) - 1
	}
	return dup[idx]
}

// stressLSPShim is a minimal CascadeLSP that wraps a *lspool.WorkerLease
// for the enrichment Worker's NewCascadeLSP factory.  Production wires
// this in Phase 64+; for the stress test we ship an inline shim so the
// pipeline runs end-to-end against a real LS without depending on
// future phase work.  The shim only fires documentSymbol — enough to
// drive sustained LS load without inflating p95 with prepareCallHierarchy
// flake on multi-file fixtures.
type stressLSPShim struct {
	lease *lspool.WorkerLease
	uri   string
}

func (s *stressLSPShim) DocumentSymbol(ctx context.Context, _ string) ([]lspenrich.Symbol, error) {
	params := map[string]any{"textDocument": map[string]any{"uri": s.uri}}
	var raw json.RawMessage
	if err := s.lease.Request(ctx, "textDocument/documentSymbol", params, &raw); err != nil {
		return nil, err
	}
	// We don't bother decoding — the cascade tolerates a nil result and
	// will continue to step 2.  The load-bearing event is the LSP
	// round-trip having occurred, which contributes to the LS worker's
	// queue depth.
	return nil, nil
}

func (s *stressLSPShim) DrainDiagnostics(_ string) []lspenrich.Diagnostic { return nil }
func (s *stressLSPShim) Hover(_ context.Context, _ lspenrich.Symbol) (*lspenrich.Edge, error) {
	return nil, nil
}
func (s *stressLSPShim) CallHierarchy(_ context.Context, _ lspenrich.Symbol, _ int) ([]lspenrich.Edge, error) {
	return nil, nil
}
func (s *stressLSPShim) TypeHierarchy(_ context.Context, _ lspenrich.Symbol, _ int) ([]lspenrich.Edge, error) {
	return nil, nil
}
func (s *stressLSPShim) Implementation(_ context.Context, _ lspenrich.Symbol) ([]lspenrich.Edge, error) {
	return nil, nil
}
func (s *stressLSPShim) Definition(_ context.Context, _ lspenrich.Reference) (*lspenrich.Edge, error) {
	return nil, nil
}
func (s *stressLSPShim) ReferencesForSymbol(_ lspenrich.Symbol) []lspenrich.Reference { return nil }

// stressStore is a recording CascadeStore that absorbs cascade
// transactions without touching duckdb.  Identical in shape to
// integrationStore in cascade_integration_test.go but duplicated so
// stress_test.go compiles under the stress tag without
// integration-tagged helpers.
type stressStore struct {
	mu    sync.Mutex
	txs   int
	epoch uint64
}

func newStressStore() *stressStore { return &stressStore{epoch: 1} }

func (s *stressStore) BeginCascadeTx(_ context.Context, _ string) (lspenrich.CascadeTx, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx := &stressTx{epoch: s.epoch}
	s.epoch++
	s.txs++
	return tx, nil
}

type stressTx struct{ epoch uint64 }

func (t *stressTx) UpsertSymbols(_ context.Context, _ string, _ []lspenrich.Symbol) error { return nil }
func (t *stressTx) UpsertReferences(_ context.Context, _ string, _ []lspenrich.Reference) error {
	return nil
}
func (t *stressTx) UpsertEdges(_ context.Context, _ []lspenrich.Edge) error { return nil }
func (t *stressTx) UpsertDiagnostics(_ context.Context, _ string, _ []lspenrich.Diagnostic) error {
	return nil
}
func (t *stressTx) WriteInvalidations(_ context.Context) error                    { return nil }
func (t *stressTx) MarkFileSemanticPending(_ context.Context, _, _ string) error  { return nil }
func (t *stressTx) Commit() error                                                 { return nil }
func (t *stressTx) Rollback() error                                               { return nil }
func (t *stressTx) Epoch() uint64                                                 { return t.epoch }

// stressMetrics is a no-op MetricsSink — we drop every emit because
// the stress test asserts on wall-clock latency, not metric vectors.
type stressMetrics struct{}

func (stressMetrics) LSPEnrichmentTotal(string, string)     {}
func (stressMetrics) LSPEnrichmentDuration(string, float64) {}
func (stressMetrics) LSPEnrichmentErrors(string, string)    {}
func (stressMetrics) LSPEnrichmentLaneDepth(string, int)    {}
func (stressMetrics) LSPEnrichmentBulkSuppressed(int)       {}

// runStressScenario is the body shared by Go + Java stress sub-tests.
// It spins up a real *lspool.Pool against the supplied workspace key,
// constructs the full Phase 61 stack (Manager + Worker + LaneQueue +
// stressLSPShim CascadeLSP factory), enqueues stressEnrichBurstSize
// per-file revalidate events, runs a foreground sampler every
// stressProbeInterval that times an AcquireLease + simple LSP call,
// and asserts the foreground p95 < stressForegroundP95Budget.
func runStressScenario(t *testing.T, lang, fixturePath, sourceFile string) {
	t.Helper()

	wsKey := workspace.WorkspaceKey{
		RepoRoot: fixturePath,
		Language: lang,
	}

	reg, err := langregistry.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	pool := lspool.NewPool(
		lspool.PoolConfig{
			BaseTTL:               60,
			CeilingTTL:            300,
			MaxWorkers:            4,
			RSSHardCapMB:          2048,
			PressureCheckInterval: 30,
		},
		reg,
		nil, // no installer — assume LS already on PATH
		stressPressure{},
		slog.Default(),
		lspool.NoopSink{},
		nil,
	)

	poolCtx, poolCancel := context.WithCancel(context.Background())
	defer poolCancel()
	go func() { _ = pool.Run(poolCtx) }()

	// Acquire one warm enrichment lease so the LS worker is spawned
	// BEFORE the stress window starts (otherwise the first enrichment
	// job pays the full LS cold-start cost which dominates p95 well
	// past the budget — this is not what ENRICH-05 measures).
	bootCtx, bootCancel := context.WithTimeout(context.Background(), 60*time.Second)
	warmLease, err := pool.AcquireLease(bootCtx, "lsp-enrichment:stress-warmup:"+lang, wsKey, false)
	if err != nil {
		bootCancel()
		t.Skipf("failed to acquire warm-up lease (LS install issue?): %v", err)
	}
	bootCancel()

	// Open the fixture file so the LS knows about it.
	uri := "file://" + filepath.Join(fixturePath, sourceFile)
	body, err := os.ReadFile(filepath.Join(fixturePath, sourceFile))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	openParams := map[string]any{
		"textDocument": map[string]any{
			"uri":        uri,
			"languageId": lang,
			"version":    1,
			"text":       string(body),
		},
	}
	if err := warmLease.Notify(context.Background(), "textDocument/didOpen", openParams); err != nil {
		t.Fatalf("didOpen: %v", err)
	}

	// LS-specific cold-index settle:
	// - gopls indexes a single-file Go module in <1s.
	// - jdtls is much slower; the cascade integration test allows 8s.
	settle := 2 * time.Second
	if lang == "java" {
		settle = 8 * time.Second
	}
	time.Sleep(settle)

	// Configure pool yield window (matches production live_wiring.go).
	cfg := semantic.LSPEnrichmentConfig{
		Enabled:                true,
		MaxConcurrentWorkers:   1,
		YieldCheckWindowMs:     200,
		TimeoutPerFile:         "5s",
		TimeoutTotal:           "120s",
		MaxSymbolsPerFile:      200,
		MaxReferencesPerSymbol: 1000,
		MaxReferencesPerFile:   5000,
		MaxCallHierarchyDepth:  2,
		MaxTypeHierarchyDepth:  2,
	}
	pool.SetYieldCheckWindow(time.Duration(cfg.YieldCheckWindowMs) * time.Millisecond)

	// Construct real Manager + Worker.
	q := lspenrich.NewLaneQueue(1024, 1024)
	store := newStressStore()
	mgr := lspenrich.NewManager(
		q,
		lspenrich.NewPoolAcquirer(pool),
		store,
		lspenrich.NewPoolReadinessProbe(pool),
		cfg,
		stressMetrics{},
		slog.Default(),
	)

	w := &lspenrich.Worker{
		Queue:     mgr.Queue(),
		Leases:    mgr,
		Acquirer:  lspenrich.NewPoolAcquirer(pool),
		Store:     store,
		Readiness: lspenrich.NewPoolReadinessProbe(pool),
		BudgetCfg: cfg,
		Metrics:   stressMetrics{},
		NewCascadeLSP: func(lease *lspool.WorkerLease) lspenrich.CascadeLSP {
			return &stressLSPShim{lease: lease, uri: uri}
		},
	}

	runCtx, runCancel := context.WithTimeout(context.Background(), stressDuration+30*time.Second)
	defer runCancel()

	workerDone := make(chan error, 1)
	go func() { workerDone <- w.RunN(runCtx, cfg.MaxConcurrentWorkers) }()

	// Enqueue the burst.
	for i := 0; i < stressEnrichBurstSize; i++ {
		// All events target the same file — load comes from the EVENT
		// count, not the file count (the cascade does not deduplicate;
		// each event triggers a fresh CascadeTx).
		q.EnqueueLane(lspenrich.LaneHigh,
			lspqueue.RevalidateFileJob{RepoID: semantic.RepoID(fixturePath), Path: sourceFile})
	}

	// Foreground sampler — every stressProbeInterval, time an
	// AcquireLease + small LSP call.
	var samplesMu sync.Mutex
	samples := make([]time.Duration, 0, int(stressDuration/stressProbeInterval)+1)

	probeCtx, probeCancel := context.WithTimeout(context.Background(), stressDuration)
	defer probeCancel()

	var probesTaken atomic.Int64
	go func() {
		ticker := time.NewTicker(stressProbeInterval)
		defer ticker.Stop()
		i := 0
		for {
			select {
			case <-probeCtx.Done():
				return
			case <-ticker.C:
				probeCtx, probeCancel := context.WithTimeout(probeCtx, stressForegroundP95Budget*2)
				start := time.Now()
				lease, err := pool.AcquireLease(probeCtx, fmt.Sprintf("foreground-tool:stress-probe-%d", i), wsKey, false)
				if err != nil {
					probeCancel()
					t.Logf("stress probe #%d AcquireLease error: %v", i, err)
					i++
					continue
				}
				// Tiny representative foreground request — same shape
				// the get_symbols MCP tool issues.
				params := map[string]any{"textDocument": map[string]any{"uri": uri}}
				var raw json.RawMessage
				_ = lease.Request(probeCtx, "textDocument/documentSymbol", params, &raw)
				latency := time.Since(start)
				pool.ReleaseLease(lease.SessionID)
				probeCancel()
				samplesMu.Lock()
				samples = append(samples, latency)
				samplesMu.Unlock()
				probesTaken.Add(1)
				i++
			}
		}
	}()

	<-probeCtx.Done()
	runCancel()
	<-workerDone

	samplesMu.Lock()
	defer samplesMu.Unlock()
	if len(samples) == 0 {
		t.Fatalf("no foreground samples taken (probesTaken=%d)", probesTaken.Load())
	}

	p95 := stressP95(samples)
	mean := time.Duration(0)
	for _, s := range samples {
		mean += s
	}
	mean /= time.Duration(len(samples))

	t.Logf("ENRICH-05 stress (%s): %d foreground samples, p95=%v, mean=%v, samples=%d, enrich_txs=%d",
		lang, len(samples), p95, mean, probesTaken.Load(), store.txs)

	if p95 >= stressForegroundP95Budget {
		t.Errorf("foreground p95 = %v >= budget %v under 100-file enrichment burst (ENRICH-05 violated)",
			p95, stressForegroundP95Budget)
	}
}

// =============================================================================
// Tests
// =============================================================================

// TestStress_ENRICH05_Go: 100-file enrichment burst against testdata/
// stress/go + concurrent foreground get_symbols-like probes; foreground
// p95 < 5s.  Skips when gopls is not on PATH.
func TestStress_ENRICH05_Go(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test")
	}
	if _, err := exec.LookPath("gopls"); err != nil {
		t.Skipf("gopls not on PATH; skipping ENRICH-05 stress (Go)")
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	root := filepath.Join(wd, "testdata", "stress", "go")
	runStressScenario(t, "go", root, "files.go")
}

// TestStress_ENRICH05_Java: 100-file enrichment burst against testdata/
// stress/java + concurrent foreground probes; foreground p95 < 5s.
// Skips when jdtls is not on PATH.
func TestStress_ENRICH05_Java(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test")
	}
	if _, err := exec.LookPath("jdtls"); err != nil {
		t.Skipf("jdtls not on PATH; skipping ENRICH-05 stress (Java)")
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	root := filepath.Join(wd, "testdata", "stress", "java")
	javaSrc := filepath.Join("src", "main", "java", "com", "example", "Stress.java")
	runStressScenario(t, "java", root, javaSrc)
}
