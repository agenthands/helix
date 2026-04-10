package lspool

import (
	"fmt"
	"time"
)

// CircuitOpenError is the typed replacement for wrapping the ErrCircuitOpen
// sentinel. It carries metadata per D-07: Language, BackoffRemaining,
// Failures, RetryAfter.
type CircuitOpenError struct {
	Language         string
	BackoffRemaining time.Duration
	Failures         int
	RetryAfter       time.Time
}

func (e *CircuitOpenError) Error() string {
	return fmt.Sprintf("circuit breaker open for %s: %d failures, retry after %s",
		e.Language, e.Failures, e.RetryAfter.Format(time.RFC3339))
}

// Is allows errors.Is(err, ErrCircuitOpen) to keep working (Pitfall 3 from
// RESEARCH). The sentinel is preserved in pool.go for backward compatibility.
func (e *CircuitOpenError) Is(target error) bool {
	return target == ErrCircuitOpen
}
