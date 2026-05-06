package lspenrich

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/kernel/lspool"
	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/live/lspqueue"
	"github.com/agenthands/helix/internal/workspace"
	"golang.org/x/sync/errgroup"
)

// LeaseProvider is the seam between Worker and the (P03) Manager that owns
// the cached per (wsKey, lang) leases.  Phase 61 B2 fix-path-A:
//
//   - Worker.processOne calls LeaseProvider.AcquireFor exactly once per
//     job and DOES NOT call Release; the Manager (P03) holds the cached
//     handle and releases it on Manager.OnWorkspaceDeactivate per CONTEXT
//     lines 492-494.
//   - Production: *Manager (P03 Task ?-) implements this interface and
//     wraps a sync.Map of cached *lspool.WorkerLease keyed by
//     (wsKey, lang); on ErrCircuitOpen / spawn-error the Manager discards
//     the failed handle so the next AcquireFor call retries.
//   - Tests: a recording fake (worker_test.go fakeLeaseProvider) that
//     counts both AcquireFor and Release calls; assertions enforce the
//     B2 invariant that Release is never invoked from the worker side.
//
// The interface is intentionally a single method — keeping it narrow
// makes the Manager wiring trivial and keeps Worker's responsibilities
// focused on per-job orchestration (readiness, budget, cascade, metrics).
type LeaseProvider interface {
	AcquireFor(ctx context.Context, wsKey workspace.WorkspaceKey, lang string) (*lspool.WorkerLease, error)
}

// CascadeLSPFactory converts a *lspool.WorkerLease into a CascadeLSP.  The
// production wiring (P03 daemon bootstrap) supplies a small adapter that
// dispatches to lease.Request with method-name routing; the unit tests
// inject a recording fake to exercise per-step behaviour without a live
// language server.
type CascadeLSPFactory func(lease *lspool.WorkerLease) CascadeLSP

// Worker drains the LaneQueue and runs the §14.4 cascade per file.  Phase
// 61 D-02 + D-04 + D-07.  Field semantics:
//
//   - Queue: shared 2-lane priority queue; Worker.RunN spawns N goroutines
//     all draining the same queue (D-02 concurrency cap).
//   - Leases: B2 LeaseProvider — production: *Manager (P03); tests: fake.
//   - Acquirer: ONLY used for ForegroundBusy (D-04 yield gate).  The worker
//     does NOT call AcquireLease directly — that path lives inside
//     Manager.AcquireFor which prefixes sessionID with "lsp-enrichment:".
//   - Store: opens per-file overlay txs on the markPending fast path
//     (readiness timeout / pre-cascade error).
//   - Readiness: Java/Rust gate consulted BEFORE AcquireFor (acceptance
//     #10).
//   - BudgetCfg: per-file budget config (TimeoutPerFile / TimeoutTotal /
//     MaxSymbolsPerFile / ...).  Worker derives the per-job ctx deadline
//     from BudgetCfg.TimeoutPerFile.
//   - Capabilities: shared per-(lang, method) MethodNotFound cache; one
//     instance per Worker, shared across cascade invocations.  Auto-
//     initialized on first RunN call (W12 invariant: cascade Capabilities
//     is never nil).
//   - Metrics: nil-safe; falls back to noopMetrics{}.
//   - Logger: nil-safe; falls back to slog.Default at first use.
//   - Now: clock injection seam for tests; nil → time.Now.
//   - NewCascadeLSP: factory injection seam for tests; production wires
//     a small adapter around *lspool.WorkerLease.Request.  Worker.processOne
//     panics if NewCascadeLSP is nil at the moment a cascade would run, so
//     production wiring MUST set it.  Tests that drop into the dropped path
//     before the cascade (W2/W4/W5/W6/W7/W8/W11) leave it nil safely.
type Worker struct {
	Queue        *LaneQueue
	Leases       LeaseProvider
	Acquirer     LeaseAcquirer
	Store        CascadeStore
	Readiness    ReadinessProbe
	BudgetCfg    semantic.LSPEnrichmentConfig
	Capabilities *CapabilityCache
	Metrics      MetricsSink
	Logger       *slog.Logger
	Now          func() time.Time

	// NewCascadeLSP constructs the CascadeLSP shim around the acquired lease.
	// Production: small adapter dispatching to lease.Request; tests: inject
	// a recording fake.  Nil is permitted for tests that exercise only the
	// readiness / dropped paths.
	NewCascadeLSP CascadeLSPFactory
}

