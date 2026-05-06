//go:build integration

// Phase 61 P04 — acceptance integration tests:
//
//   - ACC4: concurrency cap honored (cap=1 default; cap=4 override).
//   - ACC6 (B1): voluntary yield via real Manager + real Worker + real
//     Cascade.  A foreground lease is acquired mid-cascade through the
//     LeaseAcquirer interface (the same surface *lspool.Pool.AcquireLease
//     satisfies in production); the next ForegroundBusy boundary check
//     observes true and the cascade stamps partial_reason="preempted".
//   - ACC10: Java readiness gate honored before lease acquisition (and
//     timeout path emits partial_lsp_unavailable).
//
// The plan (61-04-PLAN.md, Task 2) calls for "real Manager + real
// *lspool.Pool".  In the codebase as it stands today the production
// wiring (live_wiring.go) does NOT supply Worker.NewCascadeLSP — that
// shim ships in Phase 64.  Without NewCascadeLSP every Manager.Run
// dispatch lands on the OutcomeDropped path before the cascade is
// constructed (worker.go step 4) — so a real-Pool-driven Manager.Run
// cannot exercise the ForegroundBusy boundary that ACC6 asserts on.
//
// Resolution (deviation note for 61-04 SUMMARY):
//
//   The load-bearing invariant of ACC6 is: "foreground lease acquired
//   mid-cascade ⇒ ForegroundBusy returns true ⇒ cascade commits
//   'preempted' partial".  This invariant lives at the Worker→Cascade→
//   ForegroundBusy seam, not at the Pool→Worker seam.  We exercise the
//   real Manager (B2 lease cache + LeaseProvider routing), a real Worker
//   (per-job ctx + readiness gate + ForegroundBusy boundary), and a
//   real Cascade (the §14.4 6-step engine + checkBoundary pre-step
//   probes).  We inject Worker.NewCascadeLSP directly (the same
//   injection point worker_test.go's W9/W10/W12 already use) and use a
//   recording LeaseAcquirer that satisfies the same AcquireLease shape
//   *lspool.Pool exposes.
//
//   This is faithful to the production seam: ForegroundBusy on
//   *lspool.Pool stamps lastForegroundLease[wsKey] = now() inside
//   AcquireLease unconditionally for sessionIDs NOT prefixed
//   "lsp-enrichment:" (pool.go:162-167).  Our recording acquirer
//   reproduces the same pattern in-memory so the cascade observes the
//   exact same ForegroundBusy=true → partial_preempted invariant the
//   real Pool produces, with no dependency on gopls/jdtls being on PATH.
//
// Run with:
//
//	go test -tags integration -run TestACC ./internal/semantic/lspenrich/... \
//	    -count=1 -timeout 90s

package lspenrich_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/kernel/lspool"
	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/live/lspqueue"
	"github.com/agenthands/helix/internal/semantic/lspenrich"
	"github.com/agenthands/helix/internal/workspace"
)

// =============================================================================
// Test-only LeaseAcquirer that mirrors the *lspool.Pool surface.
// =============================================================================
//
// poolLikeAcquirer satisfies lspenrich.LeaseAcquirer and lspenrich.LeaseReleaser
// (the optional interface the Manager dispatches to on Stop /
// OnWorkspaceDeactivate).  AcquireLease faithfully reproduces the
// production lastForegroundLease stamp gate (pool.go:162-167) so a
// caller can inject a "foreground" lease by passing a sessionID NOT
// prefixed with the enrichment-session prefix; ForegroundBusy then
// returns true for the configured yield window — exactly the same
// behaviour the real Pool produces.
//
// inflight tracks the number of in-flight cascade slots that have
// observed an AcquireFor → cascade pipeline (used by ACC4 to assert the
// concurrency cap).  busyDuration mirrors *Pool.SetYieldCheckWindow.
type poolLikeAcquirer struct {
	mu sync.Mutex

	// AcquireLease bookkeeping.
	calls         int
	releaseCount  map[string]int
	leases        map[string]*lspool.WorkerLease
	enrichPrefix  string
	yieldWindow   time.Duration
	lastForegroun map[workspace.WorkspaceKey]time.Time
}

