package lspenrich

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/live/lspqueue"
)

// Cascade orchestrates the §14.4 LSP cascade for one file. Phase 61 D-07.
//
// Cascade.Run executes the 6-step pipeline:
//
//  1. textDocument/documentSymbol — populates result.Symbols.
//  2. drain pushed publishDiagnostics for the file.
//  3. for each prioritized symbol: hover → callHierarchy → typeHierarchy
//     → implementation; reference budget consumed AFTER symbol-level work.
//  4. for each prioritized non-definition reference: textDocument/definition.
//
// Between every step the cascade calls foregroundBusy() and budget.HasRemain-
// ingTime; on busy=true it commits the partial OverlayTx with
// MarkFileSemanticPending(path, "preempted") and returns OutcomePartial-
// Preempted. On budget expiry it commits with "budget exhausted" and returns
// OutcomePartialBudget. On a non-MethodNotFound LSP error it commits with
// "lsp_unavailable" and returns OutcomePartialLSPUnavail. MethodNotFound on
// any cascade step is logged once at slog.Debug, marked unsupported in the
// per-(lang, method) capability cache, and the cascade continues to the
// next step.
//
// **B2 invariant** — Cascade NEVER releases the lease. The lease lifetime
// is the Manager's (P03) responsibility; the cascade receives a long-lived
// per-(workspace, language) lease via its LSP shim and must not release it.
type Cascade struct {
	// LSP is the shim that wraps the per-language WorkerLease. Tests inject a
	// recording fake; production wires a tiny adapter around *lspool.Worker-
	// Lease.Request. The shim does NOT expose Release — the Manager (P03)
	// owns the cached lease.
	LSP CascadeLSP

	// Store opens per-file overlay write transactions. Each Cascade.Run
	// commits exactly one tx (Phase 60 D-04 epoch contract).
	Store CascadeStore

	// Logger is nil-safe; nil falls through to slog.Default at first use.
	Logger *slog.Logger

	// Metrics is nil-safe; tests inject a recorder, production wires
	// ProdMetricsSink. Cascade.Run does NOT directly emit metrics today;
	// the worker emits LSPEnrichmentTotal/Duration/Errors after Cascade
	// returns. The field is reserved for per-step metric emission.
	Metrics MetricsSink

	// Capabilities tracks per-(lang, method) MethodNotFound observations so
	// repeated cascade runs in the same worker do NOT re-attempt
	// unsupported methods. MUST be non-nil — the worker (P02 Task 4)
	// constructs and shares one cache across all in-flight cascades.
	Capabilities *CapabilityCache
}

// CascadeStore is the narrow store seam used by the cascade. Production
// wiring (P03 Manager) supplies a small adapter around *store.Store.
//
// One method, one purpose: open a per-tx epoch-bumped overlay write
// transaction. Returns a CascadeTx that the cascade uses to upsert facts
// and (on partial outcomes) stamp partial_reason.
type CascadeStore interface {
	BeginCascadeTx(ctx context.Context, repoID string) (CascadeTx, error)
}

// CascadeTx is the per-cascade overlay transaction surface. Production
// adapter wraps *store.OverlayTx; the cascade unit tests use a recording
// fake. The interface is intentionally narrower than *store.OverlayTx —
// it only exposes the methods the cascade actually calls.
type CascadeTx interface {
	UpsertSymbols(ctx context.Context, path string, syms []Symbol) error
	UpsertReferences(ctx context.Context, path string, refs []Reference) error
	UpsertEdges(ctx context.Context, edges []Edge) error
	UpsertDiagnostics(ctx context.Context, path string, diags []Diagnostic) error
	WriteInvalidations(ctx context.Context) error
	MarkFileSemanticPending(ctx context.Context, path, reason string) error
	Commit() error
	Rollback() error
	Epoch() uint64
}

// CascadeLSP is the LSP-call shim consumed by Cascade.Run. Production
// wraps *lspool.WorkerLease.Request with method-name dispatch; tests inject
// a recording fake.
//
// MethodNotFound contract: any method that returns ErrMethodNotFound is
// logged once at slog.Debug, recorded in the capability cache, and the
// cascade continues to the next step. Any OTHER non-nil error commits
// the partial OverlayTx and returns OutcomePartialLSPUnavail.
type CascadeLSP interface {
	DocumentSymbol(ctx context.Context, path string) ([]Symbol, error)
	DrainDiagnostics(path string) []Diagnostic
	Hover(ctx context.Context, sym Symbol) (*Edge, error)
	CallHierarchy(ctx context.Context, sym Symbol, depth int) ([]Edge, error)
	TypeHierarchy(ctx context.Context, sym Symbol, depth int) ([]Edge, error)
	Implementation(ctx context.Context, sym Symbol) ([]Edge, error)
	Definition(ctx context.Context, ref Reference) (*Edge, error)
	ReferencesForSymbol(sym Symbol) []Reference
}

