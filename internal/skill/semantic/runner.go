package semantic

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/agenthands/helix/internal/workspace"
	"golang.org/x/sync/singleflight"
)

// ErrModeMustBeResolved is returned by IndexRunner.Run when the caller passes
// an unresolved mode ("auto" or empty). Callers MUST call ResolveAuto first
// and pass the resolved value ("full" or "incremental") to Run, so the
// singleflight key is keyed on the RESOLVED mode (closes checker B5).
var ErrModeMustBeResolved = errors.New(
	"IndexRunner.Run requires resolved mode (full|incremental); call ResolveAuto first",
)

// defaultRunnerTimeout is the per-call ceiling applied when NewIndexRunner is
// called with timeout=0. SPEC §23.1 documents 120s as the index_semantic_graph
// default.
const defaultRunnerTimeout = 120 * time.Second

// buildFn is the concrete build function that BeginSnapshot /
// WriteSnapshotFacts / CommitSnapshot under the bgCtx the runner provides.
// Production wiring lands in P64-08; tests inject a mock buildFn.
type buildFnT func(ctx context.Context, ws workspace.WorkspaceKey, mode string, st *buildState) (IndexResult, error)

// BuildState is the EXPORTED seam over the in-flight build's progress
// counters. The runner exposes this to out-of-package callers (the daemon's
// makeProductionBuildFn in internal/daemon/semantic_wiring.go) so the
// production buildFn can stamp in-flight progress without reaching into the
// runner's private buildState type.
//
// Closes a P64-08 dependency gap surfaced during execution (truth-21
// follow-up): the previous buildFnT signature pinned *buildState, which is
// package-private and unreachable from internal/daemon. The exported
// RunnerBuildFn type below uses this interface so the production wiring
// compiles cleanly.
type BuildState interface {
	// SetSnapshotID stamps the in-flight snapshot id once BeginSnapshot
	// returns. The foreground caller's timeout path reads this back via
	// r.inFlight when surfacing a partial result.
	SetSnapshotID(id uint64)
	// AddFilesIndexed atomically increments the in-flight counter. The
	// foreground caller's partial-result envelope consumes this.
	AddFilesIndexed(delta int64)
	// AddFilesReused atomically increments the reused counter (incremental
	// builds short-circuit reused files instead of reprocessing).
	AddFilesReused(delta int64)
}

// RunnerBuildFn is the EXPORTED build-function shape NewIndexRunnerWithExportedBuildFn
// accepts. Wraps a *buildState into the BuildState interface so out-of-
// package wiring (daemon) can construct buildFns against a stable surface.
type RunnerBuildFn func(ctx context.Context, ws workspace.WorkspaceKey, mode string, st BuildState) (IndexResult, error)

// SetSnapshotID stamps the snapshot id atomically.
func (b *buildState) SetSnapshotID(id uint64) { b.snapshotID.Store(id) }

// AddFilesIndexed increments the in-flight files-indexed counter.
func (b *buildState) AddFilesIndexed(delta int64) { b.filesIndexed.Add(delta) }

// AddFilesReused increments the in-flight files-reused counter.
func (b *buildState) AddFilesReused(delta int64) { b.filesReused.Add(delta) }

// buildState is the per-build progress record shared between the foreground
// (sync-with-timeout) caller and the background buildFn goroutine. The
// foreground caller reads progress fields under timeout; the background
// goroutine writes them as it makes progress so a partial result is
// observable.
type buildState struct {
	// snapshotID is atomic because the foreground caller (timeout path) reads
	// it concurrently with the background buildFn that writes the in-flight
	// id once it knows it (e.g., right after BeginSnapshot returns). Use Load/
	// Store; never read the field directly.
	snapshotID   atomic.Uint64
	startedAt    time.Time
	mode         string
	filesIndexed atomic.Int64
	filesReused  atomic.Int64
	cancel       context.CancelFunc
	done         chan struct{}
}

// IndexRunner orchestrates index_semantic_graph dispatch:
//   - singleflight.Group keyed (workspace_repoRoot, RESOLVED-mode) so concurrent
//     callers join the same in-flight build (D-02).
//   - sync-with-timeout: the foreground caller blocks up to maxMs; on timeout,
//     a partial IndexResult{Status: building, Partial: true} is returned and
//     the background build keeps running (D-04).
//   - mode-resolution: ResolveAuto answers "full" | "incremental" per D-03.
//   - B5 invariant: Run rejects mode="auto" / mode="" with
//     ErrModeMustBeResolved so the singleflight key is always keyed on the
//     resolved mode. The handler is the sole site that resolves "auto".
type IndexRunner struct {
	sf       singleflight.Group
	inFlight sync.Map // key=repoRoot string -> *buildState
	store    StoreAccessor
	buildFn  buildFnT
	timeout  time.Duration
}

// NewIndexRunner constructs a runner with the given store accessor (used by
// ResolveAuto) and buildFn (executed inside the singleflight). When timeout
// is zero, defaultRunnerTimeout (120s) is applied.
func NewIndexRunner(store StoreAccessor, buildFn buildFnT, timeout time.Duration) *IndexRunner {
	if timeout <= 0 {
		timeout = defaultRunnerTimeout
	}
	return &IndexRunner{
		store:   store,
		buildFn: buildFn,
		timeout: timeout,
	}
}