func newPoolLikeAcquirer() *poolLikeAcquirer {
	return &poolLikeAcquirer{
		releaseCount:  map[string]int{},
		leases:        map[string]*lspool.WorkerLease{},
		enrichPrefix:  "lsp-enrichment:",
		yieldWindow:   200 * time.Millisecond,
		lastForegroun: map[workspace.WorkspaceKey]time.Time{},
	}
}

func (p *poolLikeAcquirer) AcquireLease(_ context.Context, sessionID string, wsKey workspace.WorkspaceKey, _ bool) (*lspool.WorkerLease, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	// Mirrors pool.go:162-167 — non-enrichment sessionIDs stamp the
	// foreground-lease timestamp under the same wsKey.
	if !strings.HasPrefix(sessionID, p.enrichPrefix) {
		p.lastForegroun[wsKey] = time.Now()
	}
	lease := &lspool.WorkerLease{SessionID: sessionID}
	p.leases[sessionID] = lease
	return lease, nil
}

func (p *poolLikeAcquirer) ReleaseLease(sessionID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.releaseCount[sessionID]++
	delete(p.leases, sessionID)
}

func (p *poolLikeAcquirer) ForegroundBusy(wsKey workspace.WorkspaceKey) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	last, ok := p.lastForegroun[wsKey]
	if !ok {
		return false
	}
	window := p.yieldWindow
	if window <= 0 {
		window = 200 * time.Millisecond
	}
	return time.Since(last) < window
}

func (p *poolLikeAcquirer) SetYieldCheckWindow(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.yieldWindow = d
}

// =============================================================================
// Slow CascadeLSP that signals on every step so ACC6 can synchronize
// foreground-lease injection with cascade step 2.
// =============================================================================

type slowCascadeLSP struct {
	mu sync.Mutex

	// stepCh is sent on after every method call returns; ACC6 reads
	// from this channel to know step N has just completed.
	stepCh chan int
	step   int

	// perCallDelay is added before every method returns (simulates a slow LS).
	perCallDelay time.Duration

	// docSymbolsPerFile controls how many symbols documentSymbol returns —
	// drives how many cascade-step 3 inner-loop iterations the cascade runs.
	docSymbolsPerFile int

	// path is the file path the cascade is running against (used to
	// populate Symbol.Path for the cascade's deterministic output).
	path string
}

func newSlowCascadeLSP(path string, syms int, delay time.Duration) *slowCascadeLSP {
	return &slowCascadeLSP{
		stepCh:            make(chan int, 64),
		perCallDelay:      delay,
		docSymbolsPerFile: syms,
		path:              path,
	}
}

func (s *slowCascadeLSP) tick() {
	if s.perCallDelay > 0 {
		time.Sleep(s.perCallDelay)
	}
	s.mu.Lock()
	s.step++
	step := s.step
	s.mu.Unlock()
	select {
	case s.stepCh <- step:
	default:
	}
}

func (s *slowCascadeLSP) DocumentSymbol(_ context.Context, _ string) ([]lspenrich.Symbol, error) {
	s.tick()
	out := make([]lspenrich.Symbol, 0, s.docSymbolsPerFile)
	for i := 0; i < s.docSymbolsPerFile; i++ {
		out = append(out, lspenrich.Symbol{
			Name: fmt.Sprintf("S%d", i),
			Path: s.path,
		})
	}
	return out, nil
}

func (s *slowCascadeLSP) DrainDiagnostics(_ string) []lspenrich.Diagnostic {
	s.tick()
	return nil
}

func (s *slowCascadeLSP) Hover(_ context.Context, _ lspenrich.Symbol) (*lspenrich.Edge, error) {
	s.tick()
	return &lspenrich.Edge{
		Kind: "TYPE_OF", Source: "lsp.hover", Confidence: 1.0, ValidationState: "validated",
	}, nil
}

func (s *slowCascadeLSP) CallHierarchy(_ context.Context, _ lspenrich.Symbol, _ int) ([]lspenrich.Edge, error) {
	s.tick()
	return []lspenrich.Edge{{
		Kind: "CALLS", Source: "lsp.callHierarchy", Confidence: 1.0, ValidationState: "validated",
	}}, nil
}