// Symbol is the cascade-internal symbol fact emitted by documentSymbol and
// passed through hover/callHierarchy/typeHierarchy/implementation.
type Symbol struct {
	Name string
	Path string
	Kind string
}

// Reference is the cascade-internal reference fact. IsDefinition is the
// load-bearing guard for SPEC line 1261 ("definition is NEVER called on a
// definition").
type Reference struct {
	Name         string
	Path         string
	IsDefinition bool
}

// Edge is the cascade-internal edge fact. All cascade-emitted edges land
// with Confidence=1.0 + ValidationState="validated" + Source="lsp.<call>"
// (Phase 61 D-07 acceptance #9).
type Edge struct {
	Kind            string  // CALLS / EXTENDS / IMPLEMENTS / TYPE_OF / RESOLVES_TO
	Source          string  // "lsp.hover" | "lsp.callHierarchy" | …
	Confidence      float64 // always 1.0 from cascade
	ValidationState string  // always "validated" from cascade
}

// Diagnostic is the cascade-internal diagnostic fact (drained from
// publishDiagnostics).
type Diagnostic struct {
	Path     string
	Range    string // pointer back into LSP Range; Phase 62 will replace with typed shape
	Message  string
	Severity int
}

// CapabilityCache tracks per-(lang, method) unsupported observations so the
// cascade does NOT re-attempt MethodNotFound methods on the same worker.
// MUST be shared across cascade invocations within a worker — the Cascade
// struct holds a reference, the worker creates one cache per RunN call.
type CapabilityCache struct {
	mu          sync.RWMutex
	unsupported map[string]map[string]bool // lang → method → true
}

// NewCapabilityCache constructs an empty cache. The worker (P02 Task 4) and
// the cascade tests both call this directly.
func NewCapabilityCache() *CapabilityCache {
	return &CapabilityCache{unsupported: make(map[string]map[string]bool)}
}

// MarkUnsupported records that (lang, method) returned MethodNotFound. Safe
// for concurrent use across worker goroutines.
func (c *CapabilityCache) MarkUnsupported(lang, method string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.unsupported[lang] == nil {
		c.unsupported[lang] = map[string]bool{}
	}
	c.unsupported[lang][method] = true
}

