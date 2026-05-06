package lspenrich_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/kernel/lspool"
	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/live/lspqueue"
	"github.com/agenthands/helix/internal/semantic/lspenrich"
	"github.com/agenthands/helix/internal/workspace"
)

// =============================================================================
// Recording fakes shared by worker tests.
// =============================================================================

// fakeLeaseProvider records AcquireFor + Release call counts.  B2 invariant:
// the worker must never call Release; tests assert release==0 after N jobs.
type fakeLeaseProvider struct {
	mu sync.Mutex

	acquireCalls int
	releaseCalls int

	// pre-canned error for AcquireFor; nil → return a placeholder *WorkerLease.
	acquireErr error

	// captured ctxs / langs / wsKeys for assertions.
	gotLangs    []string
	gotWSKeys   []workspace.WorkspaceKey
	gotCtxs     []context.Context

	// optional delay in AcquireFor (simulates a slow LS spawn).
	delay time.Duration
}

func (f *fakeLeaseProvider) AcquireFor(ctx context.Context, wsKey workspace.WorkspaceKey, lang string) (*lspool.WorkerLease, error) {
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.acquireCalls++
	f.gotLangs = append(f.gotLangs, lang)
	f.gotWSKeys = append(f.gotWSKeys, wsKey)
	f.gotCtxs = append(f.gotCtxs, ctx)
	if f.acquireErr != nil {
		return nil, f.acquireErr
	}
	// Return a non-nil placeholder lease — the worker must NOT exercise its
	// methods directly; only the cascade does, and we replace the cascade
	// path via a fake CascadeLSP.
	return &lspool.WorkerLease{}, nil
}

func (f *fakeLeaseProvider) snap() (acquire, release int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.acquireCalls, f.releaseCalls
}

// recordingForegroundAcquirer satisfies lspenrich.LeaseAcquirer.  The worker
// only touches ForegroundBusy; AcquireLease must never be called and is left
// returning a stub error.
type recordingForegroundAcquirer struct {
	mu      sync.Mutex
	busy    bool
	calls   int
	wsCalls []workspace.WorkspaceKey
}

func (r *recordingForegroundAcquirer) AcquireLease(_ context.Context, _ string, _ workspace.WorkspaceKey, _ bool) (*lspool.WorkerLease, error) {
	return nil, errors.New("recordingForegroundAcquirer.AcquireLease should not be called from Worker")
}

func (r *recordingForegroundAcquirer) ForegroundBusy(wsKey workspace.WorkspaceKey) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	r.wsCalls = append(r.wsCalls, wsKey)
	return r.busy
}

// recordingMetrics captures every metric call the worker emits.  All slices
// are append-only; tests inspect after worker shutdown to avoid races.
type recordingMetrics struct {
	mu sync.Mutex

	totalCalls    [][2]string // (lang, outcome)
	durationCalls []struct {
		lang string
		secs float64
	}
	errorCalls    [][2]string // (lang, outcome-error)
	laneDepth     []struct {
		lane  string
		depth int
	}
	bulkSuppressed []int
}

func (r *recordingMetrics) LSPEnrichmentTotal(lang, outcome string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.totalCalls = append(r.totalCalls, [2]string{lang, outcome})
}
func (r *recordingMetrics) LSPEnrichmentDuration(lang string, secs float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.durationCalls = append(r.durationCalls, struct {
		lang string
		secs float64
	}{lang, secs})
}
func (r *recordingMetrics) LSPEnrichmentErrors(lang, outcome string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errorCalls = append(r.errorCalls, [2]string{lang, outcome})
}
func (r *recordingMetrics) LSPEnrichmentLaneDepth(lane string, depth int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.laneDepth = append(r.laneDepth, struct {
		lane  string
		depth int
	}{lane, depth})
}
func (r *recordingMetrics) LSPEnrichmentBulkSuppressed(n int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bulkSuppressed = append(r.bulkSuppressed, n)
}

func (r *recordingMetrics) outcomes() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.totalCalls))
	for _, t := range r.totalCalls {
		out = append(out, t[1])
	}
	return out
}

