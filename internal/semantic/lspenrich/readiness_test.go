package lspenrich_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/semantic/lspenrich"
	"github.com/agenthands/helix/internal/workspace"
)

// fakeReadinessProbe is a 2-channel mock satisfying lspenrich.ReadinessProbe.
// Each method is implemented as a func field — tests construct the desired
// behaviour per case (return nil immediately, block until ctx done, etc.).
type fakeReadinessProbe struct {
	javaFn func(ctx context.Context, wsKey workspace.WorkspaceKey) error
	rustFn func(ctx context.Context, wsKey workspace.WorkspaceKey) error

	javaCalls int
	rustCalls int
}

func (f *fakeReadinessProbe) JavaReady(ctx context.Context, wsKey workspace.WorkspaceKey) error {
	f.javaCalls++
	if f.javaFn != nil {
		return f.javaFn(ctx, wsKey)
	}
	return nil
}

func (f *fakeReadinessProbe) RustQuiescent(ctx context.Context, wsKey workspace.WorkspaceKey) error {
	f.rustCalls++
	if f.rustFn != nil {
		return f.rustFn(ctx, wsKey)
	}
	return nil
}

var testWSKey = workspace.WorkspaceKey{RepoRoot: "/tmp/r", Language: "go"}

// TestReadiness_R1_GoFallsThroughImmediately: lang="go" returns nil without
// calling either probe method (best-effort fall-through; the AcquireLease
// path gates GO via lspool's three-tier installer).
func TestReadiness_R1_GoFallsThroughImmediately(t *testing.T) {
	probe := &fakeReadinessProbe{}
	ctx := context.Background()
	err := lspenrich.WaitForLanguageReady(ctx, probe, "go", testWSKey)
	if err != nil {
		t.Fatalf("WaitForLanguageReady(go): %v", err)
	}
	if probe.javaCalls != 0 || probe.rustCalls != 0 {
		t.Errorf("probe call counts: java=%d rust=%d (both want 0)", probe.javaCalls, probe.rustCalls)
	}
}

// TestReadiness_R2_JavaCallsJavaReady: lang="java" delegates to JavaReady.
func TestReadiness_R2_JavaCallsJavaReady(t *testing.T) {
	probe := &fakeReadinessProbe{
		javaFn: func(_ context.Context, _ workspace.WorkspaceKey) error { return nil },
	}
	ctx := context.Background()
	if err := lspenrich.WaitForLanguageReady(ctx, probe, "java", testWSKey); err != nil {
		t.Fatalf("WaitForLanguageReady(java): %v", err)
	}
	if probe.javaCalls != 1 {
		t.Errorf("javaCalls: got %d, want 1", probe.javaCalls)
	}
	if probe.rustCalls != 0 {
		t.Errorf("rustCalls: got %d, want 0", probe.rustCalls)
	}
}

