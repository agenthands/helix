package lspenrich_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/live/lspqueue"
	"github.com/agenthands/helix/internal/semantic/lspenrich"
)

// =============================================================================
// Recording fakes — shared across cascade tests.
// =============================================================================

// fakeOverlayTx records every method call against the OverlayTx-shaped surface
// the cascade uses. We do NOT use the real *store.OverlayTx in unit tests
// because constructing one requires a live DuckDB connection; the cascade
// interacts only via the lspenrich.CascadeTx surface.
type fakeOverlayTx struct {
	mu sync.Mutex

	upsertSymbolsCalls       int
	upsertReferencesCalls    int
	upsertEdgesCalls         int
	upsertDiagnosticsCalls   int
	writeInvalidationsCalls  int

	upsertedEdges []lspenrich.Edge

	markPendingPath   string
	markPendingReason string
	markPendingCalls  int

	commitCalls   int
	rollbackCalls int

	epoch uint64
}

func newFakeOverlayTx(epoch uint64) *fakeOverlayTx { return &fakeOverlayTx{epoch: epoch} }

func (f *fakeOverlayTx) UpsertSymbols(ctx context.Context, path string, syms []lspenrich.Symbol) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.upsertSymbolsCalls++
	return nil
}

func (f *fakeOverlayTx) UpsertReferences(ctx context.Context, path string, refs []lspenrich.Reference) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.upsertReferencesCalls++
	return nil
}

func (f *fakeOverlayTx) UpsertEdges(ctx context.Context, edges []lspenrich.Edge) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.upsertEdgesCalls++
	f.upsertedEdges = append(f.upsertedEdges, edges...)
	return nil
}

// UpsertEdgesWithMerge mirrors UpsertEdges in the recording fake — both
// counters track the same surface so existing assertions keep working
// while the cascade flips to the merge entrypoint.
func (f *fakeOverlayTx) UpsertEdgesWithMerge(ctx context.Context, edges []lspenrich.Edge) error {
	return f.UpsertEdges(ctx, edges)
}

func (f *fakeOverlayTx) UpsertDiagnostics(ctx context.Context, path string, diags []lspenrich.Diagnostic) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.upsertDiagnosticsCalls++
	return nil
}

func (f *fakeOverlayTx) WriteInvalidations(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writeInvalidationsCalls++
	return nil
}

func (f *fakeOverlayTx) MarkFileSemanticPending(ctx context.Context, path, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.markPendingPath = path
	f.markPendingReason = reason
	f.markPendingCalls++
	return nil
}

func (f *fakeOverlayTx) Commit() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commitCalls++
	return nil
}

func (f *fakeOverlayTx) Rollback() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rollbackCalls++
	return nil
}

func (f *fakeOverlayTx) Epoch() uint64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.epoch
}

// fakeOverlayStore returns one fakeOverlayTx per BeginOverlayTx call,
// auto-incrementing the epoch.
type fakeOverlayStore struct {
	mu        sync.Mutex
	txs       []*fakeOverlayTx
	nextEpoch uint64
	beginErr  error
}

func newFakeOverlayStore() *fakeOverlayStore { return &fakeOverlayStore{nextEpoch: 1} }

func (s *fakeOverlayStore) BeginCascadeTx(ctx context.Context, repoID string) (lspenrich.CascadeTx, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.beginErr != nil {
		return nil, s.beginErr
	}
	tx := newFakeOverlayTx(s.nextEpoch)
	s.nextEpoch++
	s.txs = append(s.txs, tx)
	return tx, nil
}

// fakeLSP is the recording cascade-LSP shim. Each method has an injectable
// behaviour function; default is "return one trivial result and no error".
type fakeLSP struct {
	mu sync.Mutex

	// callOrder records the LSP method names in the order they were invoked.
	callOrder []string

	// errOnMethod[method] returns the given error when method is called.
	errOnMethod map[string]error

	// delayPerCall is added before every method returns (simulates a slow LS).
	delayPerCall time.Duration

	// crashAfterN — if >0, returns errLSCrash after N calls.
	crashAfterN int

	// docSymbols[path] returns the symbols for a documentSymbol call on path.
	docSymbols map[string][]lspenrich.Symbol

	// referencesPerSymbol — when >0, the prioritizeReferences output simulates
	// that many references per symbol; consumed by the per-reference loop.
	// When zero, the cascade emits zero references and the definition loop is
	// a no-op.
	referencesPerSymbol int
}

