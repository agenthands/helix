package graph

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
)

// EdgeUpsert is the value Engine.ApplyRepair hands to RepairTx.
// UpsertEdgesWithMerge. The store-side counterpart (store.EdgeRow) carries
// the same dimensions; the engine layer keeps a separate type so the
// internal/semantic/graph package never imports duckdb-go (D-14 boundary).
type EdgeUpsert struct {
	SrcNodeID, DstNodeID NodeID
	EdgeKind             string
	Source               string
	Confidence           float64
	Weight               float64
	ValidationState      string
	FactJSON             []byte
}

// RepairStore is the narrow seam Engine.ApplyRepair calls into. Production
// = an adapter wrapping *store.Store; unit tests inject a recording fake
// (see apply_repair_test.go).
//
// LockWorkspace MUST acquire the SAME per-workspace mutex BeginOverlayTx
// uses (T4 invariant: no second mutex map). The release function MUST be
// invoked exactly once after the corresponding tx terminates.
type RepairStore interface {
	LockWorkspace(repoID string) (release func())
	BeginRepairTx(ctx context.Context, repoID string) (RepairTx, error)
	CurrentGraphVersion(ctx context.Context, repoID string) (uint64, error)
}

// RepairTx is the per-tx surface Engine.ApplyRepair touches. It is the
// in-package narrow projection of *store.OverlayTx.
type RepairTx interface {
	BumpGraphVersion(ctx context.Context) (uint64, error)
	MarkSymbolsDeleted(ctx context.Context, fileIDs []uint64) error
	MarkEdgesDeleted(ctx context.Context, nodeIDs []uint64) error
	UpsertEdgesWithMerge(ctx context.Context, edges []EdgeUpsert) error
	Commit() error
	Rollback() error
}

// MetricsSink is the bounded-label metrics surface Engine + Scheduler use.
// Production binds to *obs.Metrics; tests inject a recorder. All methods
// are safe to call on a nil sink (no-op).
type MetricsSink interface {
	SemanticGraphVersionSet(workspace string, gv uint64)
	SemanticGraphRepairInc(outcome string)
	SemanticGraphPagerankObserve(scope, projection string, seconds float64)
	SemanticGraphScoreStatusInc(projection, status string)
}

// GraphVersionAdvance is the typed payload published by ApplyRepair to
// downstream consumers (P03 RankScheduler). ChangedNodes is the union of
// repair.DirtyNodes ∪ repair.RemovedNodes ∪ repair.InvalidatedIncoming ∪
// repair.InvalidatedOutgoing, sorted ascending so the scheduler does not
// have to re-sort.
type GraphVersionAdvance struct {
	RepoID       string
	Version      uint64
	ChangedNodes []NodeID
}

// Engine is the Phase 62 graph_version advance machinery. The single
// instance is constructed during daemon bootstrap (Phase 62 P03 wires it).
//
// All fields except `store` are optional: a nil logger falls back to
// slog.Default at first use; nil metrics is a no-op; nil notifyVersion
// drops the publish (the rank scheduler is the only consumer).
type Engine struct {
	store         RepairStore
	logger        *slog.Logger
	metrics       MetricsSink
	notifyVersion chan<- GraphVersionAdvance
}

// NewEngine constructs an Engine bound to the given RepairStore. The
// returned engine is safe for concurrent ApplyRepair calls — concurrency
// is bounded by the per-workspace mutex inside store.LockOverlayWorkspace.
//
// Optional dependencies (logger, metrics, notify channel) are supplied via
// setters so the Phase 62 P03 daemon-wiring path can install them after
// the daemon's metrics + tracing pipeline is initialised.
func NewEngine(store RepairStore) *Engine {
	return &Engine{store: store}
}

// SetLogger installs a slog logger on the engine. Safe to call before or
// after the engine starts servicing ApplyRepair calls.
func (e *Engine) SetLogger(l *slog.Logger) {
	if e == nil {
		return
	}
	e.logger = l
}

// SetMetrics installs a MetricsSink on the engine.
func (e *Engine) SetMetrics(m MetricsSink) {
	if e == nil {
		return
	}
	e.metrics = m
}

// SetNotifyChannel installs the GraphVersionAdvance publish channel. The
// channel is non-blocking: a full channel drops the publish and bumps the
// SemanticGraphRepairInc("error") metric (D-08: scheduler is best-effort).
func (e *Engine) SetNotifyChannel(ch chan<- GraphVersionAdvance) {
	if e == nil {
		return
	}
	e.notifyVersion = ch
}

