package lspool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"github.com/agenthands/helix/internal/kernel/jsonrpc"
	gen "github.com/agenthands/helix/protocol/gen"
)

// WorkerState represents the lifecycle state of an LS worker.
type WorkerState int32

const (
	// WorkerStarting is the initial state before the process is launched.
	WorkerStarting WorkerState = iota
	// WorkerInitializing is the state during LSP initialize/initialized handshake.
	WorkerInitializing
	// WorkerReady means the worker is fully initialized and can handle requests.
	WorkerReady
	// WorkerShuttingDown means a graceful shutdown is in progress.
	WorkerShuttingDown
	// WorkerStopped means the worker has fully stopped.
	WorkerStopped
)

// String returns the string representation of the worker state.
func (s WorkerState) String() string {
	switch s {
	case WorkerStarting:
		return "starting"
	case WorkerInitializing:
		return "initializing"
	case WorkerReady:
		return "ready"
	case WorkerShuttingDown:
		return "shutting_down"
	case WorkerStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// ErrWorkerNotReady is returned when a request is made to a non-Ready worker.
var ErrWorkerNotReady = errors.New("worker is not in Ready state")

// ErrNotSupported is returned when the LS does not support a requested capability.
var ErrNotSupported = errors.New("capability not supported by language server")

// pendingDidOpen holds a buffered didOpen notification for replay after initialization.
type pendingDidOpen struct {
	method string
	params interface{}
}

// WorkerMetrics tracks usage statistics for adaptive TTL computation.
type WorkerMetrics struct {
	mu          sync.Mutex
	StartedAt   time.Time
	LastUsedAt  time.Time
	UseCount    int64
	ReuseTimes  []time.Time // last N reuse timestamps for gap calculation
	ReuseScore  float64     // decaying score per D-03
	ColdStartMs int64       // time from Start to Ready in milliseconds
}

const maxReuseTimes = 20

// OnReuse records a reuse event and updates the decaying score.
func (m *WorkerMetrics) OnReuse() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	m.UseCount++
	elapsed := now.Sub(m.LastUsedAt).Seconds()
	// Decay: score = decay(score, elapsed) + 1
	// Using exponential decay: score * exp(-elapsed/300)
	m.ReuseScore = m.ReuseScore*math.Exp(-elapsed/300.0) + 1.0
	m.LastUsedAt = now
	m.ReuseTimes = append(m.ReuseTimes, now)
	if len(m.ReuseTimes) > maxReuseTimes {
		m.ReuseTimes = m.ReuseTimes[len(m.ReuseTimes)-maxReuseTimes:]
	}
}

// TTL computes the adaptive TTL based on decaying reuse score (per D-03).
// Returns the TTL in seconds.
func (m *WorkerMetrics) TTL(baseTTL, ceilingTTL int) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	if baseTTL == 0 {
		return 0 // TTL=0 means no idle timeout (D-07)
	}

	// ttl = clamp(base_ttl * f(score), base_ttl, ceiling)
	// f(score) = 1 + log2(1 + score)
	factor := 1.0 + math.Log2(1.0+m.ReuseScore)
	ttl := float64(baseTTL) * factor
	if ttl < float64(baseTTL) {
		ttl = float64(baseTTL)
	}
	if ttl > float64(ceilingTTL) {
		ttl = float64(ceilingTTL)
	}
	return int(ttl)
}

// IdleDuration returns the duration since last use.
func (m *WorkerMetrics) IdleDuration() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	return time.Since(m.LastUsedAt)
}

// Worker represents a single LS worker with a 5-state machine.
type Worker struct {
	id           string
	language     string
	workDir      string
	lsCommand    string
	lsArgs       []string
	quirks       QuirkAdapter
	process      *ProcessHandle
	state        atomic.Int32
	metrics      WorkerMetrics
	mu           sync.RWMutex
	pendingOpen  []pendingDidOpen // buffered during Initializing (per Pitfall 2)
	capabilities gen.ServerCapabilities
	logger       *slog.Logger
	// tracer is forwarded to ProcessHandle and ultimately jsonrpc.Conn so every
	// LS Call/Notify emits a child span. Phase 55 D-01: nil → noop fallback.
	tracer trace.Tracer
}