func newFakeLSP() *fakeLSP {
	return &fakeLSP{
		errOnMethod: make(map[string]error),
		docSymbols:  make(map[string][]lspenrich.Symbol),
	}
}

var errLSCrash = errors.New("simulated: LS crashed mid-cascade")

func (f *fakeLSP) record(method string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.callOrder = append(f.callOrder, method)
	if f.delayPerCall > 0 {
		time.Sleep(f.delayPerCall)
	}
	if f.crashAfterN > 0 && len(f.callOrder) > f.crashAfterN {
		return errLSCrash
	}
	if err, ok := f.errOnMethod[method]; ok {
		return err
	}
	return nil
}

func (f *fakeLSP) DocumentSymbol(ctx context.Context, path string) ([]lspenrich.Symbol, error) {
	if err := f.record("documentSymbol"); err != nil {
		return nil, err
	}
	if syms, ok := f.docSymbols[path]; ok {
		return syms, nil
	}
	// Default: one symbol per file.
	return []lspenrich.Symbol{{Name: "S1", Path: path}}, nil
}

func (f *fakeLSP) DrainDiagnostics(path string) []lspenrich.Diagnostic {
	_ = f.record("diagnostics")
	return nil
}

func (f *fakeLSP) Hover(ctx context.Context, sym lspenrich.Symbol) (*lspenrich.Edge, error) {
	if err := f.record("hover"); err != nil {
		return nil, err
	}
	// Hover produces one TYPE_OF edge with confidence=1.0, source=lsp.hover.
	return &lspenrich.Edge{
		Kind:            "TYPE_OF",
		Source:          "lsp.hover",
		Confidence:      1.0,
		ValidationState: "validated",
	}, nil
}

func (f *fakeLSP) CallHierarchy(ctx context.Context, sym lspenrich.Symbol, depth int) ([]lspenrich.Edge, error) {
	if err := f.record("callHierarchy"); err != nil {
		return nil, err
	}
	return []lspenrich.Edge{{
		Kind:            "CALLS",
		Source:          "lsp.callHierarchy",
		Confidence:      1.0,
		ValidationState: "validated",
	}}, nil
}

func (f *fakeLSP) TypeHierarchy(ctx context.Context, sym lspenrich.Symbol, depth int) ([]lspenrich.Edge, error) {
	if err := f.record("typeHierarchy"); err != nil {
		return nil, err
	}
	return []lspenrich.Edge{{
		Kind:            "EXTENDS",
		Source:          "lsp.typeHierarchy",
		Confidence:      1.0,
		ValidationState: "validated",
	}}, nil
}

func (f *fakeLSP) Implementation(ctx context.Context, sym lspenrich.Symbol) ([]lspenrich.Edge, error) {
	if err := f.record("implementation"); err != nil {
		return nil, err
	}
	return []lspenrich.Edge{{
		Kind:            "IMPLEMENTS",
		Source:          "lsp.implementation",
		Confidence:      1.0,
		ValidationState: "validated",
	}}, nil
}

func (f *fakeLSP) Definition(ctx context.Context, ref lspenrich.Reference) (*lspenrich.Edge, error) {
	if err := f.record("definition"); err != nil {
		return nil, err
	}
	return &lspenrich.Edge{
		Kind:            "RESOLVES_TO",
		Source:          "lsp.definition",
		Confidence:      1.0,
		ValidationState: "validated",
	}, nil
}