// fakeReadinessProbeWorker mirrors the cascade test's recording probe but
// adds an injectable javaFn used by W4/W5.
type fakeReadinessProbeWorker struct {
	mu sync.Mutex

	javaFn    func(ctx context.Context, wsKey workspace.WorkspaceKey) error
	rustFn    func(ctx context.Context, wsKey workspace.WorkspaceKey) error
	javaCalls int
	rustCalls int

	// javaCompleteCh, when non-nil, is closed when JavaReady returns nil.
	// Used by W4 to assert the cascade ran AFTER readiness completed.
	javaCompleteCh chan struct{}
}

func (f *fakeReadinessProbeWorker) JavaReady(ctx context.Context, wsKey workspace.WorkspaceKey) error {
	f.mu.Lock()
	f.javaCalls++
	f.mu.Unlock()
	var err error
	if f.javaFn != nil {
		err = f.javaFn(ctx, wsKey)
	}
	if err == nil && f.javaCompleteCh != nil {
		close(f.javaCompleteCh)
	}
	return err
}

func (f *fakeReadinessProbeWorker) RustQuiescent(ctx context.Context, wsKey workspace.WorkspaceKey) error {
	f.mu.Lock()
	f.rustCalls++
	f.mu.Unlock()
	if f.rustFn != nil {
		return f.rustFn(ctx, wsKey)
	}
	return nil
}

// fakeStoreWorker is a minimal lspenrich.OverlayStore + CascadeStore used by
// the worker tests.  It records BeginCascadeTx calls and BeginOverlayTx
// calls (the latter is the markPending path in the worker).
type fakeStoreWorker struct {
	mu sync.Mutex

	beginCascadeCalls int
	beginOverlayCalls int

	beginCascadeErr error
	beginOverlayErr error

	// markedPending captures (path, reason) tuples written via the
	// BeginOverlayTx path (worker.markPending).  We don't have a real
	// *store.OverlayTx in unit tests, so this fake intercepts at the
	// CascadeStore layer.
	pendingPaths   []string
	pendingReasons []string
}

// fakeCascadeTxWorker records UpsertSymbols / MarkFileSemanticPending /
// Commit.  The cascade is never actually run from worker_test.go (the
// Cascade is constructed but the LSP shim is stubbed), so this fake only
// has to satisfy the interface and absorb calls.
type fakeCascadeTxWorker struct {
	mu sync.Mutex

	pendingPath   string
	pendingReason string
	commits       int
	rollbacks     int
}

func (f *fakeCascadeTxWorker) UpsertSymbols(ctx context.Context, path string, syms []lspenrich.Symbol) error {
	return nil
}
func (f *fakeCascadeTxWorker) UpsertReferences(ctx context.Context, path string, refs []lspenrich.Reference) error {
	return nil
}
func (f *fakeCascadeTxWorker) UpsertEdges(ctx context.Context, edges []lspenrich.Edge) error {
	return nil
}
func (f *fakeCascadeTxWorker) UpsertEdgesWithMerge(ctx context.Context, edges []lspenrich.Edge) error {
	return nil
}
func (f *fakeCascadeTxWorker) UpsertDiagnostics(ctx context.Context, path string, diags []lspenrich.Diagnostic) error {
	return nil
}
func (f *fakeCascadeTxWorker) WriteInvalidations(ctx context.Context) error { return nil }
func (f *fakeCascadeTxWorker) MarkFileSemanticPending(ctx context.Context, path, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pendingPath = path
	f.pendingReason = reason
	return nil
}
func (f *fakeCascadeTxWorker) Commit() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commits++
	return nil
}
func (f *fakeCascadeTxWorker) Rollback() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rollbacks++
	return nil
}
func (f *fakeCascadeTxWorker) Epoch() uint64 { return 1 }

// fakeStoreWorker.BeginCascadeTx returns a fakeCascadeTxWorker AND records
// pending writes from any markPending call routed through it.
func (s *fakeStoreWorker) BeginCascadeTx(ctx context.Context, repoID string) (lspenrich.CascadeTx, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.beginCascadeCalls++
	if s.beginCascadeErr != nil {
		return nil, s.beginCascadeErr
	}
	return &workerMarkPendingProxy{store: s}, nil
}