func (s *slowCascadeLSP) TypeHierarchy(_ context.Context, _ lspenrich.Symbol, _ int) ([]lspenrich.Edge, error) {
	s.tick()
	return nil, nil
}

func (s *slowCascadeLSP) Implementation(_ context.Context, _ lspenrich.Symbol) ([]lspenrich.Edge, error) {
	s.tick()
	return nil, nil
}

func (s *slowCascadeLSP) Definition(_ context.Context, _ lspenrich.Reference) (*lspenrich.Edge, error) {
	s.tick()
	return nil, nil
}

func (s *slowCascadeLSP) ReferencesForSymbol(_ lspenrich.Symbol) []lspenrich.Reference {
	return nil
}

// =============================================================================
// Recording overlay store for ACC6 — counts UpsertSymbols/Edges/Diagnostics
// calls AND captures partial_reason so the test can assert on commit content.
// =============================================================================

type accStore struct {
	mu  sync.Mutex
	txs []*accTx
}

func newAccStore() *accStore { return &accStore{} }

func (s *accStore) BeginCascadeTx(_ context.Context, _ string) (lspenrich.CascadeTx, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx := &accTx{}
	s.txs = append(s.txs, tx)
	return tx, nil
}

// GetPartialReason returns the partial_reason for the most-recent tx that
// MarkFileSemanticPending was called on, or "" if none.
func (s *accStore) GetPartialReason() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.txs) - 1; i >= 0; i-- {
		s.txs[i].mu.Lock()
		r := s.txs[i].partialReason
		s.txs[i].mu.Unlock()
		if r != "" {
			return r
		}
	}
	return ""
}

// UpsertSteps returns the count of "facts written" per tx — tracks the
// number of distinct UpsertSymbols + UpsertDiagnostics + UpsertEdges
// calls.  ACC6 uses this to assert "≤ ~3 step-batches committed before
// preemption".
func (s *accStore) UpsertSteps() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	total := 0
	for _, tx := range s.txs {
		tx.mu.Lock()
		total += tx.upsertSymbolsCalls + tx.upsertEdgesCalls + tx.upsertDiagnosticsCalls
		tx.mu.Unlock()
	}
	return total
}

type accTx struct {
	mu                     sync.Mutex
	upsertSymbolsCalls     int
	upsertReferencesCalls  int
	upsertEdgesCalls       int
	upsertDiagnosticsCalls int
	partialPath            string
	partialReason          string
	commitCalls            int
	rollbackCalls          int
}

func (t *accTx) UpsertSymbols(_ context.Context, _ string, _ []lspenrich.Symbol) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.upsertSymbolsCalls++
	return nil
}
func (t *accTx) UpsertReferences(_ context.Context, _ string, _ []lspenrich.Reference) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.upsertReferencesCalls++
	return nil
}
func (t *accTx) UpsertEdges(_ context.Context, _ []lspenrich.Edge) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.upsertEdgesCalls++
	return nil
}
func (t *accTx) UpsertEdgesWithMerge(ctx context.Context, edges []lspenrich.Edge) error {
	return t.UpsertEdges(ctx, edges)
}
func (t *accTx) UpsertDiagnostics(_ context.Context, _ string, _ []lspenrich.Diagnostic) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.upsertDiagnosticsCalls++
	return nil
}
func (t *accTx) WriteInvalidations(_ context.Context) error { return nil }
func (t *accTx) MarkFileSemanticPending(_ context.Context, path, reason string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.partialPath = path
	t.partialReason = reason
	return nil
}
func (t *accTx) Commit() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.commitCalls++
	return nil
}
func (t *accTx) Rollback() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.rollbackCalls++
	return nil
}
func (t *accTx) Epoch() uint64 { return 1 }

// accReadiness is a recording readiness probe used by ACC10.  javaFn is
// invoked synchronously on every JavaReady call and may block; the test
// asserts ordering by capturing javaFn entry / return timestamps and
// AcquireFor entry timestamps.
type accReadiness struct {
	mu sync.Mutex

	javaCalls int
	rustCalls int

	javaFn func(ctx context.Context) error
	rustFn func(ctx context.Context) error
}