// IsUnsupported reports whether (lang, method) was previously marked
// unsupported. Read-lock-only — never blocks the cascade.
func (c *CapabilityCache) IsUnsupported(lang, method string) bool {
	if c == nil {
		return false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.unsupported[lang] == nil {
		return false
	}
	return c.unsupported[lang][method]
}

// ErrMethodNotFound is the sentinel cascade.go callers use to recognise the
// JSON-RPC -32601 error code. The shim adapter (production: wraps
// *lspool.WorkerLease.Request) returns this sentinel when the JSON-RPC
// response carries Code=-32601; the cascade test fake injects it directly.
var ErrMethodNotFound = errors.New("lspenrich: LSP MethodNotFound (-32601)")

// IsMethodNotFound reports whether err is ErrMethodNotFound (direct equality
// or wrapping). The production CascadeLSP shim adapter (in P03 daemon
// wiring) is responsible for wrapping *jsonrpc.ResponseError{Code:-32601}
// into ErrMethodNotFound before returning to the cascade — keeping the
// package boundary clean (no internal/kernel/jsonrpc import here, per
// ENRICH-01 nosemantic2kernel vet analyzer).
func IsMethodNotFound(err error) bool {
	return errors.Is(err, ErrMethodNotFound)
}

// JobForTest is a test helper that constructs a RevalidateFileJob from
// (repoID, path). Lives in cascade.go (NOT _test.go) so the
// cascade_overlay_epoch_test.go _test file in the lspenrich_test package can
// use it without recreating the import-tangle on lspqueue+semantic.
func JobForTest(repoID, path string) lspqueue.RevalidateFileJob {
	return lspqueue.RevalidateFileJob{RepoID: semantic.RepoID(repoID), Path: path}
}

// Run executes the §14.4 cascade for one file. Commits exactly one
// CascadeTx; partial outcomes stamp partial_reason via MarkFileSemantic-
// Pending. Returns the closed-enum Outcome.
//
// foregroundBusy is invoked between every cascade step. Tests inject a
// counter-based stub; production passes acquirer.ForegroundBusy(wsKey).
//
// budget is mutated in place — ConsumeSymbol and ConsumeReferences
// decrement counters; HasRemainingTime is checked before every step.
func (c *Cascade) Run(
	ctx context.Context,
	job lspqueue.RevalidateFileJob,
	lang string,
	budget *Budget,
	foregroundBusy func() bool,
) Outcome {
	logger := c.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if c.Capabilities == nil {
		c.Capabilities = NewCapabilityCache()
	}

	tx, err := c.Store.BeginCascadeTx(ctx, string(job.RepoID))
	if err != nil {
		logger.Warn("cascade: BeginCascadeTx failed", "repo", job.RepoID, "err", err)
		return OutcomeDropped
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	// checkBoundary returns "" to continue, or a sentinel Outcome string
	// when the cascade must abort. On abort it stamps partial_reason and
	// commits the tx; the caller observes (out != "") and returns out.
	checkBoundary := func(stepName string) Outcome {
		if foregroundBusy != nil && foregroundBusy() {
			_ = tx.MarkFileSemanticPending(ctx, job.Path, "preempted")
			if err := tx.Commit(); err != nil {
				logger.Warn("cascade: commit on preempt failed", "step", stepName, "err", err)
				return OutcomeDropped
			}
			committed = true
			return OutcomePartialPreempted
		}
		if !budget.HasRemainingTime(time.Now()) {
			_ = tx.MarkFileSemanticPending(ctx, job.Path, "budget exhausted")
			if err := tx.Commit(); err != nil {
				logger.Warn("cascade: commit on budget exhaustion failed", "step", stepName, "err", err)
				return OutcomeDropped
			}
			committed = true
			return OutcomePartialBudget
		}
		return ""
	}

	// commitLSPUnavail stamps partial_reason="lsp_unavailable" and commits;
	// returns OutcomePartialLSPUnavail. Used on whole-cascade LS failures
	// (LS crash mid-cascade, generic JSON-RPC errors that are NOT
	// MethodNotFound).
	commitLSPUnavail := func(stepName string, lsErr error) Outcome {
		logger.Warn("cascade: LS error — committing partial",
			"step", stepName, "path", job.Path, "err", lsErr)
		_ = tx.MarkFileSemanticPending(ctx, job.Path, "lsp_unavailable")
		if err := tx.Commit(); err != nil {
			logger.Warn("cascade: commit on lsp_unavailable failed", "step", stepName, "err", err)
			return OutcomeDropped
		}
		committed = true
		return OutcomePartialLSPUnavail
	}

	// Step 1: documentSymbol
	if o := checkBoundary("documentSymbol"); o != "" {
		return o
	}
	syms, err := c.LSP.DocumentSymbol(ctx, job.Path)
	if err != nil {
		if IsMethodNotFound(err) {
			c.Capabilities.MarkUnsupported(lang, "documentSymbol")
			logger.Debug("cascade: documentSymbol unsupported", "lang", lang)
			// Continue; without symbols the per-symbol fan-out is skipped.
			syms = nil
		} else {
			return commitLSPUnavail("documentSymbol", err)
		}
	}
	if len(syms) > 0 {
		if err := tx.UpsertSymbols(ctx, job.Path, syms); err != nil {
			logger.Warn("cascade: UpsertSymbols failed", "err", err)
		}
	}

	// Step 2: drain published diagnostics for this file.
	if o := checkBoundary("diagnostics"); o != "" {
		return o
	}
	diags := c.LSP.DrainDiagnostics(job.Path)
	if len(diags) > 0 {
		if err := tx.UpsertDiagnostics(ctx, job.Path, diags); err != nil {
			logger.Warn("cascade: UpsertDiagnostics failed", "err", err)
		}
	}

	// Step 3: prioritize + per-symbol inner loop.
	prioritized := prioritizeSymbols(syms, budget)
	for _, sym := range prioritized {
		// W10 — per-symbol budget consumption granularity:
		//   ConsumeSymbol once per symbol BEFORE per-symbol LSP calls.
		if !budget.ConsumeSymbol() {
			_ = tx.MarkFileSemanticPending(ctx, job.Path, "budget exhausted")
			if err := tx.Commit(); err != nil {
				return OutcomeDropped
			}
			committed = true
			return OutcomePartialBudget
		}

		// hover
		if o := checkBoundary("hover"); o != "" {
			return o
		}
		if edge, err := c.LSP.Hover(ctx, sym); err != nil {
			if IsMethodNotFound(err) {
				c.Capabilities.MarkUnsupported(lang, "hover")
				logger.Debug("cascade: hover unsupported", "lang", lang)
			} else {
				return commitLSPUnavail("hover", err)
			}
		} else if edge != nil {
			_ = tx.UpsertEdges(ctx, []Edge{*edge})
		}

		// callHierarchy
		if !c.Capabilities.IsUnsupported(lang, "callHierarchy") {
			if o := checkBoundary("callHierarchy"); o != "" {
				return o
			}
			if edges, err := c.LSP.CallHierarchy(ctx, sym, budget.CallDepth()); err != nil {
				if IsMethodNotFound(err) {
					c.Capabilities.MarkUnsupported(lang, "callHierarchy")
					logger.Debug("cascade: callHierarchy unsupported", "lang", lang)
				} else {
					return commitLSPUnavail("callHierarchy", err)
				}
			} else if len(edges) > 0 {
				_ = tx.UpsertEdges(ctx, edges)
			}
		}

		// typeHierarchy
		if !c.Capabilities.IsUnsupported(lang, "typeHierarchy") {
			if o := checkBoundary("typeHierarchy"); o != "" {
				return o
			}
			if edges, err := c.LSP.TypeHierarchy(ctx, sym, budget.TypeDepth()); err != nil {
				if IsMethodNotFound(err) {
					c.Capabilities.MarkUnsupported(lang, "typeHierarchy")
					logger.Debug("cascade: typeHierarchy unsupported", "lang", lang)
				} else {
					return commitLSPUnavail("typeHierarchy", err)
				}
			} else if len(edges) > 0 {
				_ = tx.UpsertEdges(ctx, edges)
			}
		}

		// implementation
		if !c.Capabilities.IsUnsupported(lang, "implementation") {
			if o := checkBoundary("implementation"); o != "" {
				return o
			}
			if edges, err := c.LSP.Implementation(ctx, sym); err != nil {
				if IsMethodNotFound(err) {
					c.Capabilities.MarkUnsupported(lang, "implementation")
					logger.Debug("cascade: implementation unsupported", "lang", lang)
				} else {
					return commitLSPUnavail("implementation", err)
				}
			} else if len(edges) > 0 {
				_ = tx.UpsertEdges(ctx, edges)
			}
		}

		// W10 — per-symbol reference budget. After all symbol-level work,
		// before per-reference go-to-definition, consume references.
		refsForSym := c.LSP.ReferencesForSymbol(sym)
		if len(refsForSym) > 0 {
			if !budget.ConsumeReferences(len(refsForSym)) {
				_ = tx.MarkFileSemanticPending(ctx, job.Path, "budget exhausted")
				if err := tx.Commit(); err != nil {
					return OutcomeDropped
				}
				committed = true
				return OutcomePartialBudget
			}
			// Step 4: per-reference textDocument/definition.
			for _, ref := range refsForSym {
				if ref.IsDefinition {
					// SPEC line 1261: NEVER call definition on a definition.
					continue
				}
				if o := checkBoundary("definition"); o != "" {
					return o
				}
				if edge, err := c.LSP.Definition(ctx, ref); err != nil {
					if IsMethodNotFound(err) {
						c.Capabilities.MarkUnsupported(lang, "definition")
						logger.Debug("cascade: definition unsupported", "lang", lang)
					} else {
						return commitLSPUnavail("definition", err)
					}
				} else if edge != nil {
					_ = tx.UpsertEdges(ctx, []Edge{*edge})
				}
			}
			// Persist the references list for the file.
			_ = tx.UpsertReferences(ctx, job.Path, refsForSym)
		}
	}

	// Phase 62 P02: invalidations are now consumed via the post-commit
	// GraphRepair handoff in internal/semantic/live/handler/handler.go.
	// The recommended path (RESEARCH Open Question 3) derives the typed
	// graphpkg.GraphRepair from the OverlayTx diff rather than a separate
	// invalidations table — handler.updateChangedFileWithKind owns the
	// derivation and calls h.rankApplier.ApplyRepair. The WriteInvalidations
	// seam is retained as a typed no-op for forward compatibility; P03 may
	// swap to a real implementation if a row-based plumbing becomes
	// preferable. See cascade.CascadeTx interface (line 84) for the seam
	// contract.
	_ = tx.WriteInvalidations(ctx)

	if err := tx.Commit(); err != nil {
		logger.Warn("cascade: final commit failed", "err", err)
		return OutcomeDropped
	}
	committed = true
	return OutcomeApplied
}

// prioritizeSymbols returns the subset of syms that fits within the per-
// file symbol budget. Phase 61 v1 is a simple "first-N" selector — Phase 62
// will replace with PageRank-driven prioritization.
func prioritizeSymbols(syms []Symbol, _ *Budget) []Symbol {
	// Trim leading whitespace from names — defensive against LSP responses
	// with leading tabs in symbol names.
	for i := range syms {
		syms[i].Name = strings.TrimSpace(syms[i].Name)
	}
	return syms
}