// RunN spawns n drain goroutines and blocks until ctx is cancelled or any
// goroutine returns a non-nil error (errgroup semantics).  All goroutines
// share the same Queue; the strict-priority Drain inside LaneQueue.Drain
// enforces high-before-background ordering.
//
// Defaults: n <= 0 → n = 1.  Now → time.Now.  Logger → slog.Default.
// Metrics → noopMetrics{}.  Capabilities → newCapabilityCache().
func (w *Worker) RunN(ctx context.Context, n int) error {
	if n <= 0 {
		n = 1
	}
	if w.Now == nil {
		w.Now = time.Now
	}
	if w.Logger == nil {
		w.Logger = slog.Default()
	}
	if w.Metrics == nil {
		w.Metrics = noopMetrics{}
	}
	if w.Capabilities == nil {
		w.Capabilities = NewCapabilityCache()
	}

	workerStart := w.Now()
	g, gctx := errgroup.WithContext(ctx)
	for i := 0; i < n; i++ {
		g.Go(func() error { return w.drain(gctx, workerStart) })
	}
	return g.Wait()
}

// drain reads jobs from the lane queue until ctx is cancelled.  After every
// job (success or failure) the drain emits LaneDepth gauges for both lanes
// so operators can alert on background-lane growth.
func (w *Worker) drain(ctx context.Context, workerStart time.Time) error {
	for {
		job, lane, err := w.Queue.Drain(ctx)
		if err != nil {
			return err
		}
		w.processOne(ctx, job, lane, workerStart)
		w.Metrics.LSPEnrichmentLaneDepth("high", w.Queue.Depth(LaneHigh))
		w.Metrics.LSPEnrichmentLaneDepth("background", w.Queue.Depth(LaneBackground))
	}
}

// processOne executes the per-file pipeline:
//
//  1. Derive language from job.Path; build per-job ctx with TimeoutPerFile
//     deadline.
//  2. Honor readiness gate (Java / Rust before AcquireFor).  Timeout →
//     markPending(lsp_unavailable) + outcome=partial_lsp_unavailable.
//  3. AcquireFor cached lease via LeaseProvider (B2 — no Release).
//     ErrCircuitOpen / ErrMaxWorkersReached → outcome=dropped, no
//     partial_reason.  Other errors → markPending(lsp_unavailable) +
//     outcome=partial_lsp_unavailable.
//  4. Build per-file Budget from cfg.
//  5. Construct Cascade with EXPLICIT Capabilities (W3 — never nil) and
//     run; record LSPEnrichmentTotal + LSPEnrichmentDuration with the
//     cascade outcome string.
func (w *Worker) processOne(ctx context.Context, job lspqueue.RevalidateFileJob, lane Lane, workerStart time.Time) {
	_ = lane // recorded via LaneDepth metric in drain()

	lang := languageFor(job.Path)
	wsKey := workspace.WorkspaceKey{
		// Phase 61 v1: repoRoot derived from job.RepoID; Manager (P03)
		// supplies the canonical (RepoRoot, Language, Toolchain) tuple
		// when wired through real workspaces.  For unit tests we use a
		// minimal key — enough to satisfy the readiness probe + lease
		// provider seams without depending on workspace lookup.
		RepoRoot: string(job.RepoID),
		Language: lang,
	}

	perFile, parseErr := time.ParseDuration(w.BudgetCfg.TimeoutPerFile)
	if parseErr != nil || perFile <= 0 {
		w.Logger.Warn("worker: bad TimeoutPerFile config",
			"err", parseErr, "value", w.BudgetCfg.TimeoutPerFile)
		w.Metrics.LSPEnrichmentTotal(lang, string(OutcomeDropped))
		return
	}
	jobCtx, cancel := context.WithTimeout(ctx, perFile)
	defer cancel()

	start := w.Now()

	// Step 1: readiness gate.  Java / Rust block here; everything else
	// falls through immediately (the AcquireFor step is the gate for
	// best-effort languages — W4 broken-LS-install scenario).
	if err := WaitForLanguageReady(jobCtx, w.Readiness, lang, wsKey); err != nil {
		w.markPending(jobCtx, job, "lsp_unavailable")
		w.Metrics.LSPEnrichmentTotal(lang, string(OutcomePartialLSPUnavail))
		w.Metrics.LSPEnrichmentErrors(lang, "readiness_timeout")
		return
	}

	// Step 2: AcquireFor cached lease (B2 — no Release).
	lease, err := w.Leases.AcquireFor(jobCtx, wsKey, lang)
	if err != nil {
		switch {
		case errors.Is(err, serr.ErrCircuitOpen):
			w.Metrics.LSPEnrichmentTotal(lang, string(OutcomeDropped))
			w.Metrics.LSPEnrichmentErrors(lang, "circuit_open")
			return
		case errors.Is(err, lspool.ErrMaxWorkersReached):
			w.Metrics.LSPEnrichmentTotal(lang, string(OutcomeDropped))
			w.Metrics.LSPEnrichmentErrors(lang, "other")
			return
		default:
			w.markPending(jobCtx, job, "lsp_unavailable")
			w.Metrics.LSPEnrichmentTotal(lang, string(OutcomePartialLSPUnavail))
			w.Metrics.LSPEnrichmentErrors(lang, "other")
			return
		}
	}

	// **B2 invariant**: NO defer lease.Release().  Lease lifetime is
	// managed by the Manager (P03) per CONTEXT lines 492-494 — released
	// on Manager.OnWorkspaceDeactivate, not per-job.

	// Step 3: build per-file budget.
	budget, err := NewBudget(start, workerStart, w.BudgetCfg)
	if err != nil {
		w.Logger.Warn("worker: NewBudget failed", "err", err)
		w.Metrics.LSPEnrichmentTotal(lang, string(OutcomeDropped))
		return
	}

	// Step 4: construct Cascade with EXPLICIT Capabilities (W3) and run.
	if w.NewCascadeLSP == nil {
		w.Logger.Error("worker: NewCascadeLSP factory is nil; cannot run cascade",
			"path", job.Path)
		w.Metrics.LSPEnrichmentTotal(lang, string(OutcomeDropped))
		return
	}
	cascade := &Cascade{
		LSP:          w.NewCascadeLSP(lease),
		Store:        w.Store,
		Logger:       w.Logger,
		Metrics:      w.Metrics,
		Capabilities: w.Capabilities, // W3 — explicit; never nil
	}
	busy := func() bool {
		if w.Acquirer == nil {
			return false
		}
		return w.Acquirer.ForegroundBusy(wsKey)
	}
	outcome := cascade.Run(jobCtx, job, lang, &budget, busy)

	// Step 5: record outcome + duration metrics.
	dur := w.Now().Sub(start).Seconds()
	w.Metrics.LSPEnrichmentDuration(lang, dur)
	w.Metrics.LSPEnrichmentTotal(lang, string(outcome))
}