// NewProductionIndexRunner is the EXPORTED constructor for production wiring
// (internal/daemon/semantic_wiring.go). It takes a RunnerBuildFn — the
// exported build-function shape that consumes the BuildState interface —
// and wraps it into the private buildFnT signature the runner uses
// internally. This keeps the existing test surface (NewIndexRunner +
// *buildState) intact while letting out-of-package callers compose against
// a stable seam.
func NewProductionIndexRunner(store StoreAccessor, buildFn RunnerBuildFn, timeout time.Duration) *IndexRunner {
	if buildFn == nil {
		return NewIndexRunner(store, nil, timeout)
	}
	wrapped := func(ctx context.Context, ws workspace.WorkspaceKey, mode string, st *buildState) (IndexResult, error) {
		return buildFn(ctx, ws, mode, st)
	}
	return NewIndexRunner(store, wrapped, timeout)
}

// Run executes (or joins an in-flight) index build for the workspace under
// the resolved mode, blocking up to maxMs milliseconds. On timeout, returns
// an IndexResult with Partial=true and Status=building; the background
// build keeps running.
//
// B5 contract: mode MUST be a resolved mode ("full" or "incremental"). Passing
// "auto" or "" returns ErrModeMustBeResolved without invoking buildFn — the
// handler is solely responsible for auto resolution via ResolveAuto.
func (r *IndexRunner) Run(ctx context.Context, ws workspace.WorkspaceKey, mode string, maxMs int) (IndexResult, error) {
	if mode == "auto" || mode == "" {
		return IndexResult{}, ErrModeMustBeResolved
	}

	// Singleflight key includes the RESOLVED mode so two callers asking for
	// different modes (e.g., "full" + "incremental") do NOT collapse, while
	// two callers asking for the same resolved mode share the same build.
	key := ws.RepoRoot + "|" + mode

	deadline := time.Duration(maxMs) * time.Millisecond
	if deadline <= 0 || deadline > r.timeout {
		deadline = r.timeout
	}

	callCtx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()
	startedAt := time.Now()

	ch := r.sf.DoChan(key, func() (any, error) {
		// CRITICAL (D-04): bgCtx is derived from context.Background(), NOT
		// the request ctx. A timed-out caller must not cancel the in-flight
		// build — the build must keep running so its eventual commit is
		// observable via get_semantic_graph_status.
		bgCtx, bgCancel := context.WithCancel(context.Background())
		st := &buildState{
			startedAt: time.Now(),
			mode:      mode,
			cancel:    bgCancel,
			done:      make(chan struct{}),
		}
		r.inFlight.Store(ws.RepoRoot, st)
		defer r.inFlight.Delete(ws.RepoRoot)
		defer close(st.done)
		defer bgCancel()
		if r.buildFn == nil {
			return IndexResult{}, errors.New("IndexRunner: buildFn not wired")
		}
		return r.buildFn(bgCtx, ws, mode, st)
	})

	select {
	case v := <-ch:
		var res IndexResult
		if v.Val != nil {
			res, _ = v.Val.(IndexResult)
		}
		res.DurationMs = time.Since(startedAt).Milliseconds()
		if v.Err != nil {
			res.Status = IndexStatusFailed
			return res, v.Err
		}
		res.Status = IndexStatusCommitted
		return res, nil

	case <-callCtx.Done():
		// Read in-flight progress for the partial result. The build keeps
		// running in the background.
		var partial IndexResult
		if v, ok := r.inFlight.Load(ws.RepoRoot); ok {
			st := v.(*buildState)
			partial.SnapshotID = st.snapshotID.Load()
			partial.FilesIndexed = st.filesIndexed.Load()
			partial.FilesReused = st.filesReused.Load()
		}
		partial.Partial = true
		partial.Status = IndexStatusBuilding
		partial.Freshness = FreshnessStale
		partial.DurationMs = deadline.Milliseconds()
		return partial, nil
	}
}

// ResolveAuto resolves mode="auto" to "full" (no committed snapshot exists)
// or "incremental" (committed snapshot exists). Per CONTEXT.md D-03.
//
// On store error (e.g., DB unreachable), returns "full" — the safer default
// because a full build can recover from a degraded snapshot history.
func (r *IndexRunner) ResolveAuto(ctx context.Context, ws workspace.WorkspaceKey) string {
	if r.store == nil {
		return "full"
	}
	latest, err := r.store.LatestCommittedSnapshot(ctx, ws.Hash())
	if err != nil || latest == 0 {
		return "full"
	}
	return "incremental"
}

// Shutdown cancels every in-flight background build so daemon shutdown drops
// builds cleanly rather than leaving zombie goroutines. Safe to call multiple
// times.
func (r *IndexRunner) Shutdown(ctx context.Context) {
	r.inFlight.Range(func(key, value any) bool {
		if st, ok := value.(*buildState); ok && st.cancel != nil {
			st.cancel()
		}
		return true
	})
}
