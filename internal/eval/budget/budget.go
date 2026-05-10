// Package budget declares the cross-cutting BreachReason type used by
// the watchdog (Plan 02) and the trace merger (Plan 03). Behavior
// (watchdog enforcement, YAML loading) lands in Plan 02's budget.go.
package budget

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"gopkg.in/yaml.v3"
)

// Budget holds the D-08 four-axis enforcement caps for a single task run.
// Zero values in a loaded budget are replaced by the corresponding Default value
// during YAML loading.
type Budget struct {
	MaxInputTokens  int `yaml:"max_input_tokens"`
	MaxOutputTokens int `yaml:"max_output_tokens"`
	MaxSeconds      int `yaml:"max_seconds"`
	MaxToolCalls    int `yaml:"max_tool_calls"`
}

// Default is the D-08 default budget applied to every task unless overridden
// by a per-task budget.yaml.
var Default = Budget{
	MaxInputTokens:  200000,
	MaxOutputTokens: 32000,
	MaxSeconds:      300,
	MaxToolCalls:    100,
}

// LoadFromFile reads a per-task budget override from a YAML file and merges it
// over the Default values. Missing keys keep their default values. Unknown keys
// cause an error (KnownFields strict mode).
func LoadFromFile(path string) (Budget, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Budget{}, fmt.Errorf("budget: read %q: %w", path, err)
	}

	// Empty file → all defaults.
	if len(bytes.TrimSpace(data)) == 0 {
		return Default, nil
	}

	// Partial-override struct: only set fields override Default.
	type override struct {
		MaxInputTokens  *int `yaml:"max_input_tokens"`
		MaxOutputTokens *int `yaml:"max_output_tokens"`
		MaxSeconds      *int `yaml:"max_seconds"`
		MaxToolCalls    *int `yaml:"max_tool_calls"`
	}

	var ov override
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&ov); err != nil {
		return Budget{}, fmt.Errorf("budget: parse %q: %w", path, err)
	}

	b := Default
	if ov.MaxInputTokens != nil {
		b.MaxInputTokens = *ov.MaxInputTokens
	}
	if ov.MaxOutputTokens != nil {
		b.MaxOutputTokens = *ov.MaxOutputTokens
	}
	if ov.MaxSeconds != nil {
		b.MaxSeconds = *ov.MaxSeconds
	}
	if ov.MaxToolCalls != nil {
		b.MaxToolCalls = *ov.MaxToolCalls
	}
	return b, nil
}

// Watchdog enforces the D-08 four-axis budget caps for a running task. It wraps
// a context that is cancelled when any axis is breached. The cancelled context
// propagates to the claude subprocess and trace collector.
//
// Usage:
//
//	wdog, cancel := NewWatchdog(ctx, budget)
//	defer cancel()
//	// ... task runs ...
//	if br := wdog.Breach(); br != nil { /* handle breach */ }
type Watchdog struct {
	budget Budget
	start  time.Time

	// innerCancel cancels the watchdog context on any breach.
	innerCancel context.CancelFunc
	innerCtx    context.Context

	mu     sync.Mutex
	breach *BreachReason

	inputTokens  atomic.Int64
	outputTokens atomic.Int64
	toolCalls    atomic.Int64
}

// NewWatchdog creates a Watchdog that enforces b against the given context.
// The returned CancelFunc must be called to release resources even when no
// breach occurs (defer cancel()).
//
// The seconds axis is enforced by a background goroutine. Token and tool-call
// axes are enforced synchronously via RecordTokens / RecordToolCall.
func NewWatchdog(ctx context.Context, b Budget) (*Watchdog, context.CancelFunc) {
	// We use a plain cancel context (not timeout) so we can set the breach
	// reason before cancellation on the seconds axis.
	innerCtx, innerCancel := context.WithCancel(ctx)

	w := &Watchdog{
		budget:      b,
		start:       time.Now(),
		innerCtx:    innerCtx,
		innerCancel: innerCancel,
	}

	// Goroutine enforces the seconds axis. It sleeps for MaxSeconds then
	// records the breach and cancels. A select on ctx.Done() lets it exit
	// early if the parent context is cancelled first.
	go func() {
		timer := time.NewTimer(time.Duration(b.MaxSeconds) * time.Second)
		defer timer.Stop()
		select {
		case <-timer.C:
			elapsed := time.Since(w.start)
			w.setBreach(&BreachReason{
				Axis:     "seconds",
				Limit:    int64(b.MaxSeconds),
				Observed: int64(elapsed.Seconds()),
			})
			innerCancel()
		case <-innerCtx.Done():
			// Parent or another axis cancelled first — nothing to do.
		}
	}()

	return w, innerCancel
}

// setBreach sets the breach if not already set (first-breach wins).
func (w *Watchdog) setBreach(br *BreachReason) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.breach == nil {
		w.breach = br
	}
}

// RecordToolCall increments the tool-call counter. If MaxToolCalls is exceeded,
// the watchdog context is cancelled and the breach is recorded.
func (w *Watchdog) RecordToolCall() {
	n := w.toolCalls.Add(1)
	if int(n) > w.budget.MaxToolCalls {
		w.setBreach(&BreachReason{
			Axis:     "tool_calls",
			Limit:    int64(w.budget.MaxToolCalls),
			Observed: n,
		})
		w.innerCancel()
	}
}

// RecordTokens adds input and output token counts. If either MaxInputTokens or
// MaxOutputTokens is exceeded, the watchdog context is cancelled and the breach
// is recorded. Input axis is checked before output.
func (w *Watchdog) RecordTokens(input, output int) {
	newInput := w.inputTokens.Add(int64(input))
	newOutput := w.outputTokens.Add(int64(output))

	if int(newInput) > w.budget.MaxInputTokens {
		w.setBreach(&BreachReason{
			Axis:     "input_tokens",
			Limit:    int64(w.budget.MaxInputTokens),
			Observed: newInput,
		})
		w.innerCancel()
		return
	}

	if int(newOutput) > w.budget.MaxOutputTokens {
		w.setBreach(&BreachReason{
			Axis:     "output_tokens",
			Limit:    int64(w.budget.MaxOutputTokens),
			Observed: newOutput,
		})
		w.innerCancel()
	}
}

// Breach returns the BreachReason that triggered context cancellation, or nil
// if no breach has occurred. The return value is nil-safe.
func (w *Watchdog) Breach() *BreachReason {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.breach
}

// Done returns the watchdog's context Done channel. Closed when any axis
// breaches or when the parent context is cancelled.
func (w *Watchdog) Done() <-chan struct{} {
	return w.innerCtx.Done()
}
