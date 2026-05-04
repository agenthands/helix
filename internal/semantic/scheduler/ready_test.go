package scheduler_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/scheduler"
)

// TestDefaultReadyPolicy pins the documented D-04 defaults.
func TestDefaultReadyPolicy(t *testing.T) {
	p := scheduler.DefaultReadyPolicy()
	if p.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", p.Timeout)
	}
	if !p.AllowPartial {
		t.Errorf("AllowPartial = false, want true")
	}
	if p.MinState != scheduler.SemanticPartial {
		t.Errorf("MinState = %q, want %q", p.MinState, scheduler.SemanticPartial)
	}
	if !p.TriggerIfCold {
		t.Errorf("TriggerIfCold = false, want true")
	}
}

// TestRequireReady_ReadyImmediate: state=Ready returns immediately.
func TestRequireReady_ReadyImmediate(t *testing.T) {
	s := scheduler.NewScheduler(nil)
	ws := semantic.WorkspaceID("ws-ready")
	s.TestSetStatus(ws, scheduler.SemanticStatus{State: scheduler.SemanticReady, FilesTotal: 10, FilesDone: 10})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	res, err := s.RequireReady(ctx, ws, scheduler.DefaultReadyPolicy())
	if err != nil {
		t.Fatalf("RequireReady err: %v", err)
	}
	if !res.Ready {
		t.Errorf("Ready=false, want true")
	}
	if res.State != scheduler.SemanticReady {
		t.Errorf("State=%q, want SemanticReady", res.State)
	}
}

// TestRequireReady_PartialWithAllowPartial: state=Partial + AllowPartial=true
// returns immediately.
func TestRequireReady_PartialWithAllowPartial(t *testing.T) {
	s := scheduler.NewScheduler(nil)
	ws := semantic.WorkspaceID("ws-partial")
	s.TestSetStatus(ws, scheduler.SemanticStatus{State: scheduler.SemanticPartial, Partial: true, FilesDone: 5, FilesTotal: 10})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	res, err := s.RequireReady(ctx, ws, scheduler.DefaultReadyPolicy())
	if err != nil {
		t.Fatalf("RequireReady err: %v", err)
	}
	if !res.Ready {
		t.Errorf("Ready=false, want true (partial+AllowPartial)")
	}
	if !res.Partial {
		t.Errorf("Partial=false, want true")
	}
}

// TestRequireReady_PartialWithoutAllowPartialWaits: state=Partial without
// AllowPartial blocks until a SemanticReady transition arrives.
func TestRequireReady_PartialWithoutAllowPartialWaits(t *testing.T) {
	s := scheduler.NewScheduler(nil)
	ws := semantic.WorkspaceID("ws-partial-strict")
	s.TestSetStatus(ws, scheduler.SemanticStatus{State: scheduler.SemanticPartial, Partial: true, FilesDone: 3, FilesTotal: 10})

	policy := scheduler.DefaultReadyPolicy()
	policy.AllowPartial = false
	policy.Timeout = 500 * time.Millisecond

	type result struct {
		res scheduler.ReadyResult
		err error
	}
	done := make(chan result, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		r, e := s.RequireReady(ctx, ws, policy)
		done <- result{r, e}
	}()

	// Give RequireReady time to install its subscriber, then publish Ready.
	time.Sleep(50 * time.Millisecond)
	s.TestSetStatus(ws, scheduler.SemanticStatus{State: scheduler.SemanticReady, FilesDone: 10, FilesTotal: 10})

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("RequireReady err: %v", r.err)
		}
		if !r.res.Ready {
			t.Errorf("Ready=false, want true after Ready transition")
		}
		if r.res.State != scheduler.SemanticReady {
			t.Errorf("State=%q, want SemanticReady", r.res.State)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("RequireReady did not return after Ready transition")
	}
}

// TestRequireReady_TriggerIfCold: not_started + TriggerIfCold causes
// ScheduleInitialExtraction to fire, transitioning state to Indexing. The
// returned subscriber should observe the Indexing transition (which doesn't
// satisfy MinState=Partial), so RequireReady continues to wait until timeout.
// Test asserts the kick happened by checking HasInflightJob.
func TestRequireReady_TriggerIfCold(t *testing.T) {
	s := scheduler.NewScheduler(nil)
	ws := semantic.WorkspaceID("ws-cold")

	if s.HasInflightJob(ws) {
		t.Fatalf("precondition: workspace already has in-flight job")
	}

	policy := scheduler.DefaultReadyPolicy()
	policy.Timeout = 80 * time.Millisecond

	res, err := s.RequireReady(context.Background(), ws, policy)
	if err != nil {
		t.Fatalf("RequireReady err: %v", err)
	}
	// Timeout best-partial: FilesDone==0 so Ready=false.
	if res.Ready {
		t.Errorf("Ready=true on timeout with FilesDone=0, want false")
	}
	if !s.HasInflightJob(ws) {
		t.Errorf("expected scheduler to have kicked an initial-extraction job (TriggerIfCold)")
	}
	if res.State != scheduler.SemanticIndexing {
		t.Errorf("State=%q, want SemanticIndexing after kick", res.State)
	}
}