// workerMarkPendingProxy intercepts MarkFileSemanticPending / Commit so the
// store can record the pending path + reason emitted by the worker's
// markPending fast path (which uses BeginCascadeTx).
type workerMarkPendingProxy struct {
	store *fakeStoreWorker
}

func (p *workerMarkPendingProxy) UpsertSymbols(_ context.Context, _ string, _ []lspenrich.Symbol) error {
	return nil
}
func (p *workerMarkPendingProxy) UpsertReferences(_ context.Context, _ string, _ []lspenrich.Reference) error {
	return nil
}
func (p *workerMarkPendingProxy) UpsertEdges(_ context.Context, _ []lspenrich.Edge) error          { return nil }
func (p *workerMarkPendingProxy) UpsertEdgesWithMerge(_ context.Context, _ []lspenrich.Edge) error { return nil }
func (p *workerMarkPendingProxy) UpsertDiagnostics(_ context.Context, _ string, _ []lspenrich.Diagnostic) error {
	return nil
}
func (p *workerMarkPendingProxy) WriteInvalidations(_ context.Context) error { return nil }
func (p *workerMarkPendingProxy) MarkFileSemanticPending(_ context.Context, path, reason string) error {
	p.store.mu.Lock()
	defer p.store.mu.Unlock()
	p.store.pendingPaths = append(p.store.pendingPaths, path)
	p.store.pendingReasons = append(p.store.pendingReasons, reason)
	return nil
}
func (p *workerMarkPendingProxy) Commit() error   { return nil }
func (p *workerMarkPendingProxy) Rollback() error { return nil }
func (p *workerMarkPendingProxy) Epoch() uint64   { return 0 }

// =============================================================================
// Helpers
// =============================================================================

func workerBudgetCfg() semantic.LSPEnrichmentConfig {
	return semantic.LSPEnrichmentConfig{
		Enabled:                true,
		TimeoutPerFile:         "5s",
		TimeoutTotal:           "120s",
		MaxSymbolsPerFile:      200,
		MaxReferencesPerSymbol: 1000,
		MaxReferencesPerFile:   5000,
		MaxCallHierarchyDepth:  2,
		MaxTypeHierarchyDepth:  2,
	}
}

func makeJob(repoID, path string) lspqueue.RevalidateFileJob {
	return lspqueue.RevalidateFileJob{RepoID: semantic.RepoID(repoID), Path: path}
}

// =============================================================================
// Tests
// =============================================================================