// markPending opens a one-shot CascadeTx and stamps partial_reason on the
// file.  Used on the pre-cascade failure paths (readiness timeout, generic
// AcquireFor error) where the cascade itself never runs but the file still
// needs a partial_reason marker.
//
// Errors during BeginCascadeTx / MarkFileSemanticPending / Commit are
// logged but not propagated — the calling site has already classified the
// outcome via metrics.
func (w *Worker) markPending(ctx context.Context, job lspqueue.RevalidateFileJob, reason string) {
	if w.Store == nil {
		return
	}
	tx, err := w.Store.BeginCascadeTx(ctx, string(job.RepoID))
	if err != nil {
		if w.Logger != nil {
			w.Logger.Warn("worker: markPending BeginCascadeTx failed",
				"path", job.Path, "reason", reason, "err", err)
		}
		return
	}
	if err := tx.MarkFileSemanticPending(ctx, job.Path, reason); err != nil {
		if w.Logger != nil {
			w.Logger.Warn("worker: MarkFileSemanticPending failed",
				"path", job.Path, "reason", reason, "err", err)
		}
		_ = tx.Rollback()
		return
	}
	if err := tx.Commit(); err != nil {
		if w.Logger != nil {
			w.Logger.Warn("worker: markPending Commit failed",
				"path", job.Path, "reason", reason, "err", err)
		}
	}
}

// languageFor maps a file path to a canonical language label.  The mapping
// is deliberately minimal — Phase 61 only needs the languages whose
// readiness gates / lspool adapters are wired (Java + Rust + the first-class
// Go/TS/JS/Python set).  Unknown extensions return the empty string; the
// readiness gate dispatch (WaitForLanguageReady) handles "" as a best-
// effort fall-through.
func languageFor(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".java":
		return "java"
	case ".rs":
		return "rust"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "typescript"
	case ".js", ".mjs", ".cjs":
		return "javascript"
	case ".jsx":
		return "javascript"
	case ".py":
		return "python"
	case ".kt", ".kts":
		return "kotlin"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp", ".hh", ".hxx":
		return "cpp"
	case ".cs":
		return "csharp"
	case ".swift":
		return "swift"
	case ".ex", ".exs":
		return "elixir"
	case ".clj", ".cljs", ".cljc":
		return "clojure"
	case ".tf":
		return "terraform"
	case ".sh", ".bash":
		return "bash"
	case ".pl", ".pm":
		return "perl"
	case ".ps1":
		return "powershell"
	case ".vue":
		return "vue"
	default:
		return ""
	}
}
