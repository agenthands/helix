package semantic

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/workspace"
)

// mockStoreAccessor is a tiny StoreAccessor used by runner tests that only
// exercise ResolveAuto. The other StoreAccessor methods are unused here.
type mockStoreAccessor struct {
	latestSnapshotID uint64
	latestErr        error
}

func (m *mockStoreAccessor) LatestCommittedSnapshot(ctx context.Context, repoID string) (uint64, error) {
	return m.latestSnapshotID, m.latestErr
}

func (m *mockStoreAccessor) CurrentGraphVersion(ctx context.Context, repoID string) (uint64, error) {
	return 0, nil
}

func (m *mockStoreAccessor) OverlayHasPendingRows(repoID string) bool { return false }

func (m *mockStoreAccessor) QueryEffectiveAdjacency(ctx context.Context, repoID, projection string) (
	map[uint64]map[uint64]float64,
	map[uint64]map[uint64]float64,
	error,
) {
	return nil, nil, nil
}

// Phase 70-04 seam stubs — runner_test.go does not exercise the
// incremental drain path; return cold-start zero values.
func (m *mockStoreAccessor) CurrentOverlayEpoch(ctx context.Context, repoID string) (uint64, error) {
	return 0, nil
}

func (m *mockStoreAccessor) OverlayChangedPathsSince(ctx context.Context, repoID string, baseEpoch uint64) ([]string, uint64, error) {
	return nil, 0, nil
}

func (m *mockStoreAccessor) LatestCommittedSnapshotBaseEpoch(ctx context.Context, repoID string) (uint64, bool, error) {
	return 0, false, nil
}

// mockBuildFn lets tests inject buildFn behavior. The "delay" simulates a slow
// build for timeout-partial tests. The atomic "calls" counter proves
// singleflight join (build invoked exactly once across N concurrent callers).
type mockBuild struct {
	calls       atomic.Int64
	delay       time.Duration
	snapshotID  uint64
	files       int64
	reused      int64
	err         error
	stillAlive  atomic.Bool // set TRUE while inside delay loop, FALSE after delay completes
	progressMid atomic.Int64
}

// makeBuildFn returns a buildFn closure suitable for NewIndexRunner. It records
// every invocation and (optionally) updates st.filesIndexed mid-flight so
// timeout-partial tests can assert progress visibility.
func (m *mockBuild) makeBuildFn() func(ctx context.Context, ws workspace.WorkspaceKey, mode string, st *buildState) (IndexResult, error) {
	return func(ctx context.Context, ws workspace.WorkspaceKey, mode string, st *buildState) (IndexResult, error) {
		m.calls.Add(1)
		m.stillAlive.Store(true)
		defer m.stillAlive.Store(false)

		// Set the in-flight snapshot id immediately so timeout-partial tests
		// can read it from r.inFlight without racing the build's first tick.
		st.snapshotID.Store(m.snapshotID)
		// Halfway through the simulated delay, bump progress counters so the
		// timeout-partial test sees > 0 partial progress.
		if m.delay > 0 {
			half := m.delay / 2
			select {
			case <-time.After(half):
				if m.progressMid.Load() > 0 {
					st.filesIndexed.Store(m.progressMid.Load())
				}
			case <-ctx.Done():
				return IndexResult{}, ctx.Err()
			}
			select {
			case <-time.After(m.delay - half):
			case <-ctx.Done():
				return IndexResult{}, ctx.Err()
			}
		}
		if m.err != nil {
			return IndexResult{}, m.err
		}
		return IndexResult{
			SnapshotID:   m.snapshotID,
			FilesIndexed: m.files,
			FilesReused:  m.reused,
		}, nil
	}
}

func newTestWorkspace() workspace.WorkspaceKey {
	return workspace.WorkspaceKey{RepoRoot: "/tmp/repo-runner-test", Language: "go", Toolchain: "go1.22"}
}

// ---------- ResolveAuto ----------

func TestIndexRunner_AutoResolution_NoSnapshot_ReturnsFull(t *testing.T) {
	store := &mockStoreAccessor{latestSnapshotID: 0}
	r := NewIndexRunner(store, nil, 0)
	got := r.ResolveAuto(context.Background(), newTestWorkspace())
	if got != "full" {
		t.Fatalf("ResolveAuto with no snapshot: got %q, want %q", got, "full")
	}
}

