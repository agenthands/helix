package semantic

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/agenthands/helix/internal/workspace"
)

// ErrModeMustBeResolved is returned by IndexRunner.Run when the caller passes
// an unresolved mode ("auto" or empty). Callers MUST call ResolveAuto first
// and pass the resolved value ("full" or "incremental") to Run, so the
// singleflight key is keyed on the RESOLVED mode (closes checker B5).
var ErrModeMustBeResolved = errors.New(
	"IndexRunner.Run requires resolved mode (full|incremental); call ResolveAuto first",
)

// buildState is the per-build progress record shared between the foreground
// (sync-with-timeout) caller and the background buildFn goroutine.
//
// Stub for the RED phase — Task 2 fills in the singleflight + sync.Map
// orchestration that uses this struct.
type buildState struct {
	snapshotID   uint64
	startedAt    time.Time
	mode         string
	filesIndexed atomic.Int64
	filesReused  atomic.Int64
	cancel       context.CancelFunc
	done         chan struct{}
}

// IndexRunner is the skeleton type that the GREEN phase (Task 2) implements.
// This stub exists so the RED-phase tests compile and fail at runtime.
type IndexRunner struct{}

// NewIndexRunner is the production constructor — Task 2 fills in the body.
func NewIndexRunner(
	store StoreAccessor,
	buildFn func(ctx context.Context, ws workspace.WorkspaceKey, mode string, st *buildState) (IndexResult, error),
	timeout time.Duration,
) *IndexRunner {
	return &IndexRunner{}
}

// Run is the production-bound singleflight + sync-with-timeout dispatcher.
// Stub for the RED phase — Task 2 implements it.
func (r *IndexRunner) Run(ctx context.Context, ws workspace.WorkspaceKey, mode string, maxMs int) (IndexResult, error) {
	panic("not implemented")
}

// ResolveAuto resolves mode="auto" to "full" or "incremental" per CONTEXT.md
// D-03. Stub for the RED phase — Task 2 implements it.
func (r *IndexRunner) ResolveAuto(ctx context.Context, ws workspace.WorkspaceKey) string {
	panic("not implemented")
}