// TestRequireReady_TriggerIfCold_Disabled: TriggerIfCold=false leaves the
// scheduler untouched.
func TestRequireReady_TriggerIfCold_Disabled(t *testing.T) {
	s := scheduler.NewScheduler(nil)
	ws := semantic.WorkspaceID("ws-cold-disabled")

	policy := scheduler.DefaultReadyPolicy()
	policy.Timeout = 50 * time.Millisecond
	policy.TriggerIfCold = false

	_, err := s.RequireReady(context.Background(), ws, policy)
	if err != nil {
		t.Fatalf("RequireReady err: %v", err)
	}
	if s.HasInflightJob(ws) {
		t.Errorf("TriggerIfCold=false should NOT kick scheduler")
	}
}

// TestRequireReady_TimeoutReturnsBestPartial: Timeout fires; Ready reflects
// FilesDone>0.
func TestRequireReady_TimeoutReturnsBestPartial(t *testing.T) {
	s := scheduler.NewScheduler(nil)
	ws := semantic.WorkspaceID("ws-timeout")
	s.TestSetStatus(ws, scheduler.SemanticStatus{State: scheduler.SemanticIndexing, FilesDone: 3, FilesTotal: 10})

	policy := scheduler.DefaultReadyPolicy()
	policy.Timeout = 30 * time.Millisecond
	policy.TriggerIfCold = false

	start := time.Now()
	res, err := s.RequireReady(context.Background(), ws, policy)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("RequireReady err: %v", err)
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("Timeout took %v, want <200ms", elapsed)
	}
	if !res.Ready {
		t.Errorf("Ready=false, want true (best-partial: FilesDone>0)")
	}
	if res.FilesDone != 3 {
		t.Errorf("FilesDone=%d, want 3", res.FilesDone)
	}
}

// TestRequireReady_FailedReturnsError: state=Failed returns ErrSemanticFailed.
func TestRequireReady_FailedReturnsError(t *testing.T) {
	s := scheduler.NewScheduler(nil)
	ws := semantic.WorkspaceID("ws-failed")
	s.TestSetStatus(ws, scheduler.SemanticStatus{State: scheduler.SemanticFailed})

	res, err := s.RequireReady(context.Background(), ws, scheduler.DefaultReadyPolicy())
	if !errors.Is(err, scheduler.ErrSemanticFailed) {
		t.Fatalf("err=%v, want ErrSemanticFailed", err)
	}
	if res.Ready {
		t.Errorf("Ready=true on failed, want false")
	}
}

// TestRequireReady_FailedDuringWait: a Failed transition mid-wait surfaces as
// the ErrSemanticFailed error.
func TestRequireReady_FailedDuringWait(t *testing.T) {
	s := scheduler.NewScheduler(nil)
	ws := semantic.WorkspaceID("ws-failed-mid")
	s.TestSetStatus(ws, scheduler.SemanticStatus{State: scheduler.SemanticIndexing})

	policy := scheduler.DefaultReadyPolicy()
	policy.Timeout = 500 * time.Millisecond
	policy.TriggerIfCold = false

	type result struct {
		res scheduler.ReadyResult
		err error
	}
	done := make(chan result, 1)
	go func() {
		r, e := s.RequireReady(context.Background(), ws, policy)
		done <- result{r, e}
	}()
	time.Sleep(30 * time.Millisecond)
	s.TestSetStatus(ws, scheduler.SemanticStatus{State: scheduler.SemanticFailed})

	select {
	case r := <-done:
		if !errors.Is(r.err, scheduler.ErrSemanticFailed) {
			t.Fatalf("err=%v, want ErrSemanticFailed", r.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("RequireReady did not return after Failed transition")
	}
}

// TestRequireReady_ContextCancel: ctx.Done() returns ctx.Err() with the
// latest known status.
func TestRequireReady_ContextCancel(t *testing.T) {
	s := scheduler.NewScheduler(nil)
	ws := semantic.WorkspaceID("ws-ctx")
	s.TestSetStatus(ws, scheduler.SemanticStatus{State: scheduler.SemanticIndexing})

	policy := scheduler.DefaultReadyPolicy()
	policy.Timeout = 5 * time.Second
	policy.TriggerIfCold = false

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	_, err := s.RequireReady(ctx, ws, policy)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v, want context.Canceled", err)
	}
}