// NewWorker creates a new LS worker.
//
// tracer is propagated to the underlying jsonrpc.Conn so each outbound LS
// JSON-RPC frame produces a lspool.lsp.{method} child span. A nil tracer
// falls back to a noop tracer (Phase 55 D-01).
func NewWorker(id, language, workDir string, lsCommand string, lsArgs []string, logger *slog.Logger, tracer trace.Tracer) *Worker {
	if tracer == nil {
		tracer = tracenoop.NewTracerProvider().Tracer("worker-noop")
	}
	w := &Worker{
		id:        id,
		language:  language,
		workDir:   workDir,
		lsCommand: lsCommand,
		lsArgs:    lsArgs,
		logger:    logger.With("worker_id", id, "language", language),
		tracer:    tracer,
	}
	w.state.Store(int32(WorkerStarting))
	return w
}

// Start launches the LS process and performs the LSP initialize handshake.
func (w *Worker) Start(ctx context.Context) error {
	startTime := time.Now()

	// Set state to Starting.
	w.state.Store(int32(WorkerStarting))

	// Apply language quirks via QuirkAdapter.
	command := w.lsCommand
	args := w.lsArgs
	var initOptions interface{}
	if w.quirks != nil {
		initOptions = w.quirks.InitOptions(w.workDir)
		// Allow quirks to inject extra command-line arguments (e.g., jdtls -data).
		if am, ok := w.quirks.(ArgsModifier); ok {
			args = am.ExtraArgs(w.workDir, args)
		}
	}

	// Start the process.
	w.process = NewProcessHandle(command, args, w.workDir, nil, w.logger, w.tracer)
	if err := w.process.Start(ctx, w.id); err != nil {
		w.state.Store(int32(WorkerStopped))
		return fmt.Errorf("starting LS process: %w", err)
	}

	// Phase 56 D-01 + D-02: wire dispatcher BEFORE Listen, BEFORE first LSP
	// call. Notifications received before OnNotification is assigned would be
	// silently dropped — that is the bug Phase 56 fixes.
	var handlers map[string]func(json.RawMessage)
	if w.quirks != nil {
		handlers = w.quirks.NotificationHandlers()
	}
	if len(handlers) > 0 {
		w.process.Conn().OnNotification = buildDispatcher(handlers, w.logger, w.id)
	}

	// Phase 56 D-11: regression assertion — fail loud if a future refactor
	// drops the wiring while quirks still declare handlers.
	if err := assertDispatcherWired(handlers, w.process.Conn(), w.logger, w.id); err != nil {
		_ = w.process.Stop(ctx)
		w.state.Store(int32(WorkerStopped))
		return err
	}

	// Phase 56 D-02: now safe to start the dispatch loop.
	w.process.StartListen(ctx)

	// Set state to Initializing.
	w.state.Store(int32(WorkerInitializing))

	// Build InitializeParams.
	rootURI := "file://" + w.workDir
	initParams := gen.InitializeParams{}
	initParams.RootUri = rootURI
	initParams.ProcessId = int32(processID())
	trueVal := true
	initParams.Capabilities = gen.ClientCapabilities{
		TextDocument: &gen.TextDocumentClientCapabilities{
			DocumentSymbol: &gen.DocumentSymbolClientCapabilities{
				HierarchicalDocumentSymbolSupport: &trueVal,
			},
		},
	}
	// Allow quirks to advertise LS-specific experimental client capabilities
	// (e.g., rust-analyzer's serverStatusNotification). Merged into
	// ClientCapabilities.Experimental via optional-interface type assertion —
	// see quirks.go ExperimentalCapabilities.
	if w.quirks != nil {
		if ec, ok := w.quirks.(ExperimentalCapabilities); ok {
			exp := map[string]any{}
			for k, v := range ec.ExperimentalCapabilities() {
				exp[k] = v
			}
			initParams.Capabilities.Experimental = exp
		}
	}
	initParams.InitializationOptions = initOptions
	initParams.WorkspaceFolders = []gen.WorkspaceFolder{
		{URI: rootURI, Name: w.workDir},
	}

	// Send initialize request.
	var initResult gen.InitializeResult
	if err := w.process.Conn().Call(ctx, "initialize", initParams, &initResult); err != nil {
		_ = w.process.Stop(ctx)
		w.state.Store(int32(WorkerStopped))
		return fmt.Errorf("LSP initialize: %w", err)
	}

	// Store server capabilities.
	w.mu.Lock()
	w.capabilities = initResult.Capabilities
	w.mu.Unlock()

	// Send initialized notification.
	if err := w.process.Conn().Notify(ctx, "initialized", gen.InitializedParams{}); err != nil {
		w.logger.Warn("initialized notification failed", "error", err)
	}

	// Call PostInitialize hook if quirks adapter is set.
	if w.quirks != nil {
		adapter := NewLSAdapter(w)
		if err := w.quirks.PostInitialize(ctx, adapter); err != nil {
			w.logger.Warn("PostInitialize hook failed", "error", err)
		}
	}

	// Replay buffered didOpen notifications.
	w.mu.Lock()
	pending := w.pendingOpen
	w.pendingOpen = nil
	w.mu.Unlock()

	for _, p := range pending {
		if err := w.process.Conn().Notify(ctx, p.method, p.params); err != nil {
			w.logger.Warn("replaying buffered didOpen failed", "method", p.method, "error", err)
		}
	}

	// Set state to Ready.
	w.state.Store(int32(WorkerReady))

	coldStart := time.Since(startTime).Milliseconds()
	w.metrics.mu.Lock()
	w.metrics.StartedAt = startTime
	w.metrics.LastUsedAt = time.Now()
	w.metrics.ColdStartMs = coldStart
	w.metrics.mu.Unlock()

	w.logger.Info("LS worker ready", "cold_start_ms", coldStart)
	return nil
}

