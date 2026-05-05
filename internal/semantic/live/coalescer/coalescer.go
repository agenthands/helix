package coalescer

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/agenthands/helix/internal/semantic/live"
	"github.com/agenthands/helix/internal/workspace"
)

// EventHandler is the consumer side of a coalescer flush: receives one
// coalesced event per call.  Returning an error logs the failure but does
// NOT abort the batch (D-02 invariant).
type EventHandler interface {
	Dispatch(ctx context.Context, ev live.SourceChangeEvent) error
}

// Logger is the minimal slog-shaped surface the Coalescer depends on.
type Logger interface {
	Warn(msg string, args ...any)
}

// MetricsSink is the minimal counter surface the Coalescer reports
// pipeline outcomes to. Production wiring is *obs.Metrics
// (helix_semantic_live_updates_total{kind, outcome}); tests pass a noop
// or a recording stub.  Per the brief's invariant #4 the {dropped,
// applied, error, no_op} outcomes MUST be emitted at the coalescer
// boundary so operators can observe pipeline health from a single
// counter family.
type MetricsSink interface {
	SemanticLiveUpdatesInc(kind, outcome string)
}

// Config controls the per-workspace Coalescer behavior.  Zero values are
// replaced by sensible defaults in New (see code for specifics).
type Config struct {
	// Debounce is the quiet-period before a flush fires.  Each new event
	// resets the debounce timer.
	Debounce time.Duration
	// MaxBatchDelay is the hard ceiling on debounce reset; once
	// MaxBatchDelay elapses since the FIRST pending event the flush fires
	// regardless of subsequent events.  Without this, a continuous stream
	// of edits would never flush.
	MaxBatchDelay time.Duration
	// BulkChangeThreshold collapses merged sets > N into a single
	// ChangeBulkUpdate event.
	BulkChangeThreshold int
	// QueueSize is the per-workspace input channel buffer.  Producers
	// drop (with the dropped counter incrementing) when the buffer is
	// full (D-02 non-blocking enqueue invariant).
	QueueSize int
	// Metrics is the counter sink for outcome emission. Nil-safe — a
	// nil Metrics defaults to a no-op (production daemon wires
	// *obs.Metrics; unit tests typically leave it nil unless they want
	// to assert label cardinality).
	Metrics MetricsSink
}

// DefaultConfig returns a Config suitable for production wiring.  60-05B
// reads the helix config and overrides as needed; tests call New
// directly with a tighter Config for fast wall-time assertions.
func DefaultConfig() Config {
	return Config{
		Debounce:            250 * time.Millisecond,
		MaxBatchDelay:       1500 * time.Millisecond,
		BulkChangeThreshold: 200,
		QueueSize:           1024,
	}
}

// Coalescer is one goroutine per workspace.  Its Run loop reads events
// from the input channel, accumulates them in a pending map, and flushes
// when either the debounce timer or the max-batch-delay timer fires.
//
// The pending map uses last-write-wins at accept time; CoalesceEvents
// re-applies the SPEC §16.2 merge rules at flush time so the producer
// path stays O(1) per event.
type Coalescer struct {
	workspaceID workspace.WorkspaceKey
	cfg         Config
	handler     EventHandler
	logger      Logger
	metrics     MetricsSink
	in          chan live.SourceChangeEvent
	drops       atomic.Uint64

	mu       sync.Mutex
	pending  map[string]live.SourceChangeEvent
	timer    *time.Timer
	maxTimer *time.Timer
}

// New constructs a Coalescer for ws.  Caller MUST call Run in a separate
// goroutine.  Zero-value config fields are populated from DefaultConfig.
func New(ws workspace.WorkspaceKey, cfg Config, handler EventHandler, logger Logger) *Coalescer {
	def := DefaultConfig()
	if cfg.Debounce <= 0 {
		cfg.Debounce = def.Debounce
	}
	if cfg.MaxBatchDelay <= 0 {
		cfg.MaxBatchDelay = def.MaxBatchDelay
	}
	if cfg.BulkChangeThreshold <= 0 {
		cfg.BulkChangeThreshold = def.BulkChangeThreshold
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = def.QueueSize
	}
	if logger == nil {
		logger = noopLogger{}
	}
	metrics := cfg.Metrics
	if metrics == nil {
		metrics = noopMetrics{}
	}
	return &Coalescer{
		workspaceID: ws,
		cfg:         cfg,
		handler:     handler,
		logger:      logger,
		metrics:     metrics,
		in:          make(chan live.SourceChangeEvent, cfg.QueueSize),
		pending:     make(map[string]live.SourceChangeEvent),
	}
}

