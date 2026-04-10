package lspool

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestCircuitOpenError_Fields(t *testing.T) {
	now := time.Now()
	e := &CircuitOpenError{
		Language:         "go",
		Failures:         3,
		BackoffRemaining: 5 * time.Second,
		RetryAfter:       now.Add(5 * time.Second),
	}
	msg := e.Error()
	if !containsStr(msg, "go") {
		t.Errorf("error message should contain language, got: %s", msg)
	}
	if !containsStr(msg, "3 failures") {
		t.Errorf("error message should contain failure count, got: %s", msg)
	}
}

func TestCircuitOpenError_Is(t *testing.T) {
	e := &CircuitOpenError{Language: "go", Failures: 1}
	if !errors.Is(e, ErrCircuitOpen) {
		t.Error("errors.Is(&CircuitOpenError{}, ErrCircuitOpen) should return true")
	}
}

func TestCircuitOpenError_Unwrap(t *testing.T) {
	inner := &CircuitOpenError{Language: "rust", Failures: 2}
	wrapped := fmt.Errorf("%w: extra context", inner)
	if !errors.Is(wrapped, ErrCircuitOpen) {
		t.Error("errors.Is on wrapped CircuitOpenError should still find ErrCircuitOpen")
	}
	if !containsStr(wrapped.Error(), "extra context") {
		t.Error("wrapped error should contain extra context")
	}
}

func TestDecorrelatedJitter(t *testing.T) {
	// After RecordFailure, backoff is in range [base, prevSleep*3] capped at maxBackoff.
	maxBackoff := 30 * time.Second
	base := time.Second

	for i := 0; i < 100; i++ {
		cb := NewCircuitBreaker("go", maxBackoff, 10, NoopSink{})
		cb.RecordFailure()
		b := cb.BackoffDuration()
		if b < base {
			t.Errorf("iteration %d: backoff %v < base %v", i, b, base)
		}
		if b > maxBackoff {
			t.Errorf("iteration %d: backoff %v > maxBackoff %v", i, b, maxBackoff)
		}
	}

	// After multiple failures, backoff should vary (jitter).
	cb := NewCircuitBreaker("go", maxBackoff, 10, NoopSink{})
	seen := make(map[time.Duration]bool)
	for i := 0; i < 50; i++ {
		cb2 := NewCircuitBreaker("go", maxBackoff, 10, NoopSink{})
		cb2.RecordFailure()
		cb2.RecordFailure()
		seen[cb2.BackoffDuration()] = true
	}
	_ = cb
	if len(seen) < 2 {
		t.Errorf("expected jittered backoff values, got only %d unique values", len(seen))
	}
}

func TestRestartBudget(t *testing.T) {
	// After 3 consecutive RecordFailure calls with restartBudget=3,
	// CanAttempt returns false even after backoff expires.
	cb := NewCircuitBreaker("go", 10*time.Millisecond, 3, NoopSink{})
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()

	// Wait for backoff to expire.
	time.Sleep(50 * time.Millisecond)

	// Should still be blocked due to restart budget exhaustion.
	if cb.CanAttempt() {
		t.Error("CanAttempt should return false after restart budget exhausted")
	}

	// RecordSuccess resets.
	cb.RecordSuccess()
	if !cb.CanAttempt() {
		t.Error("CanAttempt should return true after RecordSuccess")
	}
}

func TestSingleProbeHalfOpen(t *testing.T) {
	// After backoff expires, first CanAttempt returns true (probe admitted),
	// second concurrent CanAttempt returns false.
	cb := NewCircuitBreaker("go", 10*time.Millisecond, 10, NoopSink{})
	cb.RecordFailure()

	// Wait for backoff to expire.
	time.Sleep(50 * time.Millisecond)

	// First probe admitted.
	if !cb.CanAttempt() {
		t.Error("first CanAttempt after backoff should succeed (probe)")
	}
	// Second probe rejected.
	if cb.CanAttempt() {
		t.Error("second CanAttempt should be rejected (single-probe)")
	}

	// RecordSuccess resets probe flag.
	cb.RecordSuccess()
	if !cb.CanAttempt() {
		t.Error("CanAttempt should succeed after RecordSuccess")
	}
}

func TestProbeResetOnFailure(t *testing.T) {
	// Probe admitted, then RecordFailure -- probe flag is reset,
	// next backoff expiry admits a new probe.
	cb := NewCircuitBreaker("go", 10*time.Millisecond, 10, NoopSink{})
	cb.RecordFailure()

	time.Sleep(50 * time.Millisecond)

	// Probe admitted.
	if !cb.CanAttempt() {
		t.Error("first CanAttempt should succeed")
	}

	// Probe fails.
	cb.RecordFailure()

	// Wait for new backoff.
	time.Sleep(50 * time.Millisecond)

	// New probe should be admitted.
	if !cb.CanAttempt() {
		t.Error("CanAttempt should succeed after failure reset and backoff expiry")
	}
}

func containsStr(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && contains(s, substr)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
