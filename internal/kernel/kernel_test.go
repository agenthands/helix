package kernel

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/postfix/serena/internal/kernel/lspool"
	"github.com/postfix/serena/internal/langregistry"
	"github.com/postfix/serena/internal/workspace"
)

// recordingSessionSink captures every SessionLifecycleInc call so the
// idempotency invariant from WR-02 can be asserted.
type recordingSessionSink struct {
	mu    sync.Mutex
	calls []struct {
		Lang  string
		Phase string
	}
}

func (r *recordingSessionSink) SessionLifecycleInc(lang, phase string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, struct {
		Lang  string
		Phase string
	}{lang, phase})
}

func (r *recordingSessionSink) countByPhase(phase string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, c := range r.calls {
		if c.Phase == phase {
			n++
		}
	}
	return n
}

// TestActivateWorkspace_EmitsActivateOnce locks the WR-02 invariant: calling
// ActivateWorkspace multiple times on the same root must emit
// SessionLifecycleInc(phase="activate") exactly once for the lifetime of the
// workspace. Both lazyActivateFn and SetActivateCallback in the daemon invoke
// ActivateWorkspace, so any refactor that re-emits on cache hit will silently
// double-count.
func TestActivateWorkspace_EmitsActivateOnce(t *testing.T) {
	langReg, err := langregistry.NewRegistry()
	if err != nil {
		t.Fatalf("creating language registry: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	wsReg := workspace.NewRegistry()
	k := NewKernel(
		wsReg,
		langReg,
		nil, // installer optional
		KernelConfig{Pool: lspool.DefaultPoolConfig()},
		nil, // pressure optional
		logger,
		lspool.NoopSink{},
		nil, // tracer falls back to noop
	)

	sink := &recordingSessionSink{}
	k.SetSessionMetricsSink(sink)

	dir := t.TempDir()
	ctx := context.Background()

	// First activation: emits activate.
	if _, err := k.ActivateWorkspace(ctx, dir); err != nil {
		t.Fatalf("first ActivateWorkspace: %v", err)
	}
	// Second activation on same root: must NOT re-emit activate.
	if _, err := k.ActivateWorkspace(ctx, dir); err != nil {
		t.Fatalf("second ActivateWorkspace: %v", err)
	}
	// Third call (simulating SetActivateCallback after lazyActivateFn) — still must not re-emit.
	if _, err := k.ActivateWorkspace(ctx, dir); err != nil {
		t.Fatalf("third ActivateWorkspace: %v", err)
	}

	got := sink.countByPhase(PhaseActivate)
	if got != 1 {
		t.Fatalf("ActivateWorkspace emitted phase=activate %d times across 3 calls; want exactly 1 (WR-02 idempotency invariant)", got)
	}
}
