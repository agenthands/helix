package lspool

import (
	"math"
	"sync"
	"time"
)

// CircuitBreaker implements exponential backoff for crashy LS workers (per D-06).
// It never fully breaks -- CanAttempt always returns true after enough backoff time.
type CircuitBreaker struct {
	language    string
	failures    int
	lastFailure time.Time
	backoff     time.Duration
	maxBackoff  time.Duration
	state       float64 // CircuitClosed / CircuitHalfOpen / CircuitOpen (D-14)
	sink        MetricsSink
	mu          sync.Mutex
}

// NewCircuitBreaker creates a circuit breaker with the given language label,
// maximum backoff duration, and metrics sink. A nil sink is replaced by
// NoopSink. The initial state is CircuitClosed and is published immediately
// so that the gauge has an entry for the language from birth (D-14).
func NewCircuitBreaker(language string, maxBackoff time.Duration, sink MetricsSink) *CircuitBreaker {
	if sink == nil {
		sink = NoopSink{}
	}
	cb := &CircuitBreaker{
		language:   language,
		maxBackoff: maxBackoff,
		state:      CircuitClosed,
		sink:       sink,
	}
	cb.sink.LSPoolCircuitStateSet(language, CircuitClosed)
	return cb
}

// setStateLocked updates the circuit state gauge if the state changed. Must
// be called with cb.mu held.
func (cb *CircuitBreaker) setStateLocked(next float64) {
	if cb.state == next {
		return
	}
	cb.state = next
	cb.sink.LSPoolCircuitStateSet(cb.language, next)
}

// RecordFailure records a failure and increases the backoff exponentially.
// backoff = min(1s * 2^failures, maxBackoff). Transitions the circuit to
// CircuitOpen.
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
	cb.setStateLocked(CircuitOpen)
}

// RecordSuccess resets the failure count and backoff to zero. Transitions
// the circuit to CircuitClosed.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	cb.backoff = 0
	cb.setStateLocked(CircuitClosed)
}

// CanAttempt returns true if enough time has passed since the last failure.
// Per D-06: never fully breaks -- always allows retry after backoff period.
// A probe attempt transitions the circuit to CircuitHalfOpen.
func (cb *CircuitBreaker) CanAttempt() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.failures == 0 {
		return true
	}
	if time.Since(cb.lastFailure) >= cb.backoff {
		cb.setStateLocked(CircuitHalfOpen)
		return true
	}
	return false
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