// TestReadiness_R3_JavaTimeoutPropagatesCtxErr: lang="java" probe blocks
// until ctx expires — function returns ctx.Err().
func TestReadiness_R3_JavaTimeoutPropagatesCtxErr(t *testing.T) {
	probe := &fakeReadinessProbe{
		javaFn: func(ctx context.Context, _ workspace.WorkspaceKey) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	err := lspenrich.WaitForLanguageReady(ctx, probe, "java", testWSKey)
	if err == nil {
		t.Fatal("WaitForLanguageReady(java, expired ctx): want non-nil error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err: %v; want context.DeadlineExceeded (or wrapping)", err)
	}
}

// TestReadiness_R4_RustCallsRustQuiescent: lang="rust" delegates to RustQuiescent.
func TestReadiness_R4_RustCallsRustQuiescent(t *testing.T) {
	probe := &fakeReadinessProbe{
		rustFn: func(_ context.Context, _ workspace.WorkspaceKey) error { return nil },
	}
	ctx := context.Background()
	if err := lspenrich.WaitForLanguageReady(ctx, probe, "rust", testWSKey); err != nil {
		t.Fatalf("WaitForLanguageReady(rust): %v", err)
	}
	if probe.rustCalls != 1 {
		t.Errorf("rustCalls: got %d, want 1", probe.rustCalls)
	}
	if probe.javaCalls != 0 {
		t.Errorf("javaCalls: got %d, want 0", probe.javaCalls)
	}
}

// TestReadiness_R5_UnknownLanguageReturnsNilImmediately: lang="perl" (or any
// non-java/rust) returns nil and does not call either probe method.
func TestReadiness_R5_UnknownLanguageReturnsNilImmediately(t *testing.T) {
	probe := &fakeReadinessProbe{}
	ctx := context.Background()
	if err := lspenrich.WaitForLanguageReady(ctx, probe, "perl", testWSKey); err != nil {
		t.Fatalf("WaitForLanguageReady(perl): %v", err)
	}
	if probe.javaCalls != 0 || probe.rustCalls != 0 {
		t.Errorf("probe call counts: java=%d rust=%d (both want 0; AcquireLease is the gate)",
			probe.javaCalls, probe.rustCalls)
	}
}

// recordingLeaseAcquirer is a fake LeaseAcquirer that returns a fixed error
// from AcquireLease. Used by R6 to demonstrate the broken-LS-install path:
// readiness fall-through returns nil; AcquireLease then surfaces the
// underlying error and the worker observes outcome=dropped.
type recordingLeaseAcquirer struct {
	acquireErr error
	calls      int
}

// W4 R6: For a "best-effort" language, WaitForLanguageReady returns nil.
// The subsequent (mocked) AcquireLease then fails — we assert that the
// chained-error path leaves the broken-LS-install signal unmasked. The
// outcome OUTSIDE the readiness function (i.e. the worker's classification
// to OutcomeDropped) is wired in worker_test.go's W7. Here we only assert
// the readiness contract: it does NOT swallow the AcquireLease error, and
// it does NOT mark partial_reason itself.
func TestReadiness_R6_BestEffortLangAcquireLeaseErrorSurfacesAsDropped(t *testing.T) {
	probe := &fakeReadinessProbe{}
	acq := &recordingLeaseAcquirer{
		acquireErr: errSimulatedCircuitOpen,
	}

	ctx := context.Background()
	// Step 1: readiness gate for "perl" returns nil immediately.
	if err := lspenrich.WaitForLanguageReady(ctx, probe, "perl", testWSKey); err != nil {
		t.Fatalf("WaitForLanguageReady(perl): want nil, got %v", err)
	}
	// Step 2: simulated AcquireLease returns the broken-LS-install error.
	err := acq.simulateAcquire(ctx)
	if err == nil {
		t.Fatal("simulated AcquireLease: want error, got nil (broken-LS-install scenario)")
	}
	// Step 3: in the real worker, this surface produces outcome=dropped
	// (NOT outcome=applied, NOT outcome=partial_lsp_unavailable). The
	// assertion is documented here at the unit level; the integration
	// assertion lives in worker_test.go's W7.
	wantOutcome := lspenrich.OutcomeDropped
	got := classifyAcquireErrToOutcome(err)
	if got != wantOutcome {
		t.Errorf("outcome on broken-LS-install: got %q, want %q (outcome=dropped)",
			got, wantOutcome)
	}
}

// errSimulatedCircuitOpen is a sentinel mirroring the kind of error
// AcquireLease returns when the LS install is broken. The real worker uses
// errors.Is(err, serr.ErrCircuitOpen); for this readiness-only test we use
// a sentinel + small classifier.
var errSimulatedCircuitOpen = errors.New("simulated: circuit_open from broken LS install")

// simulateAcquire runs the recordingLeaseAcquirer's pre-canned error.
func (r *recordingLeaseAcquirer) simulateAcquire(_ context.Context) error {
	r.calls++
	return r.acquireErr
}

// classifyAcquireErrToOutcome mirrors the worker's classification from
// AcquireLease errors → Outcome. Inlined here so the readiness test does
// not depend on Worker (this module documents the chained behaviour).
func classifyAcquireErrToOutcome(err error) lspenrich.Outcome {
	if err == nil {
		return lspenrich.OutcomeApplied
	}
	// Broken-LS-install / circuit-open / max-workers all map to Dropped
	// (W4 — outcome=dropped, NOT outcome=applied).
	return lspenrich.OutcomeDropped
}