// Request sends a request to the LS, gated on Ready state.
// When the calling context carries a recording span (i.e. tracing is sampled),
// an "ls.request" span event is emitted with lsp.method, lsp.language, and
// lsp.duration_ms attributes. Gated on span.IsRecording() so the tracing-off
// path allocates nothing (D-17 budget). This is a span EVENT, not a child
// span, per D-06.
func (w *Worker) Request(ctx context.Context, method string, params interface{}, result interface{}) error {
	if WorkerState(w.state.Load()) != WorkerReady {
		return ErrWorkerNotReady
	}
	start := time.Now()
	err := w.process.Conn().Call(ctx, method, params, result)
	duration := time.Since(start)
	if err == nil {
		w.metrics.OnReuse()
	}
	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		span.AddEvent("ls.request", trace.WithAttributes(
			attribute.String("lsp.method", method),
			attribute.String("lsp.language", w.language),
			attribute.Int64("lsp.duration_ms", duration.Milliseconds()),
		))
	}
	return err
}

// Notify sends a notification to the LS.
// Per Pitfall 2: if Initializing and method is textDocument/didOpen, buffer it.
func (w *Worker) Notify(ctx context.Context, method string, params interface{}) error {
	state := WorkerState(w.state.Load())

	if state == WorkerInitializing && method == "textDocument/didOpen" {
		w.mu.Lock()
		w.pendingOpen = append(w.pendingOpen, pendingDidOpen{method: method, params: params})
		w.mu.Unlock()
		return nil
	}

	if state != WorkerReady && state != WorkerInitializing {
		return ErrWorkerNotReady
	}

	return w.process.Conn().Notify(ctx, method, params)
}

// Stop performs graceful shutdown of the LS worker.
func (w *Worker) Stop(ctx context.Context) error {
	w.state.Store(int32(WorkerShuttingDown))
	w.logger.Info("stopping LS worker")

	var err error
	if w.process != nil {
		err = w.process.Stop(ctx)
	}

	w.state.Store(int32(WorkerStopped))
	return err
}