// ReferencesForSymbol controls the synthetic reference list used by the per-
// symbol reference budget step. Tests can override referencesPerSymbol.
func (f *fakeLSP) ReferencesForSymbol(sym lspenrich.Symbol) []lspenrich.Reference {
	n := f.referencesPerSymbol
	out := make([]lspenrich.Reference, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, lspenrich.Reference{
			Path:         sym.Path,
			IsDefinition: false,
			Name:         fmt.Sprintf("ref-%d", i),
		})
	}
	return out
}

// =============================================================================
// Helpers
// =============================================================================

func cascadeBudgetCfg() semantic.LSPEnrichmentConfig {
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

func newTestCascade(store lspenrich.CascadeStore, lsp lspenrich.CascadeLSP) *lspenrich.Cascade {
	return &lspenrich.Cascade{
		Store:        store,
		LSP:          lsp,
		Capabilities: lspenrich.NewCapabilityCache(),
	}
}

func mustBudget(t *testing.T, now time.Time) lspenrich.Budget {
	t.Helper()
	b, err := lspenrich.NewBudget(now, now, cascadeBudgetCfg())
	if err != nil {
		t.Fatalf("NewBudget: %v", err)
	}
	return b
}

// cascadeNow is recomputed at test-call time so per-file/total deadlines
// (derived from cfg.TimeoutPerFile=5s + cfg.TimeoutTotal=120s) extend INTO
// the future relative to time.Now() the cascade observes — without this the
// boundary check trips immediately and every test sees OutcomePartialBudget.
func cascadeNow() time.Time { return time.Now() }

// =============================================================================
// Tests
// =============================================================================

// C1: Yield mid-cascade (acceptance #6). foregroundBusy returns true after
// call #2; partial facts commit; partial_reason="preempted"; outcome=
// OutcomePartialPreempted.
func TestCascade_C1_YieldMidCascade(t *testing.T) {
	store := newFakeOverlayStore()
	lsp := newFakeLSP()
	c := newTestCascade(store, lsp)

	callCount := 0
	busy := func() bool {
		callCount++
		// First two boundary checks: not busy. Third onward: busy.
		return callCount > 2
	}

	job := lspqueue.RevalidateFileJob{RepoID: semantic.RepoID("r"), Path: "/x.go"}
	budget := mustBudget(t, cascadeNow())
	out := c.Run(context.Background(), job, "go", &budget, busy)

	if out != lspenrich.OutcomePartialPreempted {
		t.Errorf("outcome: got %q, want OutcomePartialPreempted", out)
	}
	if len(store.txs) != 1 {
		t.Fatalf("BeginCascadeTx call count: got %d, want 1", len(store.txs))
	}
	tx := store.txs[0]
	if tx.markPendingReason != "preempted" {
		t.Errorf("partial_reason: got %q, want %q", tx.markPendingReason, "preempted")
	}
	if tx.commitCalls != 1 {
		t.Errorf("commit call count: got %d, want 1", tx.commitCalls)
	}
}

// C2: Budget exhaustion (acceptance #8). Slow LS + tight per-file budget;
// file marked partial_reason="budget exhausted"; outcome=OutcomePartialBudget.
func TestCascade_C2_BudgetExhaustion(t *testing.T) {
	store := newFakeOverlayStore()
	lsp := newFakeLSP()
	lsp.delayPerCall = 60 * time.Millisecond
	c := newTestCascade(store, lsp)

	// Budget with 100ms per-file timeout — first call (60ms) succeeds, second
	// boundary check sees deadline expired.
	cfg := cascadeBudgetCfg()
	cfg.TimeoutPerFile = "100ms"
	b, err := lspenrich.NewBudget(time.Now(), time.Now(), cfg)
	if err != nil {
		t.Fatalf("NewBudget: %v", err)
	}

	job := lspqueue.RevalidateFileJob{RepoID: semantic.RepoID("r"), Path: "/y.go"}
	out := c.Run(context.Background(), job, "go", &b, func() bool { return false })

	if out != lspenrich.OutcomePartialBudget {
		t.Errorf("outcome: got %q, want OutcomePartialBudget", out)
	}
	if len(store.txs) != 1 {
		t.Fatalf("tx count: got %d, want 1", len(store.txs))
	}
	if store.txs[0].markPendingReason != "budget exhausted" {
		t.Errorf("partial_reason: got %q, want %q", store.txs[0].markPendingReason, "budget exhausted")
	}
}

// C3: Cascade order — documentSymbol before diagnostics before per-symbol fan-out.
func TestCascade_C3_CascadeOrder(t *testing.T) {
	store := newFakeOverlayStore()
	lsp := newFakeLSP()
	// Single symbol, zero references — simplifies the order check.
	lsp.docSymbols["/z.go"] = []lspenrich.Symbol{{Name: "S1", Path: "/z.go"}}
	lsp.referencesPerSymbol = 0
	c := newTestCascade(store, lsp)

	job := lspqueue.RevalidateFileJob{RepoID: semantic.RepoID("r"), Path: "/z.go"}
	budget := mustBudget(t, cascadeNow())
	out := c.Run(context.Background(), job, "go", &budget, func() bool { return false })

	if out != lspenrich.OutcomeApplied {
		t.Errorf("outcome: got %q, want OutcomeApplied", out)
	}

	// Expected order:
	//   documentSymbol -> diagnostics -> hover -> callHierarchy -> typeHierarchy -> implementation
	want := []string{"documentSymbol", "diagnostics", "hover", "callHierarchy", "typeHierarchy", "implementation"}
	if len(lsp.callOrder) < len(want) {
		t.Fatalf("callOrder: got %v, want at least %v", lsp.callOrder, want)
	}
	for i, m := range want {
		if lsp.callOrder[i] != m {
			t.Errorf("call %d: got %q, want %q (full order: %v)", i, lsp.callOrder[i], m, lsp.callOrder)
		}
	}
}

// C4: MethodNotFound continuation. callHierarchy returns -32601; cascade
// continues to typeHierarchy; final outcome=OutcomeApplied.
func TestCascade_C4_MethodNotFoundContinues(t *testing.T) {
	store := newFakeOverlayStore()
	lsp := newFakeLSP()
	lsp.errOnMethod["callHierarchy"] = lspenrich.ErrMethodNotFound
	lsp.referencesPerSymbol = 0
	c := newTestCascade(store, lsp)

	job := lspqueue.RevalidateFileJob{RepoID: semantic.RepoID("r"), Path: "/m.go"}
	budget := mustBudget(t, cascadeNow())
	out := c.Run(context.Background(), job, "go", &budget, func() bool { return false })

	if out != lspenrich.OutcomeApplied {
		t.Errorf("outcome: got %q, want OutcomeApplied (MethodNotFound should not abort)", out)
	}
	// callHierarchy was attempted, then typeHierarchy must follow.
	sawCH := false
	sawTH := false
	for _, m := range lsp.callOrder {
		if m == "callHierarchy" {
			sawCH = true
		}
		if m == "typeHierarchy" {
			if !sawCH {
				t.Errorf("typeHierarchy fired before callHierarchy attempt: order=%v", lsp.callOrder)
			}
			sawTH = true
		}
	}
	if !sawCH || !sawTH {
		t.Errorf("expected both callHierarchy and typeHierarchy in callOrder; got %v", lsp.callOrder)
	}
}

// C5 (W1): Whole-job LS crash. Cascade commits gathered facts;
// MarkFileSemanticPending(path, "lsp_unavailable"); outcome=
// OutcomePartialLSPUnavail. Lease is NOT released by the cascade (B2).
func TestCascade_C5_LSCrashPartialLSPUnavail(t *testing.T) {
	store := newFakeOverlayStore()
	lsp := newFakeLSP()
	// Crash after 2 calls (after documentSymbol + diagnostics complete).
	lsp.crashAfterN = 2
	c := newTestCascade(store, lsp)

	job := lspqueue.RevalidateFileJob{RepoID: semantic.RepoID("r"), Path: "/c.go"}
	budget := mustBudget(t, cascadeNow())
	out := c.Run(context.Background(), job, "go", &budget, func() bool { return false })

	if out != lspenrich.OutcomePartialLSPUnavail {
		t.Errorf("outcome: got %q, want OutcomePartialLSPUnavail", out)
	}
	if len(store.txs) != 1 {
		t.Fatalf("tx count: got %d", len(store.txs))
	}
	tx := store.txs[0]
	if tx.markPendingReason != "lsp_unavailable" {
		t.Errorf("partial_reason: got %q, want %q", tx.markPendingReason, "lsp_unavailable")
	}
	if tx.commitCalls != 1 {
		t.Errorf("commit calls: got %d, want 1", tx.commitCalls)
	}
}

// C6: textDocument/definition is NEVER called on definition references.
func TestCascade_C6_DefinitionNotCalledOnDefinitions(t *testing.T) {
	store := newFakeOverlayStore()
	lsp := &fakeLSP{
		errOnMethod: make(map[string]error),
		docSymbols: map[string][]lspenrich.Symbol{
			"/d.go": {{Name: "S1", Path: "/d.go"}},
		},
	}
	// Force ReferencesForSymbol to return one IsDefinition=true reference.
	lsp.referencesPerSymbol = 1
	c := &lspenrich.Cascade{
		Store:        store,
		LSP:          &fakeLSPDefRefs{base: lsp},
		Capabilities: lspenrich.NewCapabilityCache(),
	}

	job := lspqueue.RevalidateFileJob{RepoID: semantic.RepoID("r"), Path: "/d.go"}
	budget := mustBudget(t, cascadeNow())
	out := c.Run(context.Background(), job, "go", &budget, func() bool { return false })

	if out != lspenrich.OutcomeApplied {
		t.Errorf("outcome: got %q, want OutcomeApplied", out)
	}
	for _, m := range lsp.callOrder {
		if m == "definition" {
			t.Errorf("definition was called on a definition reference (SPEC line 1261 violation): order=%v", lsp.callOrder)
		}
	}
}

// fakeLSPDefRefs wraps fakeLSP to return references with IsDefinition=true.
type fakeLSPDefRefs struct{ base *fakeLSP }

func (w *fakeLSPDefRefs) DocumentSymbol(ctx context.Context, path string) ([]lspenrich.Symbol, error) {
	return w.base.DocumentSymbol(ctx, path)
}
func (w *fakeLSPDefRefs) DrainDiagnostics(path string) []lspenrich.Diagnostic {
	return w.base.DrainDiagnostics(path)
}
func (w *fakeLSPDefRefs) Hover(ctx context.Context, sym lspenrich.Symbol) (*lspenrich.Edge, error) {
	return w.base.Hover(ctx, sym)
}
func (w *fakeLSPDefRefs) CallHierarchy(ctx context.Context, sym lspenrich.Symbol, d int) ([]lspenrich.Edge, error) {
	return w.base.CallHierarchy(ctx, sym, d)
}
func (w *fakeLSPDefRefs) TypeHierarchy(ctx context.Context, sym lspenrich.Symbol, d int) ([]lspenrich.Edge, error) {
	return w.base.TypeHierarchy(ctx, sym, d)
}
func (w *fakeLSPDefRefs) Implementation(ctx context.Context, sym lspenrich.Symbol) ([]lspenrich.Edge, error) {
	return w.base.Implementation(ctx, sym)
}
func (w *fakeLSPDefRefs) Definition(ctx context.Context, ref lspenrich.Reference) (*lspenrich.Edge, error) {
	return w.base.Definition(ctx, ref)
}
func (w *fakeLSPDefRefs) ReferencesForSymbol(sym lspenrich.Symbol) []lspenrich.Reference {
	return []lspenrich.Reference{{Path: sym.Path, IsDefinition: true, Name: "def"}}
}

// C7: Per-tx overlay_epoch advances — using a recording fake. Two consecutive
// Cascade.Run calls; second tx's epoch > first tx's epoch.
func TestCascade_C7_EpochAdvancesPerTx(t *testing.T) {
	store := newFakeOverlayStore()
	lsp := newFakeLSP()
	lsp.referencesPerSymbol = 0
	c := newTestCascade(store, lsp)

	job := lspqueue.RevalidateFileJob{RepoID: semantic.RepoID("r"), Path: "/e.go"}
	budget1 := mustBudget(t, cascadeNow())
	_ = c.Run(context.Background(), job, "go", &budget1, func() bool { return false })

	budget2 := mustBudget(t, cascadeNow())
	_ = c.Run(context.Background(), job, "go", &budget2, func() bool { return false })

	if len(store.txs) != 2 {
		t.Fatalf("tx count: got %d, want 2", len(store.txs))
	}
	if store.txs[1].Epoch() <= store.txs[0].Epoch() {
		t.Errorf("second epoch (%d) must exceed first (%d)", store.txs[1].Epoch(), store.txs[0].Epoch())
	}
}

// C8 (W6): Edges land with confidence=1.0, validation_state="validated",
// source="lsp.<call>". Typed-field assertion on the recording fake.
func TestCascade_C8_EdgeConfidenceAndProvenance(t *testing.T) {
	store := newFakeOverlayStore()
	lsp := newFakeLSP()
	lsp.referencesPerSymbol = 0
	c := newTestCascade(store, lsp)

	job := lspqueue.RevalidateFileJob{RepoID: semantic.RepoID("r"), Path: "/p.go"}
	budget := mustBudget(t, cascadeNow())
	out := c.Run(context.Background(), job, "go", &budget, func() bool { return false })
	if out != lspenrich.OutcomeApplied {
		t.Fatalf("outcome: got %q, want OutcomeApplied", out)
	}
	if len(store.txs) != 1 {
		t.Fatalf("tx count: got %d", len(store.txs))
	}
	edges := store.txs[0].upsertedEdges
	if len(edges) == 0 {
		t.Fatal("expected upserted edges from cascade, got none")
	}
	for i, e := range edges {
		if e.Confidence != 1.0 {
			t.Errorf("edge[%d].Confidence: got %v, want 1.0", i, e.Confidence)
		}
		if e.ValidationState != "validated" {
			t.Errorf("edge[%d].ValidationState: got %q, want validated", i, e.ValidationState)
		}
		if e.Source == "" || len(e.Source) < 4 || e.Source[:4] != "lsp." {
			t.Errorf("edge[%d].Source: got %q, want prefix lsp.", i, e.Source)
		}
	}
}

// C9 (W10): Per-symbol budget consumption — ConsumeReferences(len(refs)) with
// a count exceeding MaxReferencesPerSymbol must abort to OutcomePartialBudget.
func TestCascade_C9_ConsumeReferencesPerSymbolGranularity(t *testing.T) {
	store := newFakeOverlayStore()
	lsp := newFakeLSP()
	// 100 references per symbol; budget caps MaxReferencesPerSymbol=50.
	lsp.referencesPerSymbol = 100
	c := newTestCascade(store, lsp)

	cfg := cascadeBudgetCfg()
	cfg.MaxReferencesPerSymbol = 50
	t0 := cascadeNow()
	b, err := lspenrich.NewBudget(t0, t0, cfg)
	if err != nil {
		t.Fatalf("NewBudget: %v", err)
	}

	job := lspqueue.RevalidateFileJob{RepoID: semantic.RepoID("r"), Path: "/g.go"}
	out := c.Run(context.Background(), job, "go", &b, func() bool { return false })

	if out != lspenrich.OutcomePartialBudget {
		t.Errorf("outcome: got %q, want OutcomePartialBudget (refs %d > per-symbol cap %d)",
			out, lsp.referencesPerSymbol, cfg.MaxReferencesPerSymbol)
	}
	if len(store.txs) != 1 {
		t.Fatalf("tx count: got %d", len(store.txs))
	}
	if store.txs[0].markPendingReason != "budget exhausted" {
		t.Errorf("partial_reason: got %q, want budget exhausted", store.txs[0].markPendingReason)
	}
}