// Enqueue is non-blocking.  Drops with the dropped counter incrementing
// when the input channel is full (D-02 invariant: producers MUST NOT
// block). Drops emit helix_semantic_live_updates_total{kind, outcome="dropped"}
// (invariant #4) so operators can alert on pipeline saturation.
func (c *Coalescer) Enqueue(ev live.SourceChangeEvent) {
	select {
	case c.in <- ev:
	default:
		c.drops.Add(1)
		c.metrics.SemanticLiveUpdatesInc(string(ev.Kind), "dropped")
		c.logger.Warn("coalescer queue full; dropped event",
			"workspace", c.workspaceID, "path", ev.Path)
	}
}

// Drops returns the cumulative count of events dropped due to a full
// input channel.  Exposed for metrics + tests.
func (c *Coalescer) Drops() uint64 { return c.drops.Load() }

// Run drains the input channel until ctx is cancelled.  Single
// goroutine per workspace; serializes accept + flush.
//
// Callers MUST call Run in a goroutine; it returns ctx.Err() when ctx
// is cancelled.
func (c *Coalescer) Run(ctx context.Context) error {
	flush := c.makeFlush(ctx)
	for {
		select {
		case <-ctx.Done():
			c.mu.Lock()
			if c.timer != nil {
				c.timer.Stop()
			}
			if c.maxTimer != nil {
				c.maxTimer.Stop()
			}
			c.mu.Unlock()
			return ctx.Err()
		case ev := <-c.in:
			c.accept(ev, flush)
		}
	}
}

func (c *Coalescer) accept(ev live.SourceChangeEvent, flush func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := mergeKey(ev)
	prev, ok := c.pending[key]
	if !ok {
		c.pending[key] = ev
	} else {
		// Apply SPEC §16.2 merge rules at accept time so created+deleted
		// drops out of pending immediately rather than being preserved
		// as a spurious deleted event (last-write-wins would lose the
		// drop signal because CoalesceEvents only sees one event in
		// the snapshot).
		merged, keep := MergeChange(prev, ev)
		if keep {
			c.pending[key] = merged
		} else {
			delete(c.pending, key)
		}
	}
	if c.timer != nil {
		c.timer.Stop()
	}
	c.timer = time.AfterFunc(c.cfg.Debounce, flush)
	if c.maxTimer == nil {
		// Start the max-batch ceiling on the FIRST event in this batch.
		// makeFlush clears maxTimer on flush so the next batch starts a
		// fresh ceiling.
		c.maxTimer = time.AfterFunc(c.cfg.MaxBatchDelay, flush)
	}
}

func (c *Coalescer) makeFlush(ctx context.Context) func() {
	return func() {
		c.mu.Lock()
		if len(c.pending) == 0 {
			c.mu.Unlock()
			return
		}
		snapshot := make([]live.SourceChangeEvent, 0, len(c.pending))
		for _, v := range c.pending {
			snapshot = append(snapshot, v)
		}
		c.pending = make(map[string]live.SourceChangeEvent)
		if c.maxTimer != nil {
			c.maxTimer.Stop()
			c.maxTimer = nil
		}
		c.mu.Unlock()

		merged := CoalesceEvents(snapshot, c.cfg.BulkChangeThreshold)
		// D-04 / 60-04 acceptance #8: empty merged set MUST NOT call
		// Dispatch (no overlay_epoch advance for no-op flushes).
		if len(merged) == 0 {
			return
		}
		for _, ev := range merged {
			if err := c.handler.Dispatch(ctx, ev); err != nil {
				c.metrics.SemanticLiveUpdatesInc(string(ev.Kind), "error")
				c.logger.Warn("coalescer: dispatch error",
					"workspace", c.workspaceID, "path", ev.Path, "err", err)
				// continue — D-02 invariant: per-event errors do not abort batch
			} else {
				c.metrics.SemanticLiveUpdatesInc(string(ev.Kind), "applied")
			}
		}
	}
}

type noopLogger struct{}

func (noopLogger) Warn(string, ...any) {}

// noopMetrics is the default MetricsSink when Config.Metrics is nil
// (test paths and unwired daemons).
type noopMetrics struct{}

func (noopMetrics) SemanticLiveUpdatesInc(string, string) {}
