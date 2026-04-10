package lspool

import (
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"
)

// CircuitBreaker implements decorrelated jitter backoff with restart budget
// and single-probe half-open for crashy LS workers (D-04, D-05, D-06).
type CircuitBreaker struct {
	language      string
	failures      int
	lastFailure   time.Time
	backoff       time.Duration
	maxBackoff    time.Duration
	restartBudget int          // max consecutive failures before permanent open (D-05)
	state         float64      // CircuitClosed / CircuitHalfOpen / CircuitOpen (D-14)
	sink          MetricsSink
	probing       atomic.Bool  // single-probe half-open guard (D-06)
	mu            sync.Mutex
}

// NewCircuitBreaker creates a circuit breaker with the given language label,
// maximum backoff duration, restart budget, and metrics sink. A nil sink is
// replaced by NoopSink. restartBudget <= 0 defaults to 3. The initial state
// is CircuitClosed and is published immediately so that the gauge has an
// entry for the language from birth (D-14).
func NewCircuitBreaker(language string, maxBackoff time.Duration, restartBudget int, sink MetricsSink) *CircuitBreaker {
	if sink == nil {
		sink = NoopSink{}
	}
	if restartBudget <= 0 {
		restartBudget = 3
	}
	cb := &CircuitBreaker{
		language:      language,
		maxBackoff:    maxBackoff,
		restartBudget: restartBudget,
		state:         CircuitClosed,
		sink:          sink,
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

// RecordFailure records a failure and computes decorrelated jitter backoff
// per D-04: sleep = min(cap, random_between(base, prevSleep*3)).
// Transitions the circuit to CircuitOpen and resets the probe flag (D-06).
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()
	cb.probing.Store(false) // reset probe flag on failure (D-06)

	// Decorrelated jitter (D-04): sleep = min(cap, random_between(base, prevSleep*3))
	base := time.Second
	prevSleep := cb.backoff
	if prevSleep < base {
		prevSleep = base
	}
	ceiling := prevSleep * 3
	jitterRange := int64(ceiling - base + 1)
	if jitterRange <= 0 {
		jitterRange = 1
	}
	jittered := base + time.Duration(rand.Int64N(jitterRange))
	if jittered > cb.maxBackoff {
		jittered = cb.maxBackoff
	}
	cb.backoff = jittered
	cb.setStateLocked(CircuitOpen)
}

// RecordSuccess resets the failure count, backoff, and probe flag.
// Transitions the circuit to CircuitClosed.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	cb.backoff = 0
	cb.probing.Store(false) // reset probe flag (D-06)
	cb.setStateLocked(CircuitClosed)
}

// CanAttempt returns true if the circuit allows an attempt. Implements:
//   - D-05: restart budget exhausted -> circuit stays open permanently
//   - D-06: single-probe half-open via atomic CAS
func (cb *CircuitBreaker) CanAttempt() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.failures == 0 {
		return true
	}
	// D-05: restart budget exhausted -- circuit stays open permanently
	// until RecordSuccess resets or TTL expiry removes the worker.
	if cb.failures >= cb.restartBudget {
		return false
	}
	if time.Since(cb.lastFailure) >= cb.backoff {
		// D-06: single probe -- only one goroutine gets through.
		if cb.probing.CompareAndSwap(false, true) {
			cb.setStateLocked(CircuitHalfOpen)
			return true
		}
	}
	return false
}

// CircuitOpenErr creates a typed CircuitOpenError from the current circuit
// state. Thread-safe.
func (cb *CircuitBreaker) CircuitOpenErr() *CircuitOpenError {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	remaining := cb.backoff - time.Since(cb.lastFailure)
	if remaining < 0 {
		remaining = 0
	}
	return &CircuitOpenError{
		Language:         cb.language,
		BackoffRemaining: remaining,
		Failures:         cb.failures,
		RetryAfter:       cb.lastFailure.Add(cb.backoff),
	}
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