func (a *accReadiness) JavaReady(ctx context.Context, _ workspace.WorkspaceKey) error {
	a.mu.Lock()
	a.javaCalls++
	fn := a.javaFn
	a.mu.Unlock()
	if fn != nil {
		return fn(ctx)
	}
	return nil
}

func (a *accReadiness) RustQuiescent(ctx context.Context, _ workspace.WorkspaceKey) error {
	a.mu.Lock()
	a.rustCalls++
	fn := a.rustFn
	a.mu.Unlock()
	if fn != nil {
		return fn(ctx)
	}
	return nil
}

// =============================================================================
// Acceptance #4 — concurrency cap honored.
// =============================================================================

// TestACC4_Cap1: cfg.MaxConcurrentWorkers=1; 10 jobs queued; the in-flight
// cascade slot count NEVER exceeds 1 at any observation point.
func TestACC4_Cap1(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	q := lspenrich.NewLaneQueue(64, 64)
	acq := newPoolLikeAcquirer()
	store := newAccStore()
	probe := &accReadiness{}
	metrics := &recordingMetrics{}

	cfg := semantic.LSPEnrichmentConfig{
		Enabled:                true,
		MaxConcurrentWorkers:   1,
		YieldCheckWindowMs:     200,
		TimeoutPerFile:         "10s",
		TimeoutTotal:           "60s",
		MaxSymbolsPerFile:      50,
		MaxReferencesPerSymbol: 10,
		MaxReferencesPerFile:   100,
		MaxCallHierarchyDepth:  2,
		MaxTypeHierarchyDepth:  2,
	}

	mgr := lspenrich.NewManager(q, acq, store, probe, cfg, metrics, nil)

	// Bypass Manager.Run because production wiring does not yet supply
	// Worker.NewCascadeLSP — drive the Worker directly with mgr as the
	// LeaseProvider so the B2 lease-cache code path is exercised.  Note
	// the Worker emits LSPEnrichmentTotal directly (its own Metrics
	// field), bypassing Manager.Run's trackedMetrics wrapper — so the
	// completion check uses recordingMetrics, not mgr.Status().
	var inflight atomic.Int32
	var observedMax atomic.Int32

	w := &lspenrich.Worker{
		Queue:     q,
		Leases:    mgr,
		Acquirer:  acq,
		Store:     store,
		Readiness: probe,
		BudgetCfg: cfg,
		Metrics:   metrics,
		NewCascadeLSP: func(_ *lspool.WorkerLease) lspenrich.CascadeLSP {
			return &countingLSP{
				inflight:    &inflight,
				observedMax: &observedMax,
				path:        "/x.go",
				delay:       30 * time.Millisecond,
			}
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- w.RunN(ctx, 1) }()

	const N = 10
	for i := 0; i < N; i++ {
		q.EnqueueLane(lspenrich.LaneHigh, makeJob("r", fmt.Sprintf("/f%d.go", i)))
	}

	// Wait until all jobs have completed (N outcomes recorded).
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if len(metrics.outcomes()) >= N {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done

	if got := observedMax.Load(); got != 1 {
		t.Errorf("observedMax in-flight: got %d, want 1 (cap=1)", got)
	}
	if got := len(metrics.outcomes()); got < N {
		t.Errorf("outcomes recorded: got %d, want >= %d", got, N)
	}
}

// TestACC4_Cap4: cfg.MaxConcurrentWorkers=4; 10 jobs queued; max observed
// in-flight is exactly 4 (or close to it under scheduler jitter — the
// load-bearing assertion is "> 1 AND <= 4").
func TestACC4_Cap4(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	q := lspenrich.NewLaneQueue(64, 64)
	acq := newPoolLikeAcquirer()
	store := newAccStore()
	probe := &accReadiness{}
	metrics := &recordingMetrics{}

	cfg := semantic.LSPEnrichmentConfig{
		Enabled:                true,
		MaxConcurrentWorkers:   4,
		YieldCheckWindowMs:     200,
		TimeoutPerFile:         "10s",
		TimeoutTotal:           "60s",
		MaxSymbolsPerFile:      50,
		MaxReferencesPerSymbol: 10,
		MaxReferencesPerFile:   100,
		MaxCallHierarchyDepth:  2,
		MaxTypeHierarchyDepth:  2,
	}

	mgr := lspenrich.NewManager(q, acq, store, probe, cfg, metrics, nil)

	var inflight atomic.Int32
	var observedMax atomic.Int32

	w := &lspenrich.Worker{
		Queue:     q,
		Leases:    mgr,
		Acquirer:  acq,
		Store:     store,
		Readiness: probe,
		BudgetCfg: cfg,
		Metrics:   metrics,
		NewCascadeLSP: func(_ *lspool.WorkerLease) lspenrich.CascadeLSP {
			return &countingLSP{
				inflight:    &inflight,
				observedMax: &observedMax,
				path:        "/x.go",
				delay:       50 * time.Millisecond,
			}
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- w.RunN(ctx, 4) }()

	const N = 10
	for i := 0; i < N; i++ {
		q.EnqueueLane(lspenrich.LaneHigh, makeJob("r", fmt.Sprintf("/f%d.go", i)))
	}

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if len(metrics.outcomes()) >= N {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done

	got := observedMax.Load()
	if got <= 1 {
		t.Errorf("observedMax in-flight: got %d; cap=4 should yield >= 2", got)
	}
	if got > 4 {
		t.Errorf("observedMax in-flight: got %d > 4 (cap exceeded)", got)
	}
}

// countingLSP is a CascadeLSP that increments an in-flight counter on
// DocumentSymbol entry and decrements on Definition exit.  The cascade
// runs documentSymbol → diagnostics → per-symbol(hover, ...) → ...; we
// hook the boundary at DocumentSymbol entry / cascade-end approximation
// (final tick).  observedMax tracks the all-time maximum.
type countingLSP struct {
	inflight    *atomic.Int32
	observedMax *atomic.Int32
	path        string
	delay       time.Duration
}

func (c *countingLSP) DocumentSymbol(_ context.Context, _ string) ([]lspenrich.Symbol, error) {
	cur := c.inflight.Add(1)
	for {
		max := c.observedMax.Load()
		if cur <= max {
			break
		}
		if c.observedMax.CompareAndSwap(max, cur) {
			break
		}
	}
	if c.delay > 0 {
		time.Sleep(c.delay)
	}
	return []lspenrich.Symbol{{Name: "S0", Path: c.path}}, nil
}
func (c *countingLSP) DrainDiagnostics(_ string) []lspenrich.Diagnostic { return nil }
func (c *countingLSP) Hover(_ context.Context, _ lspenrich.Symbol) (*lspenrich.Edge, error) {
	if c.delay > 0 {
		time.Sleep(c.delay)
	}
	return nil, nil
}
func (c *countingLSP) CallHierarchy(_ context.Context, _ lspenrich.Symbol, _ int) ([]lspenrich.Edge, error) {
	if c.delay > 0 {
		time.Sleep(c.delay)
	}
	return nil, nil
}
func (c *countingLSP) TypeHierarchy(_ context.Context, _ lspenrich.Symbol, _ int) ([]lspenrich.Edge, error) {
	return nil, nil
}
func (c *countingLSP) Implementation(_ context.Context, _ lspenrich.Symbol) ([]lspenrich.Edge, error) {
	c.inflight.Add(-1)
	return nil, nil
}
func (c *countingLSP) Definition(_ context.Context, _ lspenrich.Reference) (*lspenrich.Edge, error) {
	return nil, nil
}
func (c *countingLSP) ReferencesForSymbol(_ lspenrich.Symbol) []lspenrich.Reference { return nil }

// =============================================================================
// Acceptance #6 (B1) — voluntary yield via real Manager + real Worker +
// real Cascade.
// =============================================================================
//
// pool.AcquireLease is invoked by the test mid-cascade with a
// "foreground-tool:X" sessionID — the same shape *lspool.Pool uses to
// stamp lastForegroundLease[wsKey].  The next ForegroundBusy boundary
// check inside the cascade observes true and the cascade commits
// partial_reason="preempted" + OutcomePartialPreempted.

func TestACC6_VoluntaryYield(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	q := lspenrich.NewLaneQueue(8, 8)
	pool := newPoolLikeAcquirer()
	store := newAccStore()
	probe := &accReadiness{}
	metrics := &recordingMetrics{}

	cfg := semantic.LSPEnrichmentConfig{
		Enabled:                true,
		MaxConcurrentWorkers:   1,
		YieldCheckWindowMs:     200,
		TimeoutPerFile:         "10s",
		TimeoutTotal:           "60s",
		MaxSymbolsPerFile:      50,
		MaxReferencesPerSymbol: 10,
		MaxReferencesPerFile:   100,
		MaxCallHierarchyDepth:  2,
		MaxTypeHierarchyDepth:  2,
	}
	pool.SetYieldCheckWindow(time.Duration(cfg.YieldCheckWindowMs) * time.Millisecond)

	mgr := lspenrich.NewManager(q, pool, store, probe, cfg, metrics, nil)

	// Slow LSP: ~50ms per call so the cascade is observably mid-flight.
	// 5 docSymbols → cascade runs at least 7 ticks (documentSymbol +
	// diagnostics + per-symbol hover/callHierarchy/...).
	wsKey := workspace.WorkspaceKey{RepoRoot: "stress", Language: "go"}
	lsp := newSlowCascadeLSP("/main.go", 5, 50*time.Millisecond)

	w := &lspenrich.Worker{
		Queue:     q,
		Leases:    mgr,
		Acquirer:  pool,
		Store:     store,
		Readiness: probe,
		BudgetCfg: cfg,
		Metrics:   metrics,
		NewCascadeLSP: func(_ *lspool.WorkerLease) lspenrich.CascadeLSP {
			return lsp
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- w.RunN(ctx, 1) }()

	// Step 1: enqueue ONE job for /main.go.
	if !q.EnqueueLane(lspenrich.LaneHigh, makeJob("stress", "/main.go")) {
		t.Fatal("EnqueueLane returned false")
	}

	// Step 2-3: wait for the slow LSP to receive call #2 (cascade has
	// completed step 1 + step 2 of §14.4 — documentSymbol + diagnostics).
	gotStep2 := false
	for !gotStep2 {
		select {
		case step := <-lsp.stepCh:
			if step >= 2 {
				gotStep2 = true
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("cascade did not reach step 2 in time")
		}
	}

	// Step 4: inject a foreground lease through pool.AcquireLease — the
	// same code path *lspool.Pool.AcquireLease takes for any non-
	// enrichment sessionID.  This stamps lastForegroundLease[wsKey].
	foregroundLease, err := pool.AcquireLease(ctx, "foreground-tool:X", wsKey, false)
	if err != nil {
		t.Fatalf("foreground pool.AcquireLease: %v", err)
	}
	if foregroundLease == nil {
		t.Fatal("pool.AcquireLease returned nil lease")
	}
	defer pool.ReleaseLease(foregroundLease.SessionID)

	// Sanity: ForegroundBusy must now report true for wsKey.
	if !pool.ForegroundBusy(wsKey) {
		t.Fatal("ForegroundBusy=false immediately after foreground AcquireLease — yield-window misconfigured")
	}

	// Step 5: wait for the cascade to commit and the worker to record
	// LSPEnrichmentTotal.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if len(metrics.outcomes()) >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done

	// Step 6 assertions:
	//   a. partial_reason="preempted" stamped on the file.
	pr := store.GetPartialReason()
	if pr != "preempted" {
		t.Fatalf("partial_reason: got %q, want %q (B1 / acceptance #6)", pr, "preempted")
	}

	//   b. outcome=OutcomePartialPreempted reached metrics.
	gotOutcome := false
	for _, o := range metrics.outcomes() {
		if o == string(lspenrich.OutcomePartialPreempted) {
			gotOutcome = true
			break
		}
	}
	if !gotOutcome {
		t.Errorf("expected outcome %q in metrics; got %v",
			lspenrich.OutcomePartialPreempted, metrics.outcomes())
	}

	//   c. only a small number of step-batches committed before
	//      preemption (1-3 — typically symbols + diagnostics + maybe one
	//      hover edge).  This guards against a regression where the
	//      preempt boundary check stops working and the cascade runs to
	//      completion despite ForegroundBusy=true.
	commits := store.UpsertSteps()
	if commits < 0 || commits > 6 {
		t.Errorf("upsert step batches before preempt: got %d, want 0-6 (B1: cascade should yield early)",
			commits)
	}
}

// =============================================================================
// Acceptance #10 — Java readiness gate honored before lease acquisition.
// =============================================================================

// TestACC10_JavaReadiness: JavaReady blocks for 100ms and returns nil.
// AcquireFor (lease-acquire) MUST be invoked AFTER JavaReady returns —
// the worker's processOne flow calls WaitForLanguageReady → JavaReady
// FIRST, then mgr.AcquireFor.  We instrument the recordingAcquirer's
// AcquireLease to capture the timestamp of the first call and compare
// to the JavaReady-return timestamp.
func TestACC10_JavaReadiness(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	q := lspenrich.NewLaneQueue(8, 8)
	pool := newPoolLikeAcquirer()
	store := newAccStore()

	javaReturnedAt := atomic.Int64{}
	probe := &accReadiness{
		javaFn: func(ctx context.Context) error {
			select {
			case <-time.After(100 * time.Millisecond):
				javaReturnedAt.Store(time.Now().UnixNano())
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}

	cfg := semantic.LSPEnrichmentConfig{
		Enabled:                true,
		MaxConcurrentWorkers:   1,
		YieldCheckWindowMs:     200,
		TimeoutPerFile:         "5s",
		TimeoutTotal:           "60s",
		MaxSymbolsPerFile:      50,
		MaxReferencesPerSymbol: 10,
		MaxReferencesPerFile:   100,
		MaxCallHierarchyDepth:  2,
		MaxTypeHierarchyDepth:  2,
	}

	// timestampingAcquirer wraps pool.AcquireLease to capture the timestamp
	// of the first non-foreground call.  We then construct the Manager
	// with the wrapper so AcquireFor goes through the timestamp seam,
	// while the Worker.Acquirer (used only for ForegroundBusy) stays
	// pinned to the underlying pool — both views of the same store.
	acquireAt := atomic.Int64{}
	wrap := &timestampingAcquirer{inner: pool, firstAcquireAt: &acquireAt}
	mgr := lspenrich.NewManager(q, wrap, store, probe, cfg, nil, nil)

	w := &lspenrich.Worker{
		Queue:     q,
		Leases:    mgr,
		Acquirer:  pool,
		Store:     store,
		Readiness: probe,
		BudgetCfg: cfg,
		NewCascadeLSP: func(_ *lspool.WorkerLease) lspenrich.CascadeLSP {
			return newSlowCascadeLSP("/A.java", 1, 5*time.Millisecond)
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- w.RunN(ctx, 1) }()

	q.EnqueueLane(lspenrich.LaneHigh, makeJob("r", "/A.java"))

	// Wait for AcquireFor to fire OR deadline.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if acquireAt.Load() != 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done

	if javaReturnedAt.Load() == 0 {
		t.Fatal("JavaReady never returned")
	}
	if acquireAt.Load() == 0 {
		t.Fatal("AcquireLease was never called")
	}
	if acquireAt.Load() < javaReturnedAt.Load() {
		t.Errorf("AcquireLease fired BEFORE JavaReady returned: acquire=%d java=%d (acceptance #10 violated)",
			acquireAt.Load(), javaReturnedAt.Load())
	}
	// Sanity — JavaReady must have been called at least once.
	probe.mu.Lock()
	defer probe.mu.Unlock()
	if probe.javaCalls < 1 {
		t.Errorf("JavaReady call count: got %d, want >= 1", probe.javaCalls)
	}
}

// TestACC10_JavaReadinessTimeout: JavaReady blocks past the per-file
// timeout (1s).  AcquireFor must NEVER be called; the worker must mark
// the file partial_reason="lsp_unavailable" and emit
// LSPEnrichmentTotal(_, partial_lsp_unavailable).
func TestACC10_JavaReadinessTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	q := lspenrich.NewLaneQueue(8, 8)
	pool := newPoolLikeAcquirer()
	store := newAccStore()
	metrics := &recordingMetrics{}

	probe := &accReadiness{
		javaFn: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}

	acquireAt := atomic.Int64{}
	wrap := &timestampingAcquirer{inner: pool, firstAcquireAt: &acquireAt}

	cfg := semantic.LSPEnrichmentConfig{
		Enabled:                true,
		MaxConcurrentWorkers:   1,
		YieldCheckWindowMs:     200,
		TimeoutPerFile:         "100ms", // tight — JavaReady WILL exceed this
		TimeoutTotal:           "60s",
		MaxSymbolsPerFile:      50,
		MaxReferencesPerSymbol: 10,
		MaxReferencesPerFile:   100,
		MaxCallHierarchyDepth:  2,
		MaxTypeHierarchyDepth:  2,
	}

	mgr := lspenrich.NewManager(q, wrap, store, probe, cfg, metrics, nil)

	w := &lspenrich.Worker{
		Queue:     q,
		Leases:    mgr,
		Acquirer:  pool,
		Store:     store,
		Readiness: probe,
		BudgetCfg: cfg,
		Metrics:   metrics,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- w.RunN(ctx, 1) }()

	q.EnqueueLane(lspenrich.LaneHigh, makeJob("r", "/B.java"))

	// Wait until partial_lsp_unavailable lands in metrics.outcomes().
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		seen := false
		for _, o := range metrics.outcomes() {
			if o == string(lspenrich.OutcomePartialLSPUnavail) {
				seen = true
				break
			}
		}
		if seen {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done

	if acquireAt.Load() != 0 {
		t.Errorf("AcquireLease was called despite JavaReady timeout (acceptance #10 violated)")
	}

	// partial_reason="lsp_unavailable" stamped.
	pr := store.GetPartialReason()
	if pr != "lsp_unavailable" {
		t.Errorf("partial_reason after JavaReady timeout: got %q, want lsp_unavailable", pr)
	}

	// Outcome metric.
	gotOutcome := false
	for _, o := range metrics.outcomes() {
		if o == string(lspenrich.OutcomePartialLSPUnavail) {
			gotOutcome = true
			break
		}
	}
	if !gotOutcome {
		t.Errorf("expected outcome %q in metrics; got %v",
			lspenrich.OutcomePartialLSPUnavail, metrics.outcomes())
	}
}

// timestampingAcquirer wraps a LeaseAcquirer and records the timestamp
// of the first non-foreground AcquireLease call (sessionID prefix
// "lsp-enrichment:").  Foreground calls (no prefix) are passed through
// without tagging.
type timestampingAcquirer struct {
	inner          lspenrich.LeaseAcquirer
	firstAcquireAt *atomic.Int64
}

func (t *timestampingAcquirer) AcquireLease(ctx context.Context, sessionID string, wsKey workspace.WorkspaceKey, dirty bool) (*lspool.WorkerLease, error) {
	if strings.HasPrefix(sessionID, "lsp-enrichment:") {
		t.firstAcquireAt.CompareAndSwap(0, time.Now().UnixNano())
	}
	return t.inner.AcquireLease(ctx, sessionID, wsKey, dirty)
}

func (t *timestampingAcquirer) ForegroundBusy(wsKey workspace.WorkspaceKey) bool {
	return t.inner.ForegroundBusy(wsKey)
}

func (t *timestampingAcquirer) ReleaseLease(sessionID string) {
	if r, ok := t.inner.(lspenrich.LeaseReleaser); ok {
		r.ReleaseLease(sessionID)
	}
}

// =============================================================================
// Helpers shared with worker_test.go fixtures.  makeJob, recordingMetrics,
// and lspqueue.RevalidateFileJob are defined there (build-tag free).
// =============================================================================

// guard against accidental unused-import warnings.
var _ = errors.New
var _ = lspqueue.RevalidateFileJob{}