// ApplyRepair is the SINGLE site that bumps graph_version (D-06). Returns
// (newGV, bumped, err):
//
//   - If repair.IsEmpty() → returns (currentGV, false, nil) WITHOUT
//     acquiring the per-workspace mutex or opening a tx.
//   - Otherwise → acquires the mutex, opens a tx, applies tombstones / edge
//     upserts, calls tx.BumpGraphVersion (the SOLE D-06 advance site),
//     commits, publishes GraphVersionAdvance to the rank scheduler.
func (e *Engine) ApplyRepair(ctx context.Context, repoID string, repair GraphRepair) (uint64, bool, error) {
	if e == nil || e.store == nil {
		return 0, false, fmt.Errorf("ApplyRepair: nil engine or store")
	}
	if repoID == "" {
		return 0, false, fmt.Errorf("ApplyRepair: empty repoID")
	}

	// D-06 short-circuit: body-only edits do NOT bump graph_version.
	if repair.IsEmpty() {
		gv, err := e.store.CurrentGraphVersion(ctx, repoID)
		if err != nil {
			return 0, false, fmt.Errorf("ApplyRepair(%q): read currentGV: %w", repoID, err)
		}
		return gv, false, nil
	}

	// T4 invariant: re-use the per-workspace mutex BeginOverlayTx uses. NO
	// second mutex map in this package.
	release := e.store.LockWorkspace(repoID)
	defer release()

	tx, err := e.store.BeginRepairTx(ctx, repoID)
	if err != nil {
		e.metricInc("error")
		return 0, false, fmt.Errorf("ApplyRepair(%q): begin tx: %w", repoID, err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	// Apply tombstones — pass uint64 slices into the store-typed surface.
	if len(repair.RemovedNodes) > 0 {
		fileIDs := nodeIDsToUint64(repair.RemovedNodes)
		if err := tx.MarkSymbolsDeleted(ctx, fileIDs); err != nil {
			e.metricInc("error")
			return 0, false, fmt.Errorf("ApplyRepair(%q): mark symbols deleted: %w", repoID, err)
		}
	}
	if len(repair.RemovedEdges) > 0 {
		// Edges are tombstoned by node-id (the store API takes an OR-match
		// on src/dst). Project to deduplicated endpoints.
		var endpoints nodeSet
		for _, e := range repair.RemovedEdges {
			endpoints.add(e.SrcNodeID)
			endpoints.add(e.DstNodeID)
		}
		ids := nodeIDsToUint64(endpoints.slice())
		if err := tx.MarkEdgesDeleted(ctx, ids); err != nil {
			e.metricInc("error")
			return 0, false, fmt.Errorf("ApplyRepair(%q): mark edges deleted: %w", repoID, err)
		}
	}
	if len(repair.UpsertedEdges) > 0 {
		ups := make([]EdgeUpsert, 0, len(repair.UpsertedEdges))
		for _, ge := range repair.UpsertedEdges {
			ups = append(ups, EdgeUpsert{
				SrcNodeID:       ge.SrcNodeID,
				DstNodeID:       ge.DstNodeID,
				EdgeKind:        ge.EdgeKind,
				Source:          ge.Source,
				Confidence:      ge.Confidence,
				Weight:          ge.Weight,
				ValidationState: ge.ValidationState,
			})
		}
		if err := tx.UpsertEdgesWithMerge(ctx, ups); err != nil {
			e.metricInc("error")
			return 0, false, fmt.Errorf("ApplyRepair(%q): upsert edges with merge: %w", repoID, err)
		}
	}

	newGV, err := tx.BumpGraphVersion(ctx)
	if err != nil {
		e.metricInc("error")
		return 0, false, fmt.Errorf("ApplyRepair(%q): bump graph_version: %w", repoID, err)
	}
	if err := tx.Commit(); err != nil {
		e.metricInc("error")
		return 0, false, fmt.Errorf("ApplyRepair(%q): commit: %w", repoID, err)
	}
	committed = true

	if e.metrics != nil {
		e.metrics.SemanticGraphVersionSet(repoID, newGV)
	}
	e.metricInc("applied")

	// Publish to the rank scheduler. Non-blocking: a full channel drops the
	// notification and bumps the error metric (D-08 best-effort).
	if e.notifyVersion != nil {
		changed := unionAndSort(
			repair.DirtyNodes,
			repair.RemovedNodes,
			repair.InvalidatedIncoming,
			repair.InvalidatedOutgoing,
		)
		select {
		case e.notifyVersion <- GraphVersionAdvance{
			RepoID:       repoID,
			Version:      newGV,
			ChangedNodes: changed,
		}:
		default:
			e.metricInc("error")
			if e.logger != nil {
				e.logger.Warn("graph: notify channel full; dropping advance",
					"repo_id", repoID, "graph_version", newGV)
			}
		}
	}

	return newGV, true, nil
}

func (e *Engine) metricInc(outcome string) {
	if e == nil || e.metrics == nil {
		return
	}
	e.metrics.SemanticGraphRepairInc(outcome)
}

// nodeIDsToUint64 returns a fresh uint64 slice — safe even though NodeID is
// already a uint64 alias (the conversion is a no-op but the copy preserves
// caller-provided slice ownership).
func nodeIDsToUint64(in []NodeID) []uint64 {
	if len(in) == 0 {
		return nil
	}
	out := make([]uint64, len(in))
	for i, n := range in {
		out[i] = uint64(n)
	}
	return out
}

// unionAndSort returns a sorted-ascending dedup of the supplied slices.
// Used by ApplyRepair to compute GraphVersionAdvance.ChangedNodes — the
// scheduler depends on the order being stable across runs (W3 invariant).
func unionAndSort(slices ...[]NodeID) []NodeID {
	var s nodeSet
	for _, sl := range slices {
		for _, n := range sl {
			s.add(n)
		}
	}
	out := s.slice()
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
