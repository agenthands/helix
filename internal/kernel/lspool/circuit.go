package lspool

import (
	"math"
	"sync"
	"time"
)

// CircuitBreaker implements exponential backoff for crashy LS workers (per D-06).
// It never fully breaks -- CanAttempt always returns true after enough backoff time.
type CircuitBreaker struct {
	failures    int
	lastFailure time.Time
	backoff     time.Duration
	maxBackoff  time.Duration
	mu          sync.Mutex
}

// NewCircuitBreaker creates a circuit breaker with the given maximum backoff duration.
func NewCircuitBreaker(maxBackoff time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxBackoff: maxBackoff,
	}
}

// RecordFailure records a failure and increases the backoff exponentially.
// backoff = min(1s * 2^failures, maxBackoff)
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()
	backoff := time.Duration(math.Pow(2, float64(cb.failures-1))) * time.Second
	if backoff > cb.maxBackoff {
		backoff = cb.maxBackoff
	}
	cb.backoff = backoff
}

// RecordSuccess resets the failure count and backoff to zero.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	cb.backoff = 0
}

// CanAttempt returns true if enough time has passed since the last failure.
// Per D-06: never fully breaks -- always allows retry after backoff period.
func (cb *CircuitBreaker) CanAttempt() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.failures == 0 {
		return true
	}
	return time.Since(cb.lastFailure) >= cb.backoff
}

// BackoffDuration returns the current backoff duration.
func (cb *CircuitBreaker) BackoffDuration() time.Duration {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.backoff
}

// Failures returns the current failure count.
func (cb *CircuitBreaker) Failures() int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.failures
}