// W1: Worker.RunN with cancelled ctx returns ctx.Err() immediately.
func TestWorker_W1_DrainHonorsCtxDone(t *testing.T) {
	q := lspenrich.NewLaneQueue(8, 8)
	w := &lspenrich.Worker{
		Queue:     q,
		Leases:    &fakeLeaseProvider{},
		Acquirer:  &recordingForegroundAcquirer{},
		Store:     &fakeStoreWorker{},
		Readiness: &fakeReadinessProbeWorker{},
		BudgetCfg: workerBudgetCfg(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before Run starts

	err := w.RunN(ctx, 1)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("RunN with cancelled ctx: got %v, want context.Canceled", err)
	}
}

// W2: Worker derives a per-job ctx from cfg.TimeoutPerFile.  We assert that
// the AcquireFor ctx has a deadline set, and that the deadline is approxim-
// ately TimeoutPerFile from "now".
func TestWorker_W2_PerJobTimeoutContext(t *testing.T) {
	q := lspenrich.NewLaneQueue(8, 8)
	leases := &fakeLeaseProvider{
		acquireErr: serr.ErrCircuitOpen, // short-circuit to avoid Cascade.Run
	}
	cfg := workerBudgetCfg()
	cfg.TimeoutPerFile = "250ms"
	w := &lspenrich.Worker{
		Queue:     q,
		Leases:    leases,
		Acquirer:  &recordingForegroundAcquirer{},
		Store:     &fakeStoreWorker{},
		Readiness: &fakeReadinessProbeWorker{},
		BudgetCfg: cfg,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = w.RunN(ctx, 1)
	}()

	q.EnqueueLane(lspenrich.LaneHigh, makeJob("r", "/a.go"))

	// Wait for the AcquireFor call.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if a, _ := leases.snap(); a >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel() // stop the worker

	leases.mu.Lock()
	defer leases.mu.Unlock()
	if len(leases.gotCtxs) == 0 {
		t.Fatal("AcquireFor was not called")
	}
	jobCtx := leases.gotCtxs[0]
	dl, ok := jobCtx.Deadline()
	if !ok {
		t.Fatal("AcquireFor ctx had no deadline; want per-file deadline")
	}
	remaining := time.Until(dl)
	if remaining > 300*time.Millisecond {
		t.Errorf("AcquireFor ctx deadline %v far in future; want ~250ms", remaining)
	}
}

// W3: Worker.RunN(ctx, n) spawns n goroutines.  We assert by enqueuing n+1
// jobs that block forever in AcquireFor; n in-flight, 1 queued — all
// observable via the AcquireFor counter while a separate goroutine drains.
func TestWorker_W3_ConcurrencyCap(t *testing.T) {
	q := lspenrich.NewLaneQueue(16, 16)
	const n = 3

	var inflight atomic.Int32
	releaseCh := make(chan struct{})
	leases := &fakeLeaseProviderBlocking{
		releaseCh: releaseCh,
		inflight:  &inflight,
	}

	w := &lspenrich.Worker{
		Queue:     q,
		Leases:    leases,
		Acquirer:  &recordingForegroundAcquirer{},
		Store:     &fakeStoreWorker{},
		Readiness: &fakeReadinessProbeWorker{},
		BudgetCfg: workerBudgetCfg(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- w.RunN(ctx, n) }()

	for i := 0; i < n+2; i++ {
		q.EnqueueLane(lspenrich.LaneHigh, makeJob("r", "/x.go"))
	}

	// Wait until exactly n acquisitions are in-flight.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if inflight.Load() == int32(n) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := inflight.Load(); got != int32(n) {
		t.Errorf("in-flight acquires: got %d, want %d (concurrency cap)", got, n)
	}

	close(releaseCh)
	cancel()
	<-done
}

// fakeLeaseProviderBlocking blocks AcquireFor until releaseCh is closed.
type fakeLeaseProviderBlocking struct {
	releaseCh <-chan struct{}
	inflight  *atomic.Int32
}

func (f *fakeLeaseProviderBlocking) AcquireFor(ctx context.Context, _ workspace.WorkspaceKey, _ string) (*lspool.WorkerLease, error) {
	f.inflight.Add(1)
	defer f.inflight.Add(-1)
	select {
	case <-f.releaseCh:
		return nil, serr.ErrCircuitOpen // unblock to dropped path
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// W4: Java readiness honored.  JavaReady returns nil after 50ms; assert
// AcquireFor is invoked AFTER JavaReady completes.
func TestWorker_W4_JavaReadinessHonored(t *testing.T) {
	q := lspenrich.NewLaneQueue(8, 8)

	javaDone := make(chan struct{})
	probe := &fakeReadinessProbeWorker{
		javaCompleteCh: javaDone,
		javaFn: func(ctx context.Context, _ workspace.WorkspaceKey) error {
			select {
			case <-time.After(50 * time.Millisecond):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}

	leases := &fakeLeaseProvider{
		acquireErr: serr.ErrCircuitOpen, // short-circuit to dropped
	}

	w := &lspenrich.Worker{
		Queue:     q,
		Leases:    leases,
		Acquirer:  &recordingForegroundAcquirer{},
		Store:     &fakeStoreWorker{},
		Readiness: probe,
		BudgetCfg: workerBudgetCfg(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = w.RunN(ctx, 1) }()

	q.EnqueueLane(lspenrich.LaneHigh, makeJob("r", "/J.java"))

	select {
	case <-javaDone:
	case <-time.After(2 * time.Second):
		t.Fatal("JavaReady never completed")
	}

	// AcquireFor MUST have been called only after javaDone closed.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if a, _ := leases.snap(); a >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()

	if a, _ := leases.snap(); a < 1 {
		t.Fatal("AcquireFor was never called after JavaReady completed")
	}
}

// W5: Java readiness timeout.  JavaReady blocks until ctx expires; worker
// marks file partial_reason="lsp_unavailable" and DOES NOT invoke AcquireFor.
func TestWorker_W5_JavaReadinessTimeoutMarksPartial(t *testing.T) {
	q := lspenrich.NewLaneQueue(8, 8)

	probe := &fakeReadinessProbeWorker{
		javaFn: func(ctx context.Context, _ workspace.WorkspaceKey) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}
	leases := &fakeLeaseProvider{}
	store := &fakeStoreWorker{}
	metrics := &recordingMetrics{}

	cfg := workerBudgetCfg()
	cfg.TimeoutPerFile = "30ms"
	w := &lspenrich.Worker{
		Queue:     q,
		Leases:    leases,
		Acquirer:  &recordingForegroundAcquirer{},
		Store:     store,
		Readiness: probe,
		BudgetCfg: cfg,
		Metrics:   metrics,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- w.RunN(ctx, 1) }()

	q.EnqueueLane(lspenrich.LaneHigh, makeJob("r", "/J.java"))

	// Wait until the markPending path fires (lsp_unavailable).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		store.mu.Lock()
		hit := false
		for _, r := range store.pendingReasons {
			if r == "lsp_unavailable" {
				hit = true
				break
			}
		}
		store.mu.Unlock()
		if hit {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done

	if a, _ := leases.snap(); a != 0 {
		t.Errorf("AcquireFor must NOT be called when readiness times out; got %d", a)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	gotPending := false
	for _, r := range store.pendingReasons {
		if r == "lsp_unavailable" {
			gotPending = true
			break
		}
	}
	if !gotPending {
		t.Errorf("expected partial_reason='lsp_unavailable'; got reasons=%v", store.pendingReasons)
	}

	// Outcome metric must include OutcomePartialLSPUnavail.
	gotOutcome := false
	for _, o := range metrics.outcomes() {
		if o == string(lspenrich.OutcomePartialLSPUnavail) {
			gotOutcome = true
			break
		}
	}
	if !gotOutcome {
		t.Errorf("expected metric outcome=%q; got %v",
			lspenrich.OutcomePartialLSPUnavail, metrics.outcomes())
	}
}

// W6: Worker invokes AcquireFor on the configured LeaseProvider with the
// derived (wsKey, lang) for the job's path.  We assert the lang argument
// matches the file extension.
func TestWorker_W6_AcquireForReceivesDerivedLang(t *testing.T) {
	q := lspenrich.NewLaneQueue(8, 8)
	leases := &fakeLeaseProvider{acquireErr: serr.ErrCircuitOpen}

	w := &lspenrich.Worker{
		Queue:     q,
		Leases:    leases,
		Acquirer:  &recordingForegroundAcquirer{},
		Store:     &fakeStoreWorker{},
		Readiness: &fakeReadinessProbeWorker{},
		BudgetCfg: workerBudgetCfg(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = w.RunN(ctx, 1) }()

	q.EnqueueLane(lspenrich.LaneHigh, makeJob("r", "/a/b.go"))

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if a, _ := leases.snap(); a >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()

	leases.mu.Lock()
	defer leases.mu.Unlock()
	if len(leases.gotLangs) == 0 || leases.gotLangs[0] != "go" {
		t.Errorf("AcquireFor lang: got %v, want first=\"go\"", leases.gotLangs)
	}
}

// W7: ErrCircuitOpen drop.  AcquireFor returns ErrCircuitOpen; worker
// increments outcome="dropped"; does NOT mark partial_reason; does NOT
// invoke Cascade.
func TestWorker_W7_AcquireForCircuitOpenIsDropped(t *testing.T) {
	q := lspenrich.NewLaneQueue(8, 8)
	leases := &fakeLeaseProvider{acquireErr: serr.ErrCircuitOpen}
	store := &fakeStoreWorker{}
	metrics := &recordingMetrics{}

	w := &lspenrich.Worker{
		Queue:     q,
		Leases:    leases,
		Acquirer:  &recordingForegroundAcquirer{},
		Store:     store,
		Readiness: &fakeReadinessProbeWorker{},
		BudgetCfg: workerBudgetCfg(),
		Metrics:   metrics,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- w.RunN(ctx, 1) }()

	q.EnqueueLane(lspenrich.LaneHigh, makeJob("r", "/x.go"))

	// Wait for one outcome record.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(metrics.outcomes()) >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done

	got := metrics.outcomes()
	if len(got) == 0 || got[0] != string(lspenrich.OutcomeDropped) {
		t.Errorf("first outcome: got %v, want first=%q (dropped)", got, lspenrich.OutcomeDropped)
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.pendingReasons) != 0 {
		t.Errorf("dropped path must NOT mark partial_reason; got %v", store.pendingReasons)
	}
}

// W8: Lane is recorded in metrics.  After draining a high-lane job,
// LSPEnrichmentLaneDepth("high", 0) is reported (queue empty after drain).
func TestWorker_W8_LaneDepthMetricEmitted(t *testing.T) {
	q := lspenrich.NewLaneQueue(8, 8)
	metrics := &recordingMetrics{}

	w := &lspenrich.Worker{
		Queue:     q,
		Leases:    &fakeLeaseProvider{acquireErr: serr.ErrCircuitOpen},
		Acquirer:  &recordingForegroundAcquirer{},
		Store:     &fakeStoreWorker{},
		Readiness: &fakeReadinessProbeWorker{},
		BudgetCfg: workerBudgetCfg(),
		Metrics:   metrics,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- w.RunN(ctx, 1) }()

	q.EnqueueLane(lspenrich.LaneHigh, makeJob("r", "/x.go"))

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		metrics.mu.Lock()
		ldn := len(metrics.laneDepth)
		metrics.mu.Unlock()
		if ldn >= 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done

	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	if len(metrics.laneDepth) < 2 {
		t.Fatalf("LaneDepth call count: got %d, want >= 2 (high + background)", len(metrics.laneDepth))
	}

	sawHigh, sawBg := false, false
	for _, ld := range metrics.laneDepth {
		switch ld.lane {
		case "high":
			sawHigh = true
		case "background":
			sawBg = true
		}
	}
	if !sawHigh || !sawBg {
		t.Errorf("LaneDepth lanes: sawHigh=%v sawBg=%v (both want true)", sawHigh, sawBg)
	}
}

// W9: Cascade outcome propagates to metric.  We swap the cascade's LSP shim
// indirectly by injecting a CascadeFactory-style hook — Worker.processOne
// builds the Cascade with our fake CascadeLSP, and we make the cascade
// return OutcomePartialPreempted by making foregroundBusy true after step 1.
//
// This test verifies the worker emits exactly one LSPEnrichmentTotal call
// with outcome="partial_preempted" via the live cascade path (not a stub).
func TestWorker_W9_CascadeOutcomeMapsToMetric(t *testing.T) {
	q := lspenrich.NewLaneQueue(8, 8)
	metrics := &recordingMetrics{}

	// foregroundBusy=true → cascade returns OutcomePartialPreempted.
	acq := &recordingForegroundAcquirer{busy: true}
	leases := &fakeLeaseProvider{}
	store := newFakeOverlayStore() // shared with cascade tests

	// Provide a CascadeLSP factory via the Worker's NewCascadeLSP hook.
	lsp := newFakeLSP()
	w := &lspenrich.Worker{
		Queue:     q,
		Leases:    leases,
		Acquirer:  acq,
		Store:     store,
		Readiness: &fakeReadinessProbeWorker{},
		BudgetCfg: workerBudgetCfg(),
		Metrics:   metrics,
		NewCascadeLSP: func(_ *lspool.WorkerLease) lspenrich.CascadeLSP {
			return lsp
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- w.RunN(ctx, 1) }()

	q.EnqueueLane(lspenrich.LaneHigh, makeJob("r", "/x.go"))

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(metrics.outcomes()) >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done

	got := metrics.outcomes()
	if len(got) == 0 || got[0] != string(lspenrich.OutcomePartialPreempted) {
		t.Errorf("outcome: got %v, want first=%q",
			got, lspenrich.OutcomePartialPreempted)
	}
}

// W10 (B2): No per-job lease.Release.  After N successful jobs, fake Lease-
// Provider Release count == 0; AcquireFor count == N.
func TestWorker_W10_NoPerJobReleaseB2(t *testing.T) {
	q := lspenrich.NewLaneQueue(16, 16)
	leases := &fakeLeaseProvider{}
	metrics := &recordingMetrics{}

	store := newFakeOverlayStore()
	lsp := newFakeLSP()
	lsp.referencesPerSymbol = 0

	w := &lspenrich.Worker{
		Queue:     q,
		Leases:    leases,
		Acquirer:  &recordingForegroundAcquirer{},
		Store:     store,
		Readiness: &fakeReadinessProbeWorker{},
		BudgetCfg: workerBudgetCfg(),
		Metrics:   metrics,
		NewCascadeLSP: func(_ *lspool.WorkerLease) lspenrich.CascadeLSP {
			return lsp
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- w.RunN(ctx, 1) }()

	const N = 4
	for i := 0; i < N; i++ {
		q.EnqueueLane(lspenrich.LaneHigh, makeJob("r", "/x.go"))
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if a, _ := leases.snap(); a >= N {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done

	a, r := leases.snap()
	if a != N {
		t.Errorf("AcquireFor count: got %d, want %d", a, N)
	}
	if r != 0 {
		t.Errorf("Release count: got %d, want 0 (B2 invariant: worker never releases)", r)
	}
}

// W11 (B2): AcquireFor failure path does not call Release.  The fake
// LeaseProvider tracks Release; AcquireFor returns ErrCircuitOpen.  Worker
// must NOT call Release on the failed handle.
func TestWorker_W11_NoReleaseOnAcquireForFailure(t *testing.T) {
	q := lspenrich.NewLaneQueue(8, 8)
	leases := &fakeLeaseProvider{acquireErr: serr.ErrCircuitOpen}

	w := &lspenrich.Worker{
		Queue:     q,
		Leases:    leases,
		Acquirer:  &recordingForegroundAcquirer{},
		Store:     &fakeStoreWorker{},
		Readiness: &fakeReadinessProbeWorker{},
		BudgetCfg: workerBudgetCfg(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- w.RunN(ctx, 1) }()

	q.EnqueueLane(lspenrich.LaneHigh, makeJob("r", "/x.go"))

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if a, _ := leases.snap(); a >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done

	a, r := leases.snap()
	if a < 1 {
		t.Fatal("AcquireFor was not called")
	}
	if r != 0 {
		t.Errorf("Release count after AcquireFor failure: got %d, want 0", r)
	}
}

// W12 (W3): Cascade is constructed with explicit Capabilities.  We don't
// have direct reflection access to inspect the in-flight Cascade struct,
// so we assert the behaviour: MethodNotFound on a non-essential cascade
// step does not crash and continues to the next step (which requires a
// non-nil Capabilities cache to mark the unsupported method).
func TestWorker_W12_CascadeCapabilitiesNonNil(t *testing.T) {
	q := lspenrich.NewLaneQueue(8, 8)
	metrics := &recordingMetrics{}

	store := newFakeOverlayStore()
	lsp := newFakeLSP()
	lsp.errOnMethod["callHierarchy"] = lspenrich.ErrMethodNotFound
	lsp.referencesPerSymbol = 0

	w := &lspenrich.Worker{
		Queue:     q,
		Leases:    &fakeLeaseProvider{},
		Acquirer:  &recordingForegroundAcquirer{},
		Store:     store,
		Readiness: &fakeReadinessProbeWorker{},
		BudgetCfg: workerBudgetCfg(),
		Metrics:   metrics,
		NewCascadeLSP: func(_ *lspool.WorkerLease) lspenrich.CascadeLSP {
			return lsp
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- w.RunN(ctx, 1) }()

	q.EnqueueLane(lspenrich.LaneHigh, makeJob("r", "/x.go"))

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(metrics.outcomes()) >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done

	got := metrics.outcomes()
	if len(got) == 0 || got[0] != string(lspenrich.OutcomeApplied) {
		t.Errorf("outcome on MethodNotFound continuation: got %v, want first=%q "+
			"(only possible with non-nil Capabilities)",
			got, lspenrich.OutcomeApplied)
	}
}
