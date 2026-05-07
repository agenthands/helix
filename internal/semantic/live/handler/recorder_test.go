// Phase 62-09 closure: tests for the FileFactDiffRecorder seam introduced
// to address 62-VERIFICATION.md truth #22 (BLOCKER): the live handler post-
// commit hook fired ApplyRepair on an empty FileFactDiff in production.
//
// RED phase: these tests fail to compile until Task 2 lands the recorder
// type, the populateRecorderForTest test seam, and the once-INFO log path.
package handler_test

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	"github.com/agenthands/helix/internal/semantic"
	graphpkg "github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/semantic/live/handler"
)

// recordingRankApplier captures every ApplyRepair invocation so the test
// can pin call counts and inspect the GraphRepair payload that flowed from
// the recorder snapshot.
type recordingRankApplier struct {
	calls []recordedApplyRepairCall
}

type recordedApplyRepairCall struct {
	repoID string
	repair graphpkg.GraphRepair
}

func (r *recordingRankApplier) ApplyRepair(_ context.Context, repoID string, repair graphpkg.GraphRepair) (uint64, bool, error) {
	r.calls = append(r.calls, recordedApplyRepairCall{repoID: repoID, repair: repair})
	return 1, true, nil
}

// recordingSlogHandler captures slog records so tests can assert that the
// once-INFO log fires exactly once per workspace per process.
type recordingSlogHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *recordingSlogHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *recordingSlogHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r.Clone())
	return nil
}
func (h *recordingSlogHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *recordingSlogHandler) WithGroup(string) slog.Handler      { return h }

// slogLoggerAdapter adapts *slog.Logger to the handler.Logger interface
// (Warn/Info with variadic any). Mirrors how the daemon adapts production
// slog handlers; reusable across recorder/empty-diff tests.
type slogLoggerAdapter struct {
	l *slog.Logger
}

func (a *slogLoggerAdapter) Warn(msg string, args ...any) { a.l.Warn(msg, args...) }
func (a *slogLoggerAdapter) Info(msg string, args ...any) { a.l.Info(msg, args...) }

// newRecorderTestHandler constructs a Handler whose store is a no-op
// fakeStore (declared in noop_test.go) and whose RankApplier is the
// recording variant the tests pin assertions on. The optional populator is
// invoked AFTER UpsertOverlayFile and BEFORE Commit (the recorder hook
// site) to simulate Phase 60 P04 / future type-resolver retrofit
// populators without pre-implementing those phases.
func newRecorderTestHandler(t *testing.T, applier *recordingRankApplier, populator func(*handler.FileFactDiffRecorder), logHandler slog.Handler) *handler.Handler {
	t.Helper()
	store := &fakeStore{tx: &fakeTx{epoch: 1}}
	var logger handler.Logger
	if logHandler != nil {
		logger = &slogLoggerAdapter{l: slog.New(logHandler)}
	}
	h := handler.New(store, goodHasher, nil, logger)
	h.SetRankApplier(applier)
	if populator != nil {
		// SetPopulateRecorderForTest is the Task 2 wiring point — the
		// unexported field on the Handler struct is reachable only through
		// this package-private setter (the field is unexported because it
		// MUST remain nil in production).
		handler.SetPopulateRecorderForTest(h, populator)
	}
	return h
}

// TestUpdateChangedFile_RecorderSeamExists pins the FileFactDiffRecorder
// shape: five record methods (one per FileFactDiff slice variant), Snapshot,
// IsEmpty. RED today (the type does not exist).
func TestUpdateChangedFile_RecorderSeamExists(t *testing.T) {
	r := &handler.FileFactDiffRecorder{}

	// Compile-time signature assertions: the methods exist with the
	// documented shapes. If any drifts, the build breaks here.
	var _ func(graphpkg.SymbolDiff) = r.RecordSymbolRemoved
	var _ func(graphpkg.SymbolDiff) = r.RecordSymbolChanged
	var _ func(graphpkg.SymbolDiff) = r.RecordSymbolAdded
	var _ func(graphpkg.GraphEdge) = r.RecordEdgeAdded
	var _ func(graphpkg.GraphEdge) = r.RecordEdgeRemoved
	var _ func() graphpkg.FileFactDiff = r.Snapshot
	var _ func() bool = r.IsEmpty

	if !r.IsEmpty() {
		t.Fatal("zero-value recorder must be empty")
	}
	r.RecordSymbolChanged(graphpkg.SymbolDiff{NodeID: 42, SignatureChanged: true})
	if r.IsEmpty() {
		t.Fatal("after RecordSymbolChanged, IsEmpty must be false")
	}
	snap := r.Snapshot()
	if len(snap.ChangedSymbols) != 1 || snap.ChangedSymbols[0].NodeID != 42 {
		t.Fatalf("snapshot did not capture the recorded change: %+v", snap)
	}
	if !snap.ChangedSymbols[0].SignatureChanged {
		t.Fatalf("snapshot dropped the SignatureChanged flag: %+v", snap.ChangedSymbols[0])
	}
}