func TestIndexRunner_AutoResolution_HasSnapshot_ReturnsIncremental(t *testing.T) {
	store := &mockStoreAccessor{latestSnapshotID: 1234}
	r := NewIndexRunner(store, nil, 0)
	got := r.ResolveAuto(context.Background(), newTestWorkspace())
	if got != "incremental" {
		t.Fatalf("ResolveAuto with committed snapshot: got %q, want %q", got, "incremental")
	}
}

// ---------- HappyPath / mode-rejection ----------

func TestIndexRunner_HappyPath_Commits(t *testing.T) {
	store := &mockStoreAccessor{}
	mb := &mockBuild{snapshotID: 42, files: 10, reused: 2}
	r := NewIndexRunner(store, mb.makeBuildFn(), 5*time.Second)
	res, err := r.Run(context.Background(), newTestWorkspace(), "full", 5000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.SnapshotID != 42 {
		t.Fatalf("SnapshotID: got %d, want 42", res.SnapshotID)
	}
	if res.Status != IndexStatusCommitted {
		t.Fatalf("Status: got %q, want %q", res.Status, IndexStatusCommitted)
	}
	if res.Partial {
		t.Fatalf("Partial: got true, want false (committed result)")
	}
	if res.FilesIndexed != 10 || res.FilesReused != 2 {
		t.Fatalf("FilesIndexed/Reused: got %d/%d, want 10/2", res.FilesIndexed, res.FilesReused)
	}
	if mb.calls.Load() != 1 {
		t.Fatalf("buildFn calls: got %d, want 1", mb.calls.Load())
	}
}

func TestIndexRunner_RejectsAutoModeAtRun(t *testing.T) {
	store := &mockStoreAccessor{}
	mb := &mockBuild{snapshotID: 99}
	r := NewIndexRunner(store, mb.makeBuildFn(), 5*time.Second)

	for _, badMode := range []string{"auto", ""} {
		_, err := r.Run(context.Background(), newTestWorkspace(), badMode, 5000)
		if !errors.Is(err, ErrModeMustBeResolved) {
			t.Fatalf("Run(%q): want ErrModeMustBeResolved, got %v", badMode, err)
		}
	}
	if mb.calls.Load() != 0 {
		t.Fatalf("buildFn must NEVER be invoked when Run rejects mode; got calls=%d", mb.calls.Load())
	}
}

// ---------- Timeout / partial ----------

func TestIndexRunner_TimeoutReturnsPartialBuilding(t *testing.T) {
	store := &mockStoreAccessor{}
	mb := &mockBuild{snapshotID: 77, delay: 5 * time.Second}
	mb.progressMid.Store(3) // mid-flight files counter
	r := NewIndexRunner(store, mb.makeBuildFn(), 10*time.Second)

	res, err := r.Run(context.Background(), newTestWorkspace(), "full", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Partial {
		t.Fatalf("Partial: got false, want true (timeout)")
	}
	if res.Status != IndexStatusBuilding {
		t.Fatalf("Status: got %q, want %q", res.Status, IndexStatusBuilding)
	}
	if res.SnapshotID != 77 {
		t.Fatalf("SnapshotID: got %d, want 77 (in-progress id)", res.SnapshotID)
	}
	if res.Freshness != FreshnessStale {
		t.Fatalf("Freshness: got %q, want %q", res.Freshness, FreshnessStale)
	}
}

func TestIndexRunner_TimeoutDoesNotCancelBackground(t *testing.T) {
	store := &mockStoreAccessor{}
	mb := &mockBuild{snapshotID: 77, delay: 800 * time.Millisecond}
	r := NewIndexRunner(store, mb.makeBuildFn(), 10*time.Second)

	_, err := r.Run(context.Background(), newTestWorkspace(), "full", 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 200ms after the timeout, the buildFn should still be running because
	// it derives bgCtx from context.Background(), NOT the request ctx (D-04).
	time.Sleep(200 * time.Millisecond)
	if !mb.stillAlive.Load() {
		t.Fatalf("background buildFn was cancelled by request ctx; expected to keep running")
	}
}