// State returns the current worker state.
func (w *Worker) State() WorkerState {
	return WorkerState(w.state.Load())
}

// Metrics returns a pointer to the worker metrics.
func (w *Worker) Metrics() *WorkerMetrics {
	return &w.metrics
}

// Language returns the language this worker serves.
func (w *Worker) Language() string {
	return w.language
}

// WorkDir returns the working directory this worker is rooted at.
func (w *Worker) WorkDir() string {
	return w.workDir
}

// ID returns the worker ID.
func (w *Worker) ID() string {
	return w.id
}

// Capabilities returns the server capabilities reported during initialization.
func (w *Worker) Capabilities() gen.ServerCapabilities {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.capabilities
}

// Conn returns the underlying JSON-RPC connection (for adapter use).
func (w *Worker) Conn() *jsonrpc.Conn {
	if w.process != nil {
		return w.process.Conn()
	}
	return nil
}

// Command returns the LS command string.
func (w *Worker) Command() string {
	return w.lsCommand
}

// Pid returns the LS process ID, or -1 if not started.
func (w *Worker) Pid() int {
	if w.process != nil {
		return w.process.Pid()
	}
	return -1
}

// SetQuirks sets the QuirkAdapter for this worker. Must be called before Start.
func (w *Worker) SetQuirks(q QuirkAdapter) {
	w.quirks = q
}

// Quirks returns the QuirkAdapter configured on this worker, or nil if none.
// Exposed for optional-interface type assertions by callers that need to
// detect per-LS capabilities (e.g., edit.RenameSymbol dispatching on
// *RustAnalyzerAdapter for the rename readiness gate and override path).
func (w *Worker) Quirks() QuirkAdapter {
	return w.quirks
}

// NormalizeSymbolName delegates symbol name normalization to the QuirkAdapter.
func (w *Worker) NormalizeSymbolName(name string) string {
	if w.quirks != nil {
		return w.quirks.NormalizeSymbolName(name)
	}
	return name
}

// processID returns the current process ID for the LSP initialize handshake.
func processID() int {
	return pid()
}

// buildDispatcher returns a NotificationFunc that routes incoming LS
// notifications to the per-method handlers registered by the active
// QuirkAdapter. Unknown methods are debug-logged and dropped (Phase 56 D-04).
// Handler panics are recovered, logged at Error level, and never crash the
// Listen goroutine (Phase 56 D-05). The handler map is defensively copied so
// subsequent mutations of the QuirkAdapter's map cannot affect routing
// (T-56-06).
func buildDispatcher(
	handlers map[string]func(json.RawMessage),
	logger *slog.Logger,
	workerID string,
) jsonrpc.NotificationFunc {
	routes := make(map[string]func(json.RawMessage), len(handlers))
	for k, v := range handlers {
		routes[k] = v
	}
	return func(method string, params json.RawMessage) {
		h, ok := routes[method]
		if !ok {
			logger.Debug("unhandled LS notification", "method", method, "worker_id", workerID)
			return
		}
		defer func() {
			if r := recover(); r != nil {
				logger.Error("LS notification handler panicked",
					"method", method, "worker_id", workerID, "panic", fmt.Sprint(r))
			}
		}()
		h(params)
	}
}

// assertDispatcherWired implements the Phase 56 D-11 regression check.
// Returns a non-nil error if quirks declared handlers but OnNotification was
// not actually assigned. Logs at Error level so the failure is loud in tests
// and production. Empty handlers always pass — there is nothing to wire.
func assertDispatcherWired(
	handlers map[string]func(json.RawMessage),
	conn *jsonrpc.Conn,
	logger *slog.Logger,
	workerID string,
) error {
	if len(handlers) == 0 {
		return nil
	}
	if conn.OnNotification == nil {
		logger.Error("dispatcher wiring lost between assignment and check",
			"worker_id", workerID, "handler_count", len(handlers))
		return fmt.Errorf("dispatcher wiring failed for worker %s", workerID)
	}
	return nil
}
