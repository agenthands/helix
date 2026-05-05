package kernel

import (
	"context"
	"sync"
	"testing"

	"github.com/agenthands/helix/internal/workspace"
)

// stubNotifier is a minimal EditNotifier used by the kernel-package
// notifier roundtrip tests. It records every OnEdit invocation so a
// test can assert call count and paths after exercising the kernel.
type stubNotifier struct {
	mu    sync.Mutex
	calls []callRecord
}

type callRecord struct {
	ws    workspace.WorkspaceKey
	paths []string
}

func (s *stubNotifier) OnEdit(ctx context.Context, ws workspace.WorkspaceKey, paths []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, callRecord{ws: ws, paths: append([]string(nil), paths...)})
	return nil
}

// newTestKernel returns a zero-value *Kernel suitable for SetEditNotifier
// / EditNotifier roundtrip testing. The notifier accessors do not depend
// on any of the real kernel subsystems (workspaces map, pool, registry,
// langReg, logger, tracer) so a bare struct literal is sufficient.
func newTestKernel(t *testing.T) *Kernel {
	t.Helper()
	return &Kernel{}
}

func TestKernel_EditNotifier_NilByDefault(t *testing.T) {
	k := newTestKernel(t)
	if got := k.EditNotifier(); got != nil {
		t.Fatalf("EditNotifier on fresh kernel: want nil, got %T", got)
	}
}

func TestKernel_EditNotifier_SetGet(t *testing.T) {
	k := newTestKernel(t)
	s := &stubNotifier{}
	k.SetEditNotifier(s)
	got := k.EditNotifier()
	if got == nil {
		t.Fatal("EditNotifier after Set: want non-nil, got nil")
	}
	if got != s {
		t.Fatalf("EditNotifier after Set: want %p, got %p", s, got)
	}
}

func TestKernel_EditNotifier_Replace(t *testing.T) {
	k := newTestKernel(t)
	a, b := &stubNotifier{}, &stubNotifier{}
	k.SetEditNotifier(a)
	k.SetEditNotifier(b)
	got := k.EditNotifier()
	if got != b {
		t.Fatalf("after second Set: want b (%p), got %p", b, got)
	}
}

func TestKernel_EditNotifier_SetNilClearsValue(t *testing.T) {
	k := newTestKernel(t)
	s := &stubNotifier{}
	k.SetEditNotifier(s)
	if got := k.EditNotifier(); got == nil {
		t.Fatal("precondition: notifier should be set")
	}
	k.SetEditNotifier(nil)
	if got := k.EditNotifier(); got != nil {
		t.Fatalf("after SetEditNotifier(nil): want nil, got %T (%v)", got, got)
	}
}

func TestKernel_EditNotifier_ConcurrentReadDuringWrite(t *testing.T) {
	// Smoke test for the atomic.Value-backed accessor under concurrent
	// readers + a single writer. We don't assert ordering — only that
	// no race or panic surfaces. Run with `go test -race` for full value.
	k := newTestKernel(t)
	a, b := &stubNotifier{}, &stubNotifier{}
	k.SetEditNotifier(a)

	var wg sync.WaitGroup
	const readers = 16
	stop := make(chan struct{})

	wg.Add(readers)
	for i := 0; i < readers; i++ {
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = k.EditNotifier()
				}
			}
		}()
	}

	for i := 0; i < 1000; i++ {
		if i%2 == 0 {
			k.SetEditNotifier(a)
		} else {
			k.SetEditNotifier(b)
		}
	}
	close(stop)
	wg.Wait()
}