// TestUpdateChangedFile_PopulatedDiffFiresApplyRepair locks the contract
// future Phase 60 P04 / type-resolver retrofit populators write through:
// when the recorder is populated with a graph-changing SymbolDiff,
// ApplyRepair fires exactly once with a non-empty GraphRepair.
func TestUpdateChangedFile_PopulatedDiffFiresApplyRepair(t *testing.T) {
	applier := &recordingRankApplier{}
	h := newRecorderTestHandler(t, applier, func(r *handler.FileFactDiffRecorder) {
		// ExportedChanged → graph-changing per D-06; populates DirtyNodes
		// + InvalidatedIncoming + InvalidatedOutgoing.
		r.RecordSymbolChanged(graphpkg.SymbolDiff{NodeID: 7, ExportedChanged: true})
	}, nil)

	if err := h.UpdateChangedFile(context.Background(), semantic.RepoID("repo-A"), "/abs/path/foo.go"); err != nil {
		t.Fatalf("UpdateChangedFile: %v", err)
	}

	if len(applier.calls) != 1 {
		t.Fatalf("expected 1 ApplyRepair call, got %d", len(applier.calls))
	}
	if applier.calls[0].repoID != "repo-A" {
		t.Errorf("repoID: got %q, want %q", applier.calls[0].repoID, "repo-A")
	}
	if applier.calls[0].repair.IsEmpty() {
		t.Fatal("ApplyRepair received an empty repair; recorder snapshot did not flow")
	}
	if len(applier.calls[0].repair.DirtyNodes) == 0 {
		t.Fatal("ExportedChanged should have produced at least one DirtyNodes entry")
	}
}

// TestUpdateChangedFile_EmptyDiffShortCircuits_OnceInfo locks today's
// production behaviour: when no populator runs, the recorder is empty,
// ApplyRepair MUST NOT be called, AND the once-INFO log fires exactly once
// per workspace per Handler instance.
func TestUpdateChangedFile_EmptyDiffShortCircuits_OnceInfo(t *testing.T) {
	applier := &recordingRankApplier{}
	rec := &recordingSlogHandler{}
	h := newRecorderTestHandler(t, applier, nil, rec)

	// Drive twice for repo-A — once-INFO must fire ONCE.
	if err := h.UpdateChangedFile(context.Background(), semantic.RepoID("repo-A"), "/abs/path/foo.go"); err != nil {
		t.Fatalf("call 1 (repo-A): %v", err)
	}
	if err := h.UpdateChangedFile(context.Background(), semantic.RepoID("repo-A"), "/abs/path/foo.go"); err != nil {
		t.Fatalf("call 2 (repo-A): %v", err)
	}
	// Drive once for repo-B — once-INFO fires AGAIN (per-workspace gate).
	if err := h.UpdateChangedFile(context.Background(), semantic.RepoID("repo-B"), "/abs/path/bar.go"); err != nil {
		t.Fatalf("call 3 (repo-B): %v", err)
	}

	if len(applier.calls) != 0 {
		t.Fatalf("ApplyRepair MUST NOT be called for empty diffs; got %d calls", len(applier.calls))
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	infoCount := 0
	for _, r := range rec.records {
		if r.Level == slog.LevelInfo {
			infoCount++
		}
	}
	if infoCount != 2 {
		t.Fatalf("expected exactly 2 INFO logs (one per workspace, once-gated); got %d", infoCount)
	}
}
